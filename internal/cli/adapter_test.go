package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (h *harness) tdir() string { return filepath.Join(h.home, "templates") }

func TestAdapterCreateFromStdinAndList(t *testing.T) {
	h := newHarness(t, "")
	h.stdin = `{{get "AWS/x"}}`
	if code := h.run("adapter", "create", "b"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
	h.stdin = "plain"
	if code := h.run("adapter", "create", "a"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
	b, _ := os.ReadFile(filepath.Join(h.tdir(), "b.tmpl"))
	if string(b) != `{{get "AWS/x"}}` {
		t.Fatalf("content %q", b)
	}
	h.stdout.Reset()
	if code := h.run("adapter", "list"); code != 0 || h.stdout.String() != "a\nb\n" {
		t.Fatalf("exit %d list %q", code, h.stdout.String())
	}
	if h.terminalOpens != 0 {
		t.Fatal("touched terminal")
	}
}

func TestAdapterCreateFrom(t *testing.T) {
	h := newHarness(t, "")
	src := filepath.Join(t.TempDir(), "src")
	os.WriteFile(src, []byte("hello"), 0o600)
	if code := h.run("adapter", "create", "x", "--from", src); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
	if b, _ := os.ReadFile(filepath.Join(h.tdir(), "x.tmpl")); string(b) != "hello" {
		t.Fatalf("content %q", b)
	}
}

func TestAdapterCreateInvalidWritesNothing(t *testing.T) {
	for _, body := range []string{`{{ unclosed`, `{{get "a//b"}}`, `{{get}}`} {
		h := newHarness(t, "")
		h.stdin = body
		if code := h.run("adapter", "create", "x"); code == 0 {
			t.Fatalf("%q accepted", body)
		}
		if _, err := os.Stat(h.tdir()); err == nil {
			t.Fatalf("%q: templates dir created", body)
		}
	}
}

func TestAdapterCreateOverwrite(t *testing.T) {
	h := newHarness(t, "")
	h.tmpl(t, "x", "old")
	h.stdin = "new"
	if code := h.run("adapter", "create", "x"); code == 0 {
		t.Fatal("overwrote without --force")
	}
	if b, _ := os.ReadFile(filepath.Join(h.tdir(), "x.tmpl")); string(b) != "old" {
		t.Fatalf("content %q", b)
	}
	h.stdin = "new"
	if code := h.run("adapter", "create", "x", "--force"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
	if b, _ := os.ReadFile(filepath.Join(h.tdir(), "x.tmpl")); string(b) != "new" {
		t.Fatalf("content %q", b)
	}
	ents, _ := os.ReadDir(h.tdir())
	if len(ents) != 1 {
		t.Fatalf("leftover files: %v", ents)
	}
}

func TestAdapterDeleteAndBadNames(t *testing.T) {
	h := newHarness(t, "")
	h.tmpl(t, "x", "v")
	if code := h.run("adapter", "delete", "nope"); code == 0 {
		t.Fatal("deleting missing succeeded")
	}
	if code := h.run("adapter", "delete", "x"); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(h.tdir(), "x.tmpl")); err == nil {
		t.Fatal("still there")
	}
	for _, n := range []string{"../x", ".x", "a/b"} {
		h.stdin = "v"
		if code := h.run("adapter", "create", n); code != 2 {
			t.Fatalf("create %q exit %d", n, code)
		}
	}
	if code := h.run("adapter"); code != 2 || !strings.Contains(h.errw.String(), "usage") {
		t.Fatalf("exit %d", code)
	}
}

func TestAdapterListEmpty(t *testing.T) {
	h := newHarness(t, "")
	if code := h.run("adapter", "list"); code != 0 || h.stdout.String() != "" {
		t.Fatalf("exit %d out %q", code, h.stdout.String())
	}
}
