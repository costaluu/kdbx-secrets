//go:build linux

package cli

import (
	"io"
	"os"
	"testing"
)

// The slave must not turn "\n" into "\r\n", or multiline values escape the redactor.
func TestPtyKeepsNewlines(t *testing.T) {
	m, s, err := openPty(os.Stdin)
	if err != nil {
		t.Skip(err)
	}
	defer m.Close()
	s.WriteString("a\nb")
	s.Close()
	got, _ := io.ReadAll(m) // ends with EIO once the slave is closed
	if string(got) != "a\nb" {
		t.Fatalf("%q", got)
	}
}
