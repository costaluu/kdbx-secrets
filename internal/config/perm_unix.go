//go:build !windows

package config

import (
	"fmt"
	"os"
	"syscall"
)

// CheckPrivate refuses a path not owned by the current user or writable by
// group/others (same rule as ssh). Reused for templates.
func CheckPrivate(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%s: not owned by the current user", path)
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s: writable by group or others (mode %04o); fix with chmod go-w", path, fi.Mode().Perm())
	}
	return nil
}
