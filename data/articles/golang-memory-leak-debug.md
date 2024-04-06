背景：新服务发布后，内存缓慢增长，推测可能出现内存泄露问题

![img](/static/image/blog/memory_growth.png)

## 思考

1. 内存缓存没有释放？
2. goroutine 没有释放？
3. 分配的数据没有回收？

## pprof 工具

Golang 作为一门工程语言，自带一个功能强大且完善的性能分析工具包 `pprof`

其中 runtime/pprof 提取堆内存使用情况，保存到 mem.profile 文件中

```go
func main() {
  f, _ := os.OpenFile("mem.profile", os.O_CREATE|os.O_RDWR, 0644)
  defer f.Close()

  // do something ...

  pprof.Lookup("heap").WriteTo(f, 0)
}
```

使用 pkg/profile 来简化，不传参数默认 CPU Profile，保存文件为 cpu.pprof，内存则为 mem.pprof

```go
func main() {
    defer profile.Start(profile.MemProfile).Stop()

    // do somthing ...
}
```

标准输出提示输出的文件位置

```text
2022/07/01 18:57:50 profile: memory profiling enabled (rate 4096), /var/folders/.../T/profile551181266/mem.pprof
2022/07/01 18:57:50 profile: memory profiling disabled, /var/folders/.../T/profile551181266/mem.pprof
```

`go tool pprof /.../mem.pprof` 启动交互式命令行，默认参数是 --inuse_space，各个参数对应如下

```text
inuse_space      使用中的内存
inuse_objects    使用中的对象
alloc_space      累计分配的内存
alloc_objects    累计生产的对象
```

### pprof top

top 会列出 5 个统计数据：

- flat: 本函数占用的内存量（不含调用的函数）
- flat%: 本函数内存占使用中内存总量的百分比
- sum%: 前面每一行 flat 百分比的和，比如第 2 行的 `77.42% = 57.56% + 19.86%`
- cum: 是累计量，main 函数调用了函数 repeat，函数 repeat 占用的内存量，也会记进来
- cum%: 是累计量占总量的百分比

```text
Type: inuse_space  // 分析数据类型
Time: Jul 1, 2022 at 6:57pm (+08)
Entering interactive mode (type "help" for commands, "o" for options)

(pprof) top5
Showing nodes accounting for 92.44kB, 100% of 92.44kB total
Showing top 5 nodes out of 25
      flat  flat%   sum%        cum   cum%
   53.21kB 57.56% 57.56%    53.21kB 57.56%  main.repeat (inline)
   18.35kB 19.86% 77.42%    26.77kB 28.96%  runtime.allocm
   16.83kB 18.20% 95.62%    16.83kB 18.20%  runtime.malg
    4.05kB  4.38%   100%     4.05kB  4.38%  runtime.acquireSudog
         0     0%   100%    53.21kB 57.56%  main.main

(pprof) top5 -cum
Showing nodes accounting for 71.56kB, 77.42% of 92.44kB total
Showing top 5 nodes out of 25
      flat  flat%   sum%        cum   cum%
         0     0%     0%    53.21kB 57.56%  main.main
   53.21kB 57.56% 57.56%    53.21kB 57.56%  main.repeat (inline)
         0     0% 57.56%    53.21kB 57.56%  runtime.main
   18.35kB 19.86% 77.42%    26.77kB 28.96%  runtime.allocm
         0     0% 77.42%    26.77kB 28.96%  runtime.newm
```

### pprof list

list 查看某个函数的代码，以及该函数每行代码的指标信息，如果函数名不明确，会进行模糊匹配（正则）

```text
(pprof) list main.repeat
Total: 92.44kB
ROUTINE ======================== main.repeat in /Users/schneesu/Desktop/tmp/main.go
   53.21kB    53.21kB (flat, cum) 57.56% of Total
         .          .     20:func repeat(s string, n int) string {
         .          .     21:  var result string
         .          .     22:  for i := 0; i < n; i++ {
   53.21kB    53.21kB     23:    result += s
         .          .     24:  }
         .          .     25:
         .          .     26:  return result
         .          .     27:}
```

