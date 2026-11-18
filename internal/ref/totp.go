package ref

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TOTP computes the code at now from a TOTP seed (otpauth:// URI, RFC 6238).
// Honors secret, period, digits and algorithm (SHA1/SHA256/SHA512). Errors
// never include the seed.
func TOTP(seed string, now time.Time) (string, error) {
	u, err := url.Parse(seed)
	if err != nil || u.Scheme != "otpauth" || u.Host != "totp" {
		return "", errors.New("TOTP seed is not an otpauth://totp URI")
	}
	q := u.Query()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.TrimRight(strings.ToUpper(strings.ReplaceAll(q.Get("secret"), " ", "")), "="))
	if err != nil || len(key) == 0 {
		return "", errors.New("TOTP seed has no valid base32 secret")
	}
	period, digits := int64(30), 6
	if p := q.Get("period"); p != "" {
		if period, err = strconv.ParseInt(p, 10, 64); err != nil || period <= 0 {
			return "", errors.New("TOTP seed has an invalid period")
		}
	}
	if d := q.Get("digits"); d != "" {
		if digits, err = strconv.Atoi(d); err != nil || digits < 1 || digits > 9 {
			return "", errors.New("TOTP seed has invalid digits")
		}
	}
	var h func() hash.Hash
	switch strings.ToUpper(q.Get("algorithm")) {
	case "", "SHA1":
		h = sha1.New
	case "SHA256":
		h = sha256.New
	case "SHA512":
		h = sha512.New
	default:
		return "", errors.New("TOTP seed has an unsupported algorithm")
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(now.Unix()/period))
	m := hmac.New(h, key)
	m.Write(msg[:])
	sum := m.Sum(nil)
	o := sum[len(sum)-1] & 0xf
	n := binary.BigEndian.Uint32(sum[o:o+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, n%mod), nil
}
