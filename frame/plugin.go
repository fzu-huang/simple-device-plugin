package frame

import (
	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
	"os"
	"syscall"
)

type GetDeviceFunc func() []*pluginapi.Device

type DevicePluginWrapper struct {
	DevicePlugin  *DevicePlugin
	ResourceName  string
	SocketName    string
	GetDeviceFunc GetDeviceFunc
}

func Build(resourceName, socketName string, getDevices GetDeviceFunc) *DevicePluginWrapper {
	if len(getDevices()) == 0 {
		glog.Warningf("Cann't get any device of %s resource.", resourceName)
		return nil
	} else {
		return &DevicePluginWrapper{
			DevicePlugin:  NewDevicePlugin(socketName, getDevices),
			ResourceName:  resourceName,
			SocketName:    socketName,
			GetDeviceFunc: getDevices,
		}
	}
}

func (dpw *DevicePluginWrapper) RunDevicePlugin() {
	glog.Infof("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		glog.Warningf("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	glog.Infof("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	restart := true

L:
	for {
		if restart {
			if dpw.DevicePlugin != nil {
				dpw.DevicePlugin.Stop()
			}

			dpw.DevicePlugin = NewDevicePlugin(dpw.SocketName, dpw.GetDeviceFunc)
			if err := dpw.DevicePlugin.Serve(); err != nil {
				glog.Warningf("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
			} else {
				restart = false
			}
		}

		select {
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				glog.Warningf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				restart = true
			}

			if event.Name == dpw.DevicePlugin.socket && event.Op&fsnotify.Remove == fsnotify.Remove {
				glog.Warningf("inotify: %s removed, restarting.", dpw.DevicePlugin.socket)
				restart = true
			}
		case err := <-watcher.Errors:
			glog.Warningf("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				glog.Infof("Received SIGHUP, restarting.")
				restart = true
			default:
				glog.Infof("Received signal \"%v\", shutting down.", s)
				dpw.DevicePlugin.Stop()
				break L
			}
		}
	}
}
