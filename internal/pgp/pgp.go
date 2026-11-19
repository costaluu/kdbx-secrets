// Package pgp does OpenPGP encryption with ProtonMail/go-crypto (ADR 0002).
package pgp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// ReadPublicKey parses an armored or binary public key and checks it can encrypt.
func ReadPublicKey(data []byte) (openpgp.EntityList, error) {
	var el openpgp.EntityList
	var err error
	if bytes.Contains(data, []byte("-----BEGIN PGP")) {
		el, err = openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	} else {
		el, err = openpgp.ReadKeyRing(bytes.NewReader(data))
	}
	if err != nil {
		// go-crypto cannot read v5/Kyber (LibrePGP post-quantum) subkeys.
		return nil, fmt.Errorf("unsupported or unreadable public key (post-quantum keys are not supported): %v", err)
	}
	if len(el) == 0 {
		return nil, errors.New("no public key found")
	}
	for _, e := range el {
		if _, ok := e.EncryptionKey(time.Now()); !ok {
			return nil, errors.New("public key has no usable encryption subkey")
		}
	}
	return el, nil
}

// Encrypt writes plain to w as an armored OpenPGP message for all recipients.
func Encrypt(w io.Writer, plain io.Reader, to openpgp.EntityList) error {
	var buf bytes.Buffer // nothing reaches w unless the whole message succeeds
	a, err := armor.Encode(&buf, "PGP MESSAGE", nil)
	if err != nil {
		return err
	}
	pt, err := openpgp.Encrypt(a, to, nil, nil, nil)
	if err != nil {
		return err
	}
	if _, err := io.Copy(pt, plain); err != nil {
		return err
	}
	if err := pt.Close(); err != nil {
		return err
	}
	if err := a.Close(); err != nil {
		return err
	}
	_, err = w.Write(buf.Bytes())
	return err
}

// MaxPlain is the largest message decrypt buffers (and so the largest it writes).
const MaxPlain = 64 << 20

// ReadPrivateKey parses an armored or binary private key.
func ReadPrivateKey(data []byte) (openpgp.EntityList, error) {
	el, err := readKeys(data)
	if err != nil {
		return nil, fmt.Errorf("unsupported or unreadable private key (post-quantum keys are not supported): %v", err)
	}
	for _, e := range el {
		if e.PrivateKey == nil {
			return nil, errors.New("not a private key")
		}
	}
	return el, nil
}

func readKeys(data []byte) (openpgp.EntityList, error) {
	if bytes.Contains(data, []byte("-----BEGIN PGP")) {
		return openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	}
	return openpgp.ReadKeyRing(bytes.NewReader(data))
}

// KeyEncrypted reports whether any key in el needs a passphrase.
func KeyEncrypted(el openpgp.EntityList) bool {
	for _, e := range el {
		if e.PrivateKey.Encrypted {
			return true
		}
		for _, s := range e.Subkeys {
			if s.PrivateKey != nil && s.PrivateKey.Encrypted {
				return true
			}
		}
	}
	return false
}

// Decrypt decrypts an armored or binary message with the private keys in el
// (unlocking encrypted ones with pass) and returns the plaintext only after
// its integrity was verified. Messages without MDC/AEAD and messages over
// MaxPlain are refused.
func Decrypt(msg []byte, el openpgp.EntityList, pass []byte) ([]byte, error) {
	for _, e := range el {
		if e.PrivateKey.Encrypted {
			if err := e.PrivateKey.Decrypt(pass); err != nil {
				return nil, errors.New("wrong passphrase for the private key")
			}
		}
		for _, s := range e.Subkeys {
			if s.PrivateKey != nil && s.PrivateKey.Encrypted {
				if err := s.PrivateKey.Decrypt(pass); err != nil {
					return nil, errors.New("wrong passphrase for the private key")
				}
			}
		}
	}
	bin := msg
	if bytes.Contains(msg, []byte("-----BEGIN PGP")) {
		blk, err := armor.Decode(bytes.NewReader(msg))
		if err != nil {
			return nil, fmt.Errorf("reading message: %w", err)
		}
		if bin, err = io.ReadAll(blk.Body); err != nil {
			return nil, fmt.Errorf("reading message: %w", err)
		}
	}
	// Stop at the encrypted data packet: its body is ciphertext, and reading on
	// would parse random bytes as packet headers (a stray tag 9 = false alarm).
	pr := packet.NewReader(bytes.NewReader(bin))
scan:
	for {
		p, err := pr.Next()
		if err != nil {
			break
		}
		switch p := p.(type) {
		case *packet.SymmetricallyEncrypted:
			if !p.IntegrityProtected {
				return nil, errors.New("message has no integrity protection (MDC/AEAD), refusing to decrypt")
			}
			break scan
		case *packet.AEADEncrypted:
			break scan
		}
	}
	md, err := openpgp.ReadMessage(bytes.NewReader(bin), el, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}
	plain, err := io.ReadAll(io.LimitReader(md.UnverifiedBody, MaxPlain+1))
	if err != nil {
		clear(plain)
		return nil, fmt.Errorf("decrypting (message corrupted or tampered): %w", err)
	}
	if len(plain) > MaxPlain {
		clear(plain)
		return nil, errors.New("message is larger than 64 MiB")
	}
	return plain, nil
}
