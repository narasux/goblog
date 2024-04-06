## 插件的定义

按照维基百科的解释：插件（plugin） 又译外挂、扩展，是一种电脑程序，通过和应用程序的互动，用来替应用程序增加一些所需要的特定的功能。最常见的有游戏、网页浏览器的插件和媒体播放器的插件。

简而言之：插件是一段独立于主代码库的代码，通过某种方式在主程序中被调用，用于扩展主程序的能力。

## Python 插件实现

开发 Python 这种解释型语言的插件是比较方便的，主要原因是 Python 有动态加载模块的能力，可以在运行时动态地加载和执行插件代码，而无需在程序开始时知道所有的插件。这种特性在容器化环境下尤其便利，我们可以在运行打包好的容器镜像的时候，额外将 Python 插件代码通过挂载卷的方式挂载到约定的目录下，便能够靠主程序本身的插件管理能力，发现，注册并使用插件。

### 举个例子

以用户管理为例，需要将来自 excel，http api，ldap，mad 等不同来源的数据转换成标准格式用于导入，这种场景下就很适合使用插件机制来实现，即使后续外部用户有对接其他来源数据的需求，也可以通过定制化插件来进行能力的扩展。

在用户管理中，我们使用抽象基类定义了一个数据源插件的关键方法：

- `__init__` 接收数据源插件配置等信息并初始化
- `fetch_departments` 获取部门信息，并以 `List[RawDataSourceDepartment]` 的格式输出
- `fetch_users` 获取用户信息，并以 `List[RawDataSourceUser]` 的格式输出
- `test_connection` 测试数据源连通性，输出测试结果 `TestConnectionResult`

```python
# plugins/base.py
class BaseDataSourcePlugin(ABC):
    """数据源插件基类"""

    id: str | DataSourcePluginEnum
    config_class: Type[BasePluginConfig]

    @abstractmethod
    def __init__(self, *args, **kwargs):
        ...

    @abstractmethod
    def fetch_departments(self) -> List[RawDataSourceDepartment]:
        """获取部门信息"""
        ...

    @abstractmethod
    def fetch_users(self) -> List[RawDataSourceUser]:
        """获取用户信息"""
        ...

    @abstractmethod
    def test_connection(self) -> TestConnectionResult:
        """连通性测试（非本地数据源需提供）"""
        ...
```

在插件管理上，我们简单地使用全局 Dict 来进行管理，并且提供 `register_plugin` 方法用于插件注册，同时也提供一些辅助方法，方便后续插件的查询与使用：

```python
# plugins/base.py
_plugin_cls_map: Dict[str | DataSourcePluginEnum, Type[BaseDataSourcePlugin]] = {}

def register_plugin(plugin_cls: Type[BaseDataSourcePlugin]):
    """注册数据源插件"""
    plugin_id = plugin_cls.id

    if not plugin_id:
        raise RuntimeError(f"plugin {plugin_cls} not provide id")

    if not plugin_cls.config_class:
        raise RuntimeError(f"plugin {plugin_cls} not provide config_class")

    if not (isinstance(plugin_id, DataSourcePluginEnum) or plugin_id.startswith(CUSTOM_PLUGIN_ID_PREFIX)):
        raise RuntimeError(f"custom plugin's id must start with `{CUSTOM_PLUGIN_ID_PREFIX}`")

    logger.info("register data source plugin: %s", plugin_id)

    _plugin_cls_map[plugin_id] = plugin_cls


def is_plugin_exists(plugin_id: str | DataSourcePluginEnum) -> bool:
    """判断插件是否存在"""
    return plugin_id in _plugin_cls_map


def get_plugin_cls(plugin_id: str | DataSourcePluginEnum) -> Type[BaseDataSourcePlugin]:
    """获取指定插件类"""
    if plugin_id not in _plugin_cls_map:
        raise NotImplementedError(f"plugin {plugin_id} not implement or register")

    return _plugin_cls_map[plugin_id]


def get_plugin_cfg_cls(plugin_id: str | DataSourcePluginEnum) -> Type[BasePluginConfig]:
    """获取指定插件的配置类"""
    return get_plugin_cls(plugin_id).config_class
```