### pprof traces

打印所有调用栈，以及调用栈的指标信息

```text
(pprof) traces repeat
Type: inuse_space
Time: Jul 1, 2022 at 6:57pm (+08)
-----------+-------------------------------------------------------
     bytes:  640B
         0   main.repeat (inline)
             main.main
             runtime.main
-----------+-------------------------------------------------------
     bytes:  416B
         0   main.repeat (inline)
             main.main
             runtime.main
-----------+-------------------------------------------------------
     bytes:  320B
...
```

### 火焰图

```bash
brew install graphviz
# 本地启动 web 服务，查看火焰图
go tool pprof -http :8080 mem.pprof
```

## 代码改造

由于测试环境模拟未能复现内存占用上升的情况，因此需要对代码进行改造，获取生产环境数据进行分析

#### 1. 添加 `net/http/pprof` + http server

前面的做法，适用于一次性任务结束后，分析获取的数据，但是对于 apiserver 这种不会停止的服务，就应该使用 `net/http/pprof` 这个包。

我们只需要 import 这个包，并在一个新的 goroutine 中 调用 `http.ListenAndServe()` 在某个端口启动一个 HTTP 服务器即可随时获取需要性能分析数据。

```go
import _ "net/http/pprof"

func main() {
    go func() {
        http.ListenAndServe("0.0.0.0:8083", nil) // nolint:errcheck
    }()
}
```

```text
go tool pprof --inuse_space http://127.0.0.1:8083/debug/pprof/heap + top list traces 三板斧
```

#### 2. Dockerfile alpine + go + src

由于之前过于精简镜像，导致生产环境的镜像，没有 Go 环境，也没有可用于 `list`，`traces` 定位的源码，需要在 alpine 镜像中安装 go 环境

```dockerfile
FROM alpine:3.15 AS runner

RUN apk add --no-cache git make musl-dev go

# Configure Go
ENV GOROOT /usr/lib/go
ENV GOPATH /go
ENV PATH /go/bin:$PATH
RUN mkdir -p ${GOPATH}/src ${GOPATH}/bin

# Source code for pprof debug
COPY --from=builder /go/src /go/src
```

#### 3. 添加 .dockerignore 文件

添加调试用源码不可包含 .git 文件，否则不管是不是开源仓库，都会认为有敏感信息泄露的风险

需要注意的是 `.git` 只能忽略 Dockerfile 所在目录的 `.git`，如果要忽略任意的 `.git`，需要添加配置 `**/.git`

## 数据分析

### 内存数据分析

