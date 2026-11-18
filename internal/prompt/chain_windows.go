//go:build windows

package prompt

import (
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ParentChain returns e.g. "aws.exe ← cmd.exe" from a process snapshot.
// Informative only; empty if the snapshot fails.
func ParentChain() string {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(snap)
	type proc struct {
		ppid uint32
		exe  string
	}
	procs := map[uint32]proc{}
	e := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err := windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		procs[e.ProcessID] = proc{e.ParentProcessID, windows.UTF16ToString(e.ExeFile[:])}
	}
	var names []string
	for pid, i := uint32(os.Getppid()), 0; pid > 0 && i < 6; i++ {
		p, ok := procs[pid]
		if !ok {
			break
		}
		names = append(names, p.exe)
		pid = p.ppid
	}
	return strings.Join(names, " ← ")
}
