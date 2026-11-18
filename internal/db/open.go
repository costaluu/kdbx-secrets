package db

import (
	"errors"
	"fmt"
	"os"
	"strings"

	kp "github.com/tobischo/gokeepasslib/v3"
)

// ErrWrongPassword: the header HMAC did not match. A corrupt or truncated file is
// told apart (ErrCorrupt).
var ErrWrongPassword = errors.New("incorrect password")

// ErrCorrupt: the file is incomplete or damaged (e.g. git-sync was writing it).
var ErrCorrupt = errors.New("database file is incomplete or corrupted, try again")

// DB is an opened, decrypted Database. Strings inside it cannot be zeroed
// (gokeepasslib limitation, see CONTEXT.md).
type DB struct {
	Root       *kp.Group
	RecycleBin kp.UUID // zero when the Database has no Recycle bin
	// Attachment returns the bytes of the Attachment with the given binary id.
	Attachment func(id int) ([]byte, error)
}

// Open decrypts the file at path. Only call after CheckFile and the prompt.
func Open(path string, password []byte) (*DB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d := kp.NewDatabase()
	d.Credentials = kp.NewPasswordCredentials(string(password))
	if err := kp.NewDecoder(f).Decode(d); err != nil {
		if strings.Contains(err.Error(), "Wrong password") {
			return nil, ErrWrongPassword
		}
		return nil, fmt.Errorf("%w (%v)", ErrCorrupt, err)
	}
	if err := d.UnlockProtectedEntries(); err != nil {
		return nil, err
	}
	if len(d.Content.Root.Groups) == 0 {
		return nil, errors.New("database has no root group")
	}
	return &DB{
		Root:       &d.Content.Root.Groups[0],
		RecycleBin: d.Content.Meta.RecycleBinUUID,
		Attachment: func(id int) ([]byte, error) {
			b := d.FindBinary(id)
			if b == nil {
				return nil, fmt.Errorf("binary %d missing", id)
			}
			return b.GetContentBytes()
		},
	}, nil
}