```text
(pprof) top20
Showing nodes accounting for 57.66MB, 92.76% of 62.16MB total
Showing top 20 nodes out of 249
      flat  flat%   sum%        cum   cum%
      25MB 40.22% 40.22%       25MB 40.22%  google.golang.org/grpc/internal/transport.newBufWriter
   15.98MB 25.70% 65.92%    15.98MB 25.70%  bufio.NewReaderSize
    2.50MB  4.03% 69.94%     2.50MB  4.03%  bytes.makeSlice
    1.50MB  2.42% 72.36%     1.50MB  2.42%  runtime.allocm
    1.50MB  2.41% 74.77%     1.50MB  2.41%  go.uber.org/zap/zapcore.newCounters
    1.50MB  2.41% 77.19%     1.50MB  2.41%  reflect.New
    1.13MB  1.82% 79.01%     1.13MB  1.82%  google.golang.org/protobuf/internal/strs.(*Builder).AppendFullName
       1MB  1.61% 80.62%     2.13MB  3.43%  google.golang.org/protobuf/internal/filedesc.(*Message).unmarshalFull
       1MB  1.61% 82.23%        1MB  1.61%  crypto/aes.(*aesCipherGCM).NewGCM
       1MB  1.61% 83.84%        1MB  1.61%  google.golang.org/grpc/internal/grpcsync.NewEvent (inline)
       1MB  1.61% 85.45%        1MB  1.61%  time.NewTicker
    0.54MB  0.86% 86.31%     0.54MB  0.86%  k8s.io/apimachinery/pkg/runtime.(*Scheme).AddKnownTypeWithName
    0.50MB  0.81% 87.12%     0.50MB  0.81%  regexp.onePassCopy
    0.50MB  0.81% 87.93%     0.50MB  0.81%  bufio.NewWriterSize
    0.50MB  0.81% 88.73%     0.50MB  0.81%  google.golang.org/protobuf/internal/impl.(*MessageInfo).makeCoderMethods
    0.50MB  0.81% 89.54%     0.50MB  0.81%  github.com/prometheus/client_golang/prometheus.(*Registry).Register
    0.50MB  0.81% 90.34%     1.50MB  2.41%  google.golang.org/grpc.DialContext
    0.50MB  0.81% 91.15%     0.50MB  0.81%  k8s.io/api/core/v1.init
    0.50MB  0.81% 91.95%     2.50MB  4.03%  crypto/tls.(*Conn).readHandshake
    0.50MB   0.8% 92.76%     0.50MB   0.8%  google.golang.org/grpc.(*ClientConn).newAddrConn
(pprof) top20 -cum
Showing nodes accounting for 40.97MB, 65.92% of 62.16MB total
Showing top 20 nodes out of 249
      flat  flat%   sum%        cum   cum%
         0     0%     0%    41.97MB 67.52%  google.golang.org/grpc.(*addrConn).connect
         0     0%     0%    41.97MB 67.52%  google.golang.org/grpc.(*addrConn).createTransport
         0     0%     0%    41.97MB 67.52%  google.golang.org/grpc.(*addrConn).resetTransport
         0     0%     0%    41.97MB 67.52%  google.golang.org/grpc.(*addrConn).tryAllAddrs
         0     0%     0%    41.47MB 66.72%  google.golang.org/grpc/internal/transport.NewClientTransport (inline)
         0     0%     0%    41.47MB 66.72%  google.golang.org/grpc/internal/transport.newHTTP2Client
         0     0%     0%    40.47MB 65.11%  google.golang.org/grpc/internal/transport.newFramer
      25MB 40.22% 40.22%       25MB 40.22%  google.golang.org/grpc/internal/transport.newBufWriter (inline)
   15.98MB 25.70% 65.92%    15.98MB 25.70%  bufio.NewReaderSize (inline)
         0     0% 65.92%     6.04MB  9.72%  runtime.main
         0     0% 65.92%     4.01MB  6.44%  crypto/tls.(*Conn).Handshake (inline)
         0     0% 65.92%     4.01MB  6.44%  crypto/tls.(*Conn).HandshakeContext (inline)
         0     0% 65.92%     4.01MB  6.44%  crypto/tls.(*Conn).clientHandshake
         0     0% 65.92%     4.01MB  6.44%  crypto/tls.(*Conn).handshakeContext
         0     0% 65.92%     4.01MB  6.44%  crypto/tls.(*clientHandshakeStateTLS13).handshake
         0     0% 65.92%     4.01MB  6.44%  google.golang.org/grpc/credentials.(*tlsCreds).ClientHandshake.func1
         0     0% 65.92%     3.50MB  5.63%  github.com/Tencent/bk-bcs/bcs-services/cluster-resources/cmd.(*clusterResourcesService).Init
         0     0% 65.92%     3.50MB  5.63%  github.com/Tencent/bk-bcs/bcs-services/cluster-resources/cmd.Start
         0     0% 65.92%     3.50MB  5.63%  main.main
         0     0% 65.92%        3MB  4.83%  crypto/tls.(*clientHandshakeStateTLS13).readServerCertificate
```

可以观察到 `google.golang.org/grpc.(*addrConn).connect` 是造成内存增长的主要原因（入口）

