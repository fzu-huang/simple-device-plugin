package examples

import (
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"github.com/fzu-huang/simple-device-plugin/frame"
	"time"
)

type MyDevicePlugin struct {
	devs   []*pluginapi.Device
	stop chan interface{}

	//use these if you want
	health chan *pluginapi.Device
}

func (m *MyDevicePlugin) startHealthCheck(){
	//fixme
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <- m.stop:
			return
		case <- t.C:
			if time.Now().Hour() %2 == 0 {
				dev := &pluginapi.Device{
					ID: "123",
					Health: pluginapi.Unhealthy,
				}
				m.health <- dev
			}
		}
	}
}

func (m *MyDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
	for {
		select {
		case <- m.stop:
			return nil
		//TODO other case
		case d := <-m.health:
			d.Health = pluginapi.Unhealthy
			s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
		}
	}
}

func (m *MyDevicePlugin) Refresh() {
	//when device plugin socket connection recreation .It should refresh self devices and re-make closed channel
	m.devs = getDevices()
	m.stop = make(chan interface{})
}

func (m *MyDevicePlugin) Destroy() {
	close(m.stop)
}

func (m *MyDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	//fixme
	return &pluginapi.AllocateResponse{}, nil
}


func getDevices() []*pluginapi.Device {
	devices := []*pluginapi.Device{}

	devices = append(devices, &pluginapi.Device{
		"123", pluginapi.Healthy,
	})
	return devices
}

func main() {
	devices :=getDevices()
	if len(devices) == 0 {
		glog.Errorf("get no device")
		return
	}

	mdp := &MyDevicePlugin{
		devs:   devices,
		health: make(chan *pluginapi.Device),
	}

	resourceName := "cpu"
	socketName := "cpu.sock"
	mdpWrapper := frame.Build(resourceName, socketName, mdp)
	go mdp.startHealthCheck()
	mdpWrapper.RunDevicePlugin()
}