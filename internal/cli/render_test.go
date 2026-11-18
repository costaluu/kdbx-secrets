package cli_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func (h *harness) tmpl(t *testing.T, name, body string) {
	t.Helper()
	d := filepath.Join(h.home, "templates")
	if err := os.MkdirAll(d, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, name+".tmpl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const paperFreeze = `{"Version":1,"AccessKeyId":{{get "AWS/paper-freeze" | json}},"SecretAccessKey":{{get "AWS/paper-freeze" | json}}}`

func TestRenderPaperFreeze(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.tmpl(t, "aws-paper-freeze", paperFreeze)
	if code := h.run("render", "aws-paper-freeze"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
	want := `{"Version":1,"AccessKeyId":"s3cr3t-value","SecretAccessKey":"s3cr3t-value"}`
	if h.stdout.String() != want {
		t.Fatalf("stdout %q", h.stdout.String())
	}
	if h.terminalOpens != 1 || !strings.Contains(h.termOut.String(), "AWS/paper-freeze") {
		t.Fatalf("opens %d prompt %q", h.terminalOpens, h.termOut.String())
	}
}

func TestRenderNewRefInRealPassFails(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	// empty on collection, so the branch only runs for real
	h.tmpl(t, "t", `{{if eq (get "AWS/paper-freeze") "s3cr3t-value"}}{{get "AWS/other"}}{{end}}`)
	if code := h.run("render", "t"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if h.terminalOpens != 1 || h.stdout.Len() != 0 || strings.Contains(h.errw.String(), "s3cr3t") {
		t.Fatalf("opens %d out %q err %q", h.terminalOpens, h.stdout.String(), h.errw.String())
	}
}

func TestRenderErrorsRewrittenNoLeak(t *testing.T) {
	cases := map[string]string{
		"syn": "a\n{{if}}\n",
		"exe": "a\nb\n{{index (get \"AWS/paper-freeze\") 99}}",
	}
	for name, body := range cases {
		h := newHarness(t, fixture("argon2d.kdbx"))
		h.tmpl(t, name, body)
		if code := h.run("render", name); code != 1 {
			t.Fatalf("%s: exit %d", name, code)
		}
		e := h.errw.String()
		if !strings.Contains(e, "template "+name+": line ") || strings.Contains(e, "s3cr3t") || strings.Contains(e, "index") {
			t.Fatalf("%s: stderr %q", name, e)
		}
	}
}

func TestRenderBadRefSyntaxBeforePrompt(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.tmpl(t, "t", `{{get "a//b"}}`)
	if code := h.run("render", "t"); code != 1 || h.terminalOpens != 0 {
		t.Fatalf("exit %d opens %d", code, h.terminalOpens)
	}
}

func TestRenderNameAndMissingBeforePrompt(t *testing.T) {
	for _, n := range []string{"../x", ".hidden", "a b", "nope"} {
		h := newHarness(t, fixture("argon2d.kdbx"))
		if code := h.run("render", n); code == 0 || h.terminalOpens != 0 {
			t.Fatalf("%q: exit %d opens %d", n, code, h.terminalOpens)
		}
	}
}

func TestRenderJSONAndBase64(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.tmpl(t, "t", `{{"a\"<b>\n" | json}} {{"hi?" | base64}}`)
	if code := h.run("render", "t"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errw.String())
	}
	if got, want := h.stdout.String(), `"a\"<b>\n" aGk/`; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderRejectsGroupWritableTemplate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission check is Unix-only (ACL check deferred)")
	}
	h := newHarness(t, fixture("argon2d.kdbx"))
	h.tmpl(t, "t", "x")
	if err := os.Chmod(filepath.Join(h.home, "templates", "t.tmpl"), 0o660); err != nil {
		t.Fatal(err)
	}
	if code := h.run("render", "t"); code != 1 || h.terminalOpens != 0 {
		t.Fatalf("exit %d opens %d", code, h.terminalOpens)
	}
}
