# WIP：Golang 单元测试中的 Mock 技术

## 背景

在单元测试中，我们经常需要测试调用数据库、第三方 API、文件系统等外部依赖的代码。然而，直接调用这些外部依赖会导致测试变慢、不稳定，甚至无法运行。

Mock 技术通过创建外部依赖的替身对象，隔离被测代码与外部系统的耦合，使测试更快速、稳定、可靠。

本文将系统介绍 Go 语言中 Mock 技术的基本概念、主流工具及其使用方法。

## 基本概念

在测试领域，测试替身是指用于替代真实对象进行测试的各种替身对象。

根据用途不同，测试替身分为五种类型：

| 类型      | 作用                                               | 例子                            |
|-----------|----------------------------------------------------|---------------------------------|
| **Dummy** | 占位符，填充参数列表，不被使用                     | 传 `nil` 给不需要的参数         |
| **Stub**  | 返回预设的固定值                                   | 调用 `GetUser()` 永远返回用户 A |
| **Mock**  | 验证行为是否符合预期，关注"有没有调用"、"调用几次" | 确认 `Save()` 方法被调用了 2 次 |
| **Fake**  | 简化的工作实现，通常用内存替代持久化               | 用 `map` 模拟数据库             |
| **Spy**   | 记录调用信息，供后续验证                           | 记录 `SendEmail()` 被调用的参数 |

### Mock vs Stub

Mock 和 Stub 的核心区别在于：

- **Stub** 关注返回值，用于提供测试数据
- **Mock** 关注行为验证，用于验证调用是否符合预期

举个例子：

```go
// Stub：只管返回数据
stubUserRepo.On("GetUser", 1).Return(&User{Name: "张三"})

// Mock：验证行为
mockUserRepo.EXPECT().Save(user).Times(1) // 必须调用一次
```

### Mock 的典型应用场景

- **数据库操作**：避免真实数据库交互
- **HTTP 请求**：避免调用第三方 API
- **时间函数**：测试时间相关逻辑

---

## Mock 技术的底层实现原理

理解 Mock 工具的底层机制，有助于我们在不同场景下做出正确的技术选型，也能在遇到诡异问题时快速定位根因。Go 生态中的 Mock 工具本质上采用了三种不同的实现策略：**代码生成**、**接口代理**和**函数打桩**。

### 三种实现策略的架构对比

| 代码生成     | 接口代理             | 函数打桩                       |
|:-------------|:---------------------|:-------------------------------|
| **工具**     | gomock               | testify/mock                   |
| **实现方式** | 编译期生成 Mock 实现 | 运行时反射调用 `mock.Called()` |
| **类型安全** | ✓                    | △                              |
| **反射开销** | 无                   | 有                             |
| **代码生成** | 需代码生成           | 无需代码生成 ✓                 |
| **适用范围** | 仅限接口             | 仅限接口                       |

### 代码生成：gomock 的实现机制

gomock 的核心思路是**在编译期生成接口的完整 Mock 实现**。`mockgen` 工具解析源代码的接口定义，生成一个实现了相同接口的结构体，该结构体的每个方法内部通过 `gomock.Controller` 来记录和验证调用行为。

**生成流程**：

```
接口定义 (.go) → mockgen 解析 AST → 生成 Mock 结构体 (.go) → 编译到测试二进制
```

**生成的 Mock 方法内部逻辑（简化）**：

```go
// mockgen 生成的 GetUser 方法大致逻辑
func (m *MockUserRepository) GetUser(id int) (*User, error) {
    m.ctrl.T.Helper()
    // 1. 在期望列表中查找匹配的 EXPECT 调用
    // 2. 验证参数是否匹配
    // 3. 记录调用次数
    // 4. 执行 Do() 回调（如果有）
    // 5. 返回预设的 Return 值
    return m.ctrl.Call(m, "GetUser", id)
}
```

**类型安全的保障**：由于 `mockgen` 直接读取接口定义并生成对应方法签名，如果接口签名发生变化但 Mock 代码未重新生成，编译阶段就会报错，而非等到运行时才发现问题。这是 gomock 相比 testify/mock 最大的优势。

### 接口代理：testify/mock 的反射调用

testify/mock 采用运行时反射机制。Mock 结构体嵌入 `mock.Mock`，在每个方法中调用 `m.Called(args...)`，`Called` 方法通过反射做以下工作：

1. 在注册的期望列表中，用**字符串匹配方法名**查找对应的 `On()` 注册项
2. 逐个比较实际参数与期望参数
3. 记录调用信息，返回预设结果

**关键风险**：

```go
// 方法名是字符串，拼错不会编译报错！
mockRepo.On("GetUsr", 1).Return(&User{Name: "张三"}, nil)
//          ^^^^^ 拼写错误，运行时才会 panic
```

这就是 testify/mock 牺牲类型安全换取灵活性的代价。

### 函数打桩：mockey 的机器指令修改

mockey（以及 gomonkey）采用了与前两者完全不同的策略——直接在运行时修改目标函数的机器码，将函数入口替换为跳转指令，跳转到 Mock 函数。

**原理示意**：

```
原始函数内存布局:
┌─────────────────────┐
│ original_func:      │
│   push rbp          │  ← 原始指令
│   mov rbp, rsp      │
│   ...               │
└─────────────────────┘

打桩后内存布局:
┌─────────────────────┐
│ original_func:      │
│   jmp mock_func     │  ← 被替换为跳转指令
│   mov rbp, rsp      │  (被覆盖)
│   ...               │
└─────────────────────┘
┌─────────────────────┐
│ mock_func:          │
│   ...mock logic...  │  ← 跳转目标
└─────────────────────┘
```

**为什么需要关闭内联优化？**

Go 编译器在开启优化时，可能会将被调用函数内联到调用方中。内联后，原函数调用消失，打桩修改函数入口就失去了目标：

```go
// 优化前：调用 GetConfig() 会跳转到函数入口，打桩有效
func IsDebugMode() bool {
    cfg := GetConfig()  // → call GetConfig
    return cfg["debug"] == "true"
}

// 内联优化后：GetConfig 的代码被直接嵌入，没有函数调用，打桩失效
func IsDebugMode() bool {
    cfg := map[string]string{}  // ← GetConfig 被内联，mockey 无法拦截
    return cfg["debug"] == "true"
}
```

因此使用 `-gcflags=-l` 禁止内联，确保所有函数调用都以 `call` 指令形式存在。

**mockey vs gomonkey 的关键差异**：

| 维度         | mockey                          | gomonkey               |
|--------------|---------------------------------|------------------------|
| 并发安全     | ✓ 使用 goroutine-local 存储     | ✗ 全局打桩，并发不安全 |
| 生命周期管理 | PatchConvey 自动还原            | 手动 Reset             |
| 平台支持     | Linux/macOS（Windows 部分支持） | Linux/macOS/Windows    |
| 打桩方式     | 函数入口跳转 + 闭包捕获         | 函数入口跳转           |

### sqlmock 的拦截机制

sqlmock 的实现原理与上述工具有本质区别——它不是 Mock 函数或接口，而是实现了 `database/sql/driver` 接口：

```
应用代码
  ↓
database/sql.DB（标准库）
  ↓
sqlmock 实现的 driver.Driver / driver.Conn
  ↓  （不经过网络）
sqlmock 期望匹配引擎
```

`sqlmock.New()` 创建的 `sql.DB` 实际上连接的是 sqlmock 自己的 driver 实现。当应用代码调用 `db.Query()` 时，标准库会委托给 driver 执行，而 sqlmock 的 driver 会将调用转发给期望匹配引擎，按照注册顺序匹配 SQL 语句和参数，返回预设的结果。

这种设计的优势在于：**应用代码完全不需要修改**，只需在测试中将真实的 `*sql.DB` 替换为 sqlmock 创建的实例即可。

### httpmock 的 Transport 替换

httpmock 的实现利用了 Go 标准库 `net/http` 的可扩展点——`http.Client.Transport` 接口：

```go
// httpmock.Activate() 做的事情：
func Activate() {
    http.DefaultTransport = httpmock.DefaultTransport
    // 将 DefaultTransport 替换为自定义的 Transport
    // 该 Transport 会先匹配注册的 URL 规则
    // 匹配成功则返回预设响应
    // 匹配失败则转发给原始 Transport（如果配置了）
}
```

理解了这个原理，你就知道为什么 httpmock 对使用自定义 `http.Client` 的代码可能失效——如果你的代码创建了 `&http.Client{Transport: myTransport}`，httpmock 默认无法拦截。解决方案是在测试中也替换该 Client 的 Transport。

---

## Go 常用 Mock 工具对比

Go 生态中有多种 Mock 工具，各有特点：

| 工具             | 特点                         | 适用场景            |
|------------------|------------------------------|---------------------|
| **gomock**       | 官方出品，代码生成，类型安全 | 接口 Mock，大型项目 |
| **testify/mock** | 手写 Mock，语法简洁          | 简单场景，快速原型  |
| **sqlmock**      | 专为 `database/sql` 设计     | 数据库操作测试      |
| **httpmock**     | Mock HTTP 客户端             | 第三方 API 调用     |
| **mockey**       | 字节出品，语法优雅，支持并发 | 函数/变量 Mock      |

