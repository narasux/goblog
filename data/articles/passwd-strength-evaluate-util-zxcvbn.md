## 背景

蓝鲸用户管理中有这么一个需求：不同租户可以配置不同的密码规则：

![img](/static/image/blog//bkuser_password_rule.png)

## 需求分析

用户管理需要根据配置的密码规则，对用户的密码进行以下的校验：

- 密码长度校验 👌
- 必须包含字符集校验 👌
- 连续性序列检查 🤔
- 密码整体强度评估 😥

长度 和 字符集校验这两个是相对简单的，使用 Python 实现大概 [三十行的样子](https://github.com/TencentBlueKing/bk-user/blob/6236bda31f6e8553f2ff56507361ff8e4eaa6af0/src/bk-user/bkuser/common/passwd/validator.py#L51)。

而连续性检查这个就没那么简单，比如键盘序，字母序，数字序这种，如果手撸的话，就需要通过字符串匹配类的算法实现，费时费力还容易有 bug!

最后的密码强度检查就更加麻烦了，应该用什么标准来评估一个密码的强度呢？举个例子：`Password123123@` 用前三条规则来看，是很不错的，有大小写字母，也有数字和特殊符号，没有过多的连续序列，长度 15 位也挺足。但肉眼可见的，这就是一个很弱的密码，非常容易被破解...

## 寻找轮子

密码的强度检查其实是个很普遍的需求，业界应该有成熟的实现，这种场景下应该避免自己重复造轮子，而是学会主动找合适的轮子。

寻找 Python 依赖包可以参考这个路径：[PyPI](https://pypi.org/) -> [Github](https://github.com/) -> [Google](https://www.google.com/)，逐个用关键字搜索（如 password + strength）一般都会有所收获；当然了，现在 LLM 大模型非常方便，让 GPT 给一些参考建议，再自己筛选下也是不错的选择。

通过这种方式，我们可以快速找到 [dropbox/zxcvbn](https://github.com/dropbox/zxcvbn) 这个工具，它提供了高性能的密码强度评估能力，不过它并不是 Python 实现的，这种情况下可以看下它的文档，或者拿项目名字 + 语言搜索下，一般来说如果一个库足够通用，会有其他语言的开源实现，实在不行，参考开源项目实现一版也比从零开始手撸方便很多。

> 💠 传送门：参考 [Python dictdiffer](https://github.com/inveniosoftware/dictdiffer) 实现的 [Golang Mapx.Differ](https://github.com/TencentBlueKing/gopkg/tree/master/mapx#differ) :D

幸运的是，在 zxcvbn 的 Readme 中就提到这个工具的 Python 实现：[dwolfhub/zxcvbn-python](https://github.com/dwolfhub/zxcvbn-python)，那就来研究下它的工作原理吧！

## zxcvbn 是怎么工作的？

zxcvbn 是一个用于评估密码强度的密码强度估计库，其目的是帮助开发者和用户获得强大且实用的密码，而不仅仅是复杂的密码。

zxcvbn 不是简单地要求密码包含数字、大写字母、小写字母和特殊字符，它通过分析密码的模式和结构来更真实地预测密码的强度，它会通过多种模式对原始密码进行匹配，再根据匹配结果计算密码熵，再给出综合评分。

### 正/反序字典匹配

zxcvbn 内置了大量常用密码、常见单词、地名、人名等数据集，可用于检查密码（或密码的逆序）是否存在于这些字典中，用户在使用的时候，也可以额外补充自己的自定义字典，以满足一些需求（如：密码不能包含当前用户名）。

匹配举例：

- 如果密码如 `password` 出现在字典中，它将被视为较弱的密码。
- 如果密码如 `drowssap` 的逆序 `password` 出现在字典中，也会被认为是弱密码。

```python
>>> from zxcvbn import zxcvbn
>>> from pprint import pprint

# 密码字典
>>> results = zxcvbn("Password123123@", user_inputs=["schnee"])
>>> pprint(results)
{
    # 评估耗时：3.6 ms
    'calc_time': datetime.timedelta(microseconds=3602),
    # 破解耗时（展示用）
    'crack_times_display': {
        # 离线，快速哈希，每秒 10 亿次
        'offline_fast_hashing_1e10_per_second': 'less than a second',
        # 离线，慢哈希，每秒 1w 次
        'offline_slow_hashing_1e4_per_second': '40 minutes',
        # 在线，没有频率控制，每秒 10 次
        'online_no_throttling_10_per_second': '28 days',
        # 在线，有频率控制，每小时 100 次
        'online_throttling_100_per_hour': '27 years',
    },
    # 破解耗时（单位：秒）
    'crack_times_seconds': {
        'offline_fast_hashing_1e10_per_second': Decimal('0.002381'),
        'offline_slow_hashing_1e4_per_second': Decimal('2381'),
        'online_no_throttling_10_per_second': Decimal('2381000'),
        'online_throttling_100_per_hour': Decimal('857160000.0000000475819383894'),
    },
    # 密码评估反馈
    'feedback': {
        'suggestions': [
            'Add another word or two. Uncommon words are better.',
            "Capitalization doesn't help very much."
        ],
        'warning': 'This is similar to a commonly used password.',
    },
    # 猜测（破解）密码期望次数
    'guesses': Decimal('23810000'),
    'guesses_log10': 7.37675939540488,
    # 原始密码
    'password': 'Password123123@',
    # 评估得分（0-4 越高越强）
    'score': 2,
    # 分割序列
    'sequence': [
        {
            'token': 'Password123',
            # 匹配模式
            'pattern': 'dictionary',
            # 字典名（当匹配模式为 dictionary 时提供）
            'dictionary_name': 'passwords',
            'guesses': 1190,
            'guesses_log10': 3.07554696139253,
            # i-j 起始-终止 位置
            'i': 0,
            'j': 10,
            # 是否使用 l33t
            # https://zh.wikipedia.org/wiki/Leet
            'l33t': False,
            # l33t 的字符
            'l33t_variations': 1,
            # 是否逆序
            'reversed': False,
            # 匹配到的字典词汇
            'matched_word': 'password123',
            'rank': 595,
            'base_guesses': 595,
            'uppercase_variations': 2,
        },
        {
            'pattern': 'bruteforce',
            'token': '123@',
            'guesses': 10000,
            'guesses_log10': 4.0,
            'i': 11,
            'j': 14,
        }
    ]
}
```

### 日期匹配

zxcvbn 通过正则表达式尝试查找密码中的日期/时间，并且以某个基准时间（2017）计算差值，距离越得分越高。

### 字母 / 数字序匹配

zxcvbn 会对密码中可能的连续序列进行检查（如大写字母序，小写字母序，数字序）其主要依据是 ascii / unicode 码序（计算差值），如 `1 -> 49`，`A -> 65`，`B -> 66`，`a -> 97` 如此这般，还是比较有趣的。

> 注：zxcvbn 的连续性检查是区分大小写的，因此如果要忽略大小写，丢进 zxcvbn 之前记得先 `.lower()` 一下～

### 重复字符（串）匹配

zxcvbn 主要使用贪婪（`(.+)\1+`） + 懒惰（`(.+?)\1+`）两种正则表达式来查找密码中的重复子串

### 空间连续性检查

查找密码中出现的空间性连续，如连续的键盘行程（如 "qwerty"）。如果密码包含这些模式，则可能被标记为弱密码。

举个例子：下面这个密码长度达到 15，同时包含了大写字母、小写字母和数字，但由于因为它包含连续键盘字母和重复数字组成的，因此检测结果中会给出提示，用户应该根据自己的实际需求，来评估是否接受这个密码。

```python
# 空间性连续 + 重复
>>> results = zxcvbn("yuikjh333787878", user_inputs=["schnee"])
>>> pprint(results)
{
    ...
    'password': 'yuikjh333787878',
    'score': 3,
    'sequence': [
        {
            'graph': 'qwerty',
            'guesses': 203315.15799004078,
            'guesses_log10': 5.3081697583037,
            'i': 0,
            'j': 5,
            'pattern': 'spatial',
            'shifted_count': 0,
            'token': 'yuikjh',
            'turns': 3
        },
        {
            'base_guesses': Decimal('12'),
            'base_matches': [
                {
                    'guesses': 11,
                    'guesses_log10': 1.041392685158225,
                    'i': 0,
                    'j': 0,
                    'pattern': 'bruteforce',
                    'token': '3'
                }
            ],
            'base_token': '3',
            'guesses': 50,
            'guesses_log10': 1.6989700043360185,
            'i': 6,
            'j': 8,
            'pattern': 'repeat',
            'repeat_count': 3.0,
            'token': '333'
        },
        {
            'base_guesses': Decimal('21'),
            'base_matches': [
                {
                    'ascending': True,
                    'guesses': 20,
                    'guesses_log10': 1.301029995663981,
                    'i': 0,
                    'j': 1,
                    'pattern': 'sequence',
                    'sequence_name': 'digits',
                    'sequence_space': 10,
                    'token': '78'
                }
            ],
            'base_token': '78',
            'guesses': Decimal('63'),
            'guesses_log10': 1.7993405494535815,
            'i': 9,
            'j': 14,
            'pattern': 'repeat',
            'repeat_count': 3.0,
            'token': '787878'
        }
    ]
}
```

> 注：zxcvbn 中的空间连续性检查非常有趣，主要是通过预设的邻接表来简化运算（虽然还是挺复杂），感兴趣的话可以研究下 :D

### Leet / l33t 匹配

Leet（英文中亦称 leetspeak 或 eleet。Leet 拼写法：L33T，L337，或 1337），又称黑客语，是指一种发源于西方国家的 BBS、在线游戏和黑客社群所使用的文字书写方式。通常是把拉丁字母转变成数字或是特殊符号，例如 `E` 写成 `3`、`A` 写成 `@` 等，或是将单词写成同音的字母或数字，如 `to` 写成 `2`、`for` 写成 `4` 等等。

举个例子：密码/口令：`Password --l33t--> P@ssw0rd`

极端的例子：维基百科 `WIKIPEDIA --l33t--> \/\/ 1 |< 1 |o 3 [) 1 4`，感觉还是有点抽象 2333

```python
# l33t
>>> results = zxcvbn("P@ssw0rd", user_inputs=["schnee"])
>>> pprint(results)
{
    ...
    'password': 'P@ssw0rd',
    'score': 0,
    'sequence': [
        {
            'base_guesses': 2,
            'dictionary_name': 'passwords',
            'guesses': 16,
            'guesses_log10': 1.2041199826559246,
            'i': 0,
            'j': 7,
            'l33t': True,
            'l33t_variations': 4,
            'matched_word': 'password',
            'pattern': 'dictionary',
            'rank': 2,
            'reversed': False,
            'sub': {'0': 'o', '@': 'a'},
            'sub_display': '@ -> a, 0 -> o',
            'token': 'P@ssw0rd',
            'uppercase_variations': 2
        }
    ]
}
```

### 密码评分

密码的每部分（根据识别出的模式）将被分配一个 **熵** 值，这代表了该部分被猜中的难度；计算密码熵需要综合考虑长度，使用的字符集，是否包含弱密码，重复等等。

总密码熵是指破解整个密码的代价（在 zxcvbn 中表现为猜测次数：guesses），熵越高，则意味着你的密码更加安全。

根据分析的结果，zxcvbn 还给予提供了密码强度的直观评分（0-4，越高越强）（其实就是一个 if-elif-else，总 guesses 越高，评分越高）

## 所以 zxcvbn 是什么单词么？

其实 zxcvbn 是键盘上相邻的几个字母。这个词常用于演示密码弱点，因为它是键盘上连续按下字母键的典型示例（另一个例子：qwerty）

**zxcvbn** 本身并没有特殊意义，但它被广泛应用于密码安全领域，尤其是与密码强度评估和密码猜测攻击相关的算法、工具和库的命名。

**谷歌翻译小彩蛋：感到突然被 Cue 🌝**

![img](/static/image/blog/vietnamese_zxcvbn.png)

## 参考资料

- <https://github.com/dropbox/zxcvbn>
- <https://github.com/dwolfhub/zxcvbn-python>
- <https://github.com/nbutton23/zxcvbn-go>
- <https://zh.wikipedia.org/wiki/Leet>