在对应的插件目录中，继承插件基类，并实现定义好的抽象方法

```python
# plugins/local/plugin.py
from bkuser.plugins.base import BaseDataSourcePlugin

class LocalDataSourcePlugin(BaseDataSourcePlugin):
    """本地数据源插件"""

    id = DataSourcePluginEnum.LOCAL
    config_class = LocalDataSourcePluginConfig

    def __init__(self, plugin_config: LocalDataSourcePluginConfig, workbook: Workbook):
        ...

    def fetch_departments(self) -> List[RawDataSourceDepartment]:
        """获取部门信息"""
        ...

    def fetch_users(self) -> List[RawDataSourceUser]:
        """获取用户信息"""
        ...

    def test_connection(self) -> TestConnectionResult:
        raise NotImplementedError(_("本地数据源不支持连通性测试"))
```

利用调用 package 时必定首先执行 `__init__.py` 的特性，在 `__init__.py` 中完成代码插件的注册：

```python
# plugins/local/__init__.py

from bkuser.plugins.base import register_plugin

from .plugin import LocalDataSourcePlugin

register_plugin(LocalDataSourcePlugin)
```

到这里还存在一个问题：如果插件目录从来没被 import，则主程序也无从得知这个插件的存在，因此还需要有一个机制，扫描约定好的插件目录，将所有插件 import 一下以触发 `register_plugin` 方法的运行。

```python
# plugins/__init__.py
import os
from importlib import import_module

from django.conf import settings


def load_plugins():
    plugin_base_dir = settings.BASE_DIR / "bkuser" / "plugins"
    for name in os.listdir(plugin_base_dir):
        if not os.path.isdir(plugin_base_dir / name):
            continue

        # NOTE: 需要先在各个插件的 __init__.py 文件中调用 register_plugin 注册插件
        import_module(f"bkuser.plugins.{name}")


load_plugins()
```

## Golang 插件实现（OldSchool）

由于 Golang 是一种编译型语言，也就注定了其不能走 Python 这种将代码挂载到目录，靠程序运行时候动态加载的野路子。

传统的 Golang 插件实现需要依赖 Go 原生提供的 plugin 包，其允许将 Go 代码编译成动态链接库（.so），在主程序运行时动态加载和链接。

具体的做法是：将插件代码编译为 `plugin.so` 文件，在主程序中通过 `plugin.Open` 函数打开一个插件文件，使用 `plugin.Lookup` 查找插件中的符号（通常是函数或者变量），判定类型（一般是接口）后直接调用。

下面是一个简单的 Golang 插件例子：

```go
// main.go
package main

type Plugin interface {
    DoSomething()
}
```

然后，我们编写一个实现了这个接口的 Go 插件，将其编译为 .so 文件：

```go
// plugin.go
package main

import "fmt"

type MyPlugin struct{}

func (p *MyPlugin) DoSomething() {
    fmt.Println("do something")
}

var Plugin MyPlugin
```

编译为动态链接库，需要指定 buildmode 为 plugin：

```bash
go build -buildmode=plugin -o plugin.so plugin.go
```

在主程序中，我们可以使用 plugin 包动态加载并使用这个插件：

```go
// main.go
package main

import (
    "plugin"
)

func main() {
    plug, err := plugin.Open("plugin.so")
    if err != nil {
        panic(err.Error())
    }
    symPlugin, err := plug.Lookup("Plugin")
    if err != nil {
        panic(err.Error())
    }

    var p Plugin
    p, ok := symPlugin.(Plugin)
    if !ok {
        panic(err.Error())
    }

    p.DoSomething()
}
```

输出结果

```shell
~/Desktop » go run main.go
do something
```

Go 语言的插件系统基于 C 语言动态库实现的，在 plugin 包的实现中，调用了 CGO 的 `dlopen`, `dlerror`, `dlsym`, `dlclose` 等函数来加载和处理动态链接库（.so），在加载完成后读取特定的 Symbol 执行功能。需要注意的是，Golang 的 plugin 包在 Windows 下是不支持。

## Golang 插件实现（RPC/HTTP）

