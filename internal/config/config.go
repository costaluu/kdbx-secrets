// Package config reads ~/.kdbx-secrets/config (the `database` key only) and
// checks ownership/permissions of the configuration folder.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the parsed configuration. Database is absolute.
type Config struct {
	Dir      string
	Database string
}

// Dir is the single place that resolves the configuration folder:
// KDBX_SECRETS_HOME (which must be absolute) or <home>/.kdbx-secrets.
func Dir(getenv func(string) string, home string) (string, error) {
	dir := getenv("KDBX_SECRETS_HOME")
	if dir == "" {
		return filepath.Join(home, ".kdbx-secrets"), nil
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("KDBX_SECRETS_HOME must be an absolute path, got %q", dir)
	}
	return dir, nil
}

// Load reads <dir>/config, with dir from Dir.
func Load(getenv func(string) string, home string) (Config, error) {
	dir, err := Dir(getenv, home)
	if err != nil {
		return Config{}, err
	}
	path := filepath.Join(dir, "config")
	if _, err := os.Stat(path); err == nil {
		if err := CheckPrivate(dir); err != nil {
			return Config{}, err
		}
		if err := CheckPrivate(path); err != nil {
			return Config{}, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("config not found: create %s with the line: database = /path/to/database.kdbx", path)
		}
		return Config{}, err
	}
	c := Config{Dir: dir}
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("%s:%d: expected 'key = value'", path, n+1)
		}
		switch k = strings.TrimSpace(k); k {
		case "database":
			c.Database = expand(strings.TrimSpace(v), dir, home)
		default:
			return Config{}, fmt.Errorf("%s:%d: unknown key %q", path, n+1, k)
		}
	}
	if c.Database == "" {
		return Config{}, fmt.Errorf("%s: missing line: database = /path/to/database.kdbx", path)
	}
	return c, nil
}

func expand(p, dir, home string) string {
	switch {
	case p == "~" || strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[1:])
	case !filepath.IsAbs(p):
		return filepath.Join(dir, p)
	}
	return p
}
