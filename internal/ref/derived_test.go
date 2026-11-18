package ref

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/racoonlemore/kdbx-secrets/internal/db"
)

func openDerived(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open("../../testdata/derived.kdbx", []byte("fixture-pass"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAttachment(t *testing.T) {
	d := openDerived(t)
	res, err := resolve(t, d, "GPG/pessoal#attachment:chave.asc")
	if err != nil || !bytes.Equal([]byte(res.Value), []byte{0x00, 0xff, 0xfe, '\n', 'k', 'e', 'y', 0x80, 0x00}) {
		t.Fatalf("%q %v", res.Value, err)
	}
	for _, in := range []string{"GPG/pessoal#attachment:nope", "AWS/no-totp#attachment:x"} {
		if _, err := resolve(t, d, in); err == nil || !strings.Contains(err.Error(), "attachment") {
			t.Errorf("%q: %v", in, err)
		}
	}
	if _, err := Parse("GPG/pessoal#attachment:"); err == nil || !strings.Contains(err.Error(), "position") {
		t.Errorf("attachment: without name must be a syntax error: %v", err)
	}
}

func TestTOTP(t *testing.T) {
	d := openDerived(t)
	// RFC 6238 vectors (SHA1): T=59 -> 94287082, T=1111111109 -> 07081804;
	// 6-digit codes are the last 6 digits.
	for _, c := range []struct {
		in   string
		sec  int64
		want string
	}{
		{"AWS/with-totp#TOTP", 59, "287082"},
		{"AWS/totp-8#TOTP", 59, "94287082"},
		{"AWS/totp-8#TOTP", 1111111109, "07081804"},
		{"AWS/with-totp#TOTP", 1111111109, "081804"},
	} {
		r, _ := Parse(c.in)
		res, err := Resolve(d, r, time.Unix(c.sec, 0))
		if err != nil || res.Value != c.want {
			t.Errorf("%v@%d: %q %v, want %q", c.in, c.sec, res.Value, err, c.want)
		}
	}
	if _, err := resolve(t, d, "AWS/no-totp#TOTP"); err == nil || !strings.Contains(err.Error(), "no TOTP seed") {
		t.Errorf("want clear no-seed error: %v", err)
	}
}

func TestSeed(t *testing.T) {
	d := openDerived(t)
	res, err := resolve(t, d, "AWS/with-totp#otp")
	if err != nil || res.Value != "otpauth://totp/x:y?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&issuer=x" {
		t.Fatalf("%q %v", res.Value, err)
	}
}

func TestReservedCollision(t *testing.T) {
	d := openDerived(t)
	for _, in := range []string{"AWS/collide-totp#TOTP", "AWS/collide-attachment#attachment:chave.asc"} {
		if _, err := resolve(t, d, in); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Errorf("%q: %v", in, err)
		}
	}
}

func TestSnapshotRejectsReservedAttributes(t *testing.T) {
	d := openDerived(t)
	for _, in := range []string{"AWS/collide-totp", "AWS/collide-attachment"} {
		r, _ := Parse(in)
		if _, _, err := Snapshot(d, r); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Errorf("%q: %v", in, err)
		}
	}
}
