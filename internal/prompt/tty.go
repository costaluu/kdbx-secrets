package prompt

import (
	"fmt"
	"os"
	"os/signal"

	"golang.org/x/term"
)

// Console is the real controlling terminal (see tty_unix.go / tty_windows.go
// for how it is opened). It reads without echo.
type Console struct{ in, out *os.File }

func (c *Console) Read(p []byte) (int, error)  { return c.in.Read(p) }
func (c *Console) Write(p []byte) (int, error) { return c.out.Write(p) }

// ReadPassword reads without echo, bounded by MaxPassword. The terminal is in
// raw mode and always restored: Ctrl-C (or a SIGINT) restores it and exits 1,
// Ctrl-D on an empty line returns ErrCancelled.
func (c *Console) ReadPassword(buf []byte) ([]byte, error) {
	fd := int(c.in.Fd())
	st, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	defer term.Restore(fd, st)
	die := func() {
		term.Restore(fd, st)
		fmt.Fprintln(c.out)
		os.Exit(1)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	go func() {
		if _, ok := <-sig; ok {
			die()
		}
	}()
	pw, err := ReadSecret(c.in, buf, true)
	if err == ErrInterrupted {
		die()
	}
	return pw, err
}

func (c *Console) Close() error {
	c.out.Close()
	return c.in.Close()
}