### 工具选型建议

- **大型项目**：选择 `gomock`，官方推荐，类型安全
- **快速原型**：选择 `testify/mock`，语法简单，无需代码生成
- **数据库测试**：选择 `sqlmock`，专业工具
- **HTTP API 测试**：选择 `httpmock`
- **现代项目**：选择 `mockey`，支持并发安全

### 深度对比：超越表面的差异

上表给出了快速选型参考，但在实际项目中，我们需要从更本质的维度来理解这些工具的差异。

#### 类型安全的代价与收益

类型安全是 Mock 工具最容易被忽视、却最容易埋坑的维度。来看一个真实的线上事故场景：

```go
// 接口签名变更：GetUser 返回值从 (*User, error) 改为 (User, error)
type UserRepository interface {
    GetUser(id int) (User, error)  // 不再返回指针
}
```

| 工具             | 后果                                                                       |
|------------------|----------------------------------------------------------------------------|
| **gomock**       | 重新 `go generate` 后编译报错，**在编译期拦截**，零风险上线                |
| **testify/mock** | `args.Get(0).(*User)` 触发 panic，**在运行时暴露**，需测试覆盖到位才能发现 |
| **mockey**       | Mock 函数签名不匹配，**在运行时 panic**，且错误信息可能难以定位根因        |

**结论**：在接口频繁演进的项目中，gomock 的编译期类型保障价值巨大。testify/mock 和 mockey 的类型安全问题可以通过 `var _ Interface = (*MockImpl)(nil)` 编译期断言部分缓解，但无法完全消除。

#### 反射开销的量化分析

testify/mock 的 `Called()` 方法每次调用都涉及反射操作。在微基准测试中，单次反射调用的额外开销约在 200-500ns 量级。对于绝大多数测试场景，这个开销可以忽略不计。但在以下场景中值得注意：

- **批量数据处理测试**：当被测方法在循环中调用 Mock 方法数千次时，反射开销会显著放大
- **基准测试**：如果你使用 `Benchmark` 来评估业务代码性能，Mock 的反射开销会污染数据
- **CI 超时**：在超大规模测试套件（10万+用例）中，累积的反射开销可能导致 CI 超时

```go
// 反射开销放大的例子
func TestBatchProcess(t *testing.T) {
    // 如果处理 10000 条记录，每条都调用 Mock 方法
    // testify/mock 的反射开销 ≈ 10000 × 300ns ≈ 3ms（可接受）
    // 但如果每条记录涉及 10 次 Mock 调用，开销 ≈ 30ms
    mockRepo.On("GetUser", mock.Anything).Return(&User{Name: "test"}, nil)
    service.BatchProcess(10000) // 内部循环调用 GetUser
}
```

#### Mock 能力边界的本质差异

不同工具的 Mock 能力边界，决定了你能测试什么、不能测试什么：

```
能力范围（从小到大）：

gomock          ──── 仅接口方法
testify/mock    ──── 仅接口方法（需手动实现）
sqlmock         ──── database/sql 驱动层
httpmock        ──── http.Transport 层
mockey          ──── 接口方法 + 普通函数 + 方法 + 全局变量
```

这个能力范围差异直接影响你的**代码设计决策**：

- 如果选用 gomock/testify，你必须**为所有需要 Mock 的依赖定义接口**
- 如果选用 mockey，你可以直接 Mock 具体函数，**不强制要求接口抽象**

这看似 mockey 更灵活，但过度依赖函数打桩会导致代码缺乏接口抽象，增加后期重构难度。**工具选型会影响架构风格，这不是技术偏好问题，而是架构决策。**

#### 各工具的隐性成本

| 工具             | 隐性成本                                                                            |
|------------------|-------------------------------------------------------------------------------------|
| **gomock**       | Mock 代码生成增加构建步骤；生成的代码量大（接口方法多时）；`go generate` 可能被遗忘 |
| **testify/mock** | 手写 Mock 样板代码；字符串匹配导致重构风险；类型断言的样板代码容易写错              |
| **sqlmock**      | SQL 匹配是字符串级别的，schema 变更时需要手动同步；不支持 ORM 的复杂查询构建器      |
| **httpmock**     | 全局替换 `DefaultTransport` 可能影响并行测试；对自定义 Transport 的代码无效         |
| **mockey**       | 需要 `-gcflags=-l` 禁止内联；平台兼容性限制；调试时堆栈信息可能被 Mock 干扰         |

下面将逐一介绍各个工具的使用方法。

---

## gomock 使用示例

gomock 是 Go 官方推荐的 Mock 框架，通过代码生成创建 Mock 对象，类型安全且功能强大。

### 安装

```bash
go install go.uber.org/mock/mockgen@latest
```

> **注意**：原 `github.com/golang/mock` 已归档停止维护，推荐使用社区维护的 `go.uber.org/mock`。

### 基本使用

假设有一个用户服务，需要 Mock 数据库访问层：

```go
// user.go
package user

// UserRepository 用户仓库接口
type UserRepository interface {
	GetUser(id int) (*User, error)
}

// User 用户实体
type User struct {
	ID   int
	Name string
}

// UserService 用户服务
type UserService struct {
	repo UserRepository
}

func (s *UserService) GetUserName(id int) (string, error) {
	user, err := s.repo.GetUser(id)
	if err != nil {
		return "", err
	}
	return user.Name, nil
}
```

**第一步**：使用 mockgen 生成 Mock 代码

```bash
mockgen -source=user.go -destination=mock_user.go -package=user
```

**第二步**：编写测试

```go
// user_test.go
package user

import (
	"testing"
	"go.uber.org/mock/gomock"
)

func TestGetUserName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockUserRepository(ctrl)

	// 设置期望：当调用 GetUser(1) 时，返回预设用户
	mockRepo.EXPECT().
		GetUser(1).
		Return(&User{ID: 1, Name: "张三"}, nil)

	service := &UserService{repo: mockRepo}
	name, err := service.GetUserName(1)

	if err != nil || name != "张三" {
		t.Errorf("期望 '张三', 得到 '%s', err=%v", name, err)
	}
}
```

### 进阶用法

#### 使用 Do() 执行自定义逻辑

```go
mockRepo.EXPECT().
    GetUser(gomock.Any()).
    Do(func(id int) {
        fmt.Printf("查询用户 ID: %d\n", id)
    }).
    Return(&User{ID: 1, Name: "张三"}, nil)
```

#### 参数匹配器

```go
// 匹配任意参数
mockRepo.EXPECT().GetUser(gomock.Any()).Return(&User{Name: "路人"}, nil)

// 匹配特定条件
mockRepo.EXPECT().GetUser(gomock.Eq(1)).Return(&User{Name: "张三"}, nil)
mockRepo.EXPECT().GetUser(gomock.Not(0)).Return(&User{Name: "非零ID用户"}, nil)
```

#### 调用次数控制

```go
mockRepo.EXPECT().GetUser(1).Times(2) // 必须调用 2 次
mockRepo.EXPECT().GetUser(2).AnyTimes() // 任意次数
mockRepo.EXPECT().GetUser(3).MinTimes(1) // 至少 1 次
mockRepo.EXPECT().GetUser(4).MaxTimes(3) // 最多 3 次
```

#### 调用顺序控制

```go
// 先调用 GetUser，再调用 SaveUser
call1 := mockRepo.EXPECT().GetUser(1).Return(&User{Name: "张三"}, nil)
mockRepo.EXPECT().SaveUser(gomock.Any()).After(call1)
```

---

## testify/mock 使用示例

testify/mock 不需要代码生成，手动创建 Mock 结构体，语法简洁，适合快速原型开发。

### 安装

```bash
go get github.com/stretchr/testify
```

### 基本使用

使用同样的用户服务示例：

```go
// user.go
package user

type UserRepository interface {
	GetUser(id int) (*User, error)
	SaveUser(user *User) error
}

type User struct {
	ID   int
	Name string
}

type UserService struct {
	repo UserRepository
}

func (s *UserService) GetUserName(id int) (string, error) {
	user, err := s.repo.GetUser(id)
	if err != nil {
		return "", err
	}
	return user.Name, nil
}
```

**第一步**：手动创建 Mock 结构体

```go
// mock_user.go
package user

import "github.com/stretchr/testify/mock"

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetUser(id int) (*User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserRepository) SaveUser(user *User) error {
	args := m.Called(user)
	return args.Error(0)
}
```

**第二步**：编写测试

```go
// user_test.go
package user

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestGetUserName(t *testing.T) {
	mockRepo := new(MockUserRepository)

	mockRepo.On("GetUser", 1).Return(&User{ID: 1, Name: "张三"}, nil)

	service := &UserService{repo: mockRepo}
	name, err := service.GetUserName(1)

	assert.NoError(t, err)
	assert.Equal(t, "张三", name)

	mockRepo.AssertExpectations(t)
}
```

### 进阶用法

#### 参数匹配与断言

