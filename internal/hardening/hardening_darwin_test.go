//go:build darwin

package hardening

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// Subprocess mode, as in the Linux test. Compiled via cross vet only; never
// run on a real Mac by the author.
func TestMain(m *testing.M) {
	if os.Getenv("HARDENING_HELPER") == "1" {
		Apply(0, os.Stderr)
		var core unix.Rlimit
		unix.Getrlimit(unix.RLIMIT_CORE, &core)
		println("core", core.Cur, core.Max)
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
	if !strings.Contains(out, "core 0 0") {
		t.Errorf("RLIMIT_CORE not zero: %s", out)
	}
	if strings.Contains(out, "PT_DENY_ATTACH") {
		t.Errorf("PT_DENY_ATTACH failed: %s", out)
	}
}
