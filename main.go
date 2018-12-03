package main

import (
	"os"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"fmt"
	"time"
)


func main() {
	hostDevPluginConfig, err := loadConfig()

	if err != nil {
		log.Fatal(err)
		os.Exit(-1)
	}

	fmt.Printf("config: %#v\n", *hostDevPluginConfig)

	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.Println("Failed to created FS watcher.")
		os.Exit(-1)
	}
	defer watcher.Close()

	log.Println("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	pluginManager, err := NewHostDevicePluginManager(hostDevPluginConfig)
	if err != nil {
		log.Fatal(err)
	}

	if err := pluginManager.Start(); err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(time.Second * 5)

L:
	for {
		select {
		case <-ticker.C:
			pluginManager.RegisterToKubelet()

		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				pluginManager.RegisterToKubelet()
			}

		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting.")
			default:
				log.Printf("Received signal \"%v\", shutting down.", s)
				pluginManager.Stop()
				break L
			}
		}
	}
}
