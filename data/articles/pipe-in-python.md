## 什么是 Pipe

Pipe 在这里指 Linux 中的管道概念，常用示例如下：

```shell
>>> tail -n 100 test.log | grep -C 5 FLAG
```

该示例中的 `|` 即为管道命令的界定符号（管道符），表示将前一个命令执行结果 `stdout` 作为下一个命令的输入 `stdin`，而 `stderr` 则会直接输出到终端，示意图如下：

![img](/static/image/blog/pipe_flow.png)

## 什么是过滤器

我们可通过管道符号将多个命令组合形成一个管道。通过这种方式使用的命令被称为过滤器。过滤器会获取输入，通过某种方式修改其内容，然后将其输出。

常用的被作为过滤器使用的命令如下所示：

| 命令    | 说明                                                |
|-------|---------------------------------------------------|
| awk   | 用于文本处理的解释性程序设计语言，通常被作为数据提取和报告的工具           |
| sed   | 用于过滤和转换文本的流编辑器                                    |
| cut   | 用于将每个输入文件（或标准输入）的每行的指定部分输出到标准输出                   |
| grep  | 用于搜索一个或多个文件中匹配指定模式的行                              |
| head  | 用于读取文件的开头部分（默认是 10 行）如果没有指定文件，则从标准输入读取            |
| tail  | 用于显示文件的结尾部分                                       |
| xargs | 将标准输出内容作为命令参数，适用于不支持管道的命令(如 ls)，需要保证每个输出不包含空格     |
| sort  | 用于对文本文件的行进行排序                                     |
| tr    | 用于转换或删除字符                                         |
| uniq  | 用于报告或忽略重复的行                                       |
| wc    | 用于打印文件中的总行数、单词数或字节数                               |

## 常用示例

```shell
# awk
# 1.获取所有包含某个前缀的 Deployment
kubectl get deployment -n default | awk '/deployment-test/{print $1}'
# 2.使用条件表达式过滤
kubectl get configmap | awk '{if ($1 != "cmap-test" && $1 != "kube-root-ca.crt") print $1}'
# 3.列出当前账号最常使用的 10 个命令
history | awk '{print $2}' | sort | uniq -c | sort -rn | head

# cut
# 1.获取当前目录下所有子目录的数量
# - 命令 ls -l 的输出中每行的首位字符表示文件的类型，若为d，表示类型是目录
# - 命令 cut -c 1 是截取每行的第一个字符
# - 命令 grep d 来获取文件类型是目录的行
# - 命令 wc -l 用来获得 grep 命令输出结果的行数，即目录个数
ls -l | cut -c 1 | grep d | wc -l

# xargs
# 1.寻找某目录下所有 python 类型文件，并列出文件信息
find ./dirname -name '*.py' | xargs ls -l
# 2.单行展示某文件内容
cat filename | xargs

# tr
# 1.展示某文件内容（所有小写字母以大写形式展示）
cat filename |tr a-z A-Z
```

## Python pipes

### 官方说明

> **The pipes module defines a class to abstract the concept of a pipeline — a sequence of converters from one file to another.**
> Because the module uses /bin/sh command lines, a POSIX or compatible shell for os.system() and os.popen() is required.

### 代码分析

```python
class Template:
    """ Unix Pipeline 抽象类 """

    def reset(self) -> None:
        """ 清空所有已经设置好的步骤 """

    def clone(self) -> Template:
        """ 深复制当前 Template 对象 """

    def debug(self, flag: bool) -> None:
        """ 设置是否使用 DEBUG 模式，若为 True 则执行时候会打印待执行命令 """

    def append(self, cmd: str, kind: str) -> None:
        """
        在 Pipeline 末尾增加一个节点

        :param cmd: 待执行指令，如 tr a-z A-Z
        :param kind: 两个字符长度的字符串，分别代表输入输出的方式，可选值有：
          1.'-' 读取其标准输入 / 写入到标准输出
          2.'f' 读取在命令行中给定的文件 / 写入在命令行中给定的文件
          3.'.' 不读取输入(必须是首个命令) / 不执行写入（必须是最后一个命令）
        """

    def prepend(self, cmd: str, kind: str) -> None:
        """ 参数同 append，添加一个节点在首位 """

    def open(self, file: str, rw: Union['r', 'w']) -> None:
        """ 关联一个文件，可选择 读/写(r/w) 模式 """

    def copy(self, infile: str, outfile: str) -> str:
        """ 通过管道将 infile 复制成 outfile """
```

