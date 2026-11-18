//go:build windows

package prompt

import "os"

// OpenConsole opens CONIN$/CONOUT$ (pending item 2: unverified when the parent
// captured stdin/stdout).
func OpenConsole() (*Console, error) {
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		in.Close()
		return nil, err
	}
	return &Console{in: in, out: out}, nil
}
