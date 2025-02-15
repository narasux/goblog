## 背景

Kubernetes（k8s）作为 CNCF 认证的首个毕业项目，如今已近乎成为容器编排领域的事实标准。

自 2015 年 7 月推出 v1.0 版本以来，k8s 保持着大约每三个月发布一个新版本的节奏，持续迭代升级。截至 2025 年 2 月，其最新版本已更新至
v1.32 ，不断为用户带来更强大的功能与更好的性能表现。

在日常工作场景中，我们最常用的 `Ingress + Service + Deployment` 模式，已然是相当成熟的功能组合，就连更新的 `CronJob` 也早在
v1.20 版本就达到了 GA 标准。

随着 k8s 的持续演进，新的功能会不断地推出，因此本文将会简单介绍下这些新的功能，并且讨论下可能的应用场景。

## 相关知识

在 k8s 的版本发布体系中，功能通常会经历 Alpha、Beta 和 GA 三个主要阶段：

### 特性功能阶段

#### Alpha

**不稳定**：该阶段的功能是实验性的，意味着可能存在大量的漏洞、错误或不稳定的情况；其功能会快速迭代，接口、行为等都可能被修改。

**默认禁用**：一般情况下 Alpha 功能是默认关闭的，用户需要手动通过特定的标志（feature gates）来启用。

**缺乏测试**：可能只经过了有限的测试，对不同场景和配置的兼容性没有进行充分验证。

#### Beta

**相对稳定**：该阶段功能已经有显著的改进，稳定性和可靠性都有了大幅提升，但可能还有一些小问题。

**默认启用**：通常情况下 Beta 功能在发布时是默认开启的，但用户仍然可以根据自己的需求选择禁用。

**更多测试**：经过了更广泛的测试，包括不同的环境、配置和使用场景，对兼容性和性能有了更深入的了解。

#### GA（General Availability）

**稳定可靠**：GA 阶段的功能被认为是稳定、可靠且经过了充分验证的，可以在生产环境中放心使用。

**接口稳定**：接口和行为已经固定下来，在后续的版本中不会轻易发生变化，并且会保证向后兼容性。

**全面支持**：官方会为 GA 功能提供全面的技术支持和文档说明，用户在使用过程中遇到问题可以得到及时的帮助。

#### 如何识别

我们可以通过查看资源的 manifest 中的 apiVersion 字段，快速判断功能特性的版本：

```yaml
# Alpha
apiVersion: paas.bk.tencent.com/v1alpha2
kind: BkApp
```

```yaml
# Beta
apiVersion: batch/v1beta1
kind: CronJob
```

```yaml
# 官方的 beta 曾经会放到 extensions 这个 groups 中
apiVersion: extensions/v1beta1
kind: Deployment
```

```yaml
# GA
apiVersion: apps/v1
kind: Deployment 
```

## 功能特性

### k8s 1.x-1.20

在较老版本的 k8s 中，有几个版本的功能更新更具有代表性：

- k8s 1.6：RBAC 以 GA（通用可用性）状态发布，提供了细粒度的访问控制机制，允许根据角色和权限来管理用户对 Kubernetes 资源的访问。
- k8s 1.9：工作负载 API 成为 GA，Deployment 和 ReplicaSet 等工作负载对象，用户可以更稳定、可靠地使用这些资源来管理容器化应用的部署和扩展。
- k8s 1.10：CRI（容器运行时接口） 达到 GA 阶段，k8s 正式支持使用不同的容器运行时（如 Docker，Containerd 等）
- k8s 1.13：CSI（容器存储接口） 达到 GA 阶段，用户可以更加方便地使用各种第三方存储解决方案 & CoreDNS 成为默认 DNS。
- k8s 1.16：CRD（自定义资源定义） 进入 GA 阶段，开发者可以放心地使用 CRD 来扩展 Kubernetes 集群的功能，定义和管理自己的自定义资源。
- k8s 1.19：Ingress 进入 GA 阶段，开发者可以使用 Ingress 来实现外部访问的负载均衡，Ingress 的出现，替代了 NodePort /
  LoadBalancer Service 的部分场景。

### k8s v1.21

#### CronJobs（v1.21 GA (Current), v1.5 Alpha, v1.8 Beta）

CronJobs 用于执行定期安排的操作，例如备份、生成报告等，开发者可以定义该间隔内作业应开始的时间点，并按照间隔不断执行。

