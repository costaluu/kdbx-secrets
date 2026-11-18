// Package db checks and opens the Database (.kdbx). It never writes.
package db

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// KDF describes the key derivation function declared in the file header.
// MemoryBytes is what the KDF will allocate (0 for AES-KDF); the hardening
// code uses it to decide whether mlockall is safe.
type KDF struct {
	Name        string // "Argon2d", "Argon2id" or "AES-KDF"
	MemoryBytes uint64
	Iterations  uint64
	Parallelism uint32
}

var signature = []byte{0x03, 0xd9, 0xa2, 0x9a, 0x67, 0xfb, 0x4b, 0xb5}

var (
	uuidArgon2d  = []byte{0xef, 0x63, 0x6d, 0xdf, 0x8c, 0x29, 0x44, 0x4b, 0x91, 0xf7, 0xa9, 0xa4, 0x03, 0xe3, 0x0a, 0x0c}
	uuidArgon2id = []byte{0x9e, 0x29, 0x8b, 0x19, 0x56, 0xdb, 0x47, 0x73, 0xb2, 0x3d, 0xfc, 0x3e, 0xc6, 0xf0, 0xa1, 0xe6}
	uuidAESKDBX3 = []byte{0xc9, 0xd9, 0xf3, 0x9a, 0x62, 0x8a, 0x44, 0x60, 0xbf, 0x74, 0x0d, 0x08, 0xc1, 0x8a, 0x4f, 0xea}
	uuidAESKDBX4 = []byte{0x7c, 0x02, 0xbb, 0x82, 0x79, 0xa7, 0x4a, 0xc0, 0x92, 0x7d, 0x11, 0x4a, 0x00, 0x64, 0x82, 0x38}
)

// ErrNotKDBX means the file does not start with the KDBX signature.
var ErrNotKDBX = errors.New("not a KDBX file (bad signature)")

// CheckFile runs before the prompt: the path must be a regular file starting
// with the KDBX signature, and the header must declare a KDF we understand.
func CheckFile(path string) (KDF, error) {
	st, err := os.Stat(path)
	if err != nil {
		return KDF{}, err
	}
	if !st.Mode().IsRegular() {
		return KDF{}, fmt.Errorf("%s: not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return KDF{}, err
	}
	defer f.Close()
	return ReadKDF(f)
}

// ReadKDF parses just enough of the header (KDBX 3.x and 4.x) to read the KDF
// parameters, without deriving any key.
func ReadKDF(r io.Reader) (KDF, error) {
	var fixed [12]byte // signature (8) + minor (2) + major (2)
	if _, err := io.ReadFull(r, fixed[:]); err != nil || !bytes.Equal(fixed[:8], signature) {
		return KDF{}, ErrNotKDBX
	}
	major := binary.LittleEndian.Uint16(fixed[10:])
	if major != 3 && major != 4 {
		return KDF{}, fmt.Errorf("unsupported KDBX major version %d", major)
	}
	kdf := KDF{Name: "AES-KDF"}
	for {
		id, data, err := readField(r, major)
		if err != nil {
			return KDF{}, fmt.Errorf("truncated KDBX header: %w", err)
		}
		switch {
		case id == 0: // end of header
			if major == 3 {
				return kdf, nil
			}
			return KDF{}, errors.New("KDBX header has no KDF parameters")
		case major == 3 && id == 6 && len(data) == 8:
			kdf.Iterations = binary.LittleEndian.Uint64(data)
		case major == 4 && id == 11:
			return parseKDFParams(data)
		}
	}
}

func readField(r io.Reader, major uint16) (id byte, data []byte, err error) {
	var idb [1]byte
	if _, err = io.ReadFull(r, idb[:]); err != nil {
		return
	}
	var n uint32
	if major == 3 {
		var b [2]byte
		if _, err = io.ReadFull(r, b[:]); err != nil {
			return
		}
		n = uint32(binary.LittleEndian.Uint16(b[:]))
	} else {
		var b [4]byte
		if _, err = io.ReadFull(r, b[:]); err != nil {
			return
		}
		n = binary.LittleEndian.Uint32(b[:])
	}
	if n > 1<<20 { // a header field is never this big; refuse before allocating
		return 0, nil, errors.New("header field too large")
	}
	data = make([]byte, n)
	_, err = io.ReadFull(r, data)
	return idb[0], data, err
}

// parseKDFParams reads the KDBX4 VariantDictionary of the KdfParameters field.
func parseKDFParams(d []byte) (KDF, error) {
	var kdf KDF
	var uuid []byte
	var rounds uint64
	bad := errors.New("malformed KDF parameters")
	if len(d) < 3 {
		return kdf, bad
	}
	d = d[2:] // dictionary version
	for len(d) > 0 && d[0] != 0 {
		if len(d) < 5 {
			return kdf, bad
		}
		kind, klen := d[0], int(binary.LittleEndian.Uint32(d[1:5]))
		d = d[5:]
		if klen < 0 || len(d) < klen+4 {
			return kdf, bad
		}
		key := string(d[:klen])
		vlen := int(binary.LittleEndian.Uint32(d[klen : klen+4]))
		d = d[klen+4:]
		if vlen < 0 || len(d) < vlen {
			return kdf, bad
		}
		val := d[:vlen]
		d = d[vlen:]
		switch {
		case key == "$UUID":
			uuid = val
		case key == "M" && kind == 0x05 && vlen == 8:
			kdf.MemoryBytes = binary.LittleEndian.Uint64(val)
		case key == "I" && kind == 0x05 && vlen == 8:
			kdf.Iterations = binary.LittleEndian.Uint64(val)
		case key == "R" && kind == 0x05 && vlen == 8:
			rounds = binary.LittleEndian.Uint64(val)
		case key == "P" && kind == 0x04 && vlen == 4:
			kdf.Parallelism = binary.LittleEndian.Uint32(val)
		}
	}
	switch {
	case bytes.Equal(uuid, uuidArgon2d):
		kdf.Name = "Argon2d"
	case bytes.Equal(uuid, uuidArgon2id):
		kdf.Name = "Argon2id"
	case bytes.Equal(uuid, uuidAESKDBX4), bytes.Equal(uuid, uuidAESKDBX3):
		kdf = KDF{Name: "AES-KDF", Iterations: rounds}
	default:
		return kdf, errors.New("unsupported KDF in KDBX header")
	}
	return kdf, nil
}
