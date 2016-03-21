package main

import (
	"apsaras/comm"
	"apsaras/comm/comp"
	"apsaras/comm/framework"
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

var shareDirPath string
var serviceIP string
var adbPath string
var mIP string
var taskLock *sync.Mutex
var devLock *sync.Mutex

//Record all devices in this slave
//serial num : device
var deviceMap map[string]comp.Device

var taskMap map[string]comp.RunTask

func main() {

	//init
	deviceMap = make(map[string]comp.Device)
	taskMap = make(map[string]comp.RunTask)
	taskLock = new(sync.Mutex)
	devLock = new(sync.Mutex)

	//read config file
	cf, err := os.Open("slave.conf")
	comm.CheckError(err)

	reader := bufio.NewReaderSize(cf, 1024)
	//share path
	line, _, err := reader.ReadLine()
	comm.CheckError(err)
	sublines := strings.Split(string(line), "=")
	if len(sublines) == 2 && sublines[0] == "share" {
		shareDirPath = sublines[1]
		fmt.Println("share file path: " + shareDirPath)
	} else {
		fmt.Println("share path error: " + string(line))
	}

	//service addr
	line, _, err = reader.ReadLine()
	comm.CheckError(err)
	sublines = strings.Split(string(line), "=")
	if len(sublines) == 2 && sublines[0] == "master" {
		serviceIP = sublines[1]
		fmt.Println("service addr is: " + serviceIP)
	} else {
		fmt.Println("service error: " + string(line))
	}

	//adb path
	line, _, err = reader.ReadLine()
	comm.CheckError(err)
	sublines = strings.Split(string(line), "=")
	if len(sublines) == 2 && sublines[0] == "adb" {
		adbPath = sublines[1]
		fmt.Println("adb path is: " + adbPath)
	} else {
		fmt.Println("adb path error: " + string(line))
	}

	cf.Close()

	//register in gob
	framework.RigisterGob()
	//start connet to master
	diaMaster()
	fmt.Println("Slave Over!")
}

//Commucate with master
func diaMaster() {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", serviceIP)
	comm.CheckError(err)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	comm.CheckError(err)
	defer conn.Close()
	mIP = conn.LocalAddr().String()
	mIP = strings.Split(mIP, ":")[0]

	//start update devices information
	updateDevInfo()
	go loopUpdateDevInfo()

	//say hi
	_, err = conn.Write([]byte(comm.HIMASTER))
	comm.CheckError(err)

	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)
	//send heart beat
	for {
		var beat comp.SlaveInfo
		beat.IP = mIP

		//copy deivces information
		devLock.Lock()

		beat.DeviceStates = make(map[string]comp.Device)
		for key, v := range deviceMap {
			beat.DeviceStates[key] = v
		}
		devLock.Unlock()

		//record all task state
		beat.TaskStates = make(map[string]comp.Task)
		taskLock.Lock()
		for ke, ts := range taskMap {
			beat.TaskStates[ke] = ts.TaskInfo
		}

		//remove finished task
		for ke, ts := range beat.TaskStates {
			if ts.State == comp.TASK_COMPLETE || ts.State == comp.TASK_FAIL {
				//TODO move file is time-consuming
				srcPath := path.Join(ts.JobId, ts.DeviceId)
				dstPath := path.Join(shareDirPath, ts.JobId)
				cmd := "cp -r " + srcPath + " " + dstPath
				comm.ExeCmd(cmd)
				delete(taskMap, ke)
			}
		}
		taskLock.Unlock()

		fmt.Println(beat.IP + ":I send beat to master!")
		//send beat
		err = encoder.Encode(&beat)
		if err != nil {
			fmt.Println(err)
			break
		}

		//get response
		conn.SetReadDeadline(time.Now().Add(comm.WAITFORDIA))
		var newTasks comp.RunTaskList
		err = decoder.Decode(&newTasks)
		if err != nil {
			fmt.Println(err)
			break
		}
		if len(newTasks.Tasks) > 0 {
			taskLock.Lock()
			for _, ts := range newTasks.Tasks {
				key := ts.TaskInfo.JobId + ":" + ts.TaskInfo.DeviceId
				_, ex := taskMap[key]
				if ex {
					fmt.Println("Error! Task have same ID!")
				} else {
					//allocte the device
					devLock.Lock()
					dev, exist := deviceMap[ts.TaskInfo.DeviceId]
					if exist && dev.State == comp.DEVICE_FREE {

						//create local file
						exist, err := comm.FileExists(ts.TaskInfo.JobId)
						if err != nil {
							fmt.Println("Cannot find out file!")
							fmt.Println(err)
							continue
						}
						if !exist {
							//create local file
							err = os.Mkdir(ts.TaskInfo.JobId, os.ModePerm)
							if err != nil {
								fmt.Println("Cannot create out file!")
								fmt.Println(err)
								continue
							}
							//make test locally
							ts.Frame = ts.Frame.MoveTestFile(ts.TaskInfo.JobId)
						}

						//run this comp
						fmt.Println("Start task: " + ts.TaskInfo.JobId + "--" + ts.TaskInfo.DeviceId)
						dev.State = comp.DEVICE_RUN
						deviceMap[ts.TaskInfo.DeviceId] = dev

						go runTask(ts)
					} else {
						ts.TaskInfo.State = comp.TASK_FAIL
						taskMap[key] = ts
						fmt.Println("Error! Device dose not exist or device is not free!")
					}
					devLock.Unlock()
					taskMap[key] = ts
				}
			}
			taskLock.Unlock()
		}
		//sleep some time before heartbeat
		time.Sleep(comm.HEARTTIME)
	}
}

