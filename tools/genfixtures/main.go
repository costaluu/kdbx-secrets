// Command genfixtures writes the committed .kdbx test fixtures with gokeepasslib.
//
// It is a STAND-IN for testdata/gen-fixtures.sh (keepassxc-cli), used because
// keepassxc-cli was not available when this was written. Files written here
// only show that our reader round-trips gokeepasslib's own writer, NOT that it
// opens KeePassXC-written files: pending item 1 stays open for that.
//
// Usage: go run ./tools/genfixtures testdata
package main

import (
	"fmt"
	"os"
	"path/filepath"

	kp "github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

const password = "fixture-pass" // known master password of every fixture

func main() {
	dir := "testdata"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	vs := map[string]*kp.Database{
		"argon2d.kdbx":    argon(kp.KdfArgon2),
		"argon2id.kdbx":   argon(kp.KdfArgon2id),
		"aes-kdbx4.kdbx":  aes4(),
		"aes-kdbx3.kdbx":  aes3(),
		"empty-pass.kdbx": argon(kp.KdfArgon2id),
		"multi.kdbx":      argon(kp.KdfArgon2),   // AWS/paper-freeze plus extraEntries
		"derived.kdbx":    argon(kp.KdfArgon2id), // attachment, TOTP seed, reserved-name collision
		"pgp.kdbx":        argon(kp.KdfArgon2id), // Keys/<name> with pub.asc and sec.asc from testdata/pgp
	}
	for name, db := range vs {
		if err := write(filepath.Join(dir, name), db); err != nil {
			fmt.Fprintln(os.Stderr, name+":", err)
			os.Exit(1)
		}
	}
}

// argon builds a KDBX4 database with deliberately cheap Argon2 parameters.
func argon(id []byte) *kp.Database {
	db := kp.NewDatabase(kp.WithDatabaseKDBXVersion4())
	p := db.Header.FileHeaders.KdfParameters
	p.UUID, p.Memory, p.Iterations, p.Parallelism = id, 1<<20, 2, 1
	return db
}

func aes4() *kp.Database {
	db := kp.NewDatabase(kp.WithDatabaseKDBXVersion4())
	p := db.Header.FileHeaders.KdfParameters
	p.UUID, p.Rounds = kp.KdfAES4, 1000
	p.Memory, p.Iterations, p.Parallelism, p.Version = 0, 0, 0, 0 // AES-KDF has no Argon2 parameters
	return db
}

func aes3() *kp.Database {
	db := kp.NewDatabase() // KDBX 3.1 only has AES-KDF
	db.Header.FileHeaders.TransformRounds = 1000
	return db
}

func val(k, v string, protected bool) kp.ValueData {
	return kp.ValueData{Key: k, Value: kp.V{Content: v, Protected: w.NewBoolWrapper(protected)}}
}

func entry(title, pass string) kp.Entry {
	e := kp.NewEntry()
	e.Values = append(e.Values, val("Title", title, false), val("Password", pass, true))
	return e
}

func write(path string, db *kp.Database) error {
	pw := password
	if filepath.Base(path) == "empty-pass.kdbx" {
		pw = "" // fixture for the empty-password case
	}
	db.Credentials = kp.NewPasswordCredentials(pw)
	e := kp.NewEntry()
	e.Values = append(e.Values,
		val("Title", "paper-freeze", false),
		val("UserName", "AKIAFIXTURE", false),
		val("Password", "s3cr3t-value", true))
	aws := kp.NewGroup()
	aws.Name = "AWS"
	aws.Entries = append(aws.Entries, e)
	if filepath.Base(path) == "multi.kdbx" {
		aws.Entries = append(aws.Entries, entry("multiline", "line1\nline2"), entry("evil\ttitle", "x"))
	}
	root := kp.NewGroup()
	root.Name = "Root"
	if filepath.Base(path) == "derived.kdbx" {
		addDerived(db, &aws, &root)
	}
	if filepath.Base(path) == "pgp.kdbx" {
		if err := addPGP(db, &root); err != nil {
			return err
		}
	}
	root.Groups = append(root.Groups, aws)
	db.Content.Root = &kp.RootData{Groups: []kp.Group{root}}
	if err := db.LockProtectedEntries(); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return kp.NewEncoder(f).Encode(db)
}

// attachmentBytes is deliberately not valid UTF-8 (tests assert exact bytes).
var attachmentBytes = []byte{0x00, 0xff, 0xfe, '\n', 'k', 'e', 'y', 0x80, 0x00}

// rfc6238 is the RFC 6238 test secret "12345678901234567890" in base32.
const rfc6238 = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

// addDerived adds GPG/pessoal (Attachment chave.asc) and, in AWS, entries with a
// TOTP seed (otp), without one, and with custom attributes named like Reserved
// names. Must run before aws is appended to root: it appends to aws.Entries.
func addDerived(db *kp.Database, aws, root *kp.Group) {
	b := db.AddBinary(attachmentBytes)
	pessoal := entry("pessoal", "gpg-pass")
	pessoal.Binaries = append(pessoal.Binaries, b.CreateReference("chave.asc"))
	gpg := kp.NewGroup()
	gpg.Name = "GPG"
	gpg.Entries = append(gpg.Entries, pessoal)
	root.Groups = append(root.Groups, gpg)

	withOTP := entry("with-totp", "pw")
	withOTP.Values = append(withOTP.Values, val("otp", "otpauth://totp/x:y?secret="+rfc6238+"&issuer=x", true))
	eightDigits := entry("totp-8", "pw")
	eightDigits.Values = append(eightDigits.Values, val("otp", "otpauth://totp/x?secret="+rfc6238+"&digits=8", true))
	collideT := entry("collide-totp", "pw")
	collideT.Values = append(collideT.Values, val("TOTP", "custom", false))
	collideA := entry("collide-attachment", "pw")
	collideA.Values = append(collideA.Values, val("attachment:chave.asc", "custom", false))
	aws.Entries = append(aws.Entries, withOTP, eightDigits, entry("no-totp", "pw"), collideT, collideA)
}

// addPGP adds Keys/<name> entries holding the testdata/pgp keys as Attachments
// pub.asc and sec.asc. The Password is the key passphrase, except for nopass
// (an unencrypted key) whose Password is deliberately wrong.
func addPGP(db *kp.Database, root *kp.Group) error {
	keys := kp.NewGroup()
	keys.Name = "Keys"
	for _, n := range []string{"ed25519", "rsa3072", "nopass", "pqc", "signonly"} {
		pass := "pgp-pass"
		if n == "nopass" {
			pass = "not-the-passphrase"
		}
		e := entry(n, pass)
		for _, kind := range []string{"pub", "sec"} {
			data, err := os.ReadFile(filepath.Join("testdata", "pgp", n+"."+kind+".asc"))
			if os.IsNotExist(err) {
				continue // pqc has no secret key
			}
			if err != nil {
				return err
			}
			e.Binaries = append(e.Binaries, db.AddBinary(data).CreateReference(kind+".asc"))
		}
		keys.Entries = append(keys.Entries, e)
	}
	root.Groups = append(root.Groups, keys)
	return nil
}
