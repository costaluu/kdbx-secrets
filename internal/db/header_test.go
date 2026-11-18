package db_test

import (
	"path/filepath"
	"testing"

	"github.com/racoonlemore/kdbx-secrets/internal/db"
)

func TestCheckFileReadsKDFFromHeader(t *testing.T) {
	for file, want := range map[string]db.KDF{
		"argon2d.kdbx":   {Name: "Argon2d", MemoryBytes: 1 << 20, Iterations: 2, Parallelism: 1},
		"argon2id.kdbx":  {Name: "Argon2id", MemoryBytes: 1 << 20, Iterations: 2, Parallelism: 1},
		"aes-kdbx4.kdbx": {Name: "AES-KDF", Iterations: 1000},
		"aes-kdbx3.kdbx": {Name: "AES-KDF", Iterations: 1000},
	} {
		got, err := db.CheckFile(filepath.Join("..", "..", "testdata", file))
		if err != nil || got != want {
			t.Errorf("%s: got %+v, %v; want %+v", file, got, err, want)
		}
	}
}
