## simple-device-plugin

simple-device-plugin 是一个简单的device-plugin构造工具，可以用来快速的构建自己想要的device-plugin。


### 关于device-plugin
device-plugin 是kubernetes的一个新特性（v1alpha），它用于管理机器上一些特定的物理设备，或是用户自己定制的资源。通过grpc与kubelet进行交互，使得这部分资源可以像cpu、memory一样被kubernetes分配和使用。

举个简单的例子，假设我们的pod中，每个容器都需要消耗一个特定的存储设备（我们这里暂时称之为`superdisk`），创建一个superdisk的device-plugin，将其运行在node端。这个device-plugin运行后会做至少以下事情：

 1. 获取当前机器上的superdisk设备，构成一个数组；
 2. 向kubelet注册一个资源，资源名可以自定义，这里假定是`hy.c/superdisk`【注意这里名字中必须包含域名前缀，并用/分隔】;
 3. 开启一个grpc服务器，kubelet会向这个grpc服务器发起`listAndWatch`请求和`allocate`请求，`listAndWatch`请求会在kubelet与device-plugin之间建立一个长连接，device-plugin会主动将device列表发给kubelet；`allocate`请求告知device-plugin kubelet要使用哪一个设备，device-plugin将为容器使用该设备返回需要的运行时参数。

如果你想，device-plugin还可以做更多：

 1. 自身健康检查，当机器上的某个或某些superdisk设备出现问题时能自动检查，将该设备健康状态置为`unhealthy`，或将之从device数组中去掉，（再通过`listAndWatch`长连接返回数组给kubelet）；
 2. 用户申请使用某个设备时，可以对该设备做预处理，比如对superdisk进行格式化；
 3. ...

 
device-plugin相关文档可以参考[k8s官方文档][1]

### 如何使用simple-device-plugin

#### 最简单的device-plugin

你需要自己编写一个获取设备的函数，该函数返回 `[]*pluginapi.Device` 。这是一个kubernetes官方规范的Device数组结构。另外，你需要指定资源的名称，以及device-plugin将要使用的socket名字。
然后执行`frame.BuildSimpleDevicePlugin`和`RunDevicePlugin`， 比如：

```
func getDevices() []*pluginapi.Device {
	return []*pluginapi.Device{}
}
simpleDP := frame.BuildSimpleDevicePlugin("netease/vpcport", "vpcport.sock", getDevices)
simpleDP.RunDevicePlugin()
```

详细案例可以参考项目根目录的`simple-dp.go`

#### 配置更灵活的device-plugin

你需要构建一个结构体，该结构体要实现一套接口，接口包含以下方法：
````
type DevicePluginImpl interface {
	ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error
	Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error)
	Refresh()
	Destroy()
}
````

这样，你便可以自定义当device-plugin收到`ListAndWatch`请求、`Allocate`请求时要做什么了。

然后执行`frame.Build`和`RunDevicePlugin`方法，比如：

````
mdp := &MyDevicePlugin{
	    //build your struct
	}
	resourceName := "cpu"
	socketName := "cpu.sock"
mdpWrapper := frame.Build(resourceName, socketName, mdp)
	mdpWrapper.RunDevicePlugin()
````

你会注意到，你写的代码完全没有涉及rpc相关，因为这部分由本工具代劳了。

*TIPS：上述接口中另两个方法`Refresh`和`Destroy`是用于和rpc服务协调使用的。每当rpc服务Stop时，会执行`Destroy`接口，退出device-plugin中用户自己运行的其他协程，避免泄漏；每当rpc服务启动时，会执行`Refresh`接口，将device-plugin更新，并准备好channel。*

  [1]: https://kubernetes.io/docs/concepts/cluster-administration/device-plugins/