注：在 v1.21 中 CronJob 默认使用性能更好的 [CronJobControllerV2](https://kubernetes.io/blog/2021/04/09/kubernetes-release-1.21-cronjob-ga/#performance-impact)。

> https://kubernetes.io/blog/2021/04/09/kubernetes-release-1.21-cronjob-ga/

#### Job 支持暂停（v1.21 Alpha (Current), v1.22 Beta, v1.24 GA）

通过设置 `.spec.suspend` 为 `true` 实现，可能的使用场景：

- 优先级：让更重要 / 执行更快的 Job 先运行
- 延迟执行：让需要更多资源的 Job 挂起到闲时（如夜间）运行

> https://kubernetes.io/blog/2021/04/12/introducing-suspended-jobs/

#### 带索引的 Job（v1.21 Alpha (Current), v1.22 Beta, v1.24 GA）

Job 默认是 `NonIndexed` 的，也就是说，Job Pod 应该被是为完全一致的，这样在分块处理数据时候不够方便。

k8s 1.21 中引入 Index Job 的概念，具体表现为将 `spec.completionMode` 设置为 `Indexed` 后，可以在注解 `batch.kubernetes.io/job-completion-index` 或环境变量 `JOB_COMPLETION_INDEX` 中获取到当前 Pod 的 Index，范围为：`[0, N)`。

注：index job 的 Pod 名称中也会包含当前 Pod 的 Index。

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: 'sample-job'
spec:
  completions: 3
  parallelism: 3
  completionMode: Indexed    # 默认为 NoIndexed
  template:
    spec:
      restartPolicy: Never
      containers:
        - command:
            - 'bash'
            - '-c'
            - 'echo "My partition: ${JOB_COMPLETION_INDEX}"'
          image: 'docker.io/library/bash'
          name: 'sample-load'
```

> https://kubernetes.io/blog/2021/04/19/introducing-indexed-jobs/

### k8s v1.22

#### Server-side Apply（v1.22 GA (Current), v1.16 Beta）

```shell
kubectl apply --server-side [--dry-run=server]
```

本特性目标是把逻辑从 `kubectl apply` 移动到 kube-apiserver 中，这可以避免编辑时所有权冲突的问题，其是通过新增的 `.meta.managedFields` 属性来跟踪对象字段的更改。

其在 apiserver 中使用 **三元合并策略** 来处理配置的更新，其会对比客户端提交的新配置，服务器上当前的配置，以及最初应用的配置，决策如何进行更新。

#### 内存资源相关：使用 cgroup v2 优化内存 QoS（Alpha）+ 内存管理器（Beta）+ 内存交换（Alpha）

在 v1.22 之前， Kubernetes 使用的是 cgroups v1 ，对于 Pod 的 QoS 其实只适用于 CPU 资源。

Kubernetes v1.22 中通过引入 cgroups v2 来提供了一个 alpha 特性，允许对内存资源也提供 QoS。

在之前的版本中，Kubernetes 不支持在 Linux 上使用交换内存，因为当涉及交换时，很难提供保证并计算 pod 内存利用率。

但是，有许多例子证明可以从支持内存交换中受益，包括提高节点稳定性、更好地支持内存开销高但工作集较小的应用程序、使用内存受限的设备以及内存灵活性。

因此 k8s v1.22 中实验性地支持 Linux 内存交换。

> https://kubernetes.io/blog/2021/11/26/qos-memory-resources/
>
> https://kubernetes.io/blog/2021/08/11/kubernetes-1-22-feature-memory-manager-moves-to-beta/
>
> https://kubernetes.io/blog/2021/08/09/run-nodes-with-swap-alpha/

#### StatefulSets 支持 minReadySeconds（v1.22 Alpha (Current), v1.23 Beta, v1.25 GA）

在某些场景下，sts 的 Pod ready 不代表就能够提供服务，需要设置 `.spec.minReadySeconds` 来确保 sts 的 Pod 准备就绪（强制等待）。

> https://kubernetes.io/blog/2021/08/27/minreadyseconds-statefulsets/

#### PodSecurity Admission（v1.22 Alpha (Current), v1.23 Beta）

`PodSecurity Admission` 是在 k8s v1.21 中被废弃的 `Pod Security Policies` 的替代品。

这个 admission controller 可以按 namespace 级别启用 Pod Security Standards ，可以有以下三种模式：

```yaml
enforce: 违反策略的 Pod 将被拒绝
audit: 违反策略的 Pod 将会添加审计注释，但其他情况下都允许
warn: 违反策略的 Pod 将会触发面向用户的警告
```

可通过如下配置文件进行控制：

```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
  - name: PodSecurity
    configuration:
      # level 必须是 privileged, baseline, restricted 之一
      # version 是 k8s 版本，如 v1.22 / latest
      defaults:
        enforce: restricted
        enforce-version: latest
        audit: baseline
        audit-version: latest
        warn: privileged
        warn-version: latest
      # 豁免相关配置：用户 / 运行时 / 命名空间
      exemptions:
        usernames: [ ... ]
        runtimeClassNames: [ ... ]
        namespaces: [ ... ]
```

除了使用 AdmissionConfiguration 资源外，还可以通过给命名空间 / Pod 打标签来快速设置 PodSecurity：

```shell
kubectl create ns custom
kubectl label ns custom podsecurity.kubernetes.io/enforce=restricted
```

> https://kubernetes.io/docs/concepts/security/pod-security-admission/
>
> https://kubernetes.io/docs/concepts/security/pod-security-standards/
>
> https://kubernetes.io/blog/2021/12/09/pod-security-admission-beta/
>
> https://kubernetes.io/blog/2022/08/23/podsecuritypolicy-the-historical-context/

### k8s v1.23

#### IPv4/IPv6 双栈（GA）

在配置双栈网络时，需要同时指定 `--node-cidr-mask-size-ipv4` 和 `--node-cidr-mask-size-ipv6` 以便于设置每个 Node 上的子网大小（之前只需要设置 `--node-cidr-mask-size`）。

> https://kubernetes.io/blog/2021/12/08/dual-stack-networking-ga/
>
> https://github.com/kubernetes/enhancements/tree/master/keps/sig-network/563-dual-stack

#### IngressClass 支持 namespace 级别的参数（GA）

`.spec.parameters.namespace` 字段当前达到 GA ，这样就可以将 IngressClass 的参数配置设置为命名空间级（方便不同团队隔离配置，但 IngressClass 本身还是集群域资源）：

```yaml
apiVersion: k8s.example.com/v1
kind: IngressParameters
metadata:
  name: my-ingress-params
  namespace: my-namespace
spec:
  timeout: "30s"
  loadBalancingAlgorithm: "round-robin"
```

```yaml
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: my-ingress-class
spec:
  controller: example.com/ingress-controller
  parameters:
    apiGroup: k8s.example.com
    kind: IngressParameters
    name: my-ingress-params
    # scope 可以是 Namespace / Cluster，
    # 用于引用不同域类型的 IngressParameters 资源
    # IngressParameters 可以是自定义资源 / ConfigMap 之类
    scope: Namespace
    namespace: my-namespace
```

> https://github.com/kubernetes/enhancements/tree/master/keps/sig-network/2365-ingressclass-namespaced-params
>
> https://kubernetes.io/zh-cn/docs/reference/kubernetes-api/service-resources/ingress-class-v1/#IngressClassSpec

#### HPA v2（GA）

从此可以告别 `autoscaling/v2beta2` 版本，虽然 v2 相比于 v1 多了很多的功能支持（如外部，自定义指标等），对于目标类型也有更多的支持（不止是百分比）。

需要注意的是，v2beta1 / v2beta2 版本被废弃而更佳推荐 v2，但是 v1 版本的 HPA 还会继续存在较长的时间。

> https://kubernetes.io/zh-cn/docs/reference/kubernetes-api/workload-resources/horizontal-pod-autoscaler-v2/

#### 支持 gRPC 探针（v1.23 Alpha (Current), v1.24 Beta, v1.27 GA）

```yaml
readinessProbe:
  grpc:
    port: 9090
    service: grpc-service
  initialDelaySeconds: 5
  periodSeconds: 10
```

#### CRD Validation 支持表达式（v1.23 Alpha (Current), v1.25 Beta, v1.29 GA）

规则定义：[Common Expression Language (CEL)](https://github.com/google/cel-spec)，通过 `x-kubernetes-validation-rules` 字段配置校验规则。

```yaml
openAPIV3Schema:
  type: object
  properties:
    spec:
      type: object
      x-kubernetes-validation-rules:
        - rule: "self.minReplicas <= self.replicas"
          message: "replicas should be greater than or equal to minReplicas."
        - rule: "self.replicas <= self.maxReplicas"
          message: "replicas should be smaller than or equal to maxReplicas."
      properties:
        minReplicas:
          type: integer
        replicas:
          type: integer
        maxReplicas:
          type: integer
      required:
        - minReplicas
        - replicas
        - maxReplicas 
```

一些常见的验证规则：

| rule                                   | effect                           |
|----------------------------------------|----------------------------------|
| self.minReplicas <= self.replicas      | 整数字段小于或等于另一个整数字段                 |
| 'Available' in self.stateCounts        | Map 中是否存在具有 “Available” 键的条目     |
| self.set1.all(e, !(e in self.set2))    | 两个集合的元素是否不相交                     |
| self == oldSelf                        | 必填字段一旦设置便不可改变                    |
| self.created + self.ttl < self.expired | “过期” 日期是否晚于 “创建” 日期加上 “ttl” 持续时间 |

> https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/2876-crd-validation-expression-language/README.md
>
> https://kubernetes.io/blog/2022/09/23/crd-validation-rules-beta/
>
> https://kubernetes.io/blog/2022/09/29/enforce-immutability-using-cel/
>
> https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules

#### 通用临时卷（GA）

通用临时卷（Generic Ephemeral Volume）是一种生命周期与 Pod 绑定的存储卷，当 Pod 被创建时，临时卷被创建并挂载到 Pod 中；当 Pod 被删除时，临时卷也随之被删除。

与 EmptyDir 的主要区别是：具有更强的扩展性，可以基于不同的存储驱动创建临时卷，除了 EmptyDir 之外，还可以使用其他存储类（StorageClass）来提供存储。同时，它支持更多的存储配置选项，如访问模式、资源请求、卷模式等，能够满足更复杂的存储需求。

> https://github.com/kubernetes/enhancements/blob/master/keps/sig-storage/1698-generic-ephemeral-volumes/README.md

### k8s v1.24

#### 完全移除 DockerShim

Docker 作为底层运行时已被弃用，取而代之的是使用为 Kubernetes 创建的容器运行时接口 (CRI) 的运行时，但 Docker 生成的容器镜像依旧可用。

> https://kubernetes.io/blog/2022/05/03/dockershim-historical-context/
>
> https://kubernetes.io/blog/2020/12/02/dont-panic-kubernetes-and-docker/
>
> https://kubernetes.io/blog/2022/02/17/dockershim-faq/

#### OpenAPI v3

Kubernetes 1.24 提供以 OpenAPI v3 格式发布其 API 的测试版支持，同理 CRD 也可以通过 OpenAPI v3 的格式定义。

#### 卷容量扩展（GA）

此功能允许 Kubernetes 用户简单地编辑他们的 `PersistentVolumeClaim` 对象并在 `spec` 中指定新的大小。

Kubernetes 将自动使用存储后端扩展卷，并扩展 Pod 正在使用的底层文件系统，如果可能的话，根本不需要任何停机时间。

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: myclaim
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi  # 修改为新容量
```

但是，并非每种卷类型都默认可扩展，某些卷类型（如 `hostPath`）根本不可扩展。

对于 CSI 卷来说，则要求 CSI 驱动程序必须具有 `EXPAND_VOLUME` 功能的控制器或其他条件。

> https://kubernetes.io/blog/2022/05/05/volume-expansion-ga/

#### StatefulSet 的最大不可用副本数（v1.24 Alpha (Current), v1.25 Beta）

StatefulSet 有两种 Pod 管理策略：

- OrderedReady：严格保证 Pod 顺序（新建时从 0 -> N-1，缩容时从 N-1 -> 0 **逐个**变更）
- Parallel: 每个 Pod 都是独立的，可以同时创建 / 缩容删除

在 v1.24 中，StatefulSet 指定策略为 OrderedReady 时，可以配置 `spec.updateStrategy.rollingUpdate.maxUnavailable` (默认值为 1，即与原来的相同）以允许同时变更多个 Pod（但依旧按原先的顺序）。

> https://kubernetes.io/blog/2022/05/27/maxunavailable-for-statefulset/

### k8s v1.25

#### cgroup v2 (GA)

cgroup v2 是 Linux cgroup API 的最新版本，其提供了统一的控制系统，增强了资源管理功能。

许多最新版本的 Linux 发行版已默认切换到 cgroup v2，因此 Kubernetes 在这些新更新的发行版上继续运行良好非常重要。

cgroup v2 相对于 cgroup v1 有几项改进，例如：

- API 中单一统一的层次结构设计
- 更安全的子树委托给容器
- 压力失速信息等新功能
- 增强资源分配管理和跨多个资源的隔离
    - 对不同类型的内存分配（网络和内核内存等）进行统一核算
    - 考虑非即时资源变化，例如页面缓存写回

某些 Kubernetes 功能专门使用 cgroup v2 来增强资源管理和隔离。

例如：

- MemoryQoS 功能可提高内存利用率，并依赖 cgroup v2 功能来实现此功能。
- kubelet 中的新资源管理功能也将利用新的 cgroup v2 功能。

注意：k8s 使用 cgroup v2 的前提条件：

- Linux 内核 >= 5.8 & 启用 cgroup v2
- 容器运行时支持 cgroup v2（如：containerd v1.4+）
- kubelet 和容器运行时配置为使用 `systemd cgroup` 驱动程序

> https://kubernetes.io/blog/2022/08/31/cgroupv2-ga-1-25/

#### 临时容器（GA）

临时容器是一种特殊的，在已经存在的 Pod 中临时运行的容器，通常用于测试和排查问题的目的，可避免重新构建服务 / 启动 Pod。

临时容器并非为承载服务而设计，因此无法配置端口，容器探针，资源配额等功能，也不会自动重启。

特性：

- 共享命名空间：临时容器与目标容器共享网络、进程、IPC 等命名空间
- 生命周期：临时容器随 Pod 终止而销毁，但不会触发 Pod 重启
- 资源限制：可以定义 CPU/内存限制（需在 ephemeralContainers 字段中配置）

使用场景

- 调试崩溃的容器：当主容器因崩溃无法执行 kubectl exec 时，通过临时容器检查日志或进程
- 网络诊断：使用 nicolaka/netshoot 镜像执行网络排查（如 tcpdump、curl）
- 动态注入工具：临时加载性能分析工具（如 strace、perf）

使用实例（kubectl）：

```shell
# 语法
kubectl debug <pod-name> -it --image=<debug-image> --target=<container-name> -- <command>

# 示例：附加一个 busybox 临时容器到名为 "my-pod" 的 Pod
kubectl debug my-pod -it --image=busybox --target=my-container -- sh
```

参数说明：

- `-it`：以交互模式附加到容器。
- `--image`：指定临时容器的镜像（需包含调试工具，如 busybox、nicolaka/netshoot）。
- `--target`：指定目标容器名称（共享其命名空间，如网络、进程）。
- `--`：后续为要在临时容器中执行的命令（如 sh、bash）。

注意：临时容器创建后，无法手动删除，只能通过重建 Pod 的方式进行删除。

> https://kubernetes.io/docs/concepts/workloads/pods/ephemeral-containers/
>
> https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container
>
> https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/

### k8s v1.26

#### Pod scheduling gates（Alpha (Current), v1.27 Beta, v1.30 GA）

创建 Pod 后，调度程序会不断尝试寻找适合该 Pod 的节点。此无限循环将持续进行，直到调度程序找到适合该 Pod 的节点，或者该 Pod 被删除。

长时间无法调度的 Pod（例如，因某些外部事件而被阻止的 Pod）会浪费调度周期，大量无法成功调度的 Pod 可能会影响到整个集群的性能。

在 v1.26 中，Kubernetes 增加了 Pod scheduling gates，可以在调度 Pod 之前检查一些条件，如果满足条件则允许调度，否则不会调度该 Pod。

其与 Finalizer 非常相似，具有非空 `spec.schedulingGates` 字段的 Pod 将显示为状态 `SchedulingGated` 并被阻止调度。

```shell
NAME       READY   STATUS            RESTARTS   AGE
test-pod   0/1     SchedulingGated   0          10s
```

使用场景：集群资源配额不足，需要控制运行中的 Pod 数量，此时可以有 webhook 为同时创建的大量 Pod 添加 Scheduling Gates，并在有资源空闲的情况下逐步恢复 Pod 的调度（与 Job 的 suspend 效果类似）。

> https://kubernetes.io/blog/2022/12/26/pod-scheduling-readiness-alpha/

### k8s v1.27

#### PersistentVolumes 单 Pod 访问模式（v1.27 Beta (Current), v1.22 Alpha, v1.29 GA）

ReadWriteOncePod 访问模式可让您将卷访问限制到集群中的单个 Pod，从而确保一次只有一个 Pod 可以写入卷，这对于需要单写入者访问存储的有状态工作负载特别有用。

注意：ReadWriteOncePod 仅支持 CSI 卷且需要依赖较高版本的 CSI sidecar。

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: single-writer-only
spec:
  accessModes:
    - ReadWriteOncePod # Allow only a single pod to access single-writer-only.
  resources:
    requests:
      storage: 1Gi
```

其他几种 AccessModes:

- ReadWriteOnce：该卷可由单个节点以读写方式安装
- ReadOnlyMany：许多节点可以以只读方式安装该卷
- ReadWriteMany：该卷可由多个节点以读写方式安装

> https://kubernetes.io/blog/2023/04/20/read-write-once-pod-access-mode-beta/
> 
> https://kubernetes.io/blog/2021/09/13/read-write-once-pod-access-mode-alpha/#what-are-access-modes-and-why-are-they-important

#### Pod 资源原地调整（v1.27 Alpha (Current)）

在较低版本的 k8s 中，是无法调整运行中的 Pod 的容器资源（CPU/内存）的，只能通过重建 Pod 进行修改，这可能对运行中的服务造成一定的影响。

在 v1.27 中，k8s 增加了 Pod 的原地调整功能，可以直接对运行中的 Pod 进行调整（kubectl edit / patch resources），且无需重建 Pod。

由于 spec 中容器 `resources` 字段允许被修改，因此 k8s 在 `containerStatuses` 中新增 `allocatedResources` 字段，表示目前 **实际**分配给 Pod 中各容器的节点资源情况。

除此此外，`containerStatuses` 中新增名为 `resources` 字段，反映容器运行时所报告的，正在运行的容器上配置的资源请求和限制（`spec` 中的 `resources` 不一定已经生效）。

最后，Pod 状态中还新增了一个 `resize` 字段，用于显示最近一次资源调整状态：

- Proposed：对请求的资源调整的确认，表明该请求已通过验证并记录。
- InProgress：值表示节点已接受资源调整请求，并且正在将该请求应用到 Pod 的容器上。
- Deferred：当前无法批准所请求的资源调整，节点将持续重试直到有足够的资源。
- Infeasible：节点无法满足所请求的资源调整，比如 Pod 要求的资源超过单节点上限。

> https://kubernetes.io/blog/2023/05/12/in-place-pod-resize-alpha/

### k8s v1.28

#### 原生 Sidecar 容器（v1.28 Alpha (Current), v1.29 Beta, v1.33 GA）

Kubernetes 1.28 向 init 容器添加了一个新字段：`restartPolicy`：

```yaml
apiVersion: v1
kind: Pod
spec:
  initContainers:
  - name: secret-fetch
    image: secret-fetch:1.0
  - name: network-proxy
    image: network-proxy:1.0
    restartPolicy: Always
  containers: [...]
```

此字段是可选的，如果设置，则唯一有效值为 Always，启用后有如下变化：

- 如果 init 容器退出，则重新启动（普通的 init 容器会执行完后退出）
- 任何后续的 init 容器在 startupProbe 成功完成后立即启动，而不是等待 sidecar 容器退出
- 由于可重启的 init 容器资源现在被添加到主容器的资源请求总和中，因此 Pod 的资源使用量计算发生了变化
- Pod 终止仍然仅取决于主容器，原生 sidecar 容器不会阻止 Pod 的退出

相比较于 sidecar 和主容器一起放 `containers` 中的优势：1. 提供启动顺序的控制（确保 sidecar 先启动），2. 不阻止 Pod 终止

碎碎念：非得用 initContainers 这个字段么，感觉可能会有歧义 :）

> https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/

### k8s v1.29

v1.29 中的新功能（Alpha）主要是底层的一些改进（或者是对 windows 系统的支持），和具体的应用关系不大，跳过。

不过还是有挺多功能在这个版本变成 Beta / GA，比如：CRD Validation 表达式，原生 Sidecar 容器等等。

### k8s v1.30

#### ValidatingAdmissionPolicy （v1.30 GA (Current), v1.24 Alpha, v1.28 Beta）

ValidatingAdmissionPolicy 是 AdmissionWebhook 的替代方案，以下两段配置是等价的：

```go
func verifyDeployment(deploy *appsv1.Deployment) error {
	var errs []error
	for i, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == "" {
			return fmt.Errorf("container %d has no name", i)
		}
		if c.SecurityContext == nil {
			errs = append(errs, fmt.Errorf("container %q does not have SecurityContext", c.Name))
		}
		if c.SecurityContext.RunAsNonRoot == nil || !*c.SecurityContext.RunAsNonRoot {
			errs = append(errs, fmt.Errorf("container %q must set RunAsNonRoot to true in its SecurityContext", c.Name))
		}
		if c.SecurityContext.ReadOnlyRootFilesystem == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
			errs = append(errs, fmt.Errorf("container %q must set ReadOnlyRootFilesystem to true in its SecurityContext", c.Name))
		}
		if c.SecurityContext.AllowPrivilegeEscalation != nil && *c.SecurityContext.AllowPrivilegeEscalation {
			errs = append(errs, fmt.Errorf("container %q must NOT set AllowPrivilegeEscalation to true in its SecurityContext", c.Name))
		}
		if c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			errs = append(errs, fmt.Errorf("container %q must NOT set Privileged to true in its SecurityContext", c.Name))
		}
	}
	return errors.NewAggregate(errs)
}
```

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "pod-security.policy.example.com"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
    - apiGroups:   ["apps"]
      apiVersions: ["v1"]
      operations:  ["CREATE", "UPDATE"]
      resources:   ["deployments"]
  # 变量
  variables:
  - name: containers
    expression: object.spec.template.spec.containers
  - name: securityContexts
    expression: 'variables.containers.map(c, c.?securityContext)'
  # 表达式
  validations:
  - expression: variables.securityContexts.all(c, c.?runAsNonRoot == optional.of(true))
    message: 'all containers must set runAsNonRoot to true'
  - expression: variables.securityContexts.all(c, c.?readOnlyRootFilesystem == optional.of(true))
    message: 'all containers must set readOnlyRootFilesystem to true'
  - expression: variables.securityContexts.all(c, c.?allowPrivilegeEscalation != optional.of(true))
    message: 'all containers must NOT set allowPrivilegeEscalation to true'
  - expression: variables.securityContexts.all(c, c.?privileged != optional.of(true))
    message: 'all containers must NOT set privileged to true'
```

