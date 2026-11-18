//go:build !linux && !windows

package prompt

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ParentChain returns e.g. "aws ← zsh" using ps (macOS and other Unixes).
// Informative only.
func ParentChain() string {
	var names []string
	for pid, i := os.Getppid(), 0; pid > 1 && i < 6; i++ {
		out, err := exec.Command("ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid)).Output()
		f := strings.Fields(string(out))
		if err != nil || len(f) < 2 {
			break
		}
		names = append(names, f[1][strings.LastIndex(f[1], "/")+1:])
		pid, _ = strconv.Atoi(f[0])
	}
	return strings.Join(names, " ← ")
}