而 `google.golang.org/grpc/internal/transport.newBufWriter (inline)` 和 `bufio.NewReaderSize (inline)` 是根本原因

```text
(pprof) list newFramer
Total: 62.16MB
ROUTINE ======================== google.golang.org/grpc/internal/transport.newFramer in /go/pkg/mod/google.golang.org/grpc@v1.42.0/internal/transport/http_util.go
         0    40.47MB (flat, cum) 65.11% of Total
         .          .    377:   if writeBufferSize < 0 {
         .          .    378:           writeBufferSize = 0
         .          .    379:   }
         .          .    380:   var r io.Reader = conn
         .          .    381:   if readBufferSize > 0 {
         .    15.47MB    382:           r = bufio.NewReaderSize(r, readBufferSize)       // buffer 1：内存分配 32KB
         .          .    383:   }
         .       25MB    384:   w := newBufWriter(conn, writeBufferSize)                 // buffer 2
         .          .    385:   f := &framer{
         .          .    386:           writer: w,
         .          .    387:           fr:     http2.NewFramer(w, r),
         .          .    388:   }
         .          .    389:   f.fr.SetMaxReadFrameSize(http2MaxFrameLen)

(pprof) list newBufWriter
Total: 62.16MB
ROUTINE ======================== google.golang.org/grpc/internal/transport.newBufWriter in /go/pkg/mod/google.golang.org/grpc@v1.42.0/internal/transport/http_util.go
      25MB       25MB (flat, cum) 40.22% of Total
         .          .    326:   onFlush func()
         .          .    327:}
         .          .    328:
         .          .    329:func newBufWriter(conn net.Conn, batchSize int) *bufWriter {
         .          .    330:   return &bufWriter{
      25MB       25MB    331:           buf:       make([]byte, batchSize*2),            // 内存分配 32KB * 2
         .          .    332:           batchSize: batchSize,
         .          .    333:           conn:      conn,
         .          .    334:   }
         .          .    335:
```

traces 查看调用关系，但是在 addrConn.connect 处戛然而止

```text
(pprof) traces newFramer
File: cluster-resources-service
Type: inuse_space
Time: Jul 1, 2022 at 11:38am (UTC)
-----------+-------------------------------------------------------
     bytes:  32kB
   15.47MB   bufio.NewReaderSize (inline)
             google.golang.org/grpc/internal/transport.newFramer
             google.golang.org/grpc/internal/transport.newHTTP2Client
             google.golang.org/grpc/internal/transport.NewClientTransport (inline)
             google.golang.org/grpc.(*addrConn).createTransport
             google.golang.org/grpc.(*addrConn).tryAllAddrs
             google.golang.org/grpc.(*addrConn).resetTransport
             google.golang.org/grpc.(*addrConn).connect
-----------+-------------------------------------------------------
     bytes:  64kB
      25MB   google.golang.org/grpc/internal/transport.newBufWriter (inline)
             google.golang.org/grpc/internal/transport.newFramer
             google.golang.org/grpc/internal/transport.newHTTP2Client
             google.golang.org/grpc/internal/transport.NewClientTransport (inline)
             google.golang.org/grpc.(*addrConn).createTransport
             google.golang.org/grpc.(*addrConn).tryAllAddrs
             google.golang.org/grpc.(*addrConn).resetTransport
             google.golang.org/grpc.(*addrConn).connect
-----------+-------------------------------------------------------
```

可以排除内存缓存增长的问题，那么，为什么会有那么多的链接呢？

怀疑是不是 websocket 链接没有释放？保持了很多的长链接？

=> 分析发现 websocket 会主动 close，k8s watch 有默认的超时时间（可优化）+ 手动起很多 websocket 链接，涨内存效果不明显

排查线索断了，看起来大概率不是 websocket 的问题

### Goroutine 数据分析

