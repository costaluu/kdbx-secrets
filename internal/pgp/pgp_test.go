package pgp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// The integrity scan must stop at the encrypted packet: reading on parses
// ciphertext as headers, and a random tag 9 refused ~2% of valid messages.
func TestDecryptNeverMistakesCiphertextForPackets(t *testing.T) {
	e, err := openpgp.NewEntity("t", "", "t@example.com", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	if err != nil {
		t.Fatal(err)
	}
	el := openpgp.EntityList{e}
	for i := range 500 {
		var msg bytes.Buffer
		if err := Encrypt(&msg, strings.NewReader("secret"), el); err != nil {
			t.Fatal(err)
		}
		if got, err := Decrypt(msg.Bytes(), el, nil); err != nil || string(got) != "secret" {
			t.Fatalf("round %d: %q %v", i, got, err)
		}
	}
}
