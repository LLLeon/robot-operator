# robot-operator

## 0. 简介

Kubernetes Operator 通过 CRD + Controller 模式实现定制化需求，使得自定义资源可以像 Kubernetes 内置资源一样运行在集群中。

架构图如下：

![CustomController](https://github.com/LLLeon/robot-operator/blob/main/imgs/CustomController.png?raw=true)

自定义控制器包括如下几个组件：

- Client：与 API Server 进行通信的客户端，提供对 K8s 内置资源和自定义资源对象进行操作的方法（Get、List、Watch、Create 等）。
- Informer：一个带有本地缓存和索引机制的、可以注册 EventHandler 的 client。它通过 Reflector 的 ListAndWatch 方法获取并监测对象实例的变化，具体做的是以下两个事情：
  - 将事件类型（CRUD）及其对应的 API 对象（称为 Delta）放入 Delta FIFO 队列并缓存。Informer 还会不断从队列里面读取 Delta，根据其事件类型，创建或更新本地对象的缓存。
  - 根据事件类型，触发 ResourceEventHandler，其实就是把 API 对象放入 WorkQueue。
- WorkQueue：使得 Informer 和 Control Loop 两个逻辑可以解耦。
- Control Loop：真正的业务逻辑，循环处理下面的事情：
  - 从 WorkQueue 获取 API 对象（只有 key，形式为对象的 `<namespace>/<name>`）。
  - 根据 API 对象，通过各种资源的 Lister 方法从 Informer 维护的缓存拿到 API 对象。
  - 将从缓存拿到的 API 对象（即期望状态）与集群中的实际状态进行对比，完成从实际状态向期望状态的转移。这在 K8s 中称为一次调谐（Reconcile）。

对 client-go 和自定义控制器各组件的介绍还可以参考[这篇文档](https://github.com/kubernetes/sample-controller/blob/master/docs/controller-client-go.md)。

这里使用 [code-generator](https://github.com/kubernetes/code-generator) 来为 CRD 资源生成代码，包括上面提到的：clientset、informers、listers 和自定义资源类型的 DeepCopy 方法。

## 1. 初始化项目

使用 go mod 初始化：

```bash
$ go mod init robot-operator
$ go get k8s.io/apimachinery@v0.21.0
$ go get k8s.io/client-go@v0.21.0
$ go get k8s.io/code-generator@v0.21.0
```

## 2. 初始化 CRD 资源类型

创建  `/pkg/api/robot/v1/doc.go`, `/pkg/api/robot/v1/types.go` 两个文件。

## 3. 编写代码生成脚本

创建 `/hack/tools.go`, `code-gen.sh` 两个文件。

## 4. 生成代码

```bash
$ go mod vendor
$ chmod -R 777 vendor
$ cd hack && sh ./code-gen.sh
```

## 5. 注册资源类型

创建 `/pkg/api/robot/v1/types.go` 文件，将自定义资源类型注册到 Scheme。

> 每一组 Controllers 都需要一个 Scheme，它解决 API 对象的序列化、反序列化与多版本 API 对象的兼容和转换问题，提供了 Kinds 与对应 Go types 的映射。

## 6. Controller

这里要编写控制循环的逻辑，先看一下 Controller 的结构：

```go
type Controller struct {
	kubeClientset  kubernetes.Interface
	robotClientset clientset.Interface

	deploymentsLister appslister.DeploymentLister
	robotsLister      robotlisters.RobotLister

	deploymentsSynced cache.InformerSynced
	robotsSynced      cache.InformerSynced

	workQueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder
}
```

基本上几个主要的组件都由 client-go 库实现了，简单说这里要实现的就是：

- 向各种资源的 Informer 注册事件处理函数。
- 不断从 workQueue 获取对象，将实际状态与期望状态做 diff，随后将实际状态向期望状态迁移。
- 更新自定义资源的状态。

## 7. 测试

创建 CRD 和 CR：

```bash
$ kubectl apply -f ./artifacts/examples/crd.yaml
$ kubectl apply -f ./artifacts/examples/robotone.yaml
```

此时可以成功获取到自定义的资源：

```bash
$ kubectl get crd
NAME                     CREATED AT
robots.robot.llleon.io   2021-04-09T10:01:56Z

$ kubectl get robot
NAME        AGE
robot-one   3d3h
```

但由于还没有启动控制器，所以还未达到期望状态。

编译好项目并启动：

```bash
$ ./robot-operator -kubeconfig=$HOME/.kube/config
```

此时再获取资源：

```bash
$ kubectl get deployment
NAME        READY   UP-TO-DATE   AVAILABLE   AGE
robot-one   2/2     2            2           9m

$ kubectl get pod
NAME                         READY   STATUS    RESTARTS   AGE
robot-one-779b7cf75b-56q8f   1/1     Running   0          9m
robot-one-779b7cf75b-92wlq   1/1     Running   0          9m
```

可以看到达到了期望状态。



