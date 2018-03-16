package frame

import (
	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
	"os"
	"time"
	"net"
	"syscall"
	"path"
)

type DevicePluginImpl interface {
	ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error
	Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error)
	Refresh()
	Destroy()
}

type GetDeviceFunc func() []*pluginapi.Device

type DevicePluginWrapper struct {
	resourceName string
	socketName string
	server *grpc.Server
	DevicePluginImpl
}

func Build(resourceName, socketName string, devicePluginI DevicePluginImpl) *DevicePluginWrapper {
	return &DevicePluginWrapper{
		DevicePluginImpl: devicePluginI,
		resourceName:  resourceName,
		socketName:    pluginapi.DevicePluginPath + socketName,
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

	needToStart := true

L:
	for {
		if needToStart {
			dpw.Refresh()
			if err := dpw.Serve(); err != nil {
				glog.Warningf("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
			} else {
				needToStart = false
			}
		}

		select {
		case event := <-watcher.Events:
			glog.Infof("catch an event of deviceplugin path: %+v \n", event)
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				glog.Warningf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				dpw.Stop()
				needToStart = true
			}

			if event.Name == dpw.socketName && event.Op&fsnotify.Remove == fsnotify.Remove {
				glog.Warningf("inotify: %s removed, restarting.", dpw.socketName)
				dpw.Stop()
				needToStart = true
			}
		case err := <-watcher.Errors:
			glog.Warningf("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				glog.Infof("Received SIGHUP, restarting.")
				dpw.Stop()
				needToStart = true
			default:
				glog.Infof("Received signal \"%v\", shutting down.", s)
				dpw.Stop()
				break L
			}
		}
	}
}

// dial establishes the gRPC communication with the registered device plugin.
func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

// Start starts the gRPC server of the device plugin
func (m *DevicePluginWrapper) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", m.socketName)
	if err != nil {
		return err
	}

	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(m.server, m)

	go m.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	conn, err := dial(m.socketName, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	return nil
}

// Stop stops the gRPC server
func (m *DevicePluginWrapper) Stop() error {
	if m.server == nil {
		return nil
	}

	m.server.Stop()
	m.server = nil

	m.Destroy()

	return m.cleanup()
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (m *DevicePluginWrapper) Register(kubeletEndpoint, resourceName string) error {
	conn, err := dial(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(m.socketName),
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

// Serve starts the gRPC server and register the device plugin to Kubelet
func (m *DevicePluginWrapper) Serve() error {
	err := m.Start()
	if err != nil {
		glog.Warningf("Could not start device plugin: %s", err)
		return err
	}
	glog.Infof("Starting to serve on %s", m.socketName)

	err = m.Register(pluginapi.KubeletSocket, m.resourceName)
	if err != nil {
		glog.Warningf("Could not register device plugin: %s", err)
		m.Stop()
		return err
	}
	glog.Infof("Registered device plugin with Kubelet")

	return nil
}

func (m *DevicePluginWrapper) cleanup() error {
	if err := os.Remove(m.socketName); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}