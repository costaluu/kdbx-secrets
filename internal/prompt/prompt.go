// Package prompt asks for the KeePass password on the controlling terminal.
package prompt

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxPassword is the longest password accepted, in bytes. Callers pass Password
// a buffer of at least this length.
const MaxPassword = 1024

// Sanitize replaces control characters with '?': the single filter for
// everything that goes to stderr or the tty (process names, Entries, References).
func Sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, s)
}

var (
	// ErrCancelled: Ctrl-D or EOF before a password was entered.
	ErrCancelled = errors.New("password prompt cancelled")
	// ErrTooLong: the password exceeds MaxPassword; reading stops at the limit.
	ErrTooLong = fmt.Errorf("password longer than %d bytes", MaxPassword)
	// ErrInterrupted: Ctrl-C in raw mode (the Console exits on it).
	ErrInterrupted = errors.New("interrupted")
)

// secretReader is implemented by the real terminal to read without echo.
type secretReader interface {
	ReadPassword(buf []byte) ([]byte, error)
}

// Password writes msg to term and reads one password into buf (len(buf) >=
// MaxPassword+1; bytes as typed, no normalization, trailing newline removed).
// The result aliases buf; the caller wipes it.
func Password(term io.ReadWriter, msg string, buf []byte) ([]byte, error) {
	fmt.Fprint(term, msg)
	defer fmt.Fprintln(term)
	if sr, ok := term.(secretReader); ok {
		return sr.ReadPassword(buf)
	}
	return ReadSecret(term, buf, false)
}

// ReadSecret reads one line into buf byte by byte, never past MaxPassword. In
// raw mode (terminal without line editing) it also handles '\r', backspace,
// Ctrl-C and Ctrl-D; otherwise EOF is the only way to cancel.
func ReadSecret(r io.Reader, buf []byte, raw bool) ([]byte, error) {
	n := 0
	var b [1]byte
	for {
		k, err := r.Read(b[:])
		if k == 1 {
			switch c := b[0]; {
			case c == '\n' || (raw && c == '\r'):
				if !raw && n > 0 && buf[n-1] == '\r' {
					n--
				}
				return buf[:n], nil
			case raw && c == 3:
				clear(buf[:n])
				return nil, ErrInterrupted
			case raw && c == 4:
				if n == 0 {
					return nil, ErrCancelled
				}
			case raw && (c == 127 || c == 8):
				for n > 0 { // drop one whole rune
					n--
					if utf8.RuneStart(buf[n]) {
						break
					}
				}
			default:
				if n >= MaxPassword && !(n == MaxPassword && !raw && c == '\r') { // room for the CR of a CRLF
					clear(buf[:n])
					return nil, ErrTooLong
				}
				buf[n] = c
				n++
			}
		}
		if err != nil {
			clear(buf[:n])
			if err == io.EOF {
				return nil, ErrCancelled
			}
			return nil, err
		}
	}
}