```go
// 匹配任意参数
mockRepo.On("GetUser", mock.Anything).Return(&User{Name: "路人"}, nil)

// 使用自定义匹配器
mockRepo.On("SaveUser", mock.MatchedBy(func(u *User) bool {
    return u.Name != ""
})).Return(nil)
```

#### 调用次数控制

```go
mockRepo.On("GetUser", 1).Return(&User{Name: "张三"}, nil).Once() // 只调用一次
mockRepo.On("GetUser", 2).Return(&User{Name: "李四"}, nil).Twice() // 调用两次
mockRepo.On("GetUser", 3).Return(&User{Name: "王五"}, nil).Times(3) // 调用三次
```

#### 返回错误

```go
mockRepo.On("GetUser", 999).Return(nil, errors.New("用户不存在"))
```

#### 执行自定义逻辑

```go
mockRepo.On("GetUser", 1).Return(&User{ID: 1, Name: "张三"}, nil).
    Run(func(args mock.Arguments) {
        fmt.Printf("查询用户 ID: %d\n", args.Get(0).(int))
    })
```

---

## sqlmock 使用示例

sqlmock 专为 `database/sql` 设计，可以在不真实连接数据库的情况下测试数据库操作。

### 安装

```bash
go get github.com/DATA-DOG/go-sqlmock
```

### 基本使用

假设有一个查询用户的服务：

```go
// user.go
package user

import "database/sql"

type User struct {
	ID   int
	Name string
}

func GetUserByID(db *sql.DB, id int) (*User, error) {
	var user User
	err := db.QueryRow("SELECT id, name FROM users WHERE id = ?", id).
		Scan(&user.ID, &user.Name)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
```

编写测试：

```go
// user_test.go
package user

import (
	"testing"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestGetUserByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("创建 mock 失败: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "张三")

	mock.ExpectQuery(`SELECT id, name FROM users WHERE id = \\?`).
		WithArgs(1).
		WillReturnRows(rows)

	user, err := GetUserByID(db, 1)

	assert.NoError(t, err)
	assert.Equal(t, 1, user.ID)
	assert.Equal(t, "张三", user.Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}
```

### 进阶用法

#### 模拟插入操作

```go
mock.ExpectExec("INSERT INTO users").
WithArgs("李四").
WillReturnResult(sqlmock.NewResult(2, 1)) // lastInsertId=2, rowsAffected=1
```

#### 模拟更新操作

```go
mock.ExpectExec("UPDATE users SET name = ?").
WithArgs("王五", 1).
WillReturnResult(sqlmock.NewResult(0, 1)) // rowsAffected=1
```

#### 模拟事务

```go
mock.ExpectBegin()

mock.ExpectExec("UPDATE users SET name = ?").
WithArgs("新名字", 1).
WillReturnResult(sqlmock.NewResult(0, 1))

mock.ExpectCommit()
```

#### 模拟错误

```go
mock.ExpectQuery("SELECT .*").
WillReturnError(sql.ErrConnDone)
```

---

## httpmock 使用示例

httpmock 用于 Mock HTTP 客户端请求，特别适合测试调用第三方 API 的代码。

### 安装

```bash
go get github.com/jarcoal/httpmock
```

### 基本使用

假设有一个调用外部 API 获取用户信息的服务：

```go
// user.go
package user

import (
	"encoding/json"
	"io"
	"net/http"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func GetUserFromAPI(url string) (*User, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user User
	err = json.Unmarshal(body, &user)
	return &user, err
}
```

编写测试：

```go
// user_test.go
package user

import (
	"testing"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestGetUserFromAPI(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://api.example.com/users/1",
		httpmock.NewJsonResponderOrPanic(200, &User{ID: 1, Name: "张三"}))

	user, err := GetUserFromAPI("https://api.example.com/users/1")

	assert.NoError(t, err)
	assert.Equal(t, 1, user.ID)
	assert.Equal(t, "张三", user.Name)
}
```

### 进阶用法

#### 使用正则匹配 URL

```go
httpmock.RegisterResponder("GET", `=~^https://api\\.example\\.com/users/\\d+$`,
httpmock.NewJsonResponderOrPanic(200, &User{ID: 1, Name: "张三"}))
```

#### 返回不同状态码

```go
// 返回 404
httpmock.RegisterResponder("GET", "https://api.example.com/users/999",
httpmock.NewStringResponder(404, "用户不存在"))

// 返回 500
httpmock.RegisterResponder("GET", "https://api.example.com/error",
httpmock.NewStringResponder(500, "服务器错误"))
```

#### 自定义响应逻辑

```go
httpmock.RegisterResponder("GET", "https://api.example.com/users/1",
    func(req *http.Request) (*http.Response, error) {
        return httpmock.NewJsonResponse(200, &User{ID: 1, Name: "动态用户"})
    })
```

#### 验证调用次数

```go
info := httpmock.GetCallCountInfo()
count := info["GET https://api.example.com/users/1"]
assert.Equal(t, 1, count)
```

---

## mockey 使用示例

mockey 是字节跳动开源的 Mock 框架，语法优雅，支持函数、方法、变量的 Mock，且并发安全。

### 安装

```bash
go get github.com/bytedance/mockey@latest
```

### 基本使用

假设有一个获取配置的函数：

```go
// config.go
package config

func GetConfig() map[string]string {
	// 从远程配置中心获取
	return nil
}

func IsDebugMode() bool {
	cfg := GetConfig()
	return cfg["debug"] == "true"
}
```

编写测试（Mock `GetConfig` 函数）：

```go
// config_test.go
package config

import (
	"testing"
	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
)

func TestIsDebugMode(t *testing.T) {
	mockey.Mock(GetConfig).To(func() map[string]string {
		return map[string]string{"debug": "true"}
	}).Build()

	result := IsDebugMode()
	assert.True(t, result)
}
```

### 配合 PatchConvey 使用

mockey 提供了 `PatchConvey`，可以自动管理 Mock 生命周期，测试结束后自动还原：

```go
package config

import (
	"testing"
	"github.com/bytedance/mockey"
	. "github.com/smartystreets/goconvey/convey"
)

func TestIsDebugModeWithPatchConvey(t *testing.T) {
	mockey.PatchConvey("测试 Debug 模式", t, func() {
		mockey.Mock(GetConfig).To(func() map[string]string {
			return map[string]string{"debug": "true"}
		}).Build()

		result := IsDebugMode()
		So(result, ShouldBeTrue)
	})
}
```

### 进阶用法

#### Mock 变量

```go
var superUsers = getSuperUsersFromRPC()

func isSuperUser(user string) bool {
    for _, superUser := range superUsers {
        if superUser == user {
            return true
        }
    }
    return false
}

// 测试
func TestIsSuperUser(t *testing.T) {
    mockey.MockValue(&superUsers).To([]string{"admin", "root"}).Build()

    assert.True(t, isSuperUser("admin"))
    assert.False(t, isSuperUser("guest"))
}
```

#### Mock 方法

```go
type UserService struct {
    Name string
}

func (s *UserService) GetName() string {
    return s.Name
}

// 测试
func TestGetName(t *testing.T) {
    user := &UserService{Name: "张三"}

    mockey.Mock((*UserService).GetName).To(func(s *UserService) string {
        return "李四"
    }).Build()

    assert.Equal(t, "李四", user.GetName())
}
```

#### 并发安全

mockey 支持并发安全，可以在并发测试中使用：

```go
func TestConcurrentMock(t *testing.T) {
    mockey.PatchConvey("并发测试", t, func() {
        mockey.Mock(GetConfig).To(func() map[string]string {
            return map[string]string{"debug": "true"}
        }).Build()

        var wg sync.WaitGroup
        for i := 0; i < 10; i++ {
            wg.Add(1)
            go func() {
                defer wg.Done()
                result := IsDebugMode()
                assert.True(t, result)
            }()
        }
        wg.Wait()
    })
}
```

### 注意事项

⚠️ **重要提醒**：

1. **关闭内联优化**：需要关闭内联优化
   ```bash
   go test -gcflags=-l -v
   ```

2. **Windows 支持**：mockey 在 Windows 上支持有限，某些场景可能失败

3. **私有方法**：可以 Mock 私有方法，但需要使用反射获取

---

## 架构设计中的 Mock 策略

Mock 不仅是测试技术，更是一种架构约束。Mock 的难易程度，直接反映了代码的设计质量。本节从架构视角探讨 Mock 策略如何反向驱动代码设计。

### 依赖注入：Mock 的架构前提

"依赖接口而非实现"是 Mock 的基础原则，但其本质是**依赖注入（Dependency Injection）**模式。依赖注入不只是"把依赖放到构造函数里"那么简单，它决定了模块间控制权的流向。

**控制反转的深层含义**：

```go
// ❌ 主动依赖：UserService 控制了 UserRepository 的创建
type UserService struct {
    repo UserRepository
}

func NewUserService() *UserService {
    return &UserService{
        repo: NewMySQLUserRepo(), // 硬编码依赖，无法替换
    }
}

