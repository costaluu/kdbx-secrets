package cli_test

import (
	"strings"
	"testing"
)

func TestInvalidReferenceSyntaxExitsTwoBeforePrompt(t *testing.T) {
	for _, bad := range []string{"", "a//b", "/a", "a/", "a#", `a\x`, "uuid:abc"} {
		h := newHarness(t, fixture("argon2d.kdbx"))
		if code := h.run("get", bad); code != 2 {
			t.Errorf("%q: exit %d", bad, code)
		}
		if h.terminalOpens != 0 || !strings.Contains(h.errw.String(), "position") {
			t.Errorf("%q: opens %d stderr %q", bad, h.terminalOpens, h.errw.String())
		}
	}
}

func TestGetStandardField(t *testing.T) {
	h := newHarness(t, fixture("argon2d.kdbx"))
	if code := h.run("get", "AWS/paper-freeze#UserName"); code != 0 || h.stdout.String() != "AKIAFIXTURE" {
		t.Fatalf("exit %d out %q err %q", code, h.stdout.String(), h.errw.String())
	}
}
