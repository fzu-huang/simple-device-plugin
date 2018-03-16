package main

import (
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
	"github.com/fzu-huang/simple-device-plugin/frame"
	"github.com/golang/glog"
	"strconv"
	"io/ioutil"
	"strings"
	"flag"
)

func getDevices() []*pluginapi.Device {
	devices := []*pluginapi.Device{}

	cpuinfo, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		glog.Warningf("read '/proc/cpuinfo' file failed. %s", err.Error())
		return devices
	}
	str_cpuinfo := string(cpuinfo)

	phyIdCounts := strings.Count(str_cpuinfo, "physical id")
	processorCounts := strings.Count(str_cpuinfo, "processor")
	if phyIdCounts != processorCounts || phyIdCounts == 0 {
		glog.Warningf("Analyse cpuinfo failed. sub-string counts: physical id: %d, processor: %d", phyIdCounts, processorCounts)
		return devices
	}
	glog.Infof("vpc port count : %d", phyIdCounts)
	for i := 0; i < phyIdCounts; i++ {
		devices = append(devices, &pluginapi.Device{
			strconv.Itoa(i), pluginapi.Healthy,
		})
	}
	return devices
}

func main() {
	flag.Parse()
	defer glog.Flush()
	simpleDP := frame.BuildSimpleDevicePlugin("netease/vpcport", "vpcport.sock", getDevices)
	simpleDP.RunDevicePlugin()
}

