## 背景

Kubernetes 集群是一个经典的 Master-Node 结构，一般来说，用于生产环境的集群至少需要 3 台 master，以便在某台 master 故障后快速切换；除此之外，一般集群中还会有两台以上的节点用于单独部署 Ingress Controller 服务（如 NginxIngressController）。

那么，如果一个集群，只部署单独的一个服务（以开发者中心为例：3 webfe + 8 web-backend + 8 celery-worker)，这其实是非常浪费的，因此实际使用中，一般都是多个服务部署在同一集群中，通过一些手段进行隔离，共享集群的资源。

## 基于命名空间的隔离

Kubernetes 原生提供了基于命名空间（namespace）的隔离，命名空间域的资源在各自的命名空间中是相对独立的，且我们能够很方便地对命名空间添加权限 & 资源配额的限制。

因此，大部分情况下，我们能为各服务分配不同的命名空间，配置好相应的权限 & 资源配额；各服务使用自己的一亩三分地，在同一个集群里面对外提供服务，实现了平摊 Master 费用的需求。

但是，这种基于命名空间的隔离存在一个 **隐藏的风险**：无法灵活地使用集群域的资源（如 crd，sc 等），对于特殊的资源类型如 DaemonSet 在使用上也会有一些局限。

举个例子：如果服务 A & 服务 B 共同部署在同一集群，两个服务各自依赖同一 crd 的不同版本，这时候就会存在冲突导致无法共享集群。

## 通过虚拟集群进行隔离

