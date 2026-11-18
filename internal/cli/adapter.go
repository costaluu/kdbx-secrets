package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/racoonlemore/kdbx-secrets/internal/config"
)

func init() { register("adapter", cmdAdapter) }

const adapterUsage = "usage: adapter create <name> [--from <file>] [--force] | delete <name> | list"

func templatesDir(env *Env) (string, error) {
	dir, err := config.Dir(env.Getenv, env.HomeDir)
	return filepath.Join(dir, "templates"), err
}

func validName(n string) bool { return tmplName.MatchString(n) && !strings.HasPrefix(n, ".") }

func cmdAdapter(env *Env, args []string) error {
	if len(args) == 0 {
		return usagef(adapterUsage)
	}
	dir, err := templatesDir(env)
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return usagef(adapterUsage)
		}
		ents, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		var names []string
		for _, e := range ents {
			if n, ok := strings.CutSuffix(e.Name(), ".tmpl"); ok && validName(n) && e.Type().IsRegular() {
				names = append(names, n)
			}
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintln(env.Stdout, n)
		}
		return nil
	case "delete":
		if len(args) != 2 || !validName(args[1]) {
			return usagef(adapterUsage)
		}
		err = os.Remove(filepath.Join(dir, args[1]+".tmpl"))
		if os.IsNotExist(err) {
			return fmt.Errorf("template %s not found", args[1])
		}
		return err
	case "create":
		return adapterCreate(env, dir, args[1:])
	}
	return usagef(adapterUsage)
}

func adapterCreate(env *Env, dir string, args []string) error {
	var name, from string
	force := false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--force":
			force = true
		case args[i] == "--from" && i+1 < len(args):
			i++
			from = args[i]
		case name == "" && !strings.HasPrefix(args[i], "--"):
			name = args[i]
		default:
			return usagef(adapterUsage)
		}
	}
	if !validName(name) {
		return usagef("invalid template name %q", name)
	}
	var r io.Reader = env.Stdin
	if from != "" {
		f, err := os.Open(from)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}
	src, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if _, _, _, err := collectTmpl(name, string(src)); err != nil {
		return err
	}

	path := filepath.Join(dir, name+".tmpl")
	if _, err := os.Lstat(dir); err == nil {
		if err := config.CheckPrivate(dir); err != nil {
			return err
		}
	}
	// ponytail: check-then-rename is racy against a concurrent create; fine for a single-user CLI.
	if _, err := os.Lstat(path); err == nil && !force {
		return fmt.Errorf("template %s already exists; use --force to overwrite", name)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*") // 0600 regardless of umask
	if err != nil {
		return err
	}
	_, werr := tmp.Write(src)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
