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

# Beta
apiVersion: batch/v1beta1
kind: CronJob

# 官方的 beta 曾经会放到 extensions 这个 groups 中
apiVersion: extensions/v1beta1
kind: Deployment

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

#### CronJobs（GA）

CronJobs 用于执行定期安排的操作，例如备份、生成报告等，开发者可以定义该间隔内作业应开始的时间点，并按照间隔不断执行。

注：在 v1.21 中 CronJob
默认使用性能更好的 [CronJobControllerV2](https://kubernetes.io/blog/2021/04/09/kubernetes-release-1.21-cronjob-ga/#performance-impact)。

> https://kubernetes.io/blog/2021/04/09/kubernetes-release-1.21-cronjob-ga/

#### Job 支持暂停（Alpha，v1.24 GA）

通过设置 `.spec.suspend` 为 `true` 实现，可能的使用场景：

- 优先级：让更重要 / 执行更快的 Job 先运行
- 延迟执行：让需要更多资源的 Job 挂起到闲时（如夜间）运行

> https://kubernetes.io/blog/2021/04/12/introducing-suspended-jobs/

#### 带索引的 Job（Alpha，v1.24 GA）

Job 默认是 `NonIndexed` 的，也就是说，Job Pod 应该被是为完全一致的，这样在分块处理数据时候不够方便。

k8s 1.21 中引入 Index Job 的概念，具体表现为将 `spec.completionMode` 设置为 `Indexed`
后，可以在注解 `batch.kubernetes.io/job-completion-index` 或环境变量 `JOB_COMPLETION_INDEX` 中获取到当前 Pod 的
Index，范围为：`[0, N)`。

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

#### Server-side Apply（GA）

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

#### StatefulSets 支持 minReadySeconds（Alpha，v1.25 GA）

在某些场景下，sts 的 Pod ready 不代表就能够提供服务，需要设置 `.spec.minReadySeconds` 来确保 sts 的 Pod 准备就绪（强制等待）。

> https://kubernetes.io/blog/2021/08/27/minreadyseconds-statefulsets/

#### PodSecurity Admission（Alpha, v1.23 Beta）

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

### k8s v1.23

#### IPv4/IPv6 双栈（GA）

在配置双栈网络时，需要同时指定 `--node-cidr-mask-size-ipv4` 和 `--node-cidr-mask-size-ipv6` 以便于设置每个 Node 上的子网大小（之前只需要设置 --node-cidr-mask-size）。

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

#### 支持 gRPC 探针（Alpha，v1.27 GA）

```yaml
readinessProbe:
  grpc:
    port: 9090
    service: grpc-service
  initialDelaySeconds: 5
  periodSeconds: 10
```

#### CRD Validation 支持表达式（Alpha，v1.25 Beta）

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

> https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/2876-crd-validation-expression-language/README.md
> 
> https://kubernetes.io/blog/2022/09/23/crd-validation-rules-beta/

#### 通用临时卷（GA）

通用临时卷（Generic Ephemeral Volume）是一种生命周期与 Pod 绑定的存储卷，当 Pod 被创建时，临时卷被创建并挂载到 Pod 中；当 Pod 被删除时，临时卷也随之被删除。

与 EmptyDir 的主要区别是：具有更强的扩展性，可以基于不同的存储驱动创建临时卷，除了 EmptyDir 之外，还可以使用其他存储类（StorageClass）来提供存储。同时，它支持更多的存储配置选项，如访问模式、资源请求、卷模式等，能够满足更复杂的存储需求。

> https://github.com/kubernetes/enhancements/blob/master/keps/sig-storage/1698-generic-ephemeral-volumes/README.md

## 小彩蛋

与 MacOS 一样，k8s 的每次大版本发布都有新的名字 & Logo，在 Release Note 中会说明 Logo 的设计理念，还是挺有创意的。

## 参考资料

- [Kubernetes Blog](https://kubernetes.io/blog/)
- [Kubernetes 1.20: The Raddest Release](https://kubernetes.io/blog/2020/12/08/kubernetes-1-20-release-announcement/)
- [Kubernetes 1.21: Power to the Community](https://kubernetes.io/blog/2021/04/08/kubernetes-1-21-release-announcement/)
- [Kubernetes 1.22: Reaching New Peaks](https://kubernetes.io/blog/2021/08/04/kubernetes-1-22-release-announcement/)
- [Kubernetes 1.23: The Next Frontier](https://kubernetes.io/blog/2021/12/07/kubernetes-1-23-release-announcement/)