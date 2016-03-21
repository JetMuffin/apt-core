package master

import (
	"apsaras/comm"
	"apsaras/comm/comp"
	"apsaras/server/models"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"gopkg.in/mgo.v2/bson"
)

type JobManager struct {
	jobMap  map[string]comp.Job
	jobid   int
	jobLock *sync.Mutex
	idLock  *sync.Mutex
}

var jobManager *JobManager = &JobManager{make(map[string]comp.Job), 0, new(sync.Mutex), new(sync.Mutex)}

func (m *JobManager) idGenerator() string {
	var id = 0
	m.idLock.Lock()
	id = m.jobid
	m.jobid = id + 1
	m.idLock.Unlock()
	return strconv.Itoa(id)
}

func (m *JobManager) addJob(job comp.Job) {
	m.jobLock.Lock()
	_, ex := m.jobMap[job.JobId]
	if ex {
		fmt.Println("Error! Job id repetitive: " + job.JobId)
	} else {
		m.jobMap[job.JobId] = job
	}
	m.jobLock.Unlock()
}

func (m *JobManager) deleteJob(id string) {
	m.jobLock.Lock()
	delete(m.jobMap, id)
	m.jobLock.Unlock()
}

//handle sub job
func (m *JobManager) submitJob(message []byte) {
	mid := m.idGenerator()

	subjob, err := comp.ParserSubJobFromJson(message)
	if err != nil {
		fmt.Println(err)
		return
	}

	var job comp.Job
	job.JobId = mid
	job.JobInfo = subjob
	job.StartTime = time.Now()
	job.LatestTime = time.Now()

	//get current device list
	devices := slaveManager.getDevices()

	//get this job
	devList := job.JobInfo.Filter.GetDeviceSet(devices)
	var taskMap map[string]comp.Task = make(map[string]comp.Task)
	for _, devId := range devList {
		var t comp.Task
		t.JobId = job.JobId
		t.DeviceId = devId
		t.TargetId = devId //勿忘初心
		t.State = comp.TASK_WAIT
		taskMap[devId] = t
	}
	job.TaskMap = taskMap

	//add this job in job manager
	m.addJob(job)
	//insert this job in db
	//TODO
}

func (m *JobManager) updateJobs(getBeat comp.SlaveInfo) {
	m.jobLock.Lock()
	taskinfo := getBeat.TaskStates
	for _, t := range taskinfo {
		jid := t.JobId
		did := t.DeviceId
		job, ex := m.jobMap[jid]
		if !ex {
			fmt.Println("Job not exist! ", jid)
			continue
		}
		_, ex = job.TaskMap[did]
		if !ex {
			fmt.Println("Device not exist! ", did)
			continue
		}
		job.TaskMap[did] = t
		m.jobMap[jid] = job
	}
	m.jobLock.Unlock()
}

//find the oldest job
func (m *JobManager) findOldJob(id string) string {

	var maxJobId string = "-1"
	oldTime := time.Now()

	m.jobLock.Lock()
	//find a old job
	for ke, jo := range m.jobMap {
		ts, ex := jo.TaskMap[id]
		if ex && ts.State == comp.TASK_WAIT {
			if jo.StartTime.Before(oldTime) {
				maxJobId = ke
				oldTime = jo.StartTime
			}
		}
	}
	m.jobLock.Unlock()
	return maxJobId
}

//find the best job
func (m *JobManager) findBestJob(id string) string {
	var maxPriority float64 = -1
	var count1 int = 0
	var maxJob1 string = "-1"

	var maxWaitTime float64 = -1
	var count2 int = 0
	var maxJob2 string = "-1"

	//find a good job
	m.jobLock.Lock()
	for ke, jo := range m.jobMap {
		ts, ex := jo.TaskMap[id]
		if ex && ts.State == comp.TASK_WAIT {
			p := jo.GetPriority()
			if p == -1 {
				if jo.GetWaitTime() > maxWaitTime {
					maxWaitTime = jo.GetWaitTime()
					maxJob2 = ke
				}
				count2++
			} else {
				if p > maxPriority {
					maxPriority = p
					maxJob1 = ke
				}
				count1++
			}
		}
	}
	m.jobLock.Unlock()

	sum := count1 + count2
	if sum <= 0 {
		return "-1"
	}

	rand.Seed(time.Now().UnixNano())
	lottery := rand.Intn(count1 + count2)
	if lottery < count1 {
		return maxJob1
	} else {
		return maxJob2
	}
}

func (m *JobManager) updateJobTaskState(jobId, taskId string, state int) {
	m.jobLock.Lock()
	job := m.jobMap[jobId]
	task := job.TaskMap[taskId]
	task.State = state
	task.StartTime = time.Now()
	job.TaskMap[taskId] = task
	job.LatestTime = time.Now()
	m.jobMap[jobId] = job
	m.jobLock.Unlock()
}

func (m *JobManager) createRuntask(bestJobId, id string) comp.RunTask {
	var rt comp.RunTask
	rt.Frame = m.jobMap[bestJobId].JobInfo.Frame
	rt.TaskInfo = m.jobMap[bestJobId].TaskMap[id]
	return rt
}

//Start find finished job
func (m *JobManager) updateJobInDB() {
	for {
		//update job status
		var finishedJobs []string = make([]string, 0)
		m.jobLock.Lock()
		for jid, job := range m.jobMap {

			all := 0
			pro := 0
			for _, ts := range job.TaskMap {
				all = all + 2
				if ts.State == comp.TASK_COMPLETE || ts.State == comp.TASK_FAIL {
					pro = pro + 2
				} else if ts.State == comp.TASK_RUN {
					pro = pro + 1
				}
			}
			rate := pro * 100 / all
			if all == pro {
				finishedJobs = append(finishedJobs, jid)
			}
			update := bson.M{"Status": rate}
			models.UpdateJobInDB(jid, update)
		}
		m.jobLock.Unlock()

		//delete finished job
		for _, id := range finishedJobs {
			m.deleteJob(id)
		}

		time.Sleep(comm.HEARTTIME)
	}
}
