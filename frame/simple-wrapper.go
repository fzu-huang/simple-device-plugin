package frame

import (
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
	"golang.org/x/net/context"
)

type GetDevicesFunc func()[]*pluginapi.Device

type SimpleDevicePlugin struct {
	devs   []*pluginapi.Device
	stop chan interface{}
	getDevFunc GetDevicesFunc
}

func (m *SimpleDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
	for {
		select {
		case <-m.stop:
			return nil
		}
	}
}

func (m *SimpleDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	return &pluginapi.AllocateResponse{}, nil
}

func (m *SimpleDevicePlugin) Refresh() {
	//when device plugin socket connection recreation .It should refresh self devices and re-make closed channel
	m.devs = m.getDevFunc()
	m.stop = make(chan interface{})
}

func (m *SimpleDevicePlugin) Destroy() {
	close(m.stop)
}

func BuildSimpleDevicePlugin(resourceName, socketName string, getDevFunc GetDevicesFunc) *DevicePluginWrapper {
	simpleDevicePlugin := &SimpleDevicePlugin{
		devs: getDevFunc(),
		stop: make(chan interface{}),
		getDevFunc: getDevFunc,
	}

	return Build(resourceName, socketName, simpleDevicePlugin)
}