// ✅ 依赖注入：调用方决定注入什么实现
func NewUserService(repo UserRepository) *UserService {
    return &UserService{repo: repo}
}
```

第一段代码中，`UserService` 主动创建了 `MySQLUserRepo`，控制权在服务自身。测试时你无法替换这个依赖，除非修改源码或使用函数打桩。

第二段代码中，`UserService` 被动接收依赖，控制权在调用方。测试时只需传入 Mock 实现，无需任何侵入式操作。

**三种注入方式对比**：

| 方式         | 示例                    | 优势             | 劣势                   |
|--------------|-------------------------|------------------|------------------------|
| 构造函数注入 | `NewService(repo)`      | 依赖明确，不可变 | 参数列表可能变长       |
| Setter 注入  | `service.SetRepo(repo)` | 灵活，可中途替换 | 依赖可能为 nil，不安全 |
| 接口字段注入 | `service.Repo = repo`   | 简单直接         | 暴露内部字段，破坏封装 |

在 Go 中，**构造函数注入是最推荐的方式**。它让依赖关系在编译期就确定，且保证了对象的不可变性（注入后不再变化）。

### 六边形架构与端口-适配器测试

六边形架构（Hexagonal Architecture，又称端口-适配器架构）将系统分为**领域核心**和**外部适配器**两部分，通过**端口（接口）**连接。这种架构天然适合 Mock 测试。

```
                    ┌──────────────────────────┐
                    │       外部世界             │
                    │  (HTTP, DB, MQ, RPC...)  │
                    └──────┬──────────┬────────┘
                           │          │
                    ┌──────▼──┐  ┌───▼───────┐
                    │ 适配器 A │  │ 适配器 B    │  ← 实现端口接口
                    │(HTTP API)│  │(DB Repo)   │
                    └──────┬──┘  └───┬───────┘
                           │          │
                    ┌──────▼──────────▼───────┐
                    │       端口（接口）        │  ← 边界，Mock 的切入点
                    │  UserServicePort         │
                    └──────┬──────────┬───────┘
                           │          │
                    ┌──────▼──────────▼───────┐
                    │      领域核心             │  ← 纯逻辑，无外部依赖
                    │  UserService            │
                    └─────────────────────────┘
```

**测试策略**：

- **领域核心**：无需 Mock，直接测试纯逻辑
- **端口**：Mock 接口，隔离领域与外部
- **适配器**：集成测试，验证与真实外部系统的交互

```go
// 端口定义
type UserPort interface {
    FindByID(id int) (*User, error)
    Save(user *User) error
}

// 领域核心：纯逻辑，依赖端口
type UserService struct {
    port UserPort // 通过端口与外部通信
}

func (s *UserService) Register(name string) (*User, error) {
    if name == "" {
        return nil, errors.New("name is required")
    }
    user := &User{Name: name}
    if err := s.port.Save(user); err != nil {
        return nil, fmt.Errorf("save user: %w", err)
    }
    return user, nil
}
```

这种架构下，测试领域逻辑只需要 Mock `UserPort`，而领域核心本身没有任何外部依赖需要处理。**架构的边界就是 Mock 的边界**——这是六边形架构最强大的测试优势。

### 领域驱动设计中的 Mock 策略

在 DDD 中，不同层的 Mock 策略有显著差异：

| 层次           | Mock 策略               | 理由                          |
|----------------|-------------------------|-------------------------------|
| **领域层**     | 尽量不 Mock，纯逻辑测试 | 领域逻辑应无副作用，无需 Mock |
| **应用层**     | Mock 仓库端口和领域服务 | 隔离领域逻辑，专注编排逻辑    |
| **基础设施层** | 用 Fake 实现或集成测试  | 需要验证与真实存储的兼容性    |
| **接口层**     | Mock 应用服务           | 专注 HTTP/gRPC 协议处理       |

**Fake vs Mock 的选择原则**：

```go
// Fake：简化的真实实现，可用于多个测试
type FakeUserRepository struct {
    users map[int]*User
}

func (r *FakeUserRepository) FindByID(id int) (*User, error) {
    return r.users[id], nil
}

func (r *FakeUserRepository) Save(user *User) error {
    r.users[user.ID] = user
    return nil
}

// Mock：精确控制行为和验证调用
mockRepo.EXPECT().Save(gomock.Any()).Return(nil)
mockRepo.EXPECT().FindByID(1).Return(&User{Name: "张三"}, nil)
```

**什么时候用 Fake，什么时候用 Mock？**

- **Fake** 适合：状态驱动的测试（"执行操作后，查询结果是什么"）；需要跨多个方法调用维护状态的场景；作为共享测试基础设施
- **Mock** 适合：行为驱动的测试（"是否按预期调用了某个方法"）；需要验证调用顺序和次数的场景；一次性使用的简单测试

### Mock 驱动设计：让测试塑造架构

Mock 驱动设计（Mock-Driven Design）是一种"测试先行"的设计方法：**如果某个依赖很难 Mock，说明设计有问题**。

**常见的设计坏味道与重构方案**：

```go
// 坏味道 1：依赖具体类型而非接口
type OrderService struct {
    db *sql.DB // 具体类型，测试必须用 sqlmock
}

// 重构：抽象出 Repository 接口
type OrderRepository interface {
    FindByID(id int) (*Order, error)
    Save(order *Order) error
}

type OrderService struct {
    repo OrderRepository // 接口，测试可用任意 Mock
}

// 坏味道 2：在方法内部创建依赖
func (s *Service) Process() error {
    resp, err := http.Get("https://api.example.com/data") // 硬编码依赖
    // ...
}

// 重构：通过参数注入依赖
type HTTPClient interface {
    Get(url string) (*http.Response, error)
}

func (s *Service) Process(client HTTPClient) error {
    resp, err := client.Get("https://api.example.com/data")
    // ...
}

// 坏味道 3：全局状态
var config = loadConfig() // 全局变量，测试间互相影响

// 重构：将配置作为依赖注入
type Config struct {
    Debug bool
}

type Service struct {
    config *Config
}
```

**可测试性的设计原则**：

1. **显式依赖**：所有外部依赖都应通过构造函数或参数传入
2. **接口隔离**：依赖的接口应该尽量小，只包含被使用的方法
3. **无全局状态**：避免包级变量，将状态封装在结构体中
4. **纯函数优先**：没有副作用的函数不需要 Mock
5. **错误显式返回**：用 `error` 返回值而非 `panic`，便于 Mock 错误场景

---

## 最佳实践

### 依赖接口而非实现

**核心原则**：想要 Mock 得当，先要有接口。

```go
// ❌ 直接依赖具体实现
type UserService struct {
db *MySQLDB // 具体 DB，无法 Mock
}

// ✅ 依赖接口
type UserService struct {
db UserRepository // 接口，可以轻松 Mock
}

type UserRepository interface {
GetUser(id int) (*User, error)
SaveUser(user *User) error
}
```

接口应遵循接口隔离原则，职责单一：

```go
// ❌ 接口太大，职责不清
type UserRepository interface {
GetUser(id int) (*User, error)
SaveUser(user *User) error
SendEmail(email string) error
GenerateReport() ([]byte, error)
}

// ✅ 接口职责单一
type UserReader interface {
GetUser(id int) (*User, error)
}

type UserWriter interface {
SaveUser(user *User) error
}
```

### Mock 代码的组织与管理

推荐目录结构：

```
project/
├── user/
│   ├── user.go
│   ├── user_test.go
│   └── mock/
│       └── user_mock.go  # 生成的 Mock 文件
```

**命名规范**：

- Mock 文件：`mock_{原文件名}.go` 或放在 `mock` 子目录
- Mock 结构体：`Mock{接口名}`

### 避免过度 Mock

过度 Mock 会让测试变得脆弱：

```go
// ❌ 过度 Mock
mockDB.EXPECT().GetUser(1).Return(&User{ID: 1})
mockDB.EXPECT().ValidateUser(gomock.Any()).Return(true)
mockDB.EXPECT().CalculateAge(gomock.Any()).Return(25)
mockDB.EXPECT().FormatName(gomock.Any()).Return("张三")
// ... 还有 20 行 Mock

// ✅ 适度 Mock：只 Mock 关键外部依赖
mockDB.EXPECT().GetUser(1).Return(&User{ID: 1, Name: "张三"})
result := service.ProcessUser(1) // 让真实逻辑跑起来
```

**判断标准**：

- 只 Mock 外部依赖（数据库、API、文件系统）
- 内部逻辑尽量走真实代码
- 如果 Mock 代码比测试代码还长，说明有问题

### 保持测试可读性

```go
// ❌ 难以理解
mockRepo.EXPECT().GetUser(1).Return(&User{ID: 1, Name: "张三"}, nil)

// ✅ 清晰明了
const testUserID = 1
const testUserName = "张三"

mockRepo.EXPECT().
GetUser(testUserID).
Return(&User{ID: testUserID, Name: testUserName}, nil)
```

### CI/CD 中的 Mock 测试

```yaml
# GitHub Actions 示例
- name: Run tests
  run: go test -v -race -coverprofile=coverage.out ./...

