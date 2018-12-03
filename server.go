package main

import (
	"net"
	"path"
	"time"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"flag"
	"strings"
	"os"
)

const (
	ResourceNamePrefix = "hostdev.k8s.io/"
	serverSock          = pluginapi.DevicePluginPath + "hostdev.sock"
)

func init() {
	spew.Config.DisablePointerAddresses = true
	spew.Config.DisableCapacities = true
	spew.Config.MaxDepth = 2
}
type DevConfig struct {
	DevName string
	// must be "rwm" or it's subsets
	Permissions string
}

type HostDevicePluginConfig struct {
	DevList []*DevConfig
}

// HostDevicePlugin represents 1 host device
type HostDevicePlugin struct {
	// device node on host, for example: /dev/mem, /dev/cuse
	DevName string
	// must be "rwm" or it's subsets
	Permissions string
	// for example: dev_mem, dev_cuse
	NormalizedName string
	// Resource name to register with kubelet, for example: hostdev/dev_mem
	ResourceName string
	// DevicePluginPath ("/var/lib/kubelet/device-plugins/") + dev_mem.sock
	UnixSockPath string
	UnixSock net.Listener
	GrpcServer *grpc.Server
	// pre-setup Device
	Dev []*pluginapi.Device
	IsRigistered bool
	StopChan   chan interface{}
}

type HostDevicePluginManager struct {
	Config *HostDevicePluginConfig
	Plugins []*HostDevicePlugin
}

func NewHostDevicePluginManager(cfg *HostDevicePluginConfig) (*HostDevicePluginManager, error) {
	mgr := HostDevicePluginManager{
		Config: cfg,
		Plugins: make([]*HostDevicePlugin, 0, 8),
	}

	for _, devCfg := range cfg.DevList {
		plugin, err := NewHostDevicePlugin(devCfg)
		if err != nil {
			log.Fatal(err)
		}
		mgr.Plugins = append(mgr.Plugins, plugin)
	}
	return &mgr, nil
}

func (mgr *HostDevicePluginManager) Stop() {
	for _, plugin := range mgr.Plugins {
		plugin.Stop()
	}
}