需要注意的是：创建 `ValidatingAdmissionPolicy` 后，还得通过 `ValidatingAdmissionPolicyBinding` 绑定到具体的一批 k8s 资源上：

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: "pod-security.policy-binding.example.com"
spec:
  policyName: "pod-security.policy.example.com"
  validationActions: ["Warn"]
  matchResources:
    namespaceSelector:
      matchLabels:
        "kubernetes.io/metadata.name": "policy-test"
```

> https://kubernetes.io/blog/2024/04/24/validating-admission-policy-ga/

#### 只读卷挂载变成真的只读（v1.30 Alpha (current), v1.31 Beta）

在 k8s 中，指定 `volumeMounts` 中的 `readOnly` 为 `true`，只能限制到挂在的目录，比如下面这个例子中的 `/mnt/*`，而它的子目录其实是没法限制的，也就是说 `/mnt/my-nfs-server/*` 还是可以写入的：

```yaml
apiVersion: v1
kind: Pod
spec:
  volumes:
    - name: mnt
      hostPath:
        path: /mnt
  containers:
    - volumeMounts:
      - name: mnt
        mountPath: /mnt
        readOnly: true
```

在 v1.30 版本并开启特性的 featureGate 后，可以在 `readOnly` 为 `true` 的前提下，设置 `recursiveReadOnly` 为 `Enabled` 以限制子目录为只读：

```yaml
readOnly: true
# NEW
# Possible values are `Enabled`, `IfPossible`, and `Disabled`.
# Needs to be specified in conjunction with `readOnly: true`.
recursiveReadOnly: Enabled
```

> https://kubernetes.io/blog/2024/04/23/recursive-read-only-mounts/

## Kubernetes Logo 彩蛋

与 MacOS 一样，k8s 的每次大版本发布都有新的名字 & Logo，在 Release Note 中会说明 Logo 的设计理念：

![img](/static/image/blog/k8s-logo.png)

### v1.22: Reaching New Peaks

在持续的疫情、自然灾害和无处不在的倦怠阴影中，Kubernetes 1.22 版本包含 53 项增强功能。这使其成为迄今为止最大的版本。

这一成就的实现完全归功于辛勤工作和充满热情的发布团队成员以及 Kubernetes 生态系统的杰出贡献者。

发布 Logo 提醒我们继续实现新的里程碑并创造新的记录。它献给所有发布团队成员、贡献者和观星者！

### v1.24：Stargazer

从古代天文学家到建造詹姆斯·韦伯太空望远镜的科学家，一代又一代的人都怀着敬畏和惊奇的心情仰望星空。

星星启发了我们，点燃了我们的想象力，并指引我们度过艰难的海上漫漫长夜。

借助此版本，我们展望未来，看看当我们的社区团结起来时，一切皆有可能。

Kubernetes 是全球数百名贡献者和数千名最终用户的成果，他们支持为数百万人服务的应用程序。

每个人都是我们天空中的一颗星星，帮助我们规划方向。

### v1.28 Planternetes

Kubernetes 的每个版本均由全球数千名多元背景的贡献者（行业专家、学生、开源新手等）协作构建，融合独特经验打造技术杰作。

该版本的 “花园” 主题象征社区成员如植物般在挑战中共同成长，通过精心培育实现生态繁荣。

这种协作精神贯穿版本迭代，推动 Kubernetes 在变化与机遇中持续演进，彰显开源社区的凝聚力与创造力。

### v1.29 Mandala

Kubernetes 社区如同曼陀罗艺术，象征多元协作的宇宙和谐。

全球贡献者（行业专家、学生、开源新手等）如同曼陀罗的繁复图案，各自以独特技能（代码开发、文档维护、漏洞修复等）编织技术生态。

正如曼陀罗创作依赖每个元素的精准配合，Kubernetes 的迭代也依托 SIG 小组协作（如 SIG Node、SIG Docs）与 KEP 提案机制，实现功能优化与安全增强。

这种共生关系既体现技术严谨性，也传递开源社区的人文温度。

### v1.30 Uwubernetes

Kubernetes 由全球数千名志愿者无偿构建，他们因兴趣、学习或热爱而参与，并在此找到归属。

v1.30 版本命名为 Uwubernetes（融合“Kubernetes”与表情符号“UwU”），象征社区的奇特，可爱与快乐。

这一设计致敬所有贡献者，感谢他们让集群稳定运行，并传递内外交融的独特热情。

## 挖个坑吧

k8s 目前有个新的网络管理资源 Gateway，是 Ingress 的进阶方案，有更强灵活性 & 可扩展性，可以满足更复杂的网络治理需求，后续可以研究下，争取再写个博客。

## 参考资料

- [Kubernetes Blog](https://kubernetes.io/blog/)
- [Kubernetes 1.18: Fit & Finish](https://kubernetes.io/blog/2020/03/25/kubernetes-1-18-release-announcement/)
- [Kubernetes 1.19: Accentuate the Paw-sitive](https://kubernetes.io/blog/2020/08/26/kubernetes-release-1.19-accentuate-the-paw-sitive/)
- [Kubernetes 1.20: The Raddest Release](https://kubernetes.io/blog/2020/12/08/kubernetes-1-20-release-announcement/)
- [Kubernetes 1.21: Power to the Community](https://kubernetes.io/blog/2021/04/08/kubernetes-1-21-release-announcement/)
- [Kubernetes 1.22: Reaching New Peaks](https://kubernetes.io/blog/2021/08/04/kubernetes-1-22-release-announcement/)
- [Kubernetes 1.23: The Next Frontier](https://kubernetes.io/blog/2021/12/07/kubernetes-1-23-release-announcement/)
- [Kubernetes 1.24：Stargazer](https://kubernetes.io/blog/2022/05/03/kubernetes-1-24-release-announcement/)
- [Kubernetes v1.25: Combiner](https://kubernetes.io/blog/2022/08/23/kubernetes-v1-25-release/)
- [Kubernetes v1.26: Electrifying](https://kubernetes.io/blog/2022/12/09/kubernetes-v1-26-release/)
- [Kubernetes v1.27: Chill Vibes](https://kubernetes.io/blog/2023/04/11/kubernetes-v1-27-release/)
- [Kubernetes v1.28: Planternetes](https://kubernetes.io/blog/2023/08/15/kubernetes-v1-28-release/)
- [Kubernetes v1.29: Mandala](https://kubernetes.io/blog/2023/12/13/kubernetes-v1-29-release/)
- [Kubernetes v1.30: Uwubernetes](https://kubernetes.io/blog/2024/04/17/kubernetes-v1-30-release/)
- [Kubernetes v1.31: Elli](https://kubernetes.io/blog/2024/08/13/kubernetes-v1-31-release/)
- [Kubernetes v1.32: Penelope](https://kubernetes.io/blog/2024/12/11/kubernetes-v1-32-release/)