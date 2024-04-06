## 什么是密码？

这里讲的密码，是指密码学中的口令（password），也称为明文，而密码加密算法，则是把明文口令加密成密文的过程实现。

## 加密算法分类

### 对称密码算法（Symmetric）

对称算法 是指加密秘钥和解密秘钥相同的密码算法，又称为 秘密秘钥算法 或 单密钥算法 。

该算法又分为分组密码算法（Block cipher）和 流密码算法（Stream cipher） 。

#### 分组密码算法（块加密算法）

1. 将明文拆分为 N 个固定长度的明文块
2. 用相同的秘钥和算法对每个明文块加密得到 N 个等长的密文块
3. 然后将 N 个密文块按照顺序组合起来得到密文

#### 流密码算法（序列密码算法）

加密：每次只加密一位或一字节明文
解密：每次只解密一位或一字节密文

常见的分组密码算法包括 AES、SM1（国密）、SM4（国密）、DES、3DES、IDEA、RC2 等；常见的流密码算法包括 RC4 等。

AES：目前安全强度较高、应用范围较广的对称加密算法
SM1：国密，采用硬件实现
SM4：国密，可使用软件实现
DES/3DES：已被淘汰或逐步淘汰的常用对称加密算法

### 非对称密码算法（Asymmetric）

非对称算法 是指加密秘钥和解密秘钥不同的密码算法，又称为 公开密码算法 或 公钥算法，该算法使用一个秘钥进行加密，用另外一个秘钥进行解密。

加密秘钥可以公开，称为公钥；解密秘钥必须保密，称为私钥
常见非对称算法包括 RSA、SM2（国密）、DH、DSA、ECDSA、ECC 等。

### 摘要算法（Digest）

摘要算法 是指把任意长度的输入消息数据转化为固定长度的输出数据的一种密码算法，又称为散列函数、哈希函数、杂凑函数、单向函数等。

摘要算法所产生的固定长度的输出数据称为摘要值、散列值或哈希值，摘要算法无秘钥。

摘要算法通常用来做数据完整性的判定，即对数据进行哈希计算然后比较摘要值是否一致。

摘要算法主要分为三大类：

- MD（Message Digest，消息摘要算法）
- SHA-1（Secure Hash Algorithm，安全散列算法）
- MAC（Message Authentication Code，消息认证码算法）
- 国密标准 SM3 也属于散列算法。

MD 系列 主要包括 MD2、MD4、MD5
SHA 系列 主要包括 SHA-1、SHA-2 系列（SHA-1 的衍生算法，
包含 SHA-224、SHA-256、SHA-384、SHA-512）

## 项目背景

