/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/4951
 * Buggy version: 81f148be566ab2b17810ad4be61a5d8beac8330f
 * fix commit-id: 2ffef1b7eb618162673c6ffabccb9ca57c7dfce3
 * Flaky: 100/100
 * Description:
 *   The root cause and patch is clearly explained in the commit
 * description. The global lock is devices.Lock(), and the device
 * lock is baseInfo.lock.Lock(). It is very likely that this bug
 * can be reproduced.
 */
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type DeviceSet struct {
	sync.Mutex
	infos            map[string]*DevInfo
	nrDeletedDevices int
}

func (devices *DeviceSet) DeleteDevice(hash string) {
	devices.Lock()
	defer devices.Unlock()

	info := devices.lookupDevice(hash)

	info.lock.Lock()
	defer info.lock.Unlock()

	devices.deleteDevice(info)
}

func (devices *DeviceSet) lookupDevice(hash string) *DevInfo {
	existing, ok := devices.infos[hash]
	if !ok {
		return nil
	}
	return existing
}

func (devices *DeviceSet) deleteDevice(info *DevInfo) {
	devices.removeDeviceAndWait(info.Name())
}

func (devices *DeviceSet) removeDeviceAndWait(devname string) {
	/// remove devices by devname
	devices.Unlock()
	time.Sleep(300 * time.Nanosecond)
	devices.Lock()
}

type DevInfo struct {
	lock sync.Mutex
	name string
}

func (info *DevInfo) Name() string {
	return info.name
}

func NewDeviceSet() *DeviceSet {
	devices := &DeviceSet{
		infos: make(map[string]*DevInfo),
	}
	info1 := &DevInfo{
		name: "info1",
	}
	info2 := &DevInfo{
		name: "info2",
	}
	devices.infos[info1.name] = info1
	devices.infos[info2.name] = info2
	return devices
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			ds := NewDeviceSet()
			/// Delete devices by the same info
			// deadlocks: x > 0
			go ds.DeleteDevice("info1")
			// deadlocks: x > 0
			go ds.DeleteDevice("info1")
		}()
	}
}
