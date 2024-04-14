## 背景

近期折腾 `bkpaas-app-operator` 的容器化部署工作，发现一个诡异的情况：

卸载并重装 Operator 之后等待一段时间，会发现 3 个 crd 只剩一个，也即 bkapp & domaingroupmapping 不翼而飞！

```shell
root@VM-xxx-xxx-centos ➜ root  kubectl get crd | grep paas
projectconfigs.paas.bk.tencent.com                   2023-02-03T14:00:51Z
```

这是灰常恐怖的一种情况，为什么更新后，会有两个 crd 陆续消失？要是在生产环境来这么一出，岂不是得提桶跑路？

## 排查过程

### 尝试复现

拿同一个 Helm Chart 反复部署更新，以及卸载后再安装，等待一段时间后都没有出现这种情况，复现失败 orz

### 查找相关 issue

最开始，我怀疑是 install / upgrade 中的什么过程导致 crd 被异常删除，翻了下 helm 的 issue，找到 [这个](https://github.com/helm/helm/issues/7505)：

Helm 在 3.0.3 版本曾短暂地加入过一段逻辑，在 uninstall 时，忽略 crd 类型的资源，但是被社区用户提出破坏了向后兼容性，因此在后续版本中该修改被 [回滚](https://github.com/helm/helm/pull/7571/files)

翻了下最新的版本，确实删除时候没对 crd 做什么特殊判断，思路到这里断了

### 查看 Helm 对于 crd 的描述

> 随着 Helm 3 的到来，我们去掉了旧的 crd-install 钩子以便获取更简单的方法。**现在可以在 chart 中创建一个名为 `crds` 的特殊目录来保存 crd。 这些 crd 不会作为模板渲染，但是运行 helm install 时可以为 chart 默认安装**。如果 crd 已经存在，会显示警告并跳过。如果希望跳过 crd 安装步骤，可以使用 `--skip-crds` 参数。

> **目前不支持使用 Helm 升级或删除 crd。由于数据意外丢失的风险，这是经过多次社区讨论后作出的明确决定**。对于如何处理 crd 及其生命周期，目前社区还未达成共识。随着发展 Helm 可能会逐渐支持这些场景。

看起来 Helm 对于 crd 的管理是非常谨慎的，基本就是：能不删，我就不删（避免误操作）

### 为什么 crd 还是会被删除

我们的 crd 配置确实是放在 crds 目录下的，为什么还是会有被删除的情况

![img](/static/image/blog/tree_operator_chart.png)

翻 [Helm 源码](https://github.com/helm/helm/blob/46103aa1df0388d07444f25e0688ffe946812ebe/pkg/chart/chart.go#L158) 看看

```go
// crdObjects returns a list of crd objects in the 'crds/' directory of a Helm chart & subcharts
func (ch *Chart) crdObjects() []crd {
	crds := []crd{}
	for _, f := range ch.Files {
		if strings.HasPrefix(f.Name, "crds/") && hasManifestExtension(f.Name) {       // 仅检查根目录下的 crds 目录！
			...
		}
	}
    ...
	return crds
}

func hasManifestExtension(fname string) bool {
	ext := filepath.Ext(fname)
	return strings.EqualFold(ext, ".yaml") || strings.EqualFold(ext, ".yml") || strings.EqualFold(ext, ".json")
}
```

原来只有在 chart **根目录** 的 `crds` 目录下的文件，会被认定为 crd（不管 Kind 是不是 crd），这些文件不会被当作模板渲染，在删除 release 的时候，其中定义的资源也不会被删除。

### 思考，是不是 crd 删除得慢

下班路上想到一种可能，是不是 crd 删除得慢，所以重新安装时候跳过 crd，在一段时间后才删除干净？

因为之前集群中存在多个 crd 创建的资源，如 bkapp 等，它们需要回收子资源后才能被删除，只有它们被删除之后，crd 才能被删除，也就是说，删除顺序如下：

`Pod -> ReplicaSet -> Deployment -> BkApp -> crd`

之前没有复现的原因是：我们刚部署新的，集群里面没有新创建的自定义资源，因此删除速度快得一批！

于是乎，重新在集群里面部署大量 bkapp，再重试卸载重装流程，终于可以复现重装一段时间后 crd 消失的情况，真相大白！

## 结论

Helm 对于 crd 的支持还不够优雅，目前社区也还[没有达成共识](https://github.com/helm/community/blob/f9e06c16d89ccea1bea77c01a6a96ae3b309f823/architecture/crds.md)

Helm 目前推荐两种方式管理 crd：

1. 使用 `/crds` 文件夹来管理 crd 配置，但缺点是 crd 下发后就固定且无法更新，仅适用于发布固定版本，看了下目前 bitnami 中的 chart 都是这种用法
2. crd 资源独立出来，和 operator 部分分隔成两个独立的 chart，分别部署 & 管理

另外我也翻了下蓝鲸的 chart 大仓，目前大部分都是把 crd 当成普通资源处理，这样的缺点是 release 被删除的话，crd 和用户创建的资源都会被删除

只有蓝鲸监控使用上面的两种做法，拆分成两个 chart，一个用于版本发布的 stack，一个独立的用于 crd 管理

```text
bkmonitor-operator-crds  # crd 允许更新的
bkmonitor-operator-stack # crd 下发后便不再更新的
```

## 补充

可以通过添加 `"helm.sh/resource-policy": keep` 注解避免资源在更新或卸载 release 时被删除。

需要注意的是，只有在 chart 中添加该注解会生效，kubectl annotate 的不会在 uninstall 时候生效（upgrade 会生效就很奇怪，可以看下 [这里](https://github.com/helm/helm/issues/8132) 的讨论）

## 参考资料

- [Helm x 自定义资源 最佳实践](https://helm.sh/zh/docs/chart_best_practices/custom_resource_definitions/)
- [Helm 3.0.3 change to not delete templated crds breaks backward compatibility](https://github.com/helm/helm/issues/7505)
- [Revert "Do not delete templated crds" #7571](https://github.com/helm/helm/pull/7571/files)
- [Helm Docs: crd Handling in Helm](https://github.com/helm/community/blob/f9e06c16d89ccea1bea77c01a6a96ae3b309f823/architecture/crds.md)
- [fix directory name in docs hip-0011 #286](https://github.com/helm/community/pull/286/files)
- [Tell Helm Not To Uninstall a Resource](https://helm.sh/docs/howto/charts_tips_and_tricks/#tell-helm-not-to-uninstall-a-resource)
- [Respect helm.sh/resource-policy annotation of remote resource #8132](https://github.com/helm/helm/issues/8132)
