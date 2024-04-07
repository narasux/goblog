# Scaling in kubernetes

> Kubernetes 为我们的服务部署与编排提供了巨大的便利，针对集群中的服务规模的调整，被我们称为扩缩容，本文将介绍 k8s 集群中的各类扩缩容场景。

## 手动扩缩容

### 调节副本数量（Scale）

我们可以通过 kubectl scale 命令，手动调节 Deployment 的副本数量

```bash
# 优雅的做法
kubectl scale deployment -n blueking svc-rabbitmq --replicas=1

# 粗暴的做法
kubectl edit deployment -n blueking svc-rabbitmq
```

## 自动扩缩容

### Pod 水平自动扩缩容（HPA/GPA）

HorizontalPodAutoscaler（HPA）能够自动更新工作负载资源（例如 Deployment 或 StatefulSet）的副本数量，目的是自动扩缩 Pod 数量以满足需求。

#### HPA 工作原理

![img](/static/image/blog/hpa_workflow.png)

HorizontalPodAutoscaler 控制 Deployment 及其 ReplicaSet 的规模，从而达到根据资源使用情况，控制 Pod 数量的效果；Pod 水平自动扩缩实现为一个间歇运行的控制回路，默认间隔为 15 秒。

在每个时间段内，控制器管理器都会根据每个 HorizontalPodAutoscaler 定义中指定的指标查询资源利用率。控制管理器找到由 scaleTargetRef 定义的目标资源，然后根据目标资源的 `.spec.selector` 标签选择 Pod，并从资源指标 API 或自定义指标获取指标 API。

对于按 Pod 统计的资源指标（如 CPU），控制器从资源指标 API 中获取每一个 HorizontalPodAutoscaler 指定的 Pod 的度量值，如果设置了目标使用率，控制器获取每个 Pod 中的容器资源使用情况，并计算资源使用率。如果设置了 target 值，将直接使用原始数据（不再计算百分比）。接下来，控制器根据平均的资源使用率或原始值计算出扩缩的比例，进而计算出目标副本数。

需要注意的是，如果 Pod 某些容器不支持资源采集，那么控制器将不会使用该 Pod 的 CPU / 内存使用率。

除了最常用的资源指标外，HPA 还支持 ContainerResource，External，Object，Pod 这四类指标，但是应用方面就不如 Resource 指标那么广泛了。

#### BCS 提供的 GPA 能力