- name: Upload coverage
  uses: codecov/codecov-action@v3
```

**CI/CD 注意事项**：

1. 使用 `-race` 检测数据竞争
2. 使用 `-cover` 统计覆盖率
3. mockey 测试需要加 `-gcflags=-l`
4. 定期更新 Mock 工具版本

---

## 复杂实战场景深入

前面的示例覆盖了各工具的基本和进阶用法，但在真实项目中，Mock 的挑战往往来自复杂的业务场景。本节深入几个常见的高难度场景，给出完整的解决方案。

### 数据库事务的 Mock 策略

事务是数据库测试中最棘手的场景。事务涉及多个操作要么全部成功、要么全部回滚，Mock 时需要确保事务边界的一致性。

**场景**：转账操作，需要在事务中完成扣款和加款

```go
// transfer.go
package transfer

import "database/sql"

func Transfer(db *sql.DB, fromID, toID int, amount float64) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback() // 安全回滚：如果 Commit 成功，Rollback 是 no-op

    // 扣款
    res, err := tx.Exec("UPDATE accounts SET balance = balance - ? WHERE id = ?", amount, fromID)
    if err != nil {
        return err
    }
    if affected, _ := res.RowsAffected(); affected == 0 {
        return errors.New("扣款账户不存在")
    }

    // 加款
    res, err = tx.Exec("UPDATE accounts SET balance = balance + ? WHERE id = ?", amount, toID)
    if err != nil {
        return err
    }
    if affected, _ := res.RowsAffected(); affected == 0 {
        return errors.New("加款账户不存在")
    }

    return tx.Commit()
}
```

**使用 sqlmock 测试事务**：

```go
func TestTransfer_Success(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()

    // 严格按顺序设置事务期望
    mock.ExpectBegin()

    mock.ExpectExec("UPDATE accounts SET balance = balance - ?").
        WithArgs(100.0, 1).
        WillReturnResult(sqlmock.NewResult(0, 1)) // 1 行受影响

    mock.ExpectExec("UPDATE accounts SET balance = balance + ?").
        WithArgs(100.0, 2).
        WillReturnResult(sqlmock.NewResult(0, 1)) // 1 行受影响

    mock.ExpectCommit()

    err = Transfer(db, 1, 2, 100.0)
    assert.NoError(t, err)

    // 验证所有期望都被满足
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransfer_RollbackOnAccountNotFound(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()

    mock.ExpectBegin()

    mock.ExpectExec("UPDATE accounts SET balance = balance - ?").
        WithArgs(100.0, 1).
        WillReturnResult(sqlmock.NewResult(0, 1)) // 扣款成功

    mock.ExpectExec("UPDATE accounts SET balance = balance + ?").
        WithArgs(100.0, 999). // 不存在的账户
        WillReturnResult(sqlmock.NewResult(0, 0)) // 0 行受影响

    // defer tx.Rollback() 会被调用
    mock.ExpectRollback()

    err = Transfer(db, 1, 999, 100.0)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "加款账户不存在")

    assert.NoError(t, mock.ExpectationsWereMet())
}
```

**关键要点**：
- sqlmock 默认按注册顺序匹配期望，这与事务的执行顺序天然匹配
- `ExpectBegin()` / `ExpectCommit()` / `ExpectRollback()` 必须与代码中的事务操作一一对应
- `defer tx.Rollback()` 在 Commit 成功后是 no-op，sqlmock 不会因为多余的 Rollback 期望而报错

### 中间件链的 Mock 方案

Go 的 HTTP 中间件模式（如 Gin、Chi、Echo）在测试时经常需要 Mock 中间件行为，特别是认证、日志等横切关注点。

**场景**：测试需要认证中间件的 API Handler

```go
// middleware.go
package api

type UserContext struct {
    UserID int
    Role   string
}

// AuthMiddleware 从请求头解析 JWT 并注入用户信息
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        userCtx, err := parseJWT(token) // 调用外部 JWT 服务
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        ctx := context.WithValue(r.Context(), "user", userCtx)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func parseJWT(token string) (*UserContext, error) {
    // 实际会调用 JWT 验证服务
    return nil, errors.New("not implemented")
}
```

**方案 1：Mock JWT 解析函数（mockey）**

```go
func TestProtectedEndpoint_WithMockey(t *testing.T) {
    mockey.PatchConvey("认证中间件测试", t, func() {
        // Mock JWT 解析，返回预设用户
        mockey.Mock(parseJWT).To(func(token string) (*UserContext, error) {
            if token == "valid-token" {
                return &UserContext{UserID: 1, Role: "admin"}, nil
            }
            return nil, errors.New("invalid token")
        }).Build()

        handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := r.Context().Value("user").(*UserContext)
            fmt.Fprintf(w, "user_id=%d,role=%s", user.UserID, user.Role)
        }))

        // 测试有效 token
        req := httptest.NewRequest("GET", "/", nil)
        req.Header.Set("Authorization", "valid-token")
        rec := httptest.NewRecorder()
        handler.ServeHTTP(rec, req)

        assert.Equal(t, http.StatusOK, rec.Code)
        assert.Contains(t, rec.Body.String(), "user_id=1")
        assert.Contains(t, rec.Body.String(), "role=admin")

        // 测试无效 token
        req2 := httptest.NewRequest("GET", "/", nil)
        req2.Header.Set("Authorization", "bad-token")
        rec2 := httptest.NewRecorder()
        handler.ServeHTTP(rec2, req2)

        assert.Equal(t, http.StatusUnauthorized, rec2.Code)
    })
}
```

**方案 2：可测试的中间件设计（依赖注入）**

```go
// 重构中间件，将 JWT 解析作为依赖注入
type JWTValidator interface {
    Validate(token string) (*UserContext, error)
}