### 示例

```python
import pipes
t = pipes.Template()

# 小写转大写命令
t.append('tr a-z A-Z', '--')

# 按行排序命令
t.append('sort', '--')

# 关联文件并写入，会自动执行各个命令
f = t.open('pipefile', 'w')
f.write('1\n4\n3\n2\nDCBA\nabcd\n')
f.close()

# <<< 18

# 读取验证写入结果
content = open('pipefile', 'r').read()
print(content)

# <<< 1
#     2
#     3
#     4
#     ABCD
#     DCBA
```

## Python pipe

管道的处理非常的清晰，因为它使用的是中缀语法，而我们使用的 Python 是前缀语法，在管道中 `ls | sort -r` 的实现（左中右），在 Python 中需要写成 `sort(ls(), reverse=True)`（中左右），那么，我们能不能在 Python 中实现管道这样的写法呢，答案是肯定的！

### 如何实现

管道符号在 Python 中其实就是`或`符号，`Julien Palard` 开发了一个 pipe 库，能够将 Python 的前缀语法转换成中缀语法，模拟管道的实现。Pipe 库核心代码如下，它简单地重载了 `__ror__` 方法

#### 核心逻辑

```python
class Pipe:
    def __init__(self, function):
        self.function = function

    def __ror__(self, other):
        return self.function(other)

    def __call__(self, *args, **kwargs):
        return Pipe(lambda x: self.function(x, *args, **kwargs))
```

#### Magic Method \_\_ror\_\_

```python
x.__or__(y) <==> x|y

x.__ior__(y) <==> x |= y

x.__ror__(y) <==> y|x

x.__xor__(y) <==> x^y
```

### pipe 示例

这个 Pipe 类可以当成函数的装饰器来使用

```python
@Pipe
def where(iterable, predicate):
    return (x for x in iterable if (predicate(x)))
```

pipe 库内置了一堆这样的处理函数，比如 `sum`, `select`, `where` 等

[更多内置函数](https://github.com/JulienPalard/Pipe#existing-pipes-in-this-module)

```python
from pipe import take_while, where, select

# 找出小于 10^6 的斐波那契数，并计算其中的偶数平方和
ret = fib() | take_while(lambda x: x < 10**6) \
      | where(lambda x: x % 2) \
      | select(lambda x: x * x)

sum(ret)
```

需要注意的是，pipe 是惰性求值的，因此可以实现一个无穷生成器而不用当心内存被用完。

除了处理数值很方便，pipe 同样可以用于处理文本，比如读取文件，统计文件中每个单词的出现次数，按从高到低的顺序排序：

```python
from re import split
from pipe import Pipe, groupby, select, count, sort
with open('test_doc.txt') as f:
    ret = f.read() | Pipe(lambda x: split(r'\s+', x)) \
        | groupby(lambda x: x) \
        | select(lambda x: (x[0], (x[1] | count))) \
        | sort(key=lambda x: x[1], reverse=True)

print(ret)
```

[更多示例参考](https://github.com/JulienPalard/Pipe#euler-project-samples)

### 总结

我们在 Python 中能够通过操作符的重载来实现 pipe 类似的中缀语法，但是这是一种对特性的滥用，不提倡在正式项目中使用，但是日常写小脚本中用用还是很酷的。

## 参考资料

1. [Linux 命令大全](https://www.runoob.com/linux/linux-command-manual.html)
2. [Shell 过滤器](http://c.biancheng.net/view/3472.html)
3. [Python pipes 文档](https://docs.python.org/3/library/pipes.html)
4. [Python pipes 源码地址](https://github.com/python/cpython/blob/3.9/Lib/pipes.py)
5. [awk 示例](http://qinghua.github.io/awk/)
6. [Python \_\_ror\_\_](https://docs.python.org/3/reference/datamodel.html?highlight=__ror__#object.__ror__)
7. [Github Pipe](https://github.com/JulienPalard/Pipe)