BCS 在 k8s 原生 HPA 的基础上，针对游戏业务等特殊场景推出了 [GeneralPodAutoscaler](https://github.com/TencentBlueKing/bk-bcs/tree/afaa017edf2b6156d7fc0090af8a874bd6a0b8e2/docs/features/bcs-general-pod-autoscaler)，GPA 除了全面覆盖 HPA 能力之外，还拥有下面列举的一些能力：

##### 基于 Limits 进行资源计算

原生的 HPA 只能通过资源配置的 requests 来计算资源的使用率，在 SaaS 应用场景（如开发者中心），是需要以 limits 来计算资源使用率的，使用 GPA 就可以支持该使用场景，只需要简单地在 annotations 中添加以下内容：

```yaml
compute-by-limits: "true"
```

##### 定时模式

针对游戏业务等有高峰/低谷期的场景，GPA 支持了按照 crontab 表达式来定时扩缩容的情况；需要注意的是：该模式使用的是零时区。

```yaml
apiVersion: autoscaling.tkex.tencent.com/v1alpha1
kind: GeneralPodAutoscaler
metadata:
  name: gpa-crontab-nginx-demo-app
  namespace: blueking
  labels:
    app: nginx-demo-app
spec:
  maxReplicas: 10
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: nginx-demo-app
  time:
    ranges:
      - desiredReplicas: 4
        # 每天 2-3 点，每隔五分钟，将副本数量维持在 4
        schedule: "*/5 2-3 * * *"
      - desiredReplicas: 6
        # 每天 4-5 点，每隔五分钟，将副本数量维持在 6
        schedule: "*/5 4-5 * * *"
```

##### webhook 模式

GPA webhook 模式支持通过访问集群内/外部署的 webhook server，根据返回的 response 判断是否进行扩缩容。

```go
// AutoscaleRequest defines the request to webhook autoscaler endpoint
type AutoscaleRequest struct {
   // UID is used for tracing the request and response.
   UID types.UID `json:"uid"`
   // Name is the name of the workload(Deployment, Statefulset...) being scaled
   Name string `json:"name"`
   // Namespace is the workload namespace
   Namespace string `json:"namespace"`
   // Parameters are the parameter that required by webhook
   Parameters map[string]string `json:"parameters"`
   // CurrentReplicas is the current replicas
   CurrentReplicas int32 `json:"currentReplicas"`
}

// AutoscaleResponse defines the response of webhook server
type AutoscaleResponse struct {
   // UID is used for tracing the request and response.
   // It should be same as it in the request.
   UID types.UID `json:"uid"`
   // Set to false if should not do scaling
   Scale bool `json:"scale"`
   // Replicas is targeted replica count from the webhookServer
   Replicas int32 `json:"replicas"`
}
```

```yaml
# GPA 使用 Webhook 模式示例
apiVersion: autoscaling.tkex.tencent.com/v1alpha1
kind: GeneralPodAutoscaler
metadata:
  name: gpa-webhook-nginx-demo-app
  namespace: blueking
  labels:
    app: nginx-demo-app
spec:
  maxReplicas: 10
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: nginx-demo-app
  webhook:
    # 集群内 webhook server 的情况
    service:
      namespace: blueking
      name: gpa-webhook-demo
      port: 5333
      path: scale
    parameters:
      buffer: "2"
    # 集群外 webhook server 的情况
    # url: http://127.0.0.1:5333/scale
    # parameters:
    #   buffer: "2"
```

### Pod 垂直自动扩缩容（VPA）

VPA 全称 Vertical Pod Autoscaler，即垂直 Pod 自动扩缩容，旨在不调整副本数的情况下，动态调整 pod 的资源，以优化资源使用情况。

它既可以缩小过度请求资源的容器，也可以根据其使用情况随时提升资源不足的容量。**注意：VPA 会修改 Pod 的资源限制值（limits）**

#### VPA 原理介绍

![img](/static/image/blog/vpa_architecture.png)

VPA 主要包括两个组件：

- VPA Controller
    - Recommendr：给出 pod 资源调整建议
    - Updater：对比建议值和当前值，不一致时驱逐 Pod
- VPA Admission Controller
    - Pod 重建时将 Pod 的资源请求量修改为推荐值

#### 工作流程

![img](/static/image/blog/vpa_workflow.png)

首先 Recommender 会根据应用当前的资源使用情况以及历史的资源使用情况，计算接下来可能的资源使用阈值，若计算出的值和当前值不一致则会给出资源调整建议。

然后 Updater 则根据这些建议进行调整，具体调整方法为：

- Updater 根据建议发现需要调整，然后调用 api 驱逐 Pod
- Pod 被驱逐后就会重建，然后再重建过程中 VPA Admission Controller 会进行拦截，根据 Recommend 来调整 Pod 的资源请求量
- 最终 Pod 重建出来就是按照推荐资源请求量重建的

> 根据上述流程可知，调整资源请求量需要重建 Pod，这是一个破坏性的操作，所以 VPA 还没有生产就绪。

#### VPA 的优点

- Pod 完全用其所需，所以集群节点使用效率高
- Pod 会被安排到具有适当可用资源的节点上
- 不必运行耗时的基准测试任务来确定 CPU 和内存请求的合适值
- VPA 可以随时调整 CPU 和内存请求，而无需执行任何操作，因此可以减少维护时间

#### VPA 的不足

- VPA 更新 pod 资源时，会导致 pod 重建
- VPA 不能保证它驱逐的 pod 能够成功重建（资源，污点等原因）
- VPA 不会驱逐不在控制器下运行的 pod
- VPA 不应与 HPA 一起使用，除非 HPA 使用的不是 CPU / 内存指标
- VPA 对大多数内存不足事件做出反应，但并非在所有情况下都如此
- VPA 性能尚未在大型集群中进行测试
- VPA 建议可能会超出可用资源（例如节点大小、可用大小、可用配额）并导致 pod 挂起，因此推荐与 `ClusterAutoscaler` 一起使用

### 应用休眠（ScaleToZero）

成熟且完善的 Serverless 平台，除能够支持副本自动扩缩容外，还需要能在没有流量时候，自动缩容到 0，并在流量到达后，快速完成冷启动并提供服务。

#### HPA 在冷启动方面的局限性

副本数量缩容到零，若 HPA 使用资源指标进行自动扩缩容，此时是没有任何指标数据的，无法做到激活服务 ——> **再起不能！**

#### knative serving

这里简单介绍下 knative 的机制

![img](/static/image/blog/knative_topo.png)

- 用户编写 ksvc（service），定义网络策略（分流），镜像，部署等信息
- operator 根据 ksvc，生成 route（管理网络），configuration（管理资源配置）
- configuration 在每次发布的时候，生成只读的 revision，相当于一次快照
- revision 管理 deployment，并配合 route，king 等做流量管理，自动休眠等功能

![img](/static/image/blog/knative_route.webp)

- route 负责网络流量的转发，默认情况下直接转发到 Revision（Pods）
- 启用应用休眠能力后，当 Pod 数量缩容到 0，会启用 Activator 组件并进行流量切换
- 当 Activator 接收到用户流量后，会激活 Revision 并缓存 API 请求，在拉起服务后转发给 Pod（无损）

> KPA 的扩缩容指标是**请求并发数/RPS(请求响应时间)**，而非传统的**CPU/内存使用**，会更加贴合业务场景，该指标依赖 sidecar 容器 Queue-proxy 上报。

##### knative 架构设计

![img](/static/image/blog/knative_arch.webp)

##### knative 各 CRD 间关系图

![img](/static/image/blog/knative_crd_relations.webp)

#### KEDA（Kubernetes Event-driven Autoscaling）

##### KEDA 架构设计

![img](/static/image/blog/keda_arch.png)

##### KEDA 工作流程

![img](/static/image/blog/keda_workflow.webp)

> NOTE: KEDA requires Kubernetes cluster version 1.24 and higher

- keda 是基于 HPA 的社区扩缩容方案，其核心扩展是添加了事件驱动机制（从名字可以看出来）
- keda CRD 主要有 ScaledObject，ScaledJobs 两种；前者一般针对 Deployment，后者针对 Job 类型

KEDA 工作流：
  1. [事件触发器](https://keda.sh/) 提供指标（来源自 nginx/db/mq ...） 
  2. keda operator 计算目标副本数并通过 metrics API 暴露  
  3. HPA 使用 external 模式接收并修改 Deployment 的 `spec.replicas`

## 参考资料

### VPA

- [Github - Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler)
- [k8s 之纵向扩缩容 vpa](http://www.lishuai.fun/2020/09/02/k8s-vpa/#/vpa)
- [VPA:垂直 Pod 自动扩缩容](https://www.lixueduan.com/posts/kubernetes/12-vpa/)
- [在 TKE 上利用 VPA 实现垂直扩缩](https://cloud.tencent.com/document/product/457/54756)
- [进阶阅读：深入理解 VPA Recommender](https://www.infoq.cn/article/z40lmwmtoyvecq6tpoik)

### Knative

- [Knative Serving](https://github.com/knative/serving)
- [Knative Serving 自动扩缩容 Autoscaler](https://www.infoq.cn/article/XWaesaR7F*TZqBUFspTl)
- [knative-serving 流量管理](https://blog.abreaking.com/article/164)
- [knative-serving 自动扩缩容机制深入理解](https://blog.abreaking.com/article/166)
- [Knative 全链路流量机制探索与揭秘](https://juejin.cn/post/6844904084005191693)
- [从 HPA 到 KPA：Knative 自动扩缩容深度分析](https://juejin.cn/post/6844904206038466568)

### KEDA

- [KEDA Concepts](https://keda.sh/docs/2.10/concepts/)
- [KEDA 官方文档](https://keda.sh/)
- [KEDA 工作原理解析](https://jishuin.proginn.com/p/763bfbd3590f)
- [KEDA-Kubernetes 中基于事件驱动的自动伸缩](https://cloud.tencent.com/developer/article/1692475)
