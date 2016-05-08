package main

import (
	"apsaras/comm"
	"apsaras/comm/comp"
	"apsaras/comm/framework"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"path"
	"sync"
	"time"
)

const (
	GET_DEVICE_CMD = "java -jar getter.jar"
)

type DeviceManager struct {
	deviceMap  map[string]comp.Device
	deviceLock *sync.Mutex
}

var deviceManager *DeviceManager = &DeviceManager{make(map[string]comp.Device), new(sync.Mutex)}

func (m *DeviceManager) loopUpdate() {
	for {
		m.updateDevInfo()
		time.Sleep(comm.UPDATEDEVINFO)
	}
}

func (m *DeviceManager) updateDevInfo() {
	adb := path.Join(getAndroidSDKPath(), framework.ADB_PATH)
	comm.ExeCmd(GET_DEVICE_CMD + " " + adb)

	exist, err := comm.FileExists("dinfo.json")
	if !exist {
		log.Println("dinfo.json not exist!", err)
		return
	}

	//read info from this json
	content, err := ioutil.ReadFile("dinfo.json")
	if err != nil {
		log.Println("dinfo.json not exist!", err)
		return
	}

	//struct this json
	var dvinfos comp.DeviceInfoSlice
	err = json.Unmarshal(content, &dvinfos)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("Devices num: ", len(dvinfos.DeviceInfos))

	//get update devices map
	newMap := make(map[string]comp.Device)
	for _, dvinfo := range dvinfos.DeviceInfos {
		var dev comp.Device
		dev.State = comp.DEVICE_FREE
		dev.Info = dvinfo
		newMap[dvinfo.Id] = dev
		//fmt.Println(dvinfo)
	}

	m.deviceLock.Lock()
	defer m.deviceLock.Unlock()

	for id, ndev := range newMap {
		dev, ok := m.deviceMap[id]
		//old device
		if ok {
			newMap[id] = dev
		} else {
			//new device
			go startMinicap(id, ndev.Info.Resolution)
		}
	}
	for id, _ := range m.deviceMap {
		_, ok := newMap[id]
		if !ok {
			//miss device
			stopMinicap(id)
		}
	}
	m.deviceMap = newMap
}

func (m *DeviceManager) getDeviceInfo() map[string]comp.Device {
	devices := make(map[string]comp.Device)
	m.deviceLock.Lock()
	defer m.deviceLock.Unlock()
	for key, v := range m.deviceMap {
		devices[key] = v
	}
	return devices
}

func (m *DeviceManager) getDevice(id string) (comp.Device, error) {
	var device comp.Device
	err := errors.New("Device not exist!")
	m.deviceLock.Lock()
	defer m.deviceLock.Unlock()

	dev, ex := m.deviceMap[id]
	if ex {
		return dev, nil
	}
	return device, err
}

func (m *DeviceManager) removeDevice(id string) {
	m.deviceLock.Lock()
	defer m.deviceLock.Unlock()

	_, ex := m.deviceMap[id]
	if ex {
		delete(m.deviceMap, id)
	}
}

func (m *DeviceManager) giveDevice(id string) bool {
	ok := false
	m.deviceLock.Lock()
	dev, ex := m.deviceMap[id]
	if ex && dev.State == comp.DEVICE_FREE {
		dev.State = comp.DEVICE_RUN
		ok = true
		m.deviceMap[id] = dev
	}
	m.deviceLock.Unlock()
	return ok
}

func (m *DeviceManager) reclaim(id string) {
	m.deviceLock.Lock()
	defer m.deviceLock.Unlock()

	dev, ex := m.deviceMap[id]
	if ex {
		dev.State = comp.DEVICE_FREE
		m.deviceMap[id] = dev
	}
}