为了解决基于命名空间隔离的一些短板，Loft Labs 推出了虚拟集群 ([vcluster](https://github.com/loft-sh/vcluster)) 方案，通过在 k8s 集群中虚拟化出一个独立的 k8s 集群（Cluster in Cluster）以获得更好的隔离性（也算是多租户 :D）。

### 什么是 vcluster

正如其名称：虚拟集群，`vcluster` 并不是一个真正的 k8s 集群，它是工作在 k8s 集群上的一套组件，用于将集群分割成独立的虚拟子集群。

虚拟集群拥独立的控制面（包含 `ControllerManager`，`存储（如 etcd）`，`apiserver`，以及可选的 `scheduler`）；除此之外，还有一个 `syncer` 用于同步虚拟 & 宿主集群中资源的状态；需要注意的是单个虚拟集群 **拥有且仅拥有** 宿主集群中的一个命名空间。

![img](/static/image/blog/vcluster_arch.png)

### 为什么需要 vcluster

虚拟集群可以隔离一个集群内数百个不同的租户工作负载，复用一些基础组件，提高资源利用率，摊平 Master 成本等等。

![img](/static/image/blog/with_vs_without_vcluster.png)

虚拟集群拥有自己的 apiserver，这使得它们比命名空间隔离的隔离性更好，又比单独的 Kubernetes 集群更便宜。

![img](/static/image/blog/vcluster_comparison.png)

虚拟集群相较于 “基于命名空间的隔离” 这种共享集群的方式，有以下的优点：

1. 支持使用集群域资源：由于 vcluster 方案中，集群域资源存储于独立的 etcd 中，因此能够由用户自由管理，不会影响到底层集群。在基于命名空间隔离的共享集群中，集群域资源只能由管理员进行分配，且无法支持如部署多版本的 crd 的场景。
2. 独立的 k8s 控制面：由于基于命名空间隔离的共享集群中，所有命名空间还是共享 apiserver，etcd，scheduler，很难对于命名空间组进行请求或者存储的限制，不恰当的操作（如压测）可能拖垮整个共享集群，而 vcluster 由于其具有更强的独立性，即使挂了也只会影响对应的 vcluster。

### vcluster 实现细节

虚拟集群拥有独立的 ApiServer，允许通过通过 api 或 kubectl 等方式访问并管理集群。

当用户在集群中创建工作负载（如 Deployment）时，会发生以下的情况：

- vcluster 会将 Deployment 及其子资源 ReplicaSet，Pod 的配置创建好并存储起来

- vcluster syncer 会在宿主集群中，属于该虚拟集群的命名空间下，创建新的 Pod，并实时同步该 Pod 的状态给到虚拟集群（Pod 名称规则：`${pod_name}-x-${namespace}-x-${vcluster_name}`）

#### 资源同步策略

- Pods: 所有在虚拟集群中启动的 Pod 都被重写，然后在宿主集群中的虚拟集群的命名空间中启动。ServiceAccountToken、环境变量、DNS 和其他配置以指向虚拟集群而不是宿主集群。在 Pod 中看起来，似乎是在虚拟集群而不是宿主集群中启动的。

- Services: 所有服务和端点都在宿主集群中的虚拟集群的命名空间中被重写和创建，且虚拟和宿主集群共享相同的集群 IP。这也意味着可以从虚拟集群内部访问宿主集群中的服务，而不会造成任何性能损失。

- PersistentVolumeClaims：如果在虚拟集群中创建 PVC，它们将在宿主集群中的虚拟集群的命名空间中被创建。如果它们在宿主集群中被绑定，则相应的 PV 信息将同步回虚拟集群。

- Configmaps & Secrets：挂载到 pod 的虚拟集群中的 ConfigMaps 或 Secrets 将同步到宿主集群，所有其他 configmaps 或 secrets 将纯粹保留在虚拟集群中。

- 其他资源：Deployment、StatefulSet、CRD、ServiceAccount 等不同步到宿主集群，仅存在于虚拟集群中。

#### vcluster 网络与 DNS

![img](/static/image/blog/vcluster_networking.svg)

默认情况下，Ingress，Service 资源都会被同步到宿主集群以确保网络通畅。

##### Pod <-> Pod

由于虚拟集群中的 Pod 是实际运行在宿主集群中的同个命名空间下，且他们拥有相同的 IP，因此 Pod 之间可以通过 IP 互相通信。

##### Pod <-> Service

尽管 vcluster 中的所有 service 都运行在同一个命名空间下，但是在 Pod 中，并不能通过 svc 名称直接进行访问，vcluster 有自己的 DNS 服务，确保通过 svc 访问的场景下，与真正的 k8s 集群表现一致：`(svc_name).(namespace).svc.cluster.local`

##### vcluster 中使用 clb

一般建议采用 clb 直通 service 的方式，通过 annotations 为 clb 绑定转发目标

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.kubernetes.io/tke-existed-lbid: lb-xxxxxxxx
  name: bk-ingress-nginx
  namespace: bk-ingress-nginx
```

### 当前 vcluster 可能存在的问题

- nodeSelector 不会生效，vcluster 中只能看到 vcluster-vnode，底层集群实际会有成百上千的节点
- 主机资源受限制，很多宿主机资源如 `hostNetwork`, `hostIPC`, `hostPID`, `hostPath` 等都不可用
- DaemonSet 使用有一定的限制，不会在每个底层集群的节点上都起 Pod（不然大集群里面起几千个？）

## 总结

vcluster 方案在很大程度上解决了基于命名空间隔离的独立性不足 & 难以共用集群域资源的问题，但是在一些比较极端的场景下，终究还是不如正在的独立的 K8s 集群。是否应该使用 vcluster 方案，还是要结合实际的使用场景，综合分析考虑（No Silver Bullet）。

## 参考资料

- [vcluster github repo](https://github.com/loft-sh/vcluster)
- [What are Virtual Kubernetes Clusters?](https://www.vcluster.com/docs/what-are-virtual-clusters)
- [Virtual cluster – extending namespace based multi-tenancy with a cluster view](https://www.cncf.io/blog/2019/06/20/virtual-cluster-extending-namespace-based-multi-tenancy-with-a-cluster-view/)
- [A Virtual Cluster Based Kubernetes Multi-tenancy Design](https://docs.google.com/document/d/1EELeVaduYZ65j4AXg9bp3Kyn38GKDU5fAJ5LFcxt2ZU/edit)
