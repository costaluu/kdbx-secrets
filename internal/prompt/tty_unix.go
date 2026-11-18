//go:build !windows

package prompt

import "os"

// OpenConsole opens /dev/tty, so the prompt works even when stdin/stdout are
// redirected. Fails when there is no controlling terminal (cron, GUI launch).
func OpenConsole() (*Console, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &Console{in: f, out: f}, nil
}
