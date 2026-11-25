//go:build !linux

package cli

import (
	"errors"
	"os"
)

// ponytail: PTY only on Linux; elsewhere a redacted child writes to a pipe and
// loses colors. posix_openpt via cgo-free ioctls if macOS users miss them.
func openPty(*os.File) (master, slave *os.File, err error) {
	return nil, nil, errors.New("no pty")
}
