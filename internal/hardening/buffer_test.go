package hardening

import (
	"bytes"
	"errors"
	"testing"
)

func TestLockBufferFailureIsWarning(t *testing.T) {
	var w bytes.Buffer
	fail := func([]byte) error { return errors.New("boom") }
	unlock := lockBuffer([]byte("x"), &w, fail, fail)
	unlock() // must not call the failing unlock or panic
	if !bytes.Contains(w.Bytes(), []byte("boom")) {
		t.Errorf("no warning: %q", w.String())
	}
}

func TestLockBufferReal(t *testing.T) {
	var w bytes.Buffer
	LockBuffer(make([]byte, 4096), &w)()
	LockBuffer(nil, &w)()
}
