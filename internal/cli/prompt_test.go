package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/racoonlemore/kdbx-secrets/internal/cli"
)

func TestThreeWrongAttemptsFailAndLaterTrySucceeds(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.termIn = "a\nb\nc\nfixture-pass\n" // 4th is never read
	if code := h.run("get", "AWS/paper-freeze"); code != 1 || h.stdout.Len() != 0 {
		t.Fatalf("exit %d out %q", code, h.stdout.String())
	}
	if !strings.Contains(h.errw.String(), "incorrect password") {
		t.Fatalf("stderr %q", h.errw.String())
	}
	for _, in := range []string{"a\nfixture-pass\n", "a\nb\nfixture-pass\n"} {
		h = newHarness(t, fixture("argon2d.kdbx"))
		h.termIn = in
		if code := h.run("get", "AWS/paper-freeze"); code != 0 || h.stdout.String() != "s3cr3t-value" {
			t.Fatalf("%q: exit %d out %q", in, code, h.stdout.String())
		}
	}
}

func TestTruncatedFileIsNotWrongPassword(t *testing.T) {
	b, err := os.ReadFile(fixture("argon2d.kdbx"))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "t.kdbx")
	if err := os.WriteFile(p, b[:len(b)-40], 0o600); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, p)
	if code := h.run("get", "AWS/paper-freeze"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if e := h.errw.String(); !strings.Contains(e, "incomplete or corrupted") || strings.Contains(e, "incorrect password") {
		t.Fatalf("stderr %q", e)
	}
}

func TestEOFCancelsWithoutConsumingAttempt(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.termIn = "wrong\n" // then EOF: cancel, not a failed attempt
	if code := h.run("get", "AWS/paper-freeze"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if e := h.errw.String(); !strings.Contains(e, "cancelled") || strings.Contains(e, "incorrect password after") {
		t.Fatalf("stderr %q", e)
	}
	if n := strings.Count(h.termOut.String(), "KeePass password"); n != 2 {
		t.Fatalf("prompts %d", n)
	}
}

func TestEmptyPasswordOpensEmptyFixture(t *testing.T) {
	h := newHarness(t, fixture("empty-pass.kdbx"))
	h.termIn = "\n"
	if code := h.run("get", "AWS/paper-freeze"); code != 0 || h.stdout.String() != "s3cr3t-value" {
		t.Fatalf("exit %d err %q", code, h.errw.String())
	}
}

func TestOversizedPasswordRefusedWithoutOpening(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.termIn = strings.Repeat("x", 1025) + "\n"
	if code := h.run("get", "AWS/paper-freeze"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if e := h.errw.String(); !strings.Contains(e, "1024") || strings.Contains(e, "incorrect") {
		t.Fatalf("stderr %q", e)
	}
}

func TestPromptShowsChainAndSanitizedReferences(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	cli.Run(&cli.Env{
		Args:         []string{"get", "AWS/paper-freeze"},
		Getenv:       func(string) string { return h.home },
		ParentChain:  func() string { return "aws ← zsh" },
		OpenTerminal: func() (cli.Terminal, error) { h.termR = strings.NewReader(h.termIn); return fakeTerm{h}, nil },
	})
	if s := h.termOut.String(); !strings.Contains(s, "aws ← zsh") || !strings.Contains(s, "AWS/paper-freeze") {
		t.Fatalf("prompt %q", s)
	}
	h2 := newHarness(t, fixture("argon2d.kdbx"))
	h2.run("get", "AWS/x\x1b[2Jy")
	if strings.ContainsRune(h2.termOut.String(), 0x1b) {
		t.Fatal("escape sequence leaked to the terminal")
	}
}

func TestNoTerminalIsClear(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	code := cli.Run(&cli.Env{
		Args: []string{"get", "AWS/paper-freeze"}, Stderr: &h.errw,
		Getenv:       func(string) string { return h.home },
		OpenTerminal: func() (cli.Terminal, error) { return nil, os.ErrNotExist },
	})
	if code != 1 || !strings.Contains(h.errw.String(), "no terminal") {
		t.Fatalf("exit %d stderr %q", code, h.errw.String())
	}
}
