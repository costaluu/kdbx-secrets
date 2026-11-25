//go:build linux

package cli

import (
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// openPty returns a pseudo-terminal pair sized like like, so a redacted child
// still sees a terminal (colors, progress bars). OPOST is off: otherwise "\n"
// becomes "\r\n" and a multiline value slips past the redactor.
// ponytail: no SIGWINCH forwarding; a resize during the run is not seen by the child.
func openPty(like *os.File) (master, slave *os.File, err error) {
	master, err = os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	fd := int(master.Fd())
	n, err := unix.IoctlGetUint32(fd, unix.TIOCGPTN)
	if err == nil {
		err = unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0)
	}
	if err == nil {
		slave, err = os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR|unix.O_NOCTTY, 0)
	}
	if err != nil {
		master.Close()
		return nil, nil, err
	}
	sfd := int(slave.Fd())
	if t, err := unix.IoctlGetTermios(sfd, unix.TCGETS); err == nil {
		t.Oflag &^= unix.OPOST
		unix.IoctlSetTermios(sfd, unix.TCSETS, t)
	}
	if ws, err := unix.IoctlGetWinsize(int(like.Fd()), unix.TIOCGWINSZ); err == nil {
		unix.IoctlSetWinsize(sfd, unix.TIOCSWINSZ, ws)
	}
	return master, slave, nil
}
