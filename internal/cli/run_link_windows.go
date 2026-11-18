//go:build windows

package cli

import (
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

func bind(*exec.Cmd) {}

// adopt puts the started child in a Job Object with KILL_ON_JOB_CLOSE. The job
// handle is never closed: Windows closes it when the run dies, killing the child.
// ponytail: tiny window between Start and assignment; CREATE_SUSPENDED if it matters.
func adopt(c *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return err
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(c.Process.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(job, h)
}
