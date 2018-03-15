# simple-device-plugin
a simple kubernetes device-plugin frame

一个简单的k8s deviceplugin插件框架，用于创建 “无需进行额外配置”的设备的device-plugin。

参考example.go的例子构造自定义的资源的获取方式：
````
func getCpuDevices() []*pluginapi.Device
````

即可构建该资源的device-plugin

关于device-plugin的内容请参考[k8s官方文档](https://kubernetes.io/docs/concepts/cluster-administration/device-plugins/)