对于希望插件系统能够更加独立，不依赖于 Golang 静态链接的限制，也不想用 CGO & 动态链接库的场景，可以使用 RPC 机制来开发插件，通过网络将调用主体（Client）和插件服务器（Server）进行分离，这样的好处有：插件服务器可以为更多的客户端服务，甚至插件服务器编程语言可以和客户端不同，只要他们的 RPC 协议是一致的即可。下面是一个简单的例子：

```go
// main.go
package main

import (
    "net"
    "net/rpc"
)

type PluginRPC interface {
    DoSomething(input string, output *string) error
}

type PluginRPCServer struct{}

func (s *PluginRPCServer) DoSomething(input string, output *string) error {
    *output = "hello, " + input
    return nil
}

func main() {
    plugin := PluginRPCServer{}
    rpc.Register(plugin)

    listener, err := net.Listen("tcp", ":8080")
    if err != nil {
        panic(err.Error())
    }

    for {
        conn, err := listener.Accept()
        if err != nil {
            continue
        }
        go rpc.ServeConn(conn)
    }
}
```

```go
// client.go
package main

import (
    "fmt"
    "net/rpc"
)

func main() {
    client, err := rpc.Dial("tcp", "127.0.0.1:8080")
    if err != nil {
        panic(err.Error())
    }

    var reply string
    if err = client.Call("PluginRPCServer.DoSomething", "World", &reply); err != nil {
        panic(err.Error())
    }
    fmt.Println("exec result:", reply)
}
```

基于 HTTP 的插件模式与 RPC 的类似，此处就不在赘述。

## Golang 插件实现（子命令/执行转发）

helm 和 kubectl 插件机制采用的是子命令模型。在这个模型中，插件被作为独立的可执行文件（executables）实现，并且按照特定的命名约定放置在系统的可执行路径（PATH）中。

当用户输入一个插件命令时，例如 kubectl plugin-name，kubectl 的主程序会在 PATH 中搜索符合命名约定的可执行文件。一旦找到合适的插件可执行文件，主程序 kubectl 就会将命令行的执行转发给插件程序，插件程序接管后续的操作并直接与用户进行交互。

举个例子，假设存在一个名为 kubectl-image 的 kubectl 插件，用户通过命令行键入 kubectl image。kubectl 会在 PATH 中查找名为 kubectl-image 的可执行文件，并传递所有的剩余参数给它。

> 注：如果 kubectl-image 插件不存在，你可以通过 krew 来安装该插件

插件可使用任何编程语言编写，只要最终可执行文件遵循 kubectl 的插件命名和调用规则。这种设计便于插件的开发和部署，因为它们只需满足简单的外部合约，而无需关心复杂的内部通信机制或协议。

Helm 的插件机制与 kubectl 相似，不过与 kubectl 依赖 krew 管理插件不同的是，helm 本身就支持对插件的管理（plugin 子命令）。

```bash
>>> helm plugin

Manage client-side Helm plugins.

Usage:
  helm plugin [command]

Available Commands:
  install     install one or more Helm plugins
  list        list installed Helm plugins
  uninstall   uninstall one or more Helm plugins
  update      update one or more Helm plugins

>>> helm plugin install https://github.com/chartmuseum/helm-push.git
Downloading and installing helm-push v0.10.4 ...
https://github.com/chartmuseum/helm-push/releases/download/v0.10.4/helm-push_0.10.4_darwin_amd64.tar.gz
Installed plugin: cm-push

>>> helm plugin list
NAME    VERSION DESCRIPTION                      
cm-push 0.10.4  Push chart package to ChartMuseum
```

## 参考资料

- [Python 优雅地实现插件架构](https://developer.aliyun.com/article/308565)
- [从 0 到 1，如何徒手撸个 Python 插件系统？](https://juejin.cn/post/7081182679193878541)
- [Go 语言设计与实现 - 插件系统](https://draveness.me/golang/docs/part4-advanced/ch08-metaprogramming/golang-plugin/)
- [Go plugins - Exploring Go's plugin package](https://www.aadhav.me/posts/go-plugins)
- [The Helm Plugins Guide](https://helm.sh/docs/topics/plugins/)
