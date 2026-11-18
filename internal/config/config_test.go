package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setup(t *testing.T, body string) (dir string, env func(string) string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "home")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		if err := os.WriteFile(filepath.Join(dir, "config"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, func(k string) string {
		if k == "KDBX_SECRETS_HOME" {
			return dir
		}
		return ""
	}
}

func TestLoadPaths(t *testing.T) {
	dir, env := setup(t, "# c\n\ndatabase = ~/a.kdbx\n")
	c, err := Load(env, "/h")
	if err != nil || c.Database != filepath.Join("/h", "a.kdbx") {
		t.Fatalf("%v %q", err, c.Database)
	}
	os.WriteFile(filepath.Join(dir, "config"), []byte("database = rel/b.kdbx\n"), 0o600)
	c, _ = Load(env, "/h")
	if c.Database != filepath.Join(dir, "rel", "b.kdbx") {
		t.Fatal(c.Database)
	}
}

func TestLoadErrors(t *testing.T) {
	_, env := setup(t, "")
	if _, err := Load(env, "/h"); err == nil || !strings.Contains(err.Error(), "config not found") {
		t.Fatal(err)
	}
	_, env = setup(t, "# only\n")
	if _, err := Load(env, "/h"); err == nil || !strings.Contains(err.Error(), "missing line: database") {
		t.Fatal(err)
	}
	_, env = setup(t, "database = /x\nfoo = 1\n")
	if _, err := Load(env, "/h"); err == nil || !strings.Contains(err.Error(), `"foo"`) {
		t.Fatal(err)
	}
	if _, err := Load(func(string) string { return "rel/dir" }, "/h"); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatal(err)
	}
}

func TestLoadPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permissions")
	}
	dir, env := setup(t, "database = /x\n")
	os.Chmod(dir, 0o770)
	if _, err := Load(env, "/h"); err == nil || !strings.Contains(err.Error(), dir) {
		t.Fatal("group-writable dir accepted", err)
	}
	os.Chmod(dir, 0o700)
	os.Chmod(filepath.Join(dir, "config"), 0o602)
	if _, err := Load(env, "/h"); err == nil || !strings.Contains(err.Error(), "config") {
		t.Fatal("other-writable config accepted", err)
	}
}