//Update devices infomation at intervals
func loopUpdateDevInfo() {
	for {
		time.Sleep(comm.UPDATEDEVINFO)
		updateDevInfo()
	}
}

//Update devices
//If the number of devices change then return true, otherwise return false.
func updateDevInfo() {
	comm.ExeCmd("java -jar getter.jar " + adbPath)

	exist, err := comm.FileExists("dinfo.json")
	if !exist {
		fmt.Println("dinfo.json not exist!")
		os.Exit(1)
	}
	comm.CheckError(err)

	//read info from this json
	content, err := ioutil.ReadFile("dinfo.json")
	comm.CheckError(err)

	//struct this json
	var dvinfos comp.DeviceInfoSlice
	err = json.Unmarshal(content, &dvinfos)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		return
	}

	//get update devices map
	newMap := make(map[string]comp.Device)
	for _, dvinfo := range dvinfos.DeviceInfos {
		var dev comp.Device
		//dev.IP = mIP
		dev.State = comp.DEVICE_FREE
		dev.Info = dvinfo
		newMap[dvinfo.Id] = dev
		//fmt.Println(dvinfo)
	}

	devLock.Lock()
	for id, dev := range deviceMap {
		_, ok := newMap[id]
		if ok {
			newMap[id] = dev
		}
	}
	deviceMap = newMap
	devLock.Unlock()
	fmt.Println("Get devices info finished!")
}

////Allot devices to tasks
//func allotDevice() {
//	taskLock.Lock()
//	for key, ts := range taskMap {
//		if ts.TaskInfo.State == task.TASK_QUEUE {
//			devLock.Lock()
//			dev, exist := deviceMap[ts.TaskInfo.DeviceId]
//			if exist && dev.State == comp.DEVICE_FREE {
//				//run this task
//				fmt.Println("Start task: " + ts.TaskInfo.JobId + "--" + ts.TaskInfo.DeviceId)
//				dev.State = comp.DEVICE_RUN
//				deviceMap[ts.TaskInfo.DeviceId] = dev
//				go runTask(ts)
//				ts.TaskInfo.State = task.TASK_RUN
//				taskMap[key] = ts
//			} else if !exist {
//				ts.TaskInfo.State = task.TASK_FAIL
//				taskMap[key] = ts
//			}
//			devLock.Unlock()
//		}
//	}
//	taskLock.Unlock()
//}

//Run a test task
func runTask(ts comp.RunTask) {
	jobId := ts.TaskInfo.JobId
	devId := ts.TaskInfo.DeviceId
	key := jobId + ":" + devId

	taskOutPath := path.Join(jobId, devId)
	err := os.Mkdir(taskOutPath, os.ModePerm)
	if err != nil {
		fmt.Println("Cannot create out file!")
		fmt.Println(err)
		ts.TaskInfo.FinishTime = time.Now()
		ts.TaskInfo.State = comp.TASK_FAIL
		taskLock.Lock()
		taskMap[key] = ts
		taskLock.Unlock()
		return
	}
	//start task
	ts.Frame.TaskExecutor(jobId, devId)
	ts.TaskInfo.FinishTime = time.Now()
	ts.TaskInfo.State = comp.TASK_COMPLETE

	//Update device information
	devLock.Lock()
	dev, exist := deviceMap[devId]
	if exist {
		dev.State = comp.DEVICE_FREE
		deviceMap[devId] = dev
	}
	devLock.Unlock()

	//update task information
	taskLock.Lock()
	_, exist = taskMap[key]
	if exist {
		taskMap[key] = ts
	}
	taskLock.Unlock()
}
