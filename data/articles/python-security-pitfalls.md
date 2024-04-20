## Assert

Python 支持以优化的方式执行代码（如 .pyo），这能使代码运行得更快，内存用得更少；当程序被大规模使用，或者可用的资源很少（如嵌入式应用）时，这种方法尤其有效。

然而，当代码被优化时，程序中的 `assert` 语句都会被忽略，如果使用 `assert` 来作权限检查，则可能会出现越权的情况。

比如下面的例子，直接运行时候是能正常生效的，能够拦截非超级用户的访问，但是在优化后该拦截将会失效，尽管早已不推荐使用 `assert` 语句来做安全相关的检查，但是在某些古早项目中还是偶有出现。

```python
def superuser_action(request, user):
    assert user.is_superuser
    # execute action as super user
```

## MakeDirs

`os.makedirs` 函数可用于创建一个或多个文件夹。它的第二个参数 mode 用于指定创建的文件夹的默认权限。在下面代码的第 2 行中，文件夹 A/B/C 是用 rwx (0o700) 权限创建的。这意味着只有当前用户（所有者）拥有这些文件夹的读、写和执行权限。

```python
def init_directories(request):
    os.makedirs("A/B/C", mode=0o700)
    return HttpResponse("Done!")
```

在 Python < 3.7 版本中，以上代码创建出的文件夹 A、B 和 C 的权限都是 700。但是，在 Python >= 3.7 版本的 [更新]((https://docs.python.org/3/whatsnew/3.7.html#os)) 中，只有最后一个文件夹 C 的权限为 700，其它文件夹 A 和 B 的权限为默认的 755。

因此，在新版本的 Python 中，`os.makedirs` 函数等价于 Linux 的这条命令：`mkdir -m 700 -p A/B/C`。有些开发者没有意识到版本之间的差异，这已经在 Django 中造成了一个权限越级漏洞 [CVE-2020-24583](https://nvd.nist.gov/vuln/detail/CVE-2020-24583)。

## os.path.join

`os.path.join(path, *paths)` 函数用于将多个文件路径连接成一个组合的路径。第一个参数通常包含了基础路径，而之后的每个参数都被当做组件拼接到基础路径后。

```python
import os

username = "anonymous"

def read_file(filename):
    filepath = os.path.join("data", "workspace", username, filename)
    return "read file content: " + filepath

print(read_file("../../../etc/passwd"))
```

然而，我们可以通过在路径中加入 `../xxx` 的方式，访问不属于指定基础目录下的文件，**GateOne (SSH client)** 即存在该问题，目前该 [issue](https://github.com/liftoff/GateOne/issues/747) 还没有被修复。那么在加入以下检查之后，是否还有其他的安全风险呢？

```python
import os

username = "anonymous"

def read_file(filename):
    filepath = os.path.join("data", "workspace", username, filename)
    
    # ensure path no include "."
    if filepath.find(".") != -1:
        return "Failed!"

    return "read file content: " + filepath

print(read_file("/etc/passwd"))
```

该函数有一个少有人知的特性。如果拼接的某个路径以 / 开头，那么包括基础路径在内的所有前缀路径都将被删除，该路径将被视为绝对路径。官方文档中说明了这一行为：

> If a component is an absolute path, all previous components are thrown away and joining continues from the absolute path component.

也就是说，只要攻击者传入绝对路径，便可以访问任意的系统文件，比如 `/etc/passwd`。

那么，采用以下加固的方式，是否还有其他的安全风险呢？

```python
import os

username = "anonymous"

def read_file(filename):
    filepath = os.path.join("data", "workspace", username, filename)
    
    # ensure path no include "." and filename start with "/"
    if filepath.find(".") != -1 or filename.startswith("/"):
        return "Failed!"

    return "read file content: " + filepath

username = "/etc"
print(read_file("passwd"))
```

这里有一个容易忽视的变量: `username`，如果是完善的框架 / 用户管理系统，则能够确保用户名都是合规的字符串；但如果出现如创建用户时候没有限制用户名，或者使用可编辑的 nickname 作为路径组件的疏忽，那么就会给攻击者可乘之机。例子如 `username == /etc filename == passwd`

## re.match vs re.search

```python
import re

def check_sql_injection(test_str):
    pattern = re.compile(r".*(union)|(select).*")

    if re.match(pattern, test_str):
        return "re.match get result"

    if re.search(pattern, test_str):
        return "re.search get result"

print(check_sql_injection("test union"))

print(check_sql_injection("test\n union"))
```

`re.match` 与 `re.search` 的异同之处：match 不会匹配新行，而 search 则会匹配整个字符串

[更多区别](https://docs.python.org/3/library/re.html#search-vs-match)

## 任意的临时文件

`tempfile.NamedTemporaryFile` 函数用于创建具有特定名称的临时文件。但是，prefix（前缀）和 suffix（后缀）参数很容易受到路径遍历攻击。如果攻击者控制了这些参数之一，他就可以在文件系统中的任意位置创建出一个临时文件。下面的示例揭示了开发者可能遇到的一个陷阱。

```python
def touch_tmp_file(prefix):
    tmp_file = tempfile.NamedTemporaryFile(prefix=id)
    return "tmp file: {tmp_file} created!"
```

在第 2 行中，用户输入的 id 被当作临时文件的前缀。如果攻击者传入的 id 参数是 `/../var/www/test`，则会创建出这样的临时文件：`/var/www/test_zdllj17`（前提是基础路径必须是存在的）；粗看起来，这可能是无害的，但它会为攻击者创造出挖掘更复杂的漏洞的基础。

## Unicode 编码碰撞

Unicode 字符会被映射成码点。由于 Unicode 尝试将多种人类语言统一起来，导致不同的字符很有可能拥有相同的 `layout`。例如，小写的土耳其语 ı（没有点）的字符是英语中大写的 I。在拉丁字母中，字符 i 也是用大写的 I 表示。在 Unicode 标准中，这两个不同的字符都以大写形式映射到同一个码点。

这种行为是可以被利用的，实际上已经在 Django 中导致了一个严重的漏洞 [CVE-2019-19844](https://nvd.nist.gov/vuln/detail/CVE-2019-19844)。下面的代码是一个重置密码的示例。

```python
from django.core.mail import send_mail
from django.http import HttpResponse
from vuln.models import User

def reset_pw(request):
    email = request.GET['email']
    result = User.objects.filter(email=email.upper()).first()
    if not result:
        return HttpResponse("User not found!")

    send_mail('Reset Password','Your new pw: 123456.', 'from@example.com', [email], fail_silently=False)
    return HttpResponse("Password reset email send!")
```

第 6 行代码获取了用户输入的 email，第 7-9 行代码检查这个 email 值，查找是否存在具有该 email 的用户。如果用户存在，则第 10 行代码依据第 6 行中输入的 email 地址，给用户发送邮件。需要指出的是，第 7-9 行中对邮件地址的检查是不区分大小写的，使用了 `upper` 函数。

至于攻击，我们假设数据库中存在一个邮箱地址为 `foo@mix.com` 的用户。那么，攻击者可以简单地传入 `foo@mıx.com` 作为第 6 行中的 email，其中 i 被替换为土耳其语 ı。第 7 行代码将邮箱转换成大写，结果是 `FOO@MIX.COM`。这意味着找到了一个用户，因此会发送一封重置密码的邮件。

然而，邮件被发送到第 6 行未转换的邮件地址，也就是包含了土耳其语的 ı。换句话说，其他用户的密码被发送到了攻击者控制的邮件地址。为了防止这个漏洞，可以在调用 `send_mail` 的时候替换成使用数据库中的用户邮箱。即使发生编码冲突，攻击者在这种情况下也得不到任何好处（但原邮箱主人会受到重置密码的邮件）。

## URL 查询参数解析

在 Python < 3.7.10 的版本中，`;`, `&` 均可被视为 URL 参数中的分隔符，从而被 `urllib.parse.parse_qs()` 和 `urllib.parse.parse_qsl()` 顺利解析出来，但这其实是不符合 W3C 规范的。

假设这么一个例子，前端是 PHP 程序，而后台是 Python 程序，攻击者像前端发送以下的 GET 请求：

`GET https://victim.com/?a=1;b=2`

PHP 前端只识别出一个查询参数“a”，其内容为“1;b=2”，校验通过然后将其直接转发给内部的 Python 程序:

`GET https://internal.backend/?a=1;b=2`

这时候，Python 程序会识别出两个参数，即 `a=1 && b=2`，这种查询参数解析的差异可能会导致致命的安全漏洞，比如 Django 中的 Web 缓存投毒漏洞[CVE-2021-23336](https://nvd.nist.gov/vuln/detail/CVE-2021-23336)。以下是攻击的例子：

XSS 攻击尝试：
`GET /?link=http://google.com&utm_content=1;link='><t>alert(1)</script>`

尝试绕过白名单：
`GET http://xxx.com/download_and_exec/?link=http://ok.com/1.script$extra=somthing;link=http://danger.com/1.script`

这个 [Bug](https://bugs.python.org/issue42967) 已在 3.7.10 版本中被 [修复](https://docs.python.org/3/whatsnew/3.7.html#notable-changes-in-python-3-7-10)。

## eval

eval 是 Python 中一个强大但也危险的函数，可用于解析字符串内容并执行，但是其执行能力也可以被这样使用

`eval("__import__('os').system('clear')", {})`

想象一下，如果执行的命令是 `rm -rf /` 或者 `:(){:|:&};:` 会是很可怕的后果。

在 python 中，有一个 eval 的替代品：`ast.literal_eval()`，可基于语法分析，将字符串解析为标准 Python 结构，若不是标准结构，则会抛出异常。

如果确实有解析 Python 字符串的需求，可以使用 [该方法](https://docs.python.org/3/library/ast.html#ast.literal_eval)。

## 参考资料

1. [10 Unknown Security Pitfalls for Python](https://blog.sonarsource.com/10-unknown-security-pitfalls-for-python)
2. [Django security releases issued: 3.0.1, 2.2.9, and 1.11.27](https://www.djangoproject.com/weblog/2019/dec/18/security-releases/)
3. [python bug issue42967](https://bugs.python.org/issue42967)
4. [Using python's eval() vs. ast.literal_eval()](https://stackoverflow.com/questions/15197673/using-pythons-eval-vs-ast-literal-eval)
