[mapx](https://github.com/TencentBlueKing/gopkg/tree/master/mapx) 是简单实用的 Golang Map 工具包，它提供了 Map 键检测、嵌套值获取/赋值、对象比较等实用能力。

## Getter

Getter 类方法提供根据指定路径从嵌套 Map 结构中取值的能力，减轻编写递归/循环逻辑的负担。

核心函数 `GetItems` 通过提供的 paths 参数（可以是 `string` 或 `[]string` 类型）从嵌套的 map 中获取指定路径上的值。当 paths 是字符串时，函数会根据 `.` 符号将其分割成多个子路径。GetItems 的内部实现通过递归方式在多层嵌套的 map 中追踪给定的路径。

除此之外还提供了一系列快捷函数，如 `GetBool`、`GetInt64`、`GetStr`、`GetList` 和 `GetMap`。这些函数能够迅速获取到期望的数据类型，并提供默认值。例如，如果找不到指定路径上的值，`GetInt64` 函数将返回一个默认值 `int64(0)`。

此外，`Get` 函数允许开发者自定义默认值。当路径上的值不存在时，函数将返回传入的默认值，这样可以简化处理嵌套 Map 结构的过程。

示例代码如下，可以点击 [这里](https://go.dev/play/p/YqS2eqOI6A6) 运行试试

```go
package main

import (
 "fmt"

 "github.com/TencentBlueKing/gopkg/mapx"
)

func main() {
 manifest := map[string]any{
  "metadata": map[string]any{
   "namespace": "kube-system",
   "name":      "kube-proxy-c6zwn",
  },
  "spec": map[string]any{
   "replicas": int64(3),
   "selector": map[string]any{
    "matchLabels": map[string]any{
     "app": "kube-proxy",
    },
   },
  },
 }

 // GetItems 方法支持根据指定路径，从嵌套的 Map 中获取值
 name, err := mapx.GetItems(manifest, "metadata.name")
 // kube-proxy-c6zwn, <nil>
 fmt.Println(name, err)

 // 说指定路径的值不存在，则返回非空 error
 strategy, err := mapx.GetItems(manifest, "spec.strategy")
 // <nil>, key strategy not exist
 fmt.Println(strategy, err)

 // Get 方法支持根据指定路径，从嵌套的 Map 中获取值，若获取失败，返回提供的默认值
 noExists := mapx.Get(manifest, "spec.noExists", map[string]any{"foo": "bar"})
 // map[foo:bar]
 fmt.Println(noExists)

 // GetInt64，GetStr, GetList, GetMap, GetBool 等快捷方法，
 // 支持返回具体类型的数据，默认值为对应类型的空值
 name = mapx.GetStr(manifest, "metadata.name")
 // kube-proxy-c6zwn
 fmt.Println(name)

 replicas := mapx.GetInt64(manifest, "spec.replicas")
 // 3
 fmt.Println(replicas)

 noExists = mapx.GetMap(manifest, "spec.noExists")
 // map[]
 fmt.Println(noExists)
}
```

## Setter

Setter 类方法与 Getter 相反，它提供根据指定路径为嵌套 Map 结构中赋值的能力。

核心函数 `SetItems` 接收三个参数：一个嵌套的 map 结构 `obj`，一个指定的路径 `paths` 以及一个值 `val`。paths 参数和 `GetItems` 的一样，也支持 `string` 和 `[]string` 两种类型，如果路径参数不是以上两种类型之一，则会返回 `ErrInvalidPathType` 错误。

该函数本身是递归实现的，其会首先检测是否已到达嵌套 map 的最后一层，若已到达，则将指定值赋给对应的 Key（不论原始值是否存在）。若尚未到达，则会检查当前 key 是否存在且其值类型是否为 `map[string]any`，若满足条件则会递归调用，直到路径被遍历完毕。若中途遇到类型不匹配等问题，则会返回具体的错误。

示例代码如下，可以点击 [这里](https://go.dev/play/p/LvYuBK3X73p) 运行试试

```go
package main

import (
 "fmt"

 "github.com/TencentBlueKing/gopkg/mapx"
)

func main() {
 manifest := map[string]any{
  "metadata": map[string]any{
   "namespace": "kube-system",
   "name":      "kube-proxy-c6zwn",
  },
  "spec": map[string]any{
   "replicas": int64(3),
  },
 }

 // SetItems 方法能给嵌套的 Map 赋值
 err := mapx.SetItems(manifest, "spec.replicas", int64(5))
 // <nil>
 fmt.Println(err)

 // 指定路径前缀不存在
 err = mapx.SetItems(manifest, "status.replicas", int64(5))
 // key status not exists or obj[key] not map[string]interface{} type
 fmt.Println(err)

 // 路径某个节点类型并不是 map[string]any
 err = mapx.SetItems(manifest, "spec.replicas.foo", "bar")
 // key replicas not exists or obj[key] not map[string]interface{} type
 fmt.Println(err)
}
```

## Differ

在日常编程中，我们有时会需要比较两个 Map 之间的差异，以获取具体的新增、修改和删除的细节。

为解决这个问题， mapx 提供了 Differ 工具，其实现部分参考了 Python 中的 [dictdiffer](https://dictdiffer.readthedocs.io/en/latest/)，是一个简单可靠的 Map 对比工具。

Differ 的使用非常简单：`mapx.NewDiffer(oldMap, newMap).Do()`，其中 oldMap, newMap 类型均需要为 `map[string]any`。

Differ 执行返回的结果类型为 `[]DiffRet (alias: DiffRetList)`，`DiffRet` 拥有四个属性，分别是：

- Action：Diff 结果类型，分为 新增，删除，变更 三种
- Dotted：元素路径，为 `.` 拼接而成的字符串，比如 `metadata.name`；需要注意的是，若路径中某个 key 包含 `.` 则输出结果中会包含小括号，如 []string{"k1", "k2.2", "k3"} -> "k1.(k2.2).k3"
- OldVal：原始值，当 Action 为 `新增` 时，该值为 nil
- NewVal：变更值，当 Action 为 `删除` 时，该值为 nil

除此之外，DiffRetList 支持进行排序，默认输出结果将会根据 Action 类型以及 Dotted 字母序排序。

Differ 的实现核心的是 `handle`，`handleMap`，`handleList` 三个方法，其分别处理 `any`，`map[string]any`，`[]any` 这三种不同的数据类型，具体的处理逻辑如下所示：

![img](/static/image/blog/mapx.Differ_workflow.png)

注意：子项比较只支持 `[]any{}`, `map[string]any` 类型

举个例子：

`[]int{1, 2} -> []int{1, 2, 3}` => `DiffRet{Action: "Change", ...}`

`[]any{1, 2} -> []any{1, 2, 3}` => `DiffRet{Action: "Add", Dotted: "xxx[2]", NewVal: 3}`

示例代码如下，可以点击 [这里](https://go.dev/play/p/xUuSbOr6X9H) 运行试试

```go
package main

import (
 "fmt"

 "github.com/TencentBlueKing/gopkg/mapx"
)

func main() {
 oldMap := map[string]any{
  "a1": map[string]any{
   "b1": map[string]any{
    "c1": map[string]any{
     "d1": "v1",
     "d2": "v2",
     "d3": 3,
     "d4": []any{4, 5},
     "d5": nil,
     "d6": []any{
      6.1, 6.2, 6.3, 6.4, 6.5,
     },
    },
   },
  },
  "a2": []any{
   map[string]any{
    "b2": map[string]any{
     "c2": []any{
      "d1",
      map[string]any{
       "e1": "v1",
       "e2": "v2",
      },
      map[string]string{
       "e3": "v3",
       "e4": "v4",
      },
      2,
     },
    },
    "b3": []any{
     "c3", "c4", 5,
    },
   },
  },
 }

 newMap := map[string]any{
  "a1": map[string]any{
   "b1": map[string]any{
    "c1": map[string]any{
     "d1": "v1",
     // change a1.b1.c1.d2 v2->v1
     "d2": "v1",
     // remove a1.b1.c1.d3 ...
     // add a1.b1.c1.d7 ...
     "d7": 3,
     // remove a1.b1.c1.d4[1] ...
     "d4": []any{4},
     // change a1.b1.c1.d5 nil->"nil"
     "d5": "nil",
     // change a1.b1.c1.d6[2] 6.3->6.4
     // change a1.b1.c1.d6[3] 6.4->6.5
     // change a1.b1.c1.d6[4] 6.5->6.3
     "d6": []any{
      6.1, 6.2, 6.4, 6.5, 6.3,
     },
    },
   },
  },
  "a2": []any{
   map[string]any{
    "b2": map[string]any{
     "c2": []any{
      // change a2[0].b2.c2[0] d1->d2
      "d2",
      map[string]any{
       // change a2[0].b2.c2[1].e1 v1->v2
       "e1": "v2",
       // remove a2[0].b2.c2[1].e2 ...
       // add a2[0].b2.c2[1].e3 ...
       "e3": "v2",
       // add a2[0].b2.c2[1].e4 ...
       "e4": "v4",
       // add a2[0].b2.c2[1].(e5.f1) ...
       "e5.f1": "v5",
      },
      // change a2[0].b2.c2[2] ...
      map[string]string{
       "e3": "v4", // 只是 v3->v4, 但是 map[string]string 不会展开
       "e4": "v4",
      },
      // change a2[0].b2.c2[3] 2->1
      1,
      // add a2[0].b2.c2[4] 2
      2,
     },
    },
    // change a2[0].b3[0] "c3"->"c4"
    // change a2[0].b3[2] 5->6
    // add a2[0].b3[3] 7
    "b3": []any{
     "c4", "c4", 6, 7,
    },
   },
  },
  // add a3 ...
  "a3": map[string]any{
   "b4": "v1",
  },
 }

 diffRets := mapx.NewDiffer(oldMap, newMap).Do()
 // Action  Dotted                  OldVal             NewVal
 // Add     a1.b1.c1.d7             <nil>              3
 // Add     a2[0].b2.c2[1].(e5.f1)  <nil>              v5
 // Add     a2[0].b2.c2[1].e3       <nil>              v2
 // Add     a2[0].b2.c2[1].e4       <nil>              v4
 // Add     a2[0].b2.c2[4]          <nil>              2
 // Add     a2[0].b3[3]             <nil>              7
 // Add     a3                      <nil>              map[b4:v1]
 // Change  a1.b1.c1.d2             v2                 v1
 // Change  a1.b1.c1.d5             <nil>              nil
 // Change  a1.b1.c1.d6[2]          6.3                6.4
 // Change  a1.b1.c1.d6[3]          6.4                6.5
 // Change  a1.b1.c1.d6[4]          6.5                6.3
 // Change  a2[0].b2.c2[0]          d1                 d2
 // Change  a2[0].b2.c2[1].e1       v1                 v2
 // Change  a2[0].b2.c2[2]          map[e3:v3 e4:v4]   map[e3:v4 e4:v4]
 // Change  a2[0].b2.c2[3]          2                  1
 // Change  a2[0].b3[0]             c3                 c4
 // Change  a2[0].b3[2]             5                  6
 // Remove  a1.b1.c1.d3             3                  <nil>
 // Remove  a1.b1.c1.d4[1]          5                  <nil>
 // Remove  a2[0].b2.c2[1].e2       v2                 <nil>
 fmt.Println("Action\tDotted\tOldVal\tNewVal")
 for _, r := range diffRets {
  fmt.Println(r.Action, r.Dotted, r.OldVal, r.NewVal)
 }
}
```

## 总结

mapx 是一个高效实用的 Golang Map 工具包，其旨在简化处理 Map 数据结构的各类任务。mapx 工具包易于集成，使用简单，配合现有的 Go 项目或库，可以大幅提高编码效率和代码可读性。在复杂 Map 数据处理以及 Map 差异检查等场景，mapx 都将是您的得力助手 :D
