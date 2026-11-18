package prompt

import (
	"fmt"
	"os"
	"strings"
)

// ParentChain returns e.g. "aws ← zsh": the parent processes, nearest first.
// Informative only, not authentication.
func ParentChain() string {
	var names []string
	for pid, i := os.Getppid(), 0; pid > 1 && i < 6; i++ {
		comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		stat, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if len(comm) == 0 {
			break
		}
		names = append(names, strings.TrimSpace(string(comm)))
		// stat: "pid (comm) S ppid ..."; comm may hold spaces, so cut after the last ')'.
		s := string(stat)
		pid = 0
		if j := strings.LastIndex(s, ")"); j >= 0 {
			fmt.Sscanf(s[j+1:], " %s %d", new(string), &pid)
		}
	}
	return strings.Join(names, " ← ")
}
