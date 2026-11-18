//go:build unix

package cli_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestAdapterCreatePermsUnderUmask002(t *testing.T) {
	old := syscall.Umask(0o002)
	defer syscall.Umask(old)
	h := newHarness(t, "")
	h.stdin = "v"
	if code := h.run("adapter", "create", "x"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
	d, _ := os.Stat(h.tdir())
	f, _ := os.Stat(filepath.Join(h.tdir(), "x.tmpl"))
	if d.Mode().Perm() != 0o700 || f.Mode().Perm() != 0o600 {
		t.Fatalf("dir %v file %v", d.Mode().Perm(), f.Mode().Perm())
	}
}
