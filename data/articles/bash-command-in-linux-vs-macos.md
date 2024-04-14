## 背景

故事是这样的，我们一般构建镜像是直接在 `Dockerfile` 所在目录下，通过 `COPY` 或 `ADD` 将源码添加到构建容器中，执行编译等步骤。

而 bcs 各个模块构建镜像的统一流程是：通过 `makefile` 将源码等文件，复制到指定构建目录，然后执行 `docker build` 构建镜像。

之前做适配，在本地验证 ok 后，就把 `makefile` 提交了，但是统一出的镜像却发现模板文件找不到了，通过 [dive](https://github.com/wagoodman/dive) 工具分析，发现目录不对：

```text
[actual]                      [excepted]
workspace                     workspace
├── etc                       ├── etc
├── example                   ├── example
├── tmpl                      ├── tmpl
│   └── tmpl                  │   ├── layout
│       ├── layout            │   ├── manifest
│       ├── manifest          │   └── schema
│       └── schema            ├── lc_msgs.yaml
├── lc_msgs.yaml              └── swagger
└── swagger
```

```makefile
# 简化后的 makefile
cluster-resources: pre
    mkdir -p ${PACKAGEPATH}/bcs-services/cluster-resources
    ...
    # form tmpl & schema files
    mkdir -p ${PACKAGEPATH}/bcs-services/cluster-resources/tmpl/
    cp -R ${BCS_SERVICES_PATH}/cluster-resources/pkg/resource/form/tmpl/ ${PACKAGEPATH}/bcs-services/cluster-resources/tmpl/
    ...
```

通过测试发现，在 MacOS 下当目标目录存在时，以上的命令会将 tmpl 目录下的 **子目录**，复制到目标目录下，也就是正确的情况。而在 Linux 环境下，当目标目录存在时，会把 **整个 tmpl 目录** 复制到目标目录下。

那么问题简单了，不要默认创建目录即可；但是也不禁思考：原来 MacOS 下和 Linux 下的 cp 命令还是有些许的差异的，由于这两个是我们开发常用的系统，会不会有更多的常用命令，其实是有所差异的呢？

## 深入研究下

翻阅一些资料 & 博客文章，常见的 MacOS 与 Linux 下命令行工具差异如下：

### cp

`cp -r source/ dest/`

| cp -r        | MacOS                            | Linux                      |
| ------------ | -------------------------------- | -------------------------- |
| 目标目录存在   | 将 source 下的内容复制到 dest 目录下 | 将 source 复制到 dest 目录下 |
| 目标目录不存在 | 将 source 目录复制为 dest           | 将 source 目录复制为 dest   |

解决方案：`cp -r source/. dest/` 确保是只复制目录下的内容

### file

```bash
# MacOS
# -i            do not further classify regular files
# -I, --mime    output MIME type strings (--mime-type and --mime-encoding)
> file -i Readme.md 
Readme.md: regular file

> file -I Readme.md 
Readme.md: text/plain; charset=utf-8

# Linux
# -i, --mime    output MIME type strings (--mime-type and --mime-encoding)
> file -i Readme.md
Readme.md: text/plain; charset=utf-8

> file -I Readme.md
file: invalid option -- 'I'
```

MacOS 系统下要获取文件内容类型，应该使用 `file -I`，小写字母 `i` 是特殊指定选项，表示为如果目标对象是普通文件，则不进行分类。

而在 Linux 下只能使用 `file -i`，大写字母 `I` 不是合法的选项。

### ps

Liunx 中可以通过 `ps auxf` 命令查看进程树（父子进程关系）

```bash
root      6078  0.0  0.2 1230264 38268 ?       Ssl  Nov18   9:50 /usr/bin/containerd
root      7023  0.1  0.0 108744  5324 ?        Sl   Nov18  19:08  \_ containerd-shim -namespace moby -workdir /var/lib/containerd/io.containerd.runtime.v1.linux/moby/cca
root      7041  0.0  0.0 722944  9100 ?        Ssl  Nov18   0:33  |   \_ diving
root      8465  0.0  0.0 108808  3888 ?        Sl   10:52   0:00  \_ containerd-shim -namespace moby -workdir /var/lib/containerd/io.containerd.runtime.v1.linux/moby/42d
root      8482  0.0  0.0   6000  2808 ?        Ss   10:52   0:00  |   \_ nginx: master process nginx -g daemon off;
101       8555  0.0  0.0   6456  1468 ?        S    10:52   0:00  |       \_ nginx: worker process
root     10958  0.0  0.0 108616  4184 ?        Sl   10:55   0:00  \_ containerd-shim -namespace moby -workdir /var/lib/containerd/io.containerd.runtime.v1.linux/moby/75c
root     10975  0.1  0.0   1568   252 ?        Ss   10:55   0:00      \_ sleep 10000
root      6094  0.0  0.0  10852  1572 ?        S    Nov18   0:10 /bin/bash /usr/local/sa/agent/watchdog.sh
root     11041  0.0  0.0   7476   380 ?        S    10:55   0:00  \_ sleep 60
...
```

但是 MacOS 的原生 ps 则没有该功能，如果需要类似功能，需要安装 pstree，功能与 Linux 的 pstree 相似，但是缺少很多信息。

```bash
brew install pstree
```

### head

Linux 下的 head 支持指定 -n 负数，不展示最后的 x 行，但是 MaxOS 下不可行

```bash
tmp.txt 为一个 10 行的文件，每行一个数字从 1-10
# Linux
~  head -n 4 tmp.txt
1
2
3
4
~  head -n -4 tmp.txt
1
2
3
4
5
6

# MacOS
» head -n 4 tmp.txt
1
2
3
4

» head -n -4 tmp.txt
head: illegal line count -- -4
```

### sed

在 linux 下，将文件中的 aaa 替换成 bbb

`sed -i 's/aaa/bbb/g' test.py`

但是在 MacOS 下，则需要另外指定一个文件后缀，会把 sed 执行前的文件备份一下

`sed -i '.bak' 's/aaa/bbb/g' test.py`

执行完成后，test.py 会被修改，但是原来的文件内容则保存到了 test.py.bak 文件中，如果不需要备份文件，则可以指定一个空字符串

`sed -i '' 's/aaa/bbb/g' test.py`

另外，其他的比如正则表达式也有略微的区别。

## 总结

> The major problem is that the MacOS coreutils are FreeBSD-based while the utilities you are used to are most likely from the GNU project. The FreeBSD coreutils are not always compatible with the GNU coreutils. There are performance and behavioral differences between the GNU and FreeBSD versions of sed, grep, ps, and other utilities.
>
> You can install the GNU coreutils but they have g- prefixes (e.g. gcat for cat). It's not a good idea to replace the MacOS coreutils with the GNU coreutils.

MacOS 与 Linux 下 Bash 命令行的细微差别，主要的原因是 MacOS 的 coreutils 其实是基于 FreeBSD 的，而 Linux 中的是基于 GNU 项目的，两者很相似但是并非完全兼容，各个命令之前存在一定的性能 & 行为的差异。

当然了，在 MacOS 中也可以通过执行以下命令来安装 GNU 的 coreutils，但这应该是个很蛋疼 & 没必要的事情...

```bash
brew install coreutils findutils gnu-tar gnu-sed gawk gnutls gnu-indent gnu-getopt grep
```

## 参考资料

- [macOS and Linux Treat 'cp' Differently](https://twilblog.github.io/macos/linux/cp/copy/2017/02/03/macos-unix-cp-madness.html)
- [How does Mac's command line compare to Linux?](https://superuser.com/questions/179368/how-does-macs-command-line-compare-to-linux)
- [What are the differences between using the terminal on a mac vs linux?](https://stackoverflow.com/questions/8051145/what-are-the-differences-between-using-the-terminal-on-a-mac-vs-linux)
- [Bash in Linux v.s Mac OS](https://unix.stackexchange.com/questions/82244/bash-in-linux-v-s-mac-os)
- [Differences between sed on Mac OSX and other "standard" sed?](https://unix.stackexchange.com/questions/13711/differences-between-sed-on-mac-osx-and-other-standard-sed)
- [How to replace Mac OS X utilities with GNU core utilities?](https://apple.stackexchange.com/questions/69223/how-to-replace-mac-os-x-utilities-with-gnu-core-utilities)
