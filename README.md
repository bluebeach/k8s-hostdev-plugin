# hostdev device plugin for Kubernetes

## Introduction

`hostdev` is a [device plugin](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/resource-management/device-plugin.md) for Kubernetes to configure devices under host /dev into PODs through [device cgroup](https://www.kernel.org/doc/Documentation/cgroup-v1/devices.txt).

The hostdev binary should be runing on each node and talk with kubelet vai unix sockets under '/var/lib/kubelet/device-plugins/'. This could be achieved by a daemonset which will be explained later.

## Build

```
# make help
# make bin
# make img
# make push
```



## Install on Kubernetes
#### 1. edit the 'containers.[*].args' part of hostdev-plugin-ds.yaml:
```
      containers:
      - name: hostdev
        args: ["--devs", "/dev/mem:r,/dev/fuse:rwm"]
```

The above args means 2 devices to be supported: 
- /dev/mem with permission "r"
- /dev/fuse with permission "rwm" 


#### 2. Install the daemonset
```
# kubectl create -f hostdev-plugin-ds.yaml
```

check the pods status:
```
# kubectl -n kube-system get pods
NAME                          READY     STATUS    RESTARTS   AGE
hostdev-device-plugin-2wfhn   1/1       Running   0          25m
hostdev-device-plugin-bzd7w   1/1       Running   0          25m
```

#### 3. check the device full name to be used in business POD spec
```
# kubectl get node/$NODE_NAME -o jsonpath='{.status.allocatable}'
map[pods:29 cpu:940m hostdev.k8s.io/dev_mem:1 hostdev.k8s.io/dev_fuse:1 memory:618688Ki]
```
Notes the full names:
- hostdev.k8s.io/dev_mem for /dev/mem
- hostdev.k8s.io/dev_fuse for /dev/fuse

## Configure POD spec

```
apiVersion: v1
kind: Pod
metadata:
  name: ng
spec:
  containers:
    - name: ng
      image: nginx:alpine
      securityContext:
        capabilities:
          add: ["SYS_RAWIO"]
      resources:
        limits:
          hostdev.k8s.io/dev_mem: 1
```

PODs with such resourece definition will get asigned a /dev/mem device.


## TODO
- dynamic config by ConfigMap