func AuthMiddlewareWithDI(validator JWTValidator, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        userCtx, err := validator.Validate(token)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        ctx := context.WithValue(r.Context(), "user", userCtx)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

```go
// 使用 gomock 测试
func TestProtectedEndpoint_WithDI(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockValidator := NewMockJWTValidator(ctrl)
    mockValidator.EXPECT().
        Validate("valid-token").
        Return(&UserContext{UserID: 1, Role: "admin"}, nil)

    handler := AuthMiddlewareWithDI(mockValidator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := r.Context().Value("user").(*UserContext)
        fmt.Fprintf(w, "user_id=%d", user.UserID)
    }))

    req := httptest.NewRequest("GET", "/", nil)
    req.Header.Set("Authorization", "valid-token")
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
}
```

**两种方案对比**：

| 维度       | mockey 打桩          | 依赖注入 + gomock  |
|------------|----------------------|--------------------|
| 代码侵入性 | 零（不改业务代码）   | 需重构中间件签名   |
| 类型安全   | 运行时检查           | 编译期保证         |
| 可维护性   | 依赖函数名，重构风险 | 依赖接口，重构安全 |
| 适用场景   | 遗留代码，快速验证   | 新项目，长期维护   |

### gRPC 服务的 Mock 方案

gRPC 在微服务架构中广泛使用，其 Mock 测试方案与 HTTP 有本质区别。

**场景**：订单服务调用库存服务的 gRPC 接口

```go
// inventory.proto
// service InventoryService {
//   rpc CheckStock(CheckStockRequest) returns (CheckStockResponse);
//   rpc ReserveStock(ReserveStockRequest) returns (ReserveStockResponse);
// }
```

**方案 1：使用 gomock 生成 gRPC 客户端 Mock**

```bash
# 使用 protoc-gen-go-gRPC 生成的接口 + mockgen
mockgen -source=inventory_grpc.pb.go -destination=mock/inventory_mock.go -package=mock
```

```go
func TestCreateOrder_CheckStock(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mock.NewMockInventoryServiceClient(ctrl)

    // 库存充足
    mockClient.EXPECT().
        CheckStock(gomock.Any(), &pb.CheckStockRequest{SkuId: "SKU001", Quantity: 10}).
        Return(&pb.CheckStockResponse{Available: true}, nil)

    // 预留库存
    mockClient.EXPECT().
        ReserveStock(gomock.Any(), &pb.ReserveStockRequest{SkuId: "SKU001", Quantity: 10}).
        Return(&pb.ReserveStockResponse{Success: true}, nil)

    service := NewOrderService(mockClient)
    order, err := service.CreateOrder("SKU001", 10)

    assert.NoError(t, err)
    assert.NotNil(t, order)
}
```

**方案 2：使用 gRPC 内置的 Fake Server**

gRPC 生态自带了更优雅的方案——在测试中启动一个真实的 gRPC 服务器，但使用 Fake 实现：

```go
func TestCreateOrder_WithFakeGRPCServer(t *testing.T) {
    // 启动测试 gRPC 服务器
    lis, err := net.Listen("tcp", "localhost:0")
    require.NoError(t, err)

    fakeServer := &FakeInventoryServer{
        stock: map[string]int{"SKU001": 100, "SKU002": 0},
    }

    s := grpc.NewServer()
    pb.RegisterInventoryServiceServer(s, fakeServer)
    go s.Serve(lis)
    defer s.Stop()

    // 连接测试服务器
    conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
    require.NoError(t, err)
    defer conn.Close()

    client := pb.NewInventoryServiceClient(conn)

    service := NewOrderService(client)
    order, err := service.CreateOrder("SKU001", 10)

    assert.NoError(t, err)
    assert.NotNil(t, order)
}

// Fake 实现：维护内存状态，行为更接近真实
type FakeInventoryServer struct {
    pb.UnimplementedInventoryServiceServer
    stock map[string]int
    mu    sync.Mutex
}

func (s *FakeInventoryServer) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.CheckStockResponse, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    return &pb.CheckStockResponse{
        Available: s.stock[req.SkuId] >= int(req.Quantity),
    }, nil
}

func (s *FakeInventoryServer) ReserveStock(ctx context.Context, req *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.stock[req.SkuId] < int(req.Quantity) {
        return &pb.ReserveStockResponse{Success: false}, nil
    }
    s.stock[req.SkuId] -= int(req.Quantity)
    return &pb.ReserveStockResponse{Success: true}, nil
}
```

**Fake Server vs Mock Client 的选择**：

| 维度     | Mock Client (gomock)     | Fake Server              |
|----------|--------------------------|--------------------------|
| 测试粒度 | 精确控制每次调用的返回值 | 模拟真实行为逻辑         |
| 测试覆盖 | 只能测预设路径           | 可以探索更多边界情况     |
| 维护成本 | 接口变更时需重新生成     | 需要维护 Fake 实现       |
| 运行速度 | 极快（无网络）           | 快（本地回环）           |
| 真实度   | 低（不经过 gRPC 栈）     | 高（完整 gRPC 调用链路） |

**建议**：核心业务逻辑用 Mock Client 精确测试边界条件；关键集成路径用 Fake Server 验证端到端行为。

### 并发场景下的 Mock 状态管理

并发测试中的 Mock 问题不只是"用哪个工具"，更是"如何保证 Mock 状态不泄漏"。

**问题 1：Mock 期望被并发消耗**

```go
// ❌ 错误：多个 goroutine 竞争同一个 EXPECT
func TestConcurrentFetch(t *testing.T) {
    ctrl := gomock.NewController(t)
    mockRepo := NewMockUserRepository(ctrl)

    // 只注册了 1 次期望，但 10 个 goroutine 都会调用
    mockRepo.EXPECT().GetUser(1).Return(&User{Name: "张三"}, nil)

    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            service.GetUserName(1) // 第 2 个 goroutine 开始 panic
        }()
    }
    wg.Wait()
}
```

**解决方案**：

```go
// ✅ 方案 1：使用 AnyTimes 允许任意调用次数
mockRepo.EXPECT().GetUser(gomock.Any()).AnyTimes().Return(&User{Name: "张三"}, nil)

// ✅ 方案 2：使用 mockey 的并发安全 Mock
mockey.PatchConvey("并发安全测试", t, func() {
    mockey.Mock(GetUser).To(func(id int) (*User, error) {
        return &User{Name: "张三"}, nil
    }).Build()

    // 多个 goroutine 安全并发调用
})

// ✅ 方案 3：使用 testify/mock 的 thread-safe 模式
mockRepo.On("GetUser", mock.Anything).Return(&User{Name: "张三"}, nil)
// testify/mock 内部有互斥锁保护
```

**问题 2：Mock 状态泄漏到其他测试**

```go
// ❌ 错误：httpmock 在 t.Cleanup 之前被其他测试看到
func TestA(t *testing.T) {
    httpmock.Activate()
    httpmock.RegisterResponder("GET", "https://api.example.com/a", ...)
    // 忘记 Deactivate
}

func TestB(t *testing.T) {
    // TestB 可能意外匹配到 TestA 注册的 Responder
    resp, _ := http.Get("https://api.example.com/a") // 返回了 TestA 的 Mock 数据
}
```

**解决方案**：

```go
// ✅ 始终使用 defer 确保清理
func TestA(t *testing.T) {
    httpmock.Activate()
    defer httpmock.DeactivateAndReset() // 确保清理

    httpmock.RegisterResponder("GET", "https://api.example.com/a", ...)
}

// ✅ 或者使用 t.Cleanup（Go 1.14+）
func TestA(t *testing.T) {
    httpmock.Activate()
    t.Cleanup(httpmock.DeactivateAndReset)
}
```

### 时间相关逻辑的 Mock

时间是测试中极其常见的痛点。`time.Now()` 和 `time.Sleep()` 如果不处理好，会导致测试不可控。

**方案 1：依赖注入时钟接口**

```go
// 定义时钟接口
type Clock interface {
    Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type fakeClock struct {
    fixed time.Time
}

func (c fakeClock) Now() time.Time { return c.fixed }

// 业务代码依赖时钟接口
type Scheduler struct {
    clock Clock
}

func (s *Scheduler) IsWeekend() bool {
    return s.clock.Now().Weekday() == time.Saturday || s.clock.Now().Weekday() == time.Sunday
}
```

```go
func TestIsWeekend(t *testing.T) {
    scheduler := &Scheduler{clock: fakeClock{fixed: time.Date(2024, 3, 2, 0, 0, 0, 0, time.UTC)}}
    assert.True(t, scheduler.IsWeekend()) // 2024-03-02 是周六
}
```

**方案 2：使用 mockey 打桩 time.Now**

```go
func TestIsWeekend_Mockey(t *testing.T) {
    mockey.PatchConvey("周末判断", t, func() {
        saturday := time.Date(2024, 3, 2, 0, 0, 0, 0, time.UTC)
        mockey.Mock(time.Now).To(func() time.Time {
            return saturday
        }).Build()

        scheduler := &Scheduler{clock: realClock{}}
        assert.True(t, scheduler.IsWeekend())
    })
}
```

**方案 3：使用专门的测试时钟库**

```go
import "go.uber.org/atomic"

type TestClock struct {
    now atomic.Int64 // Unix timestamp
}

func NewTestClock(t time.Time) *TestClock {
    c := &TestClock{}
    c.now.Store(t.Unix())
    return c
}

func (c *TestClock) Now() time.Time {
    return time.Unix(c.now.Load(), 0)
}

func (c *TestClock) Advance(d time.Duration) {
    c.now.Add(int64(d.Seconds()))
}

// 测试中可以推进时间
func TestScheduler(t *testing.T) {
    clock := NewTestClock(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)) // 周五
    scheduler := &Scheduler{clock: clock}

    assert.False(t, scheduler.IsWeekend()) // 周五

    clock.Advance(24 * time.Hour) // 推进 1 天到周六
    assert.True(t, scheduler.IsWeekend()) // 周六
}
```

**三种方案对比**：

| 方案         | 代码侵入性             | 灵活性                         | 适用场景             |
|--------------|------------------------|--------------------------------|----------------------|
| 时钟接口注入 | 中（需定义接口和注入） | 高（可精确控制、推进时间）     | 新项目，核心业务逻辑 |
| mockey 打桩  | 零                     | 中（可固定时间，不能推进）     | 遗留代码快速验证     |
| 专用测试时钟 | 中（需引入依赖）       | 最高（支持时间推进、并发安全） | 复杂时间相关业务     |

---

## 性能与可靠性深度分析

Mock 测试并非没有代价。本节从性能和可靠性两个维度深入分析 Mock 测试的隐性成本与应对策略。

### Mock 测试的执行性能瓶颈

Mock 测试通常比真实调用快，但不当使用反而会引入性能问题。

**1. 反射调用的开销**

testify/mock 依赖反射实现方法调度，每次 `Called()` 都涉及反射类型查找和参数装箱/拆箱：

```go
// testify/mock 内部核心逻辑（简化）
func (m *Mock) Called(args ...interface{}) Arguments {
    // 1. 遍历所有注册的 On() 调用，匹配方法名和参数 —— O(n) 复杂度
    // 2. 对每个参数执行 reflect.DeepEqual 比较
    // 3. 返回值通过反射装箱
}
```

当一个 Mock 对象上注册了大量 `On()` 调用时，匹配效率会线性下降。对于上千条 Mock 规则的测试套件，这并非可以忽略的开销。

**对比基准**（单次调用，微秒级）：

| 操作                | 耗时（ns/op） | 说明                 |
|---------------------|---------------|----------------------|
| 直接函数调用        | ~2            | 零开销               |
| 接口方法调用        | ~5            | 虚函数表查找         |
| gomock EXPECT 调用  | ~50           | 期望匹配 + 类型断言  |
| testify/mock Called | ~200          | 反射匹配 + 参数装箱  |
| mockey 函数打桩调用 | ~5            | 修改后的函数直接执行 |

**2. 代码生成的编译开销**

gomock 生成的 Mock 代码会增加编译时间。在一个有 50+ 接口的中型项目中，生成的 Mock 代码可能超过 5000 行，每次 `go test` 都需要编译这些代码。

**优化建议**：

```bash
# 利用 Go 的编译缓存，避免重复编译
go test -count=1 ./...  # -count=1 禁用缓存，用于基准测试
go test ./...            # 正常运行，利用缓存

# 将 Mock 生成与测试编译分离
mockgen -source=user.go -destination=mock/user_mock.go
# 提交生成的代码到版本控制，避免 CI 中重复生成
```

**3. 函数打桩的全局影响**

mockey 和 gomonkey 通过修改运行时函数指令实现打桩，这个过程本身有开销，但更严重的是对其他 goroutine 的影响——打桩是全局生效的，并发测试中的非预期干扰可能导致性能毛刺甚至测试失败。

### 不同工具的内存占用与 GC 压力

Mock 工具在运行时会创建额外的数据结构来维护期望和调用记录，这些都会增加内存分配和 GC 压力。

| 工具         | 主要内存开销来源                          | GC 影响                               |
|--------------|-------------------------------------------|---------------------------------------|
| gomock       | `ctrl` 对象、期望列表、调用记录           | 低，对象生命周期与测试函数一致        |
| testify/mock | `Mock` 对象、`Arguments` 切片（反射装箱） | 中，每次调用创建新的 `Arguments` 对象 |
| sqlmock      | 连接对象、期望队列、行数据                | 低，连接与测试函数生命周期绑定        |
| httpmock     | 全局 Transport、Responder 映射            | 低，但全局状态需手动清理              |
| mockey       | 打桩信息、函数指令备份                    | 极低，打桩信息为固定大小的结构体      |

**内存泄漏的常见模式**：

```go
// ❌ testify/mock：未清理的 On() 注册导致内存累积
func TestManyMocks(t *testing.T) {
    mockRepo := new(MockUserRepository)
    for i := 0; i < 10000; i++ {
        // 每次循环都注册新的 On()，旧的不清理
        mockRepo.On("GetUser", i).Return(&User{}, nil)
    }
    // mockRepo 的内部 map 膨胀到 10000 条
}

// ✅ 使用 Reset() 或创建新的 Mock 对象
func TestManyMocks(t *testing.T) {
    for i := 0; i < 10000; i++ {
        mockRepo := new(MockUserRepository) // 每次新建
        mockRepo.On("GetUser", i).Return(&User{}, nil)
        // 测试逻辑...
    }
}
```

### Mock 状态泄漏的诊断方法

Mock 状态泄漏是最隐蔽的测试问题之一：测试 A 设置的 Mock 状态被测试 B 意外使用，导致测试结果不稳定（时过时不过）。

**典型症状**：

- 测试单独运行通过，整体运行失败
- 测试执行顺序影响结果
- 并发测试中出现随机的断言失败

**诊断方法**：

```go
// 方法 1：使用 t.Cleanup 确保清理
func TestWithCleanup(t *testing.T) {
    httpmock.Activate()
    t.Cleanup(httpmock.DeactivateAndReset) // 无论测试是否失败，都会清理

    httpmock.RegisterResponder("GET", "https://api.example.com/users/1",
        httpmock.NewJsonResponderOrPanic(200, &User{Name: "张三"}))
    // ...
}

// 方法 2：使用子测试隔离 Mock 状态
func TestUserService(t *testing.T) {
    t.Run("GetUserName", func(t *testing.T) {
        ctrl := gomock.NewController(t) // 每个子测试独立的 Controller
        mockRepo := NewMockUserRepository(ctrl)
        // ...
    })

    t.Run("SaveUser", func(t *testing.T) {
        ctrl := gomock.NewController(t) // 独立的 Controller，不会互相干扰
        mockRepo := NewMockUserRepository(ctrl)
        // ...
    })
}

// 方法 3：使用 mockey.PatchConvey 自动管理生命周期
func TestWithMockey(t *testing.T) {
    mockey.PatchConvey("测试组", t, func() {
        mockey.Mock(GetConfig).To(func() map[string]string {
            return map[string]string{"debug": "true"}
        }).Build()
        // PatchConvey 结束时自动还原，不会泄漏
    })
}
```

**全局状态泄漏的检测工具**：

```bash
# 使用 -count 标志检测测试的不稳定性
go test -count=10 ./...  # 运行 10 次，检查是否有不一致的结果

# 使用 -race 检测数据竞争（可能是状态泄漏的信号）
go test -race ./...
```


### Mock 维护成本与测试金字塔的平衡

Mock 测试的维护成本是真实存在的。接口签名变更、业务逻辑调整都可能导致大量 Mock 代码需要同步修改。如何在测试覆盖率和维护成本之间取得平衡，是一个架构决策问题。

**测试金字塔模型**：

```
        /  E2E  \           少量，高成本，高价值
       / 集成测试  \         适量，验证模块间协作
      /  Mock 单元测试 \     大量，快速，稳定
     /   纯逻辑单元测试  \   大量，零依赖，最快
```

**Mock 测试的维护成本曲线**：

```
维护成本
  │
  │              ╱ 过度 Mock
  │            ╱   （Mock 代码 > 业务代码）
  │          ╱
  │        ╱  ─ ─ ─ 最佳平衡点
  │      ╱
  │    ╱ 适度 Mock
  │  ╱
  │╱ 欠缺 Mock
  └─────────────────────→ Mock 使用量
```

**降低维护成本的实践**：

1. **优先使用 Fake 而非 Mock**：Fake 实现一次，多处使用，接口变更只需修改一处
2. **最小化 Mock 范围**：只 Mock 外部边界，内部逻辑走真实代码
3. **利用代码生成**：gomock 的 `//go:generate` 确保 Mock 与接口同步
4. **共享测试套件**：将通用的 Mock 设置封装为 test helper
5. **定期审视 Mock 代码**：删除不再需要的 Mock，避免"僵尸测试"

---

## Mock、Fake 与集成测试的边界

单元测试中，Mock 和 Fake 是两种主流的隔离手段，而集成测试则选择了另一条路——不隔离，而是用真实依赖。理解三者的边界，才能在正确的场景做出正确的选择。

### Mock vs Fake：哲学分歧

Mock 和 Fake 的根本分歧在于**验证目标的抽象层次**：

- **Mock** 验证的是**交互契约**——"你有没有调用 `Save()` 方法？调了几次？参数是什么？"
- **Fake** 验证的是**状态结果**——"执行完操作后，查询结果是否符合预期？"

```go
// Mock 风格：验证交互
func TestCreateOrder_Mock(t *testing.T) {
    ctrl := gomock.NewController(t)
    mockRepo := NewMockOrderRepository(ctrl)

    mockRepo.EXPECT().Save(gomock.Any()).Return(nil)  // 验证 Save 被调用
    mockRepo.EXPECT().PublishEvent(gomock.Any()).Return(nil) // 验证事件发布

    service := NewOrderService(mockRepo)
    err := service.CreateOrder(&Order{ID: 1})

    assert.NoError(t, err)
    // ctrl.Finish() 会验证所有期望都被满足
}

// Fake 风格：验证状态
func TestCreateOrder_Fake(t *testing.T) {
    fakeRepo := NewFakeOrderRepository() // 内存实现
    service := NewOrderService(fakeRepo)

    err := service.CreateOrder(&Order{ID: 1, Item: "书"})
    assert.NoError(t, err)

    // 通过查询验证结果，而非验证方法调用
    order, _ := fakeRepo.FindByID(1)
    assert.Equal(t, "书", order.Item)
}
```

### Fake 的实现模式与适用场景

Fake 不是随意的简化实现，它需要在**足够真实**和**足够简单**之间取得平衡。

**1. 内存 Fake：最常见的模式**

```go
type FakeUserRepository struct {
    mu    sync.RWMutex
    users map[int]*User
    nextID int
}

func NewFakeUserRepository() *FakeUserRepository {
    return &FakeUserRepository{
        users:  make(map[int]*User),
        nextID: 1,
    }
}

func (r *FakeUserRepository) FindByID(id int) (*User, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    user, ok := r.users[id]
    if !ok {
        return nil, ErrNotFound
    }
    return user, nil
}

func (r *FakeUserRepository) Save(user *User) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if user.ID == 0 {
        user.ID = r.nextID
        r.nextID++
    }
    r.users[user.ID] = user
    return nil
}

func (r *FakeUserRepository) Delete(id int) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    delete(r.users, id)
    return nil
}
```

**2. Fake 的陷阱：与真实行为不一致**

```go
// ❌ Fake 忽略了真实数据库的唯一约束
func (r *FakeUserRepository) Save(user *User) error {
    r.users[user.ID] = user // 直接覆盖，没有唯一性检查
    return nil
}

// ✅ Fake 模拟真实数据库的唯一约束
func (r *FakeUserRepository) Save(user *User) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    for _, existing := range r.users {
        if existing.Email == user.Email && existing.ID != user.ID {
            return ErrDuplicateEmail // 模拟唯一约束冲突
        }
    }
    r.users[user.ID] = user
    return nil
}
```

**3. Fake 的适用场景**

| 场景          | Fake 是否合适 | 理由                                       |
|---------------|---------------|--------------------------------------------|
| 仓储层        | ✅ 非常合适    | 接口规整，Fake 实现简单且可复用            |
| 缓存层        | ✅ 合适        | 用 `map` + TTL 模拟缓存行为                |
| 消息队列      | ⚠️ 部分合适    | 可以模拟基本收发，但难以模拟消息丢失、重复 |
| 分布式锁      | ❌ 不合适      | 并发语义复杂，Fake 难以准确模拟            |
| 外部 HTTP API | ❌ 不合适      | 行为由第三方控制，Fake 无法保证一致性      |

### 集成测试：何时跳出 Mock 的舒适区

Mock 测试的信心上限是 **"代码是否按我理解的方式工作"**，而非 **"代码是否在真实环境中工作"**。集成测试弥补了这个差距。

**必须使用集成测试的场景**：

1. **SQL 语句的正确性**：sqlmock 只验证"你执行了这条 SQL"，但不验证 SQL 是否能被数据库正确执行
2. **事务边界的正确性**：Mock 无法发现事务未正确提交或回滚的问题
3. **并发安全性**：Mock 无法验证真实并发场景下的数据竞争
4. **第三方 API 兼容性**：Mock 的响应格式可能与 API 实际返回不一致

**集成测试的实用方案**：

```go
// 使用 Testcontainers 启动真实数据库
func TestUserRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过集成测试")
    }

    ctx := context.Background()

    // 启动 PostgreSQL 容器
    pgContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:15-alpine"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
    )
    require.NoError(t, err)
    defer pgContainer.Terminate(ctx)

    // 获取真实连接字符串
    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    // 使用真实数据库测试
    db, err := sql.Open("postgres", connStr)
    require.NoError(t, err)
    defer db.Close()

    // 运行迁移
    runMigrations(t, db)

    repo := NewUserRepository(db)

    // 真实数据库操作测试
    user := &User{Name: "张三", Email: "zhangsan@test.com"}
    err = repo.Save(user)
    assert.NoError(t, err)

    found, err := repo.FindByID(user.ID)
    assert.NoError(t, err)
    assert.Equal(t, "张三", found.Name)
}
```

### 三种策略的选择决策树

```
                     需要测试什么？
                          │
              ┌───────────┼────---───────┐
              │           │              │
         纯逻辑正确性？  交互行为正确性？  端到端正确性？
              │           │              │
         纯单元测试     需要隔离吗？      集成测试
         (无需隔离)        │           (真实依赖)
                    ┌────┼────┐
                    │         │
               状态驱动？   行为驱动？
                    │         │
                  Fake       Mock
```

**决策要点**：

| 问题                                | 选择       | 理由                              |
|-------------------------------------|------------|-----------------------------------|
| 测试的是领域逻辑？                  | 纯单元测试 | 无外部依赖，不需要隔离            |
| 需要验证方法调用顺序和次数？        | Mock       | 行为验证是 Mock 的核心优势        |
| 需要跨多个方法维护状态？            | Fake       | Fake 有真实的内部状态             |
| 需要验证 SQL 在真实数据库中的执行？ | 集成测试   | 只有真实数据库才能保证 SQL 正确性 |
| 测试的是 HTTP 协议处理？            | httpmock   | 专注协议层，不关心业务逻辑        |
| 需要验证事务隔离级别？              | 集成测试   | 事务语义无法通过 Mock 准确模拟    |

**混合策略：分层验证**

在一个健康的项目中，三种策略应该共存而非互斥：

```go
// 第 1 层：纯单元测试（无 Mock）
func TestUserValidation(t *testing.T) {
    user := &User{Name: "", Email: "bad"}
    err := user.Validate()
    assert.Error(t, err)
}

// 第 2 层：Mock 测试（交互验证）
func TestUserService_CreateUser(t *testing.T) {
    ctrl := gomock.NewController(t)
    mockRepo := NewMockUserRepository(ctrl)
    mockRepo.EXPECT().Save(gomock.Any()).Return(nil)

    service := NewUserService(mockRepo)
    err := service.CreateUser(&User{Name: "张三"})
    assert.NoError(t, err)
}

// 第 3 层：Fake 测试（状态验证）
func TestUserService_CreateAndFind(t *testing.T) {
    fakeRepo := NewFakeUserRepository()
    service := NewUserService(fakeRepo)

    service.CreateUser(&User{Name: "张三"})
    user, _ := service.FindByName("张三")
    assert.Equal(t, "张三", user.Name)
}

// 第 4 层：集成测试（端到端验证）
func TestUserRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过集成测试")
    }
    // 使用真实数据库...
}
```

---

## 常见问题与解决方案

### Mock 导致测试与实现脱节

**问题**：接口变了，Mock 没更新，测试通过了但生产挂了。

**解决方案**：

```go
// 使用 gomock 的 mockgen 自动生成，保证类型一致
//go:generate mockgen -source=user.go -destination=mock/user_mock.go

// 在测试文件中添加：
func TestMain(m *testing.M) {
    // 确保 Mock 实现了接口
    var _ UserRepository = (*MockUserRepository)(nil)
    os.Exit(m.Run())
}
```

### 私有方法如何 Mock

**问题**：第三方库的私有方法没法 Mock。

**解决方案**：

```go
// 方案 1：封装一层，暴露接口
type EmailSender interface {
    Send(to, subject, body string) error
}

type thirdPartySender struct {
    client *thirdparty.Client
}

func (s *thirdPartySender) Send(to, subject, body string) error {
    return s.client.privateSend(to, subject, body)
}

// 测试时 Mock EmailSender 接口即可

// 方案 2：使用 gomonkey 打桩（有平台限制）
patch := gomonkey.ApplyFunc(thirdparty.GetConfig, func () *Config {
    return &Config{Debug: true}
})
```

### 第三方库依赖如何处理

**问题**：代码依赖了无法修改的第三方库。

**解决方案**：

```go
// 方案 1：使用 gomonkey 打桩
patch := gomonkey.ApplyFunc(thirdparty.GetConfig, func() *Config {
    return &Config{Debug: true}
})

// 方案 2：封装适配器
type ConfigProvider interface {
    GetConfig() *Config
}

type thirdPartyAdapter struct{}

func (a *thirdPartyAdapter) GetConfig() *Config {
    return thirdparty.GetConfig()
}
```

### 并发测试中的 Mock 问题

**问题**：并发测试时 Mock 状态混乱。

**解决方案**：

```go
// ❌ 错误：共享 Mock 对象
func TestConcurrent(t *testing.T) {
    mockRepo := NewMockUserRepository(ctrl)

    for i := 0; i < 10; i++ {
        go func () {
            mockRepo.EXPECT().GetUser(gomock.Any()).Return(&User{}, nil)
        }()
    }
}

// ✅ 正确：每个 goroutine 独立 Mock
func TestConcurrent(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
wg.Add(1)
go func (id int) {
defer wg.Done()
ctrl := gomock.NewController(t)
mockRepo := NewMockUserRepository(ctrl)
// ...
}(i)
}
wg.Wait()
}
```

## 总结

### 工具选型速查表

| 场景                | 推荐工具     | 理由                         |
|---------------------|--------------|------------------------------|
| 大型项目，接口 Mock | gomock       | 官方推荐，类型安全，代码生成 |
| 快速原型，简单测试  | testify/mock | 语法简洁，无需生成           |
| 数据库测试          | sqlmock      | 专业工具，模拟 SQL 操作      |
| HTTP API 测试       | httpmock     | Mock HTTP 请求               |
| 现代项目，并发场景  | mockey       | 语法优雅，并发安全           |

### 测试策略选型速查表

| 测试目标              | 推荐策略              | 理由                            |
|-----------------------|-----------------------|---------------------------------|
| 领域逻辑正确性        | 纯单元测试（无 Mock） | 无外部依赖，最快最稳定          |
| 模块交互行为验证      | Mock                  | 精确控制行为，验证调用契约      |
| 跨方法状态流转验证    | Fake                  | 真实内部状态，可复用            |
| SQL 与事务正确性      | 集成测试              | 只有真实数据库才能保证 SQL 正确 |
| 服务间 API 契约一致性 | 合同测试（Pact）      | 消费者驱动，自动验证契约        |
| 系统级韧性验证        | 混沌工程              | 注入真实故障，验证降级策略      |

### 参考资料

**官方文档**：

- [gomock GitHub](https://github.com/golang/mock)
- [testify GitHub](https://github.com/stretchr/testify)
- [sqlmock GitHub](https://github.com/DATA-DOG/go-sqlmock)
- [httpmock GitHub](https://github.com/jarcoal/httpmock)
- [mockey GitHub](https://github.com/bytedance/mockey)

**推荐阅读**：

- [Go 测试最佳实践](https://go.dev/doc/tutorial/add-a-test)
- [Martin Fowler - TestDouble](https://martinfowler.com/bliki/TestDouble.html)
- [Martin Fowler - Mock Isn't A Stub](https://martinfowler.com/articles/mocksArentStubs.html)
- [六边形架构（Alistair Cockburn）](https://alistair.cockburn.us/hexagonal-architecture/)
- [Go Mock 实战指南](https://medium.com/@rosberryopensource/go-mock-implementation-tutorial-390e5e7c9c5f)