func (mgr *HostDevicePluginManager) Start() error {
	for _, plugin := range mgr.Plugins {
		err := plugin.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (mgr *HostDevicePluginManager) RegisterToKubelet() error {
	for _, plugin := range mgr.Plugins {
		if plugin.IsRigistered {
			continue
		}
		err := plugin.RegisterToKubelet()
		if err != nil {
			return err
		}
	}
	return nil
}


var flagDevList = flag.String("devs", "",
	"The list of devices seperated by comma. For example: /dev/mem:rwm,/dev/ecryptfs:r")

func ParseDevConfig(dev string)(*DevConfig, error) {
	if dev == "" {
		return nil, fmt.Errorf("Must have arg --devs , for example, --devs /dev/mem:rwm")
	}
	devCfg := DevConfig{}
	s := strings.Split(dev, ":")
	if len(s) != 2 {
		return nil, fmt.Errorf("ParseDevConfig failed for: %s. Must have 1 [:], for example, /dev/mem:rwm", dev)
	}
	devCfg.DevName = s[0]
	devCfg.Permissions = s[1]

	fileInfo, err := os.Stat(devCfg.DevName)
	if err != nil {
		return nil, fmt.Errorf("ParseDevConfig failed for: %s. stat of %s failed: %v",
			dev, devCfg.DevName, err)
	}
	if (fileInfo.Mode() & os.ModeDevice) == 0 {
		return nil, fmt.Errorf("ParseDevConfig failed for: %s. %s is not a device file",
			dev, devCfg.DevName)
	}

	if len(devCfg.Permissions) > 3 || len(devCfg.Permissions) == 0 {
		return nil, fmt.Errorf("ParseDevConfig failed for: %s. Invalid permission string: %s. length must 1,2,3",
			dev, devCfg.Permissions)
	}

	for _, c := range "rwm" {
		if strings.Index(devCfg.Permissions, string(c)) != strings.LastIndex(devCfg.Permissions, string(c)) {
			return nil, fmt.Errorf("ParseDevConfig failed for: %s. Invalid permission string: %s. dup of %c",
				dev, devCfg.Permissions, c)
		}
	}


	for _, c := range devCfg.Permissions {
		if !strings.Contains("rwm", string(c)) {
			return nil, fmt.Errorf("ParseDevConfig failed for: %s. Invalid permission string: %s. Must be subset of rwm",
				dev, devCfg.Permissions)
		}
	}

	return &devCfg, nil
}

// for unit test
func LoadConfigImpl(arguments []string) (*HostDevicePluginConfig, error) {
	// Parse command-line arguments
	//flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	flag.CommandLine.Parse(arguments)

	cfg := HostDevicePluginConfig{
		DevList: make([]*DevConfig, 0, 2),
	}
	devs := strings.Split(*flagDevList, ",")
	for _, dev := range devs {
		devCfg, err := ParseDevConfig(dev)
		if err != nil {
			return nil, err
		}
		cfg.DevList = append(cfg.DevList, devCfg)
	}

	return &cfg, nil
}

func loadConfig() (*HostDevicePluginConfig, error) {
	return LoadConfigImpl(os.Args[1:])
}

func NomalizeDevName(devName string) (string,error) {
	if devName[0] != '/' {
		return "", fmt.Errorf("Invalid dev name, must start with // ")
	}
	return strings.Replace(devName[1:], "/", "_", -1), nil
}

// NewHostDevicePlugin returns an initialized HostDevicePlugin
func NewHostDevicePlugin(devCfg *DevConfig) (*HostDevicePlugin, error) {
	normalizedName, err := NomalizeDevName(devCfg.DevName)
	if err != nil {
		return nil, err
	}

	devs := []*pluginapi.Device {
		&pluginapi.Device{ID: devCfg.DevName, Health: pluginapi.Healthy},
	}

	return &HostDevicePlugin{
		DevName: 		devCfg.DevName,
		Permissions:    devCfg.Permissions,
		NormalizedName: normalizedName,
		ResourceName:   ResourceNamePrefix + normalizedName,
		UnixSockPath:   pluginapi.DevicePluginPath + normalizedName,
		Dev:			devs,
		StopChan: 		make(chan interface{}),
		IsRigistered: false,
	}, nil
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
		fmt.Errorf("dial error: %v\n", err)
		return nil, err
	}

	return c, nil
}

// Start starts the gRPC server of the device plugin
func (plugin *HostDevicePlugin) Start() error {
	sock, err := net.Listen("unix", plugin.UnixSockPath)
	if err != nil {
		return err
	}

	plugin.UnixSock = sock

	plugin.GrpcServer = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(plugin.GrpcServer, plugin)

	go plugin.GrpcServer.Serve(plugin.UnixSock)

	// Wait for server to start by launching a blocking connection
	conn, err := dial(plugin.UnixSockPath, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	return nil
}

// Stop stops the gRPC server
func (plugin *HostDevicePlugin) Stop() error {
	if plugin.GrpcServer == nil {
		return nil
	}

	plugin.GrpcServer.Stop()
	plugin.GrpcServer = nil
	close(plugin.StopChan)

	return nil
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (plugin *HostDevicePlugin) RegisterToKubelet() error {
	conn, err := dial(pluginapi.KubeletSocket, 5*time.Second)
	if err != nil {
		log.Errorf("fail to dial %s: %v\n", pluginapi.KubeletSocket, err)
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(plugin.UnixSockPath),
		ResourceName: plugin.ResourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		plugin.IsRigistered = false
		log.Errorf(" Register %s error: %v\n", plugin.DevName, err)
		return err
	}
	plugin.IsRigistered = true
	log.Infof(" Register %s success\n%s\n", plugin.DevName, spew.Sprint(plugin))
	return nil
}

// ListAndWatch lists devices and update that list according to the health status
func (plugin *HostDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {

	s.Send(&pluginapi.ListAndWatchResponse{Devices: plugin.Dev})

	ticker := time.NewTicker(time.Second * 10)

	for {
		select {
		case <-plugin.StopChan:
			return nil
		case <-ticker.C:
			s.Send(&pluginapi.ListAndWatchResponse{Devices: plugin.Dev})
		}
	}
	return nil
}


// Allocate which return list of devices.
func (plugin *HostDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	//spew.Printf("Context: %#v\n", ctx)
	spew.Printf("AllocateRequest: %#v\n", *r)

	response := pluginapi.AllocateResponse{}

	devSpec := pluginapi.DeviceSpec {
		HostPath: plugin.DevName,
		ContainerPath: plugin.DevName,
		Permissions: plugin.Permissions,
	}

	//log.Debugf("Request IDs: %v", r)
	var devicesList []*pluginapi.ContainerAllocateResponse

	devicesList = append(devicesList, &pluginapi.ContainerAllocateResponse{
		Envs: make(map[string]string),
		Annotations: make(map[string]string),
		Devices: []*pluginapi.DeviceSpec{&devSpec},
		Mounts: nil,
	})

	response.ContainerResponses = devicesList

	spew.Printf("AllocateResponse: %#v\n", devicesList)

	return &response, nil
}

func (plugin *HostDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{PreStartRequired: false}, nil
}

func (plugin *HostDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

