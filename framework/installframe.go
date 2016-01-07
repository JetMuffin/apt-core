package framework

import (
	"fmt"
	"nata/tools"
	"os"
	"path"
)

//Robotium
//a test framework based on Robotium
type InstallFrame struct {
	AppPath string
	PkgName string
}

//Roborium executor
func (f InstallFrame) TaskExecutor(jobId, deviceId string) {
	outPath := path.Join(jobId, deviceId, OUTPATH)
	file, err := os.Create(outPath)
	if err != nil {
		fmt.Println("InstallFrame create out file err!")
		fmt.Println(err)
	}

	var out string = ""

	cmd := "adb -s " + deviceId + " uninstall " + f.PkgName
	tools.ExeCmd(cmd)
	cmd = "adb -s " + deviceId + " install " + f.AppPath
	out += tools.ExeCmd(cmd)
	cmd = "adb -s " + deviceId + " shell monkey -p " + f.PkgName + " -v 1"
	out += tools.ExeCmd(cmd)
	cmd = "adb -s " + deviceId + " uninstall " + f.PkgName
	tools.ExeCmd(cmd)

	file.WriteString(out)
	file.Sync()
	file.Close()
}

//move test file to target file
func (f InstallFrame) MoveTestFile(disPath string) FrameStruct {
	jobPath := path.Join(disPath, tools.APPNAME)
	cmd := "cp " + f.AppPath + " " + jobPath
	tools.ExeCmd(cmd)
	f.AppPath = jobPath

	return f
}
