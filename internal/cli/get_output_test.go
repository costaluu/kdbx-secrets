package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/racoonlemore/kdbx-secrets/internal/cli"
)

// runTTY is like h.run but with stdout reported as a terminal.
func (h *harness) runTTY(args ...string) int {
	return cli.Run(&cli.Env{
		Args:        args,
		Getenv:      func(k string) string { return map[string]string{"KDBX_SECRETS_HOME": h.home}[k] },
		Stdout:      &h.stdout,
		Stderr:      &h.errw,
		StdoutIsTTY: true,
		OpenTerminal: func() (cli.Terminal, error) {
			h.terminalOpens++
			h.termR = strings.NewReader(h.termIn)
			return fakeTerm{h}, nil
		},
		Now: func() time.Time { return time.Unix(0, 0) },
	})
}

func TestGetRefusesTTYWithoutShowBeforePrompt(t *testing.T) {
	h := newHarness(t, fixture("multi.kdbx"))
	if code := h.runTTY("get", "AWS/paper-freeze"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if h.terminalOpens != 0 || h.stdout.Len() != 0 || !strings.Contains(h.errw.String(), "--show") {
		t.Fatalf("opens %d out %q err %q", h.terminalOpens, h.stdout.String(), h.errw.String())
	}
	h = newHarness(t, fixture("multi.kdbx"))
	if code := h.runTTY("get", "--show", "AWS/paper-freeze"); code != 0 || h.stdout.String() != "s3cr3t-value" {
		t.Fatalf("exit %d out %q err %q", code, h.stdout.String(), h.errw.String())
	}
}

func TestGetSanitizesControlCharsInPromptAndStderr(t *testing.T) {
	h := newHarness(t, fixture("multi.kdbx"))
	// existing entry whose title has an escape sequence: the prompt must not carry ESC
	if code := h.run("get", "AWS/evil\ttitle"); code != 0 {
		t.Fatalf("exit %d err %q", code, h.errw.String())
	}
	if strings.ContainsRune(h.termOut.String(), '\t') || !strings.Contains(h.termOut.String(), "evil?title") {
		t.Fatalf("prompt %q", h.termOut.String())
	}
	h = newHarness(t, fixture("multi.kdbx"))
	if code := h.run("get", "AWS/nope\x1b[31m\a"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if strings.ContainsAny(h.errw.String(), "\x1b\a") {
		t.Fatalf("stderr %q", h.errw.String())
	}
}

func TestGetManyRefsOnePromptInOrder(t *testing.T) {
	h := newHarness(t, fixture("multi.kdbx"))
	if code := h.run("get", "AWS/evil\ttitle", "AWS/paper-freeze", "AWS/evil\ttitle"); code != 0 {
		t.Fatalf("exit %d err %q", code, h.errw.String())
	}
	if h.terminalOpens != 1 || h.stdout.String() != "x\ns3cr3t-value\nx\n" {
		t.Fatalf("opens %d out %q", h.terminalOpens, h.stdout.String())
	}
}

func TestGetManyRefsMultilineFailsWithoutOutput(t *testing.T) {
	h := newHarness(t, fixture("multi.kdbx"))
	if code := h.run("get", "AWS/paper-freeze", "AWS/multiline"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if h.stdout.Len() != 0 || strings.Contains(h.errw.String(), "line1") || !strings.Contains(h.errw.String(), "AWS/multiline") {
		t.Fatalf("out %q err %q", h.stdout.String(), h.errw.String())
	}
}
