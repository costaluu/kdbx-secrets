package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writableFixture copies a fixture so the test may write it.
func writableFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(fixture(name))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), filepath.Base(name))
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// fakeKeePassXC is a keepassxc-cli stand-in that logs "args\nstdin---" per call.
func fakeKeePassXC(t *testing.T) (cmd, log string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stand-in")
	}
	dir := t.TempDir()
	log = filepath.Join(dir, "log")
	cmd = filepath.Join(dir, "kx")
	script := "#!/bin/sh\n{ echo \"$@\"; cat; echo ---; } >> " + log + "\n"
	if err := os.WriteFile(cmd, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return cmd, log
}

func TestFillCallsKeePassXCOnlyForWhatIsMissing(t *testing.T) {
	kx, log := fakeKeePassXC(t)
	p := writableFixture(t, "argon2d.kdbx")
	h := newHarness(t, p)
	h.env = map[string]string{"KEEPASSXC_CLI": kx}
	h.tmpl(t, "x", `{{get "AWS/paper-freeze#UserName"}} {{get "AWS/paper-freeze#URL"}}
{{get "Dev/GitHub/bot#UserName"}} {{get "Dev/GitHub/bot"}} {{get "Dev/GitLab/bot"}}`)
	h.termIn = fixturePass + "\nhttps://aws\nrobot\nghp_secret\nglpat\n"
	if code := h.run("adapter", "fill", "x"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
	got, _ := os.ReadFile(log)
	want := strings.Join([]string{
		"edit -q --url=https://aws " + p + " AWS/paper-freeze\n" + fixturePass + "\n---",
		"mkdir -q " + p + " Dev\n" + fixturePass + "\n---",
		"mkdir -q " + p + " Dev/GitHub\n" + fixturePass + "\n---",
		"add -q --username=robot -p " + p + " Dev/GitHub/bot\n" + fixturePass + "\nghp_secret\n---",
		"mkdir -q " + p + " Dev/GitLab\n" + fixturePass + "\n---", // Dev made once
		"add -q -p " + p + " Dev/GitLab/bot\n" + fixturePass + "\nglpat\n---",
	}, "\n") + "\n"
	if string(got) != want {
		t.Fatalf("calls:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(h.stdout.String(), "ghp_secret") || strings.Contains(h.termOut.String(), "ghp_secret") {
		t.Fatal("secret echoed")
	}
	if !strings.Contains(h.stdout.String(), "filled Dev/GitHub/bot\n") {
		t.Fatalf("stdout %q", h.stdout.String())
	}
}

func TestFillNothingMissingRunsNothing(t *testing.T) {
	kx, log := fakeKeePassXC(t)
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.env = map[string]string{"KEEPASSXC_CLI": kx}
	h.tmpl(t, "x", `{{get "AWS/paper-freeze#UserName"}} {{get "AWS/paper-freeze"}}`)
	if code := h.run("adapter", "fill", "x"); code != 0 || !strings.Contains(h.stdout.String(), "nothing to fill") {
		t.Fatalf("exit %d out %q err %s", code, h.stdout.String(), h.errw.String())
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("keepassxc-cli called")
	}
}

func TestFillRefusesBeforeWriting(t *testing.T) {
	kx, log := fakeKeePassXC(t)
	for body, want := range map[string]string{
		`{{get "AWS/paper-freeze#TOTP"}}`:                 "cannot write this field",
		`{{get "AWS/paper-freeze#attachment:k.pem"}}`:     "cannot write this field",
		`{{get "AWS/paper-freeze#region"}}`:               "cannot write this field",
		`{{get "New/entry#region"}}`:                      "cannot write this field",
		`{{get "uuid:00112233445566778899aabbccddeeff"}}`: "entry not found",
		`{{get "New/entry"}}`:                             "empty value",
	} {
		h := newHarness(t, fixture("argon2d.kdbx"))
		h.env = map[string]string{"KEEPASSXC_CLI": kx}
		h.tmpl(t, "x", body)
		h.termIn = fixturePass + "\n\n"
		if code := h.run("adapter", "fill", "x"); code != 1 || !strings.Contains(h.errw.String(), want) {
			t.Fatalf("%s: exit %d err %q", body, code, h.errw.String())
		}
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("keepassxc-cli called")
	}
}

func TestFillWithoutKeePassXC(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.env = map[string]string{"KEEPASSXC_CLI": filepath.Join(t.TempDir(), "absent")}
	h.tmpl(t, "x", `{{get "New/entry"}}`)
	if code := h.run("adapter", "fill", "x"); code != 1 || !strings.Contains(h.errw.String(), "KEEPASSXC_CLI") {
		t.Fatalf("exit %d err %q", code, h.errw.String())
	}
	if h.terminalOpens != 0 {
		t.Fatal("prompted without keepassxc-cli")
	}
}

func TestFillUsage(t *testing.T) {
	h := newHarness(t, "")
	if code := h.run("adapter", "fill"); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if code := h.run("adapter", "fill", "missing"); code != 1 || !strings.Contains(h.errw.String(), "not found") {
		t.Fatalf("exit %d err %q", code, h.errw.String())
	}
	if h.terminalOpens != 0 {
		t.Fatal("prompted before validating")
	}
}

// TestFillRealKeePassXC runs only where keepassxc-cli exists (KEEPASSXC_CLI or
// PATH). Under flatpak, KEEPASSXC_CLI needs --filesystem=/tmp.
func TestFillRealKeePassXC(t *testing.T) {
	kx := os.Getenv("KEEPASSXC_CLI")
	if kx == "" {
		if _, err := exec.LookPath("keepassxc-cli"); err != nil {
			t.Skip("keepassxc-cli not available")
		}
	}
	for _, f := range []string{"argon2id.kdbx", "aes-kdbx3.kdbx", "keepassxc/rich.kdbx"} {
		t.Run(f, func(t *testing.T) {
			p := writableFixture(t, f)
			before, _ := os.ReadFile(p)
			h := newHarness(t, p)
			h.env = map[string]string{"KEEPASSXC_CLI": kx}
			h.tmpl(t, "x", `{{get "AWS/paper-freeze#URL"}} {{get "AWS/paper-freeze"}} {{get "Dev/new/bot#UserName"}} {{get "Dev/new/bot"}}`)
			h.termIn = fixturePass + "\nhttps://aws\nrobot\nnew-secret\n"
			if code := h.run("adapter", "fill", "x"); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errw.String())
			}
			if after, _ := os.ReadFile(p); bytes.Equal(before, after) {
				t.Fatal("file not written")
			}
			h2 := newHarness(t, p)
			if code := h2.run("get", "--show", "AWS/paper-freeze#URL", "AWS/paper-freeze", "Dev/new/bot#UserName", "Dev/new/bot"); code != 0 {
				t.Fatalf("exit %d: %s", code, h2.errw.String())
			}
			if h2.stdout.String() != "https://aws\ns3cr3t-value\nrobot\nnew-secret\n" {
				t.Fatalf("stdout %q", h2.stdout.String())
			}
		})
	}
}
