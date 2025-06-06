## 各式各样的 Python 字符串

| 形式  | 名称          | 特性                  | 示例                |
|-----|-------------|---------------------|-------------------|
| f"" | 格式化字符串      | 允许嵌入表达式             | f"Value: {x}"     |
| r"" | 原始字符串       | 禁用转义字符              | r"C:\Users\file"  |
| b"" | 字节串         | 字节类型数据              | b"binary\x00data" |
| u"" | Unicode 字符串 | Python 2 中用于支持多语言字符 | u"hello world"    |
| ""  | 字符串         | 很普通的字符串             | "hello world"     |

## t-strings

在 Python 3.14 中，Python 字符串家族将迎来一位新成员：`t-strings` [模板字符串（PEP-750）](https://peps.python.org/pep-0750/)。

按照官方说法，其针对 `f-strings` 设计上的一些局限之处做出来改进，据称能够提升 Python 在字符串处理领域的安全性和灵活性。

### 设计初衷

自 Python 3.6 通过 [PEP-498](https://peps.python.org/pep-0498/) & [PEP-701](https://peps.python.org/pep-0701/) 引入 f-strings 以来，尽管其简洁高效的特性广受好评，但其实现本身仍存在显著局限：

- 注入攻击风险：直接拼接用户输入可能导致 SQL 注入或 XSS 攻击

```python
# 不安全的 f-string 示例

# SQL 注入
sql_query = f"SELECT * FROM users WHERE name = '{user_input}'"
# XSS 攻击
html_content = f"<div>{user_input}</div>"  
```

- 缺乏预处理机制：`f-strings` 在定义时会立即求值，无法在生成字符串前对插值内容进行转义或验证。

```python
>>> name = "Mike"
>>> greeting = f"hello, my name is {name}"

>>> greeting
'hello, my name is Mike'

>>> type(greeting)
# 无法再对 name 做处理
<class 'str'>
```

`t-strings` 的核心目标是延迟渲染，通过结构化保留插值上下文，为安全处理提供操作空间。

### 核心机制

`t-strings` 使用前缀 `t` 定义，返回 `Template` 对象（来自模块 `string.templatelib`），而非直接生成字符串：

```python
>>> name = "world"
>>> tmpl = t"hello {name}!"
>>> type(tmpl)
<class 'string.templatelib.Template'>
```

`Template` 对象包含了静态文本与动态插值（Interpolation），支持开发者对其进行精细操作：

```python
>>> tmpl.strings
('hello ', '!')
>>> tmpl.values
('world',)
>>> tmpl.interpolations[0]
# value, expression, conversion, format_spec
Interpolation('world', 'name', None, '')  

# 遍历模板对象时，每个项可能是 str 也可能是 Interpolation 对象
>>> for it in tmpl:
...     print(type(it), it)
...     
<class 'str'> hello 
<class 'string.templatelib.Interpolation'> Interpolation('world', 'name', None, '')
<class 'str'> !
```

#### 类型转换标识符（conversion）

通过 `!` 指定的值转换方式，支持三种标准转换符：

| 符号 | 含义            | 等效函数         |
|----|---------------|--------------|
| s  | 字符串化          | str(value)   |
| r  | 安全转义（repr 格式） | repr(value)  |
| a  | ASCII 安全转义    | ascii(value) |

若未指定，则 conversion 为 `None`，即直接使用 value。

```python
# conversion 属性值为 'r'（启用 repr 转换）
template = t"Raw: {user_input!r}"
```

#### 格式化规则（format_spec）

`:` 后定义的格式化指令，语法与 `f-strings` 完全兼容。

常见用途：

- 数字精度：`{pi:.2f}` → 保留2位小数
- 文本对齐：`{title:^20}` → 居中且宽度20
- 填充字符：`{id:>10}` → 右对齐，宽度10，用 ` 填充

```python
# 右对齐，宽度8
template = t"Name: {name:>8}"
```

## t-strings 实战场景

### HTML 转义

```python
import html


def safe_html(template: Template) -> str:
    parts = []
    for item in template:
        if isinstance(item, str):
            parts.append(item)
        else:
            # 对用户的输入进行转义
            parts.append(html.escape(str(item.value)))

    return "".join(parts)

user_input = "<script>alert('XSS')</script>"
safe_output = safe_html(t"<div>{user_input}</div>")
# safe_output => <div>&lt;script&gt;alert(&#x27;XSS&#x27;)&lt;/script&gt;</div>
```

### SQL 参数化（防止注入攻击）

```python
def safe_sql(template: Template) -> tuple:
    query_parts, params = [], []
    for item in template:
        if isinstance(item, str):
            query_parts.append(item)
        else:
            query_parts.append("?")
            # 分离参数与查询结构
            params.append(item.value)

    return "".join(query_parts), params

user_id = "1'; DROP TABLE users;--"
query, params = safe_sql(t"SELECT * FROM users WHERE id = {user_id}")
# query => "SELECT * FROM users WHERE id = ?"
# params => ["1'; DROP TABLE users;--"]
```
### 多格式输出 文本 -> Json / XML

```python
import json

data = {"name": "Python", "version": 3.14}
template = t"Language: {data['name']}, Version: {data['version']}"


def to_json(tpl: Template) -> str:
    kv = {
        item.expression.split("'")[1]: item.value 
        for item in tpl.interpolations  
    }
    return json.dumps(kv)

print(to_json(template))  
# {"name": "Python", "version": 3.14}


def to_xml(tpl: Template) -> str:
    parts = ["<root>"]
    for item in tpl.interpolations:
        key = item.expression.split("'")[1]
        parts.append(f"<{key}>{item.value}</{key}>")

    parts.append("</root>")
    return "".join(parts)

print(to_xml(template))
# <root><name>Python</name><version>3.14</version></root>
```

### 使用 t-strings 实现 f-strings

```python
from typing import Any, Literal
from string.templatelib import Template, Interpolation

def convert(value: Any, conversion: Literal["a", "r", "s"] | None) -> Any:
    if conversion == "a":
        return ascii(value)

    if conversion == "r":
        return repr(value)

    if conversion == "s":
        return str(value)

    return value

def f(template: Template) -> str:
    parts = []
    for item in template:
        match item:
            case str() as s:
                parts.append(s)
            case Interpolation(value, _, conversion, format_spec):
                value = convert(value, conversion)
                value = format(value, format_spec)
                parts.append(value)

    return "".join(parts)


name = "World"
value = 42
templated = t"Hello {name!r}, value: {value:.2f}"
formatted = f"Hello {name!r}, value: {value:.2f}"
assert f(templated) == formatted
```

## t-strings 与 f-strings 对比

| 特性    | f-strings | t-strings          |
|-------|-----------|--------------------|
| 返回类型  | str       | Template 对象        |
| 渲染时机  | 立即求值      | 延迟渲染（需显式处理）        |
| 安全性   | 低（直接拼接）   | 高（可预转义）            |
| 结构化访问 | 不支持       | 支持（字符串/插值分离）       |
| 适用场景  | 简单拼接      | Web 模板、SQL、日志等安全场景 |


## 总结

`t-strings` 是 Python 对字符串模板化的一次尝试，主要目的是在保持与 `f-strings` 相似语法的前提下，通过结构化模板 + 延迟渲染的机制，提供更加安全灵活的字符串处理能力。

`t-strings` 并非想替代 `f-strings`，而是希望为高风险场景提供安全兜底能力，目标是让开发者能够在不牺牲简洁性的前提下，构建更健壮的应用。

`t-strings` 在安全防护，多态输出，延迟处理，异步渲染等方面会有更大的应用场景，但就日常的业务逻辑中，我个人认为还是不如 `f-strings` 那么实用 :P

## 参考资料

- [PEP 750 – Template Strings](https://peps.python.org/pep-0750/)
- [PEP 498 – Literal String Interpolation](https://peps.python.org/pep-0498/)
- [PEP 701 – Syntactic formalization of f-strings](https://peps.python.org/pep-0701/)
- [davepeck / pep750-examples](https://github.com/davepeck/pep750-examples/tree/main)
