package comp

import (
	"apsaras/comm"
	"apsaras/comm/framework"
	"errors"
	"time"

	"github.com/bitly/go-simplejson"
)

//job id, framework and device list, time
type Job struct {
	JobId      string
	JobInfo    SubJob
	TaskMap    map[string]Task
	StartTime  time.Time
	FinishTime time.Time
	LatestTime time.Time
}

type JobMap struct {
	Map map[string]Job
}

type JobBrief struct {
	JobId      string
	StartTime  string
	FrameKind  string
	FilterKind string
	Status     int
}

type SubJob struct {
	FrameKind  string
	Frame      framework.FrameStruct
	FilterKind string
	Filter     FilterInterface
}

func (job Job) ToBrief() JobBrief {
	var jbr JobBrief
	jbr.JobId = job.JobId
	jbr.StartTime = job.StartTime.Format("2006-01-02 15:04:05")
	jbr.FrameKind = job.JobInfo.FrameKind
	jbr.FilterKind = job.JobInfo.FilterKind
	jbr.Status = 0
	return jbr
}

//parser submited job from json
func ParserSubJobFromJson(content []byte) (SubJob, error) {

	js, err := simplejson.NewJson(content)
	comm.CheckError(err)
	framekind, err := js.Get("FrameKind").String()
	comm.CheckError(err)
	filterkind, err := js.Get("FilterKind").String()
	comm.CheckError(err)

	var sj SubJob

	switch framekind {
	case framework.FRAME_ROBOT:
		sj.FrameKind = framework.FRAME_ROBOT
		var rf framework.RobotFrame

		appPath, err1 := js.Get("Frame").Get("AppPath").String()
		testPath, err2 := js.Get("Frame").Get("TestPath").String()
		if err1 != nil || err2 != nil {
			err := errors.New("Robotium framework error in json file! File path of App and Test are needed!")
			return sj, err
		}
		rf.AppPath = appPath
		rf.TestPath = testPath
		sj.Frame = rf
	case framework.FRAME_MONKEY:
		sj.FrameKind = framework.FRAME_MONKEY
		var mf framework.MonkeyFrame

		appPath, err1 := js.Get("Frame").Get("AppPath").String()
		argu, err2 := js.Get("Frame").Get("Argu").String()
		pkg, err3 := js.Get("Frame").Get("PkgName").String()
		if err1 != nil || err2 != nil || err3 != nil {
			err := errors.New("MonkeyFrame error in json file! AppPath, Argu and PkgName are needed!")
			return sj, err
		}
		mf.AppPath = appPath
		mf.Argu = argu
		mf.PkgName = pkg
		sj.Frame = mf
	case framework.FRAME_INSTALL:
		sj.FrameKind = framework.FRAME_INSTALL
		var inf framework.InstallFrame
		appPath, err1 := js.Get("Frame").Get("AppPath").String()
		pkg, err2 := js.Get("Frame").Get("PkgName").String()
		if err1 != nil || err2 != nil {
			err := errors.New("InstallFrame error in json file! File path of App and package are needed!")
			return sj, err
		}
		inf.AppPath = appPath
		inf.PkgName = pkg
		sj.Frame = inf
	default:
		err := errors.New("Unknow Framework!")
		return sj, err
	}

	switch filterkind {
	case FILTER_SPECIFYDEVICES:
		sj.FilterKind = FILTER_SPECIFYDEVICES
		var filter SpecifyDevFilter
		idList, err1 := js.Get("Filter").Get("IdList").StringArray()
		if err1 != nil {
			err := errors.New("SpecifyDevices filter error in json file! IdList is needed!")
			return sj, err
		}
		filter.IdList = idList
		sj.Filter = filter

	case FILTER_SPECIFYATTR:
		sj.FilterKind = FILTER_SPECIFYATTR
		var filter SpecifyAttrFilter
		qt, err1 := js.Get("Filter").Get("Quantity").Int()
		at, err2 := js.Get("Filter").Get("Attr").String()
		vl, err3 := js.Get("Filter").Get("Value").String()
		if err1 != nil || err2 != nil || err3 != nil {
			err := errors.New("SpecifyAttr filter error in json file! Quantity, Attr and Value  are needed!")
			return sj, err
		}
		filter.Attr = at
		filter.Quantity = qt
		filter.Value = vl
		sj.Filter = filter

	case FILTER_COMPATIBILITY:
		sj.FilterKind = FILTER_COMPATIBILITY
		var filter CompatibilityFilter
		qt, err1 := js.Get("Filter").Get("Quantity").Int()
		dt, err2 := js.Get("Filter").Get("Dominate").String()
		if err1 != nil || err2 != nil {
			err := errors.New("Compatibility filter error in json file! Quantity, Dominate are needed!")
			return sj, err
		}
		filter.Dominate = dt
		filter.Quantity = qt
		sj.Filter = filter

	default:
		err := errors.New("Unknow Filter!")
		return sj, err
	}

	return sj, nil
}

//Get the priority of this job
func (job Job) GetPriority() float64 {
	waitTime := job.GetWaitTime()
	var devTime float64 = 0
	for _, ts := range job.TaskMap {
		switch ts.State {
		case TASK_COMPLETE:
			fallthrough
		case TASK_FAIL:
			devTime += ts.FinishTime.Sub(ts.StartTime).Seconds()
		case TASK_RUN:
			devTime += time.Now().Sub(ts.StartTime).Seconds()
		}
	}
	if devTime != 0 {
		return waitTime / devTime
	}
	//no task of this job has run
	return -1
}

//Get wait time of this job
func (job Job) GetWaitTime() float64 {
	waitTime := time.Now().Sub(job.LatestTime).Seconds()
	return waitTime
}