蓝鲸对数据库加密的支持：[EncryptField](https://github.com/TencentBlueKing/bkpaas-python-sdk/blob/master/sdks/blue-krill/blue_krill/models/fields.py) 实现了对 SM4 / AES 加密的支持（对称加密）

用户管理需要的：支持 sm3 存储用户密码到数据库，希望兼容 [Django 的密码管理框架](https://docs.djangoproject.com/en/3.2/topics/auth/passwords/)（摘要，不可逆）

## 慢 Hash & 盐

慢 Hash -> 通过多次重复 hash，提升密码对抗暴力破解 / 彩虹表的安全性

PBKDF2（基于密码的密钥派生函数 2）是这一类加密算法的代表者

PBKDF2 将伪随机函数（如 HMAC）与盐值一起应用于输入密码或密码词组，并多次重复该过程以生成派生密钥，然后可以将其用作后续过程的加密密钥。增加的计算工作使密码破解变得更加困难，这被称为密钥延伸。

![img](/static/image/blog/pbkdf2_nist.png)

```python
>>> from bkuser.common.hashers import make_password
>>> make_password("1234567890")
'pbkdf2_sha256$260000$rQTWskRWPS6XZdzqYNs3Up$0sAtd815XhiVgAJe8eVME4v2b3PPqf9h1S3iXDYb6T4='
# 加密算法 pbkdf2_sha256
# 迭代次数 26w
# 盐 rQTWskRWPS6XZdzqYNs3Up
# 密文 0sAtd815XhiVgAJe8eVME4v2b3PPqf9h1S3iXDYb6T4=
```

## pbkdf2 支持 sm3

```python
# django.utils.crypto.pbkdf2
def pbkdf2(password, salt, iterations, dklen=0, digest=None):
    """Return the hash of password using pbkdf2."""
    ...
    return hashlib.pbkdf2_hmac(digest().name, password, salt, iterations, dklen)

# hashlib
try:
    # OpenSSL's PKCS5_PBKDF2_HMAC requires OpenSSL 1.0+ with HMAC and SHA
    from _hashlib import pbkdf2_hmac
except ImportError:
    ...

    def pbkdf2_hmac(hash_name, password, salt, iterations, dklen=None):
        ...
```

pbkdf2_hmac 仅支持 hashlib 内建的 hash 函数（如：md5, sha1, sha256...）无法进行扩展

幸运的是，这里还有一个 python 实现的 pbkdf2_hmac（deprecated），对比多种实现，这个版本性能较好（虽然性能是 \_hashlib.pbkdf2_hmac 的 1/4）

[bkuser \_pbkdf2_hmac_sm3](https://github.com/TencentBlueKing/bk-user/blob/a099745574f5de51433d2603c4fb0748d24d99d2/src/bk-user/bkuser/common/hashers/pbkdf2.py#L21)

## 性能对比

```python
import binascii
from tongsuopy. crypto. hashes import Hash, Sh3
from hashlib import sha256 as _sha256

def sm3(data: bytes) -> bytes:
    h = Hash(SM3))
    h.update(data)
    ret = h.finalize
    return binascii. hexlify(ret)

def sha256(data: bytes) -> str:
    return _sha256(data).hexdigest
```

```python
>>> from bkuser.common.encrypter.hashers import sha256
>>> from bkuser.common.encrypter.hashers import sm3
>>> import time
>>> def timeit(func, cnt):
...     st = time. time ()
...     for i in range (cnt):
...         if 1 & 1:
...             func (b"abcdefghijklmnopgrstuvwxyz")
...         else:
...             func (b"zyxwvutsraponmlkjihgfedcba")
...     print("total time cost:", time.time() - st)
...
>>> timeit(sha256, 1)
total time cost: 2.9325485229492188-05
>>> timeit(sm3, 1)
total time cost: 0.013191938400268555
>>> timeit(sha256, 10000)
total time cost: 0.005854129791259766
>>> timeit(sm3, 10000)
total time cost: 0.0515899658203125
>>> timeit(sha256, 100000)
total time cost: 0.04007291793823242
>>> timeit(sm3, 100000)
total time cost: 0.4731888771057129
```

sm3 性能是 sha256 的 1/15 + python 实现的 pbkdf2_hmac 是内建的 1/4，因此 \_pbkdf2_hmac_sm3 性能大概是 pbkdf2_hmac + sha256 的 1/70，性能有点差，而且难以进一步优化

目前的解决方案：降低慢 hash 迭代的次数 26w -> 2.6w，综合性能为 Django 默认的 PBKDF2Hasher 的 1/7，单次加密约 150ms

## 其他更强的加密算法

如 scrypt, argon2 可以基于 RAM 做加密，对破解者来说意味着更高的成本（内存），但是 argon2 无法修改底层的 hash 函数，且 Django 默认的 Argon2Hasher，单次加密需要消耗 100MB 内存（很可怕的量），即使可以设置加密时使用的内存大小，但是也不适合使用在我们的场景。

## 参考资料

- [常见密码算法分类](https://zhuanlan.zhihu.com/p/37654380)
- [Argon2 with SHA-256 instead of Blake2](https://crypto.stackexchange.com/questions/53260/argon2-with-sha-256-instead-of-blake2)
- [密码哈希的方法：PBKDF2，Scrypt，Bcrypt 和 ARGON2](https://zhuanlan.zhihu.com/p/113971205)
- [为什么现在密码加密依然大多选择 pbkdf2 而不是 argon2](https://www.v2ex.com/t/938519)
- [密钥派生函数 Scrypt、Bcrypt 与 Argon2](https://zhuanlan.zhihu.com/p/612120129)
- [密码学系列之: 1Password 的加密基础 PBKDF2](https://www.cnblogs.com/flydean/p/15346657.html)
- [Argon2 vs bcrypt vs. scrypt](https://stytch.com/blog/argon2-vs-bcrypt-vs-scrypt/)
