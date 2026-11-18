package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/racoonlemore/kdbx-secrets/internal/cli"
)

const fixturePass = "fixture-pass"

// harness drives cli.Run through its injected environment only.
type harness struct {
	home          string
	stdout, errw  bytes.Buffer
	termIn        string // what the user "types" at the terminal
	termR         *strings.Reader
	termOut       bytes.Buffer
	terminalOpens int
	stdin         string
}

type fakeTerm struct{ h *harness }

func (f fakeTerm) Read(p []byte) (int, error)  { return f.h.termR.Read(p) }
func (f fakeTerm) Write(p []byte) (int, error) { return f.h.termOut.Write(p) }
func (f fakeTerm) Close() error                { return nil }

// newHarness makes a config dir whose config points at database (a path).
func newHarness(t *testing.T, database string) *harness {
	t.Helper()
	h := &harness{home: t.TempDir(), termIn: fixturePass + "\n"}
	if err := os.Chmod(h.home, 0o700); err != nil { // TempDir honors umask; config requires private dir
		t.Fatal(err)
	}
	if database != "" {
		cfg := "# fixture\ndatabase = " + database + "\n"
		if err := os.WriteFile(filepath.Join(h.home, "config"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

func (h *harness) run(args ...string) int {
	return cli.Run(&cli.Env{
		Args:   args,
		Getenv: func(k string) string { return map[string]string{"KDBX_SECRETS_HOME": h.home}[k] },
		Stdin:  strings.NewReader(h.stdin),
		Stdout: &h.stdout,
		Stderr: &h.errw,
		OpenTerminal: func() (cli.Terminal, error) {
			h.terminalOpens++
			h.termR = strings.NewReader(h.termIn)
			return fakeTerm{h}, nil
		},
		Now:     func() time.Time { return time.Unix(0, 0) },
		Version: "1.2.3",
	})
}

func fixture(name string) string {
	p, _ := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	return p
}

func TestGetPasswordFromEveryKDB(t *testing.T) {
	for _, f := range []string{"argon2d.kdbx", "argon2id.kdbx", "aes-kdbx4.kdbx", "aes-kdbx3.kdbx"} {
		t.Run(f, func(t *testing.T) {
			h := newHarness(t, fixture(f))
			if code := h.run("get", "AWS/paper-freeze"); code != 0 {
				t.Fatalf("exit %d, stderr %q", code, h.errw.String())
			}
			if got := h.stdout.String(); got != "s3cr3t-value" {
				t.Fatalf("stdout %q", got)
			}
			if h.termOut.Len() == 0 {
				t.Fatal("prompt should have been written to the terminal")
			}
		})
	}
}

func TestMissingSignatureFailsBeforePrompt(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "x.kdbx")
	if err := os.WriteFile(bad, []byte("definitely not a kdbx file"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, bad)
	if code := h.run("get", "AWS/paper-freeze"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if h.terminalOpens != 0 || h.termOut.Len() != 0 {
		t.Fatal("terminal must stay untouched")
	}
	if h.stdout.Len() != 0 {
		t.Fatalf("stdout %q", h.stdout.String())
	}
}

func TestConfigErrorsExitOneBeforePrompt(t *testing.T) {
	h := newHarness(t, "") // no config file
	if code := h.run("get", "AWS/paper-freeze"); code != 1 || h.terminalOpens != 0 {
		t.Fatalf("exit %d opens %d", code, h.terminalOpens)
	}
	if !strings.Contains(h.errw.String(), "config") {
		t.Fatalf("stderr %q", h.errw.String())
	}
}

func TestWrongPasswordExitsOne(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.termIn = "nope\n"
	if code := h.run("get", "AWS/paper-freeze"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(h.errw.String(), "s3cr3t-value") || h.stdout.Len() != 0 {
		t.Fatal("leak")
	}
}

func TestUnknownReferencePathExitsOne(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	if code := h.run("get", "AWS/does-not-exist"); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	for _, args := range [][]string{{}, {"bogus"}, {"get"}} {
		h := newHarness(t, fixture("argon2d.kdbx"))
		if code := h.run(args...); code != 2 {
			t.Errorf("%v: exit %d", args, code)
		}
		if h.terminalOpens != 0 {
			t.Errorf("%v: terminal touched", args)
		}
	}
}

func TestVersion(t *testing.T) {
	h := newHarness(t, "")
	if code := h.run("version"); code != 0 || h.stdout.String() != "1.2.3\n" {
		t.Fatalf("exit %d out %q", code, h.stdout.String())
	}
	var out bytes.Buffer
	cli.Run(&cli.Env{Args: []string{"version"}, Stdout: &out, Stderr: &out})
	if out.String() != "dev\n" {
		t.Fatalf("fallback: %q", out.String())
	}
}
