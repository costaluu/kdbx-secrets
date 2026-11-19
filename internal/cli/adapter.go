package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/racoonlemore/kdbx-secrets/internal/config"
	"github.com/racoonlemore/kdbx-secrets/internal/prompt"
	"github.com/racoonlemore/kdbx-secrets/internal/ref"
)

func init() { register("adapter", cmdAdapter) }

const adapterUsage = "usage: adapter create <name> [--from <file>] [--force] | delete <name> | list | fill <name>"

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
	case "fill":
		if len(args) != 2 || !validName(args[1]) {
			return usagef(adapterUsage)
		}
		return adapterFill(env, args[1])
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

// fillFlags maps the fields keepassxc-cli add/edit can set (ref.Writable) to
// their option; Password goes through stdin (-p), never through argv.
var fillFlags = map[string]string{"UserName": "--username=", "URL": "--url=", "Notes": "--notes="}

// adapterFill asks for every value the template uses that is missing in the
// Database and has keepassxc-cli write them: KeePassXC's own writer, so this
// binary never writes a .kdbx (ADR 0006). Existing values are never changed.
func adapterFill(env *Env, name string) error {
	_, refs, seen, err := loadTmpl(env, name)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		return fmt.Errorf("template %s uses no reference", name)
	}
	kx, err := keepassxcCLI(env)
	if err != nil {
		return err
	}
	pw := make([]byte, 0, prompt.MaxPassword+2) // password + "\n", as keepassxc-cli reads it
	defer env.hold(pw[:cap(pw)])()
	d, term, err := env.unlockTerm("adapter fill "+name+" ("+strings.Join(refs, ", ")+")",
		func(p []byte) { pw = append(append(pw[:0], p...), '\n') })
	if err != nil {
		return err
	}
	defer term.Close()

	type job struct {
		path   string
		gap    ref.Gap
		fields []string
		vals   map[string]string
		refs   []string
	}
	var jobs []*job
	byPath := map[string]*job{}
	for _, s := range refs { // plan everything before asking anything
		r := seen[s]
		gap, err := ref.Plan(d, r, env.Now())
		if err != nil {
			return err
		}
		if !gap.Missing && !gap.NewEntry {
			continue
		}
		p := strings.Join(r.Path, "/")
		j := byPath[p]
		if j == nil {
			j = &job{path: p, gap: gap, vals: map[string]string{}}
			byPath[p] = j
			jobs = append(jobs, j)
		}
		j.refs = append(j.refs, s)
		if gap.Missing && !slices.Contains(j.fields, r.Field) {
			j.fields = append(j.fields, r.Field)
		}
	}
	if len(jobs) == 0 {
		fmt.Fprintf(env.Stdout, "nothing to fill: every reference of %s is set\n", name)
		return nil
	}

	buf := make([]byte, prompt.MaxPassword+1)
	defer env.hold(buf)()
	for _, j := range jobs {
		for _, f := range j.fields {
			msg := fmt.Sprintf("Value for %s#%s: ", prompt.Sanitize(j.path), f)
			var v []byte
			if f == "Password" {
				v, err = prompt.Password(term, msg, buf)
			} else {
				fmt.Fprint(term, msg)
				v, err = prompt.ReadSecret(term, buf, false)
			}
			if err == nil && len(v) == 0 {
				err = fmt.Errorf("reference %q: empty value", j.path+"#"+f)
			}
			if err != nil {
				return err
			}
			j.vals[f] = string(v)
			clear(buf)
		}
	}

	cfg, err := config.Load(env.Getenv, env.HomeDir)
	if err != nil {
		return err
	}
	made := map[string]bool{}
	for _, j := range jobs {
		for _, g := range j.gap.NewGroups {
			if !made[g] {
				if err := kx.run(pw, "mkdir", "-q", cfg.Database, g); err != nil {
					return err
				}
				made[g] = true
			}
		}
		cmd, in := "edit", pw
		if j.gap.NewEntry {
			cmd = "add"
		}
		args := []string{cmd, "-q"}
		for _, f := range j.fields {
			if f == "Password" {
				args = append(args, "-p")
				in = append(append(append([]byte{}, pw...), j.vals[f]...), '\n')
			} else {
				args = append(args, fillFlags[f]+j.vals[f])
			}
		}
		err := kx.run(in, append(args, cfg.Database, j.path)...)
		if len(in) > len(pw) {
			clear(in)
		}
		if err != nil {
			return err
		}
		for _, s := range j.refs {
			fmt.Fprintln(env.Stdout, "filled", s)
		}
	}
	return nil
}

// keepassxcCLI is the command that runs keepassxc-cli: KEEPASSXC_CLI split on
// spaces (e.g. "flatpak run --command=keepassxc-cli org.keepassxc.KeePassXC")
// or keepassxc-cli from PATH.
type keepassxcCmd []string

func keepassxcCLI(env *Env) (keepassxcCmd, error) {
	kx := strings.Fields(env.Getenv("KEEPASSXC_CLI"))
	if len(kx) == 0 {
		kx = []string{"keepassxc-cli"}
	}
	if _, err := exec.LookPath(kx[0]); err != nil {
		return nil, fmt.Errorf("adapter fill writes through keepassxc-cli, which was not found (%v); install KeePassXC or set KEEPASSXC_CLI, e.g. KEEPASSXC_CLI='flatpak run --command=keepassxc-cli org.keepassxc.KeePassXC'", err)
	}
	return kx, nil
}

// run feeds stdin (the Database password line first, as keepassxc-cli prompts
// for it) and reports keepassxc-cli's own message on failure.
func (kx keepassxcCmd) run(stdin []byte, args ...string) error {
	c := exec.Command(kx[0], append(kx[1:], args...)...)
	c.Stdin = bytes.NewReader(stdin)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keepassxc-cli %s failed (%v): %s", args[0], err, prompt.Sanitize(strings.TrimSpace(string(out))))
	}
	return nil
}
