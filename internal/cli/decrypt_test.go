package cli_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/racoonlemore/kdbx-secrets/internal/cli"
)

func keyRef(name, kind string) string { return "Keys/" + name + "#attachment:" + kind + ".asc" }

func runDecrypt(t *testing.T, stdin string, args ...string) (*harness, int) {
	t.Helper()
	h := newHarness(t, fixture("pgp.kdbx"))
	h.stdin = stdin
	return h, h.run(append([]string{"decrypt"}, args...)...)
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(pgpFixture(name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDecryptGPGMessages(t *testing.T) {
	// msg-ed25519/ocb/nopass are AEAD/OCB (gpg 2.5 default); rsa3072 covers RSA.
	for _, c := range [][2]string{{"ed25519", "msg-ed25519.asc"}, {"ed25519", "msg-ocb.asc"}, {"rsa3072", "msg-rsa3072.asc"}, {"nopass", "msg-nopass.asc"}} {
		h, code := runDecrypt(t, readFixture(t, c[1]), "--key", keyRef(c[0], "sec"), "--show")
		if code != 0 || h.stdout.String() != "hello gpg\n" {
			t.Errorf("%s: exit %d out %q err %q", c[1], code, h.stdout.String(), h.errw.String())
		}
	}
}

func TestDecryptUnencryptedKeyIgnoresPassword(t *testing.T) {
	// the nopass entry's Password is wrong on purpose; it must not be used.
	h, code := runDecrypt(t, readFixture(t, "msg-nopass.asc"), "--key", keyRef("nopass", "sec"), "--show")
	if code != 0 || h.stdout.String() != "hello gpg\n" {
		t.Fatalf("exit %d err %q", code, h.errw.String())
	}
}

func TestEncryptToDecryptKeyRoundTripWithR(t *testing.T) {
	h := newHarness(t, fixture("pgp.kdbx"))
	h.stdin = "round trip"
	if code := h.run("encrypt", "--to", keyRef("ed25519", "pub"), "-r", pgpFixture("rsa3072.pub.asc")); code != 0 {
		t.Fatalf("encrypt: %d %s", code, h.errw.String())
	}
	if h.terminalOpens != 1 {
		t.Fatalf("prompt opens: %d", h.terminalOpens)
	}
	msg := h.stdout.String()
	for _, k := range []string{"ed25519", "rsa3072"} {
		h, code := runDecrypt(t, msg, "--key", keyRef(k, "sec"), "--show")
		if code != 0 || h.stdout.String() != "round trip" {
			t.Fatalf("%s: %d %q %s", k, code, h.stdout.String(), h.errw.String())
		}
	}
}

func TestDecryptRefusesTTYWithoutShow(t *testing.T) {
	h := newHarness(t, fixture("pgp.kdbx"))
	h.stdin = readFixture(t, "msg-ed25519.asc")
	var out bytes.Buffer
	code := cli.Run(&cli.Env{
		Args: []string{"decrypt", "--key", keyRef("ed25519", "sec")}, Stdin: strings.NewReader(h.stdin),
		Stdout: &out, Stderr: &h.errw, StdoutIsTTY: true,
		OpenTerminal: func() (cli.Terminal, error) { t.Fatal("terminal touched"); return nil, nil },
	})
	if code != 1 || out.Len() != 0 || !strings.Contains(h.errw.String(), "--show") {
		t.Fatalf("%d %q %q", code, out.String(), h.errw.String())
	}
}

func TestDecryptUsage(t *testing.T) {
	if _, code := runDecrypt(t, ""); code != cli.ExitUsage {
		t.Fatalf("no --key: %d", code)
	}
}

// sedNoMDC builds a message with a legacy (tag 9) encrypted data packet,
// which has no integrity protection, for the ed25519 fixture key.
func sedNoMDC(t *testing.T) string {
	t.Helper()
	el, err := openpgp.ReadArmoredKeyRing(strings.NewReader(readFixture(t, "ed25519.pub.asc")))
	if err != nil {
		t.Fatal(err)
	}
	ek, _ := el[0].EncryptionKey(time.Now())
	sessKey := make([]byte, 32)
	rand.Read(sessKey)
	var bin bytes.Buffer
	if err := packet.SerializeEncryptedKey(&bin, ek.PublicKey, packet.CipherAES256, sessKey, nil); err != nil {
		t.Fatal(err)
	}
	// literal data packet
	body := []byte("no integrity")
	var lit bytes.Buffer
	lit.Write([]byte{0xc0 | 11, byte(6 + len(body)), 'b', 0, 0, 0, 0, 0})
	lit.Write(body)
	// OpenPGP CFB with resync, AES-256
	blk, _ := aes.NewCipher(sessKey)
	bs := blk.BlockSize()
	prefix := make([]byte, bs+2)
	rand.Read(prefix[:bs])
	prefix[bs], prefix[bs+1] = prefix[bs-2], prefix[bs-1]
	ct := make([]byte, bs+2+lit.Len())
	cipher.NewCFBEncrypter(blk, make([]byte, bs)).XORKeyStream(ct[:bs+2], prefix)
	cipher.NewCFBEncrypter(blk, ct[2:bs+2]).XORKeyStream(ct[bs+2:], lit.Bytes())
	if len(ct) >= 192 {
		t.Fatal("one-octet length only")
	}
	bin.Write([]byte{0xc0 | 9, byte(len(ct))})
	bin.Write(ct)
	var out bytes.Buffer
	a, _ := armor.Encode(&out, "PGP MESSAGE", nil)
	a.Write(bin.Bytes())
	a.Close()
	return out.String()
}

func TestDecryptRefusesMessageWithoutIntegrity(t *testing.T) {
	h, code := runDecrypt(t, sedNoMDC(t), "--key", keyRef("ed25519", "sec"), "--show")
	if code != 1 || h.stdout.Len() != 0 || !strings.Contains(h.errw.String(), "integrity") {
		t.Fatalf("%d %q %q", code, h.stdout.String(), h.errw.String())
	}
}

func TestDecryptTamperedWritesNothing(t *testing.T) {
	h := newHarness(t, fixture("pgp.kdbx"))
	h.stdin = "tamper me, a message long enough to flip a byte in"
	if h.run("encrypt", "--to", keyRef("ed25519", "pub")) != 0 {
		t.Fatal(h.errw.String())
	}
	blk, err := armor.Decode(strings.NewReader(h.stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	var bin bytes.Buffer
	bin.ReadFrom(blk.Body)
	b := bin.Bytes()
	b[len(b)-3] ^= 0x01
	var out bytes.Buffer
	a, _ := armor.Encode(&out, "PGP MESSAGE", nil)
	a.Write(b)
	a.Close()
	h2, code := runDecrypt(t, out.String(), "--key", keyRef("ed25519", "sec"), "--show")
	if code != 1 || h2.stdout.Len() != 0 {
		t.Fatalf("%d %q %q", code, h2.stdout.String(), h2.errw.String())
	}
}

func TestDecryptOver64MiB(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	h := newHarness(t, fixture("pgp.kdbx"))
	h.stdin = strings.Repeat("x", 64<<20+1)
	if h.run("encrypt", "--to", keyRef("ed25519", "pub")) != 0 {
		t.Fatal(h.errw.String())
	}
	h2, code := runDecrypt(t, h.stdout.String(), "--key", keyRef("ed25519", "sec"), "--show")
	if code != 1 || h2.stdout.Len() != 0 || !strings.Contains(h2.errw.String(), "64 MiB") {
		t.Fatalf("%d %d %q", code, h2.stdout.Len(), h2.errw.String())
	}
}

func TestDecryptPostQuantumKeyClearError(t *testing.T) {
	h, code := runDecrypt(t, readFixture(t, "msg-ed25519.asc"), "--key", keyRef("pqc", "pub"), "--show")
	if code != 1 || !strings.Contains(h.errw.String(), "post-quantum") {
		t.Fatalf("%d %q", code, h.errw.String())
	}
}