```text
Type: goroutine
(pprof) top20
Showing nodes accounting for 1368, 99.85% of 1370 total
Dropped 85 nodes (cum <= 6)
Showing top 20 nodes out of 36
      flat  flat%   sum%        cum   cum%
      1368 99.85% 99.85%       1368 99.85%  runtime.gopark
         0     0% 99.85%        436 31.82%  bufio.(*Reader).Read
         0     0% 99.85%          8  0.58%  bufio.(*Reader).ReadLine
         0     0% 99.85%          8  0.58%  bufio.(*Reader).ReadSlice
         0     0% 99.85%          9  0.66%  bufio.(*Reader).fill
         0     0% 99.85%        444 32.41%  bytes.(*Buffer).ReadFrom
         0     0% 99.85%        444 32.41%  crypto/tls.(*Conn).Read
         0     0% 99.85%        444 32.41%  crypto/tls.(*Conn).readFromUntil
         0     0% 99.85%        444 32.41%  crypto/tls.(*Conn).readRecord (inline)
         0     0% 99.85%        444 32.41%  crypto/tls.(*Conn).readRecordOrCCS
         0     0% 99.85%        444 32.41%  crypto/tls.(*atLeastReader).Read
         0     0% 99.85%        436 31.82%  golang.org/x/net/http2.(*Framer).ReadFrame
         0     0% 99.85%        436 31.82%  golang.org/x/net/http2.readFrameHeader
         0     0% 99.85%        445 32.48%  google.golang.org/grpc.(*ccBalancerWrapper).watcher
         0     0% 99.85%        436 31.82%  google.golang.org/grpc/internal/transport.(*controlBuffer).get
         0     0% 99.85%        435 31.75%  google.golang.org/grpc/internal/transport.(*http2Client).reader
         0     0% 99.85%        436 31.82%  google.golang.org/grpc/internal/transport.(*loopyWriter).run
         0     0% 99.85%        435 31.75%  google.golang.org/grpc/internal/transport.newHTTP2Client.func3
         0     0% 99.85%        446 32.55%  internal/poll.(*FD).Read
         0     0% 99.85%        450 32.85%  internal/poll.(*pollDesc).wait

(pprof) top20 -cum
Showing nodes accounting for 1368, 99.85% of 1370 total
Dropped 85 nodes (cum <= 6)
Showing top 20 nodes out of 36
      flat  flat%   sum%        cum   cum%
      1368 99.85% 99.85%       1368 99.85%  runtime.gopark
         0     0% 99.85%        900 65.69%  runtime.selectgo
         0     0% 99.85%        450 32.85%  internal/poll.(*pollDesc).wait
         0     0% 99.85%        450 32.85%  internal/poll.(*pollDesc).waitRead (inline)
         0     0% 99.85%        450 32.85%  internal/poll.runtime_pollWait
         0     0% 99.85%        450 32.85%  runtime.netpollblock
         0     0% 99.85%        446 32.55%  internal/poll.(*FD).Read
         0     0% 99.85%        446 32.55%  net.(*conn).Read
         0     0% 99.85%        446 32.55%  net.(*netFD).Read
         0     0% 99.85%        445 32.48%  google.golang.org/grpc.(*ccBalancerWrapper).watcher
         0     0% 99.85%        444 32.41%  bytes.(*Buffer).ReadFrom
         0     0% 99.85%        444 32.41%  crypto/tls.(*Conn).Read
         0     0% 99.85%        444 32.41%  crypto/tls.(*Conn).readFromUntil
         0     0% 99.85%        444 32.41%  crypto/tls.(*Conn).readRecord (inline)
         0     0% 99.85%        444 32.41%  crypto/tls.(*Conn).readRecordOrCCS
         0     0% 99.85%        444 32.41%  crypto/tls.(*atLeastReader).Read
         0     0% 99.85%        437 31.90%  io.ReadAtLeast
         0     0% 99.85%        437 31.90%  io.ReadFull (inline)
         0     0% 99.85%        436 31.82%  bufio.(*Reader).Read
         0     0% 99.85%        436 31.82%  golang.org/x/net/http2.(*Framer).ReadFrame
```

