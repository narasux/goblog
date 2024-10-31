## 背景

故事是这样的：最近给 [blueking-paas](https://github.com/TencentBlueKing/blueking-paas) 项目的 apiserver 模块升级下 Django（3.2.25 -> 4.2.16），本来开开心心地写着代码听着歌，结果就出 Bug 了。

## 案件现场

升级到 Django 4.2 后，访问后台管理页面（admin42）开始出现需要反复登录的情况，具体情况如下：

进入 / 刷新 admin42 页面 -> 重定向到登录页面，登录后正常进入 admin42（全程没有任何 warning / error 日志）

这就很诡异了，首次登录 / 登录态失效，让我重新登录可以理解，我刚刚登录进来，刷新下页面就要重新登录，凭什么？

## 犯罪分析

首先需要确定到底是不是 Django 问题，降低版本到 3.2，测试 OK，升级到 4.2，爆炸，所以基本可以确定，Django 升级后的某个实现差异，导致目前的用户认证体系失效。

翻下 Django 4.0/1/2 的 [Release Notes](https://docs.djangoproject.com/en/4.2/releases/4.2/#backwards-incompatible-changes-in-4-2)，没看到啥相关的信息，搜索下 issue，stack overflow 啥的，也没有类似的讨论 ~~(难道只有我一个人...)~~。

那就整点狠货，直接看代码，Django 用户认证相关逻辑的实现在 `django.contrib.auth` 模块中，Github 上拉 [Compare](https://github.com/django/django/compare/stable/3.2.x...stable/4.2.x) 看看。

简单翻一翻，虽然代码变化挺多，但是好像很多都是代码样式优化，Django 3.2 -> 4.2 的用户认证并没有很大的改动（毕竟是核心模块，没有重构应该也不会有很大的变化）

那就没办法了，只能本地调试看看，反复尝试几次，发现每次都是可以稳定复现的，那么查出原因就只是时间问题了，掏出 Pycharm 进 Debugger，开搞～

### Q1：什么原因导致的重定向到登录？

```python
MIDDLEWARE = [
    ...
    "django.contrib.sessions.middleware.SessionMiddleware",
    ...
    "django.contrib.auth.middleware.AuthenticationMiddleware",
    ...
    "bkpaas_auth.middlewares.CookieLoginMiddleware",
    "paasng.infras.accounts.middlewares.SiteAccessControlMiddleware",
    "paasng.infras.accounts.middlewares.PrivateTokenAuthenticationMiddleware",
    "apigw_manager.apigw.authentication.ApiGatewayJWTGenericMiddleware",
    "apigw_manager.apigw.authentication.ApiGatewayJWTAppMiddleware",
    "paasng.infras.accounts.middlewares.WrapUsernameAsUserMiddleware",
    "apigw_manager.apigw.authentication.ApiGatewayJWTUserMiddleware",
    "paasng.infras.accounts.middlewares.AuthenticatedAppAsUserMiddleware",
    "blue_krill.auth.client.VerifiedClientMiddleware",
    "paasng.infras.accounts.internal.user.SysUserFromVerifiedClientMiddleware",
    ...
]
```

Blueking PaaS Apiserver 是个有一定的年代的项目，其中使用了大量的中间件，与用户认证/身份转换相关的，就有上面这一堆...

通过 Pycharm Debug 可以发现，在普通用户认证中间件（`CookieLoginMiddleware`）中，有这么一段逻辑：

```python
class CookieLoginMiddleware(MiddlewareMixin):
    """Call auth.login when user credential cookies changes"""

    def process_request(self, request):
        backend = UniversalAuthBackend()
        credentials = backend.get_credentials(request)

        if not credentials:
            auth.logout(request)
            return self.get_response(request)

        # 如果 cookie 与 session 中的 token 不一致
        if self.should_authenticate(request, backend, credentials):
            # 重新执行认证 & 登录
            self.authenticate_and_login(request, credentials)
            ...

        return self.get_response(request)
```

测试发现一个特别奇怪的情况：

1. 如果 cookie 与 session 中的 token 不一致，则会触发登录（重定向到登录页面），登录后会成功进入页面
2. 如果 cookie 与 session 中的 token 一致，无需登录，但还是会被重定向到登录页面（wtf?）
3. 情况 2 检查 `self.get_response(request)` 结果，status_code 是 200 而非重定向的 302

从现象来看，登录态合法的情况下，重定向的原因不是出在 `CookieLoginMiddleware` 这里。

那就继续 Debug，一个一个看，看看在哪个中间件执行后，response status_code 变成 302!

![img](/static/image/blog/which_middleware_cause_302.png)

在一顿鸡飞狗跳之后，凶手出现了：`SiteAccessControlMiddleware`，一个用于保护后台管理服务安全的中间件，它是这么滴干活：

```python
class SiteAccessControlMiddleware(MiddlewareMixin):
    """Control who can visit which paths in macro way"""

    def process_request(self, request):
        if request.path_info.startswith("/admin42/"):
            if request.user.is_anonymous or not request.user.is_authenticated:
                # 用户验证失败，重定向到登录页面
                return HttpResponseRedirect(...)

            ...
        return None
```

接着 Debug 可以发现，在 Django 3.2 与 4.2 中，当 cookies 与 session 中的 token 一致时，`request.user.is_anonymous` 居然是不一样的，也就是说，我的凭证是对的，但是没有登录上！

![img](/static/image/blog/dj_32_valid_user.png)

![img](/static/image/blog/dj_42_anonymous_user.png)

没有登录上，那就大概应该也许只能是用户认证的时候出问题了（Django 吃我一拳！）

### Q2：为什么用户认证会出问题？

回到 Django 用户认证部分看看，有这么一段逻辑：

```python
# django.contrib.auth.middleware.get_user
def get_user(request):
    if not hasattr(request, '_cached_user'):
        request._cached_user = auth.get_user(request)
    return request._cached_user


# django.contrib.auth.middleware.AuthenticationMiddleware
class AuthenticationMiddleware(MiddlewareMixin):
    def process_request(self, request):
        ...
        request.user = SimpleLazyObject(lambda: get_user(request))
```

在 `AuthenticationMiddleware` 中会完成对 `request.user` 的赋值，但其实是个 lazy 对象，具体初始化还是得在后面取值时候才调用的 `get_user()` 获取，具体实现如下：

```python
# django.contrib.auth.get_user
def get_user(request):
    """
    Return the user model instance associated with the given request session.
    If no user is retrieved, return an instance of `AnonymousUser`.
    """
    from .models import AnonymousUser
    user = None
    try:
        user_id = _get_user_session_key(request)
        backend_path = request.session[BACKEND_SESSION_KEY]
    except KeyError:
        pass
    else:
        if backend_path in settings.AUTHENTICATION_BACKENDS:
            # 通过 backend.get_user() 获取目前登录的用户
            backend = load_backend(backend_path)
            user = backend.get_user(user_id)
            ...

    return user or AnonymousUser()
```

从代码可以发现，`auth.get_user()` 实际上还是调用的 `backend.get_user()`，而目前看用的 backend 是 `bkpaas_auth.backends.UniversalAuthBackend`，那就得找为什么在不同 Django 版本中，`UniversalAuthBackend.get_user()` 表现不一致。

```python
# bkpaas_auth.backends.UniversalAuthBackend
class UniversalAuthBackend:
    """An universal cookie auth backend.

    This backend is to be used in conjunction with the ``CookieLoginMiddleware``
    found in the middleware module of this package.
    """

    request: HttpRequest

    def __init__(self):
        ...

    def authenticate(self, request: HttpRequest, auth_credentials: Dict) -> Optional[Union[User, AnonymousUser]]:
        ...

    def get_user(self, user_id):
        """Get user from current session"""
        # 这里有段逻辑，对 reqeust 是否为空做检查
        if not hasattr(self, "request"):
            return None

        token = self.get_token_from_session(self.request)
        if token:
            return create_user_from_token(token)
        return None
```

翻 `UniversalAuthBackend.get_user()` 源码看下，发现有个 `request` 属性，在 `get_user` 时候会提前判断，如果为空，就直接返回 `None`，也就是会导致 `request.user` 是 `AnonymousUser`。

这里有个很奇怪的点，就是 Django 3.2 / 4.2 里面，一个 `request` 是有值的，但是一个没值（`None`），为什么会有这个情况？

而且更奇怪的是：request 是怎么来的，看 `__init__` 中也没有对 `self.request` 做初始化？

这块困扰了许久，没办法，只能看堆栈，看看从哪里调用 / 初始化的 Backend，仔细看下，确实有收获：

![img](/static/image/blog/dj_32_monkey_patch.png)

在 Django 3.2 中，出现了 `monkey.py` 字样（蕉蕉？蕉蕉！），居然是用 monkey patch（函数代码替换）的方式注入 `backend.request` 么（这谁一开始就能想到呢 OuO）

![img](/static/image/blog/dj_42_no_patch.png)

对比发现 Django 4.2 中就是老老实实走的上面提到的 `django.contrib.auth.middleware.get_user`，那么，为什么 Django 4.2 中 monkey patch 没有生效呢？

### Q3：为什么 monkey patch 没有生效？

按照 `bkpaas_auth` 的 monkey patch 方法 `patch_middleware_get_user` 名称，可以快速找到在 `infras/accounts/apps.py` 的 `AppsConfig.ready()` 中做了 patch：

```python
# paasng.infras.accounts.apps
from paasng.utils.addons import PlugableAppConfig


class AppsConfig(PlugableAppConfig):
    name = "paasng.infras.accounts"

    def ready(self):
        super().ready()
        # Patch get_user function when project/app is ready to make auth system works
        from bkpaas_auth.monkey import patch_middleware_get_user

        patch_middleware_get_user()
```

在 Django 中目前主流的 patch 有三个地方：

- 在 settings 中进行 patch，项目启动时候是先读取配置的，像 `pymysql.install_as_MySQLdb()` 这样的 patch 就会在这里注入
- 在 `AppConfig.ready()` 中进行 patch，这里主要是考虑到一些 patch 需要依赖 App 初始化（models 注册等）
- 在单元测试中，通过 mock 工具进行动态的替换，如 `@mock.patch("paasng.accessories.log.views.logs.instantiate_log_client") as client_factory`

这里是第二种场景，由于用户认证需要依赖 DB 中的 User model，因此只能在 `ready()` 中进行 patch，那么为什么 patch 没有生效呢？

难道是在 `ready()` 中进行 patch 不再被支持？比如说在执行 `ready()` 之前，middleware 已经被 import & use，因此 patch 了个寂寞？

但是这种调整 Django 启动时加载的顺序是不太可能的，这是一个很大的变动，肯定得在 release notes 中有所体现，翻了下没有，自己翻代码看下 middleware 是否提前使用过也不是很现实...

那就继续掏出 Pycharm 进行 debug，通过打断点可以发现：

在 Django 3.2 中，`ready()` 中的 patch 有被正常执行

![img](/static/image/blog/dj_32_exec_custom_ready.png)

但是在 Django 4.2 中，它咻地一下跳了过去，执行的是基类（AppConfig）中的 `ready()` WTF?

![img](/static/image/blog/dj_42_exec_base_ready.png)

那么，问题就指向了为什么自定义的 `AppsConfig.ready()` 没有被执行？

> 注：
>
> 1. `a = 1 + 1` 是为了便于打断点加的逻辑，不是 Django 框架的代码，不然 `AppConfig.ready()` 没有代码，丝滑得停不下来
> 2. 其实如果细心一点，可以发现 dj32 里面是 `AppsConfig`，dj42 里面是 `AppConfig`，一个是自定义类的实例，一个是基类的实例（哦，这可恶的命名！）

### Q4：为什么自定义 `AppConfig.ready()` 没有执行？

简化 `paasng.infras.account.apps` 中的 AppsConfig 的定义如下：

```python
# paasng.utils.addons
from django.apps import AppConfig


class PlugableAppConfig(AppConfig):

    def ready(self):
        # do something...
```

```python
# paasng.infras.accounts.apps
from paasng.utils.addons import PlugableAppConfig


class CustomAppConfig(PlugableAppConfig):
    name = "apps.demo"

    def ready(self):
        super().ready()
        # do something...
```

Python 是支持类继承的，然后在子类中写同名的函数，是可以替换掉父类的实现的，如果还需要调用父类的实现，那就写个 `super().xxx()`。

这应该是个比较基础的知识，但是为啥这里会不生效？~~(天塌了，Python 学不存在了！)~~

仔细 Debug 后发现，不太对啊，为啥在 Django 3.2 中，是 `AppsConfig`，而到了 Django 4.2 中变成了 `AppConfig`，这不就是基类嘛，可不就得执行基类的 `ready()`！~~（好耶！Python 学回来了！）~~

### Q5：为什么实例化 AppConfig 用的是 Base AppConfig 而不是 Custom AppConfig？

这问题就变得简单了，直接扒代码，看看 Django 在哪里做的 AppConfig 初始化：

```python
class AppConfig:
    """Class representing a Django application and its configuration."""
    ...

    @classmethod
    def create(cls, entry):
        """Factory that creates an app config from an entry in INSTALLED_APPS."""
        # create() eventually returns app_config_class(app_name, app_module).
        app_config_class = None
        app_name = None
        app_module = None

        # If import_module succeeds, entry points to the app module.
        try:
            app_module = import_module(entry)
        except Exception:
            pass
        else:
            # If app_module has an apps submodule that defines a single
            # AppConfig subclass, use it automatically.
            # To prevent this, an AppConfig subclass can declare a class
            # variable default = False.
            # If the apps module defines more than one AppConfig subclass,
            # the default one can declare default = True.
            if module_has_submodule(app_module, APPS_MODULE_NAME):
                mod_path = "%s.%s" % (entry, APPS_MODULE_NAME)
                mod = import_module(mod_path)
                # Check if there's exactly one AppConfig candidate,
                # excluding those that explicitly define default = False.
                app_configs = [
                    (name, candidate)
                    for name, candidate in inspect.getmembers(mod, inspect.isclass)
                    if (
                        issubclass(candidate, cls)
                        and candidate is not cls
                        and getattr(candidate, "default", True)
                    )
                ]
                if len(app_configs) == 1:
                    app_config_class = app_configs[0][1]
                else:
                    # Check if there's exactly one AppConfig subclass,
                    # among those that explicitly define default = True.
                    app_configs = [
                        (name, candidate)
                        for name, candidate in app_configs
                        if getattr(candidate, "default", False)
                    ]
                    if len(app_configs) > 1:
                        candidates = [repr(name) for name, _ in app_configs]
                        raise RuntimeError(...)
                    elif len(app_configs) == 1:
                        app_config_class = app_configs[0][1]

            # Use the default app config class if we didn't find anything.
            if app_config_class is None:
                app_config_class = cls
                app_name = entry

        # If import_string succeeds, entry is an app config class.
        if app_config_class is None:
            ...
```

看 Django 4.2 源码可以发现，在较高版本中（[3.2+](https://github.com/django/django/commit/3f2821af6bc48fa8e7970c1ce27bc54c3172545e)）不需要显示在 `__init__.py` 中指定 `default_app_config`，Django 自己会自动探测 `apps.py` 中的所有 AppConfig 的子类，并挑选合适的作为 AppConfig，如果无法选择，则抛出异常或使用默认的 Base AppConfig。

那么，为啥 Django 从 3.2 升级到 4.2 后，这个逻辑就崩了呢？还是老办法，看版本代码 diff，即可发现在 Django 4.1 中，对 `default_app_config` 配置的支持被[完全移除](https://github.com/django/django/commit/75d6c4ae6df93c4c4d8621aced3a180afa18a6cb)，这就是问题所在。

按理说，绝大部分项目在 `default_app_config` 的支持被移除后，应该是没有问题的，因为自动探测还是生效的。

<br/>

自动探测的逻辑是：

- 对于某个 APP，检查所有的 AppConfig 的子类放在列表中
- 对于该列表，过滤掉 **显式** 指定 `AppConfig.default = False` 的类，如果唯一，则使用
- 如果上一步不唯一，则寻找 **显式** 指定 `AppConfig.default = True` 的类，如果唯一，则使用，不唯一则抛异常
- 如果还是不唯一，则使用默认的 Base AppConfig

<br/>

这就容易看出问题来了，如果 `apps.py` 中存在多个 `AppConfig` 的子类（import 的也算），那就很容易出问题，因为历史代码是不可能有指定 `default` 属性的值的，即在 `default_app_config` 失效后，如果有多个 `AppConfig` 的子类，就有非常大的概率直接使用到 `Base AppConfig`，且没有任何的 warning（坑爹呢这是 o.O）

## 一把抓住，即刻修复

翻下 Django 关于 AppConfig 的文档，确实有提及[这一点](https://docs.djangoproject.com/en/4.2/ref/applications/#configuring-applications)，不过对于升级版本的开发者来说，如果 release notes 中没有提及，就只能等出问题再慢慢排查了，这点不是很友好。

> To configure an application, create an **apps.py** module inside the application, then define a subclass of AppConfig there.
>
> When **INSTALLED_APPS** contains the dotted path to an application module, by default, if Django finds exactly one AppConfig subclass in the **apps.py** submodule, it uses that configuration for the application. This behavior may be disabled by setting **AppConfig.default** to **False**.
>
> If the apps.py module contains more than one AppConfig subclass, Django will look for a single one where **AppConfig.default** is **True**.
>
> If no AppConfig subclass is found, the base **AppConfig** class will be used.

问题解决起来也简单，显式标记 `PlugableAppConfig.default = False` 或标记 `CustomAppConfig.default = True` 都是可以的，当然，两者都要就更加稳妥咯 :D

```python
# paasng.utils.addons
from django.apps import AppConfig

class PlugableAppConfig(AppConfig):
    default = False  # 设置非默认

    def ready(self):
        # do something...
```

```python
from paasng.utils.addons import PlugableAppConfig

# paasng.infras.accounts.apps
class CustomAppConfig(PlugableAppConfig):
    name = "apps.demo"
    default = True  # 设置为默认

    def ready(self):
        super().ready()
        # do something...
```

## 给 Django 提提建议？

通过一套组合拳下来，问题总算是找到并解决了，虽然有所收获，但还是消耗了我不少的时间精力，如果 Django 能更好地给提示，比如说无法在多个子类中做出选择而使用 `Base AppConfig` 时，能否来个 warning？不要悄咪咪地搞事情，这样不好 :）

那么，[Ticket](https://code.djangoproject.com/ticket/35869) & [PR](https://github.com/django/django/pull/18727) 走起（Django 使用自己的网站而非 Github 来管理 Issue），看回复大概率还是会被合并的 hhh

## 结束语

写博客的时候想到，这次的问题出现，和瑞士奶酪效应 & 多米诺骨牌效应有点关系。

![img](/static/image/blog/swiss_cheese.png)

Django 的 `AppConfig` 探测机制其实是相当完善的，考虑到了大部分的情况，但是在 历史代码没有指定 `default` + 多个 `AppConfig` 子类 的特殊场景的处理还是不够合理，而 Django 4.1 中移除对 `default_app_config` 的支持导致了最后一块能拦截危险的 Swiss 被撤掉，因此引发了上面这个 Bug，新增的 PR 其实也可以算是再加一片 Swiss，通过显式的警告，提醒开发者关注这个可能的风险点。

![img](/static/image/blog/dominoes.png)

多米诺骨牌效应也比较好理解：`AppConfig` 失效导致 monkey patch 失效，patch 失效导致用户认证失败，认证失败导致登录无限重定向...

不过需要注意的是，多米诺骨牌还有一个特点：存在分支，即登录无限重定向只是分支之一，`AppConfig` 失效还有一个问题是会导致 `import handlers` 失效，继而影响 Django 信号的处理，但是这个现象没有那么明显，可能更难被发现（除非有做充足的测试）。

![img](/static/image/blog/dominoes_safe_point.png)

在多米诺游戏中，有一种称为安全点的做法，即在所有骨牌放置完成之前，每间隔一段放置一个短一些的骨牌（或抽走一块）如果游戏被未预期地启动，可以减少损失。

编程其实也是个多米诺游戏，我们也可以在代码中设置类似的安全点，拿上面这个例子来说：

1. 我们能否在 `UniversalAuthBackend.get_user()` 中发现 `request` 为 None 时，不是直接返回，而是抛出异常 / 打印 warning 日志？
2. 如果代码中有使用 monkey patch，是否应该显式打印出日志？（可以在版本对比时，通过日志看出点蛛丝马迹，而非走 Debug 看堆栈大法）

以上是本次 Debug 的记录 & 一些思考，总的来说，还是蛮有意思的一次经历～
