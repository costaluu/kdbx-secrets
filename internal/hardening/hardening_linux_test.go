//go:build linux

package hardening

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// Subprocess mode: the re-executed test binary lowers the limits, applies the
// hardening and reports what it sees. The variable carries no secret.
func TestMain(m *testing.M) {
	if os.Getenv("HARDENING_HELPER") == "1" {
		unix.Setrlimit(unix.RLIMIT_MEMLOCK, &unix.Rlimit{Cur: 1 << 20, Max: 1 << 20})
		Apply(64<<20, os.Stderr)
		d, _ := unix.PrctlRetInt(unix.PR_GET_DUMPABLE, 0, 0, 0, 0)
		var core unix.Rlimit
		unix.Getrlimit(unix.RLIMIT_CORE, &core)
		println("dumpable", d, "core", core.Cur, core.Max)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestApplyInSubprocess(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "HARDENING_HELPER=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helper failed: %v\n%s", err, stderr.String())
	}
	out := stderr.String()
	if !strings.Contains(out, "dumpable 0 core 0 0") {
		t.Errorf("hardening not applied: %s", out)
	}
	if !strings.Contains(out, "continuing without memory lock") {
		t.Errorf("missing no-lock warning: %s", out)
	}
}