可以看出确实很多的链接没释放，通过 traces 也找不到源码中调用的地方...

```text
(pprof) traces runtime.gopark
File: cluster-resources-service
Type: goroutine
Time: Jul 1, 2022 at 12:24pm (UTC)
-----------+-------------------------------------------------------
       615   runtime.gopark
             runtime.selectgo
             google.golang.org/grpc.(*ccBalancerWrapper).watcher
-----------+-------------------------------------------------------
       605   runtime.gopark
             runtime.netpollblock
             internal/poll.runtime_pollWait
             ...
             golang.org/x/net/http2.readFrameHeader
             golang.org/x/net/http2.(*Framer).ReadFrame
             google.golang.org/grpc/internal/transport.(*http2Client).reader
-----------+-------------------------------------------------------
       605   runtime.gopark
             runtime.selectgo
             google.golang.org/grpc/internal/transport.(*controlBuffer).get
             google.golang.org/grpc/internal/transport.(*loopyWriter).run
             google.golang.org/grpc/internal/transport.newHTTP2Client.func3
```

## GG ?

冷静思考，哪里使用了 grpc：1. 普通短连接 API，2. websocket，3. 通过 服务发现 + grpc 调用其他模块 API！

![img](/static/image/blog/addrConn_find_result.png)

从内存分配能溯源的最上层 `addrConn` 找，通过利用 IDE 的过滤条件，缩小范围，grpc 包中只有 7 个文件，53 处声明。

```go
// tearDown starts to tear down the addrConn.
//
// Note that tearDown doesn't remove ac from ac.cc.conns, so the addrConn struct
// will leak. In most cases, call cc.removeAddrConn() instead.
func (ac *addrConn) tearDown(err error) {...}
```

翻看 addrConn 相关方法，可以发现 addrConn 有回收相关的方法（tearDown），同时还指向了一个 `cc.removeAddrConn()` 的方法，顺路找到：

```go
// Close tears down the ClientConn and all underlying connections.
func (cc *ClientConn) Close() error {...}
```

原来 grpc client 连接不再使用需要手动关闭，否则会有内存泄露的问题！

另外，为什么测试环境最开始无法复现，原因是这个其他模块 API 调用加上了缓存，测试环境只有一个集群，所以在缓存生效期内，只会有一个未关闭的 grpc 链接。

这也涉及到了，缓存干扰测试环境复现问题的讨论了，但是缓存也避免了生产环境直接打爆，但是没有缓存的话，是不是开发时候就可以发现这个问题？前提是开发时候有关注内存情况（比如设置 resources.limits.memory 从而去关注 OOM 问题）。

总之，一通改造（干掉缓存，关闭 close）后测试，内存数据，pprof 指标均正常，修复上线生产环境，观察一段时间，确认内存数据基本稳定，问题解决。

![img](/static/image/blog/memory_health.png)

## 反思

这个代码好像是抄的别人给的例子，难道是自己抄完优化时候，小手一抖干掉了 `.Close()` ？

翻聊天记录，看源码确实没有 Close，反馈到相关同学，回复说确实曾经出现内存泄露的问题，但是这个坑不知为何，一直都在，遂 push 他们填坑，至少要把注释加上，避免别人反复掉坑。

## 总结

- 内存增长不要慌，监控告警用起来
- 巧用工具来定位，深思熟虑找问题
- 修复前后做对比，及时总结加反馈

## 参考资料

1. [你不知道的 Go 之 pprof](https://darjun.github.io/2021/06/09/youdontknowgo/pprof/)
2. [实战 Go 内存泄露](https://segmentfault.com/a/1190000019222661)
3. [How to install Go in alpine linux（注：最高赞的亲测不行，没仔细研究原因）](https://stackoverflow.com/questions/52056387/how-to-install-go-in-alpine-linux)
4. [ignore all .git folders in .dockerignore](https://stackoverflow.com/questions/54793349/ignore-all-git-folders-in-dockerignore)
