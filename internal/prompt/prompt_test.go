package prompt

import (
	"errors"
	"strings"
	"testing"
)

func TestReadSecretBoundedAndCancel(t *testing.T) {
	buf := make([]byte, MaxPassword+1)
	if _, err := ReadSecret(strings.NewReader(strings.Repeat("a", MaxPassword+1)+"\n"), buf, false); !errors.Is(err, ErrTooLong) {
		t.Fatalf("too long: %v", err)
	}
	pw, err := ReadSecret(strings.NewReader(strings.Repeat("a", MaxPassword)+"\r\n"), buf, false)
	if err != nil || len(pw) != MaxPassword {
		t.Fatalf("at limit: %d %v", len(pw), err)
	}
	if _, err := ReadSecret(strings.NewReader("\x04"), buf, true); !errors.Is(err, ErrCancelled) {
		t.Fatalf("raw ctrl-d: %v", err)
	}
	pw, err = ReadSecret(strings.NewReader("ab\x7fc\r"), buf, true)
	if err != nil || string(pw) != "ac" {
		t.Fatalf("raw edit: %q %v", pw, err)
	}
}
