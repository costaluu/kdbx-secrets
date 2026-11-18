package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/racoonlemore/kdbx-secrets/internal/config"
	"github.com/racoonlemore/kdbx-secrets/internal/ref"
)

func init() { register("render", cmdRender) }

var (
	tmplName  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	tmplLine  = regexp.MustCompile(`^template: [^:]*:(\d+)`)
	errNewRef = errors.New("reference not listed in the collection pass")
)

// rewriteTmplErr hides the original text (it may echo values): only line and kind stay.
func rewriteTmplErr(name, kind string, err error) error {
	line := "?"
	if m := tmplLine.FindStringSubmatch(err.Error()); m != nil {
		line = m[1]
	}
	if errors.Is(err, errNewRef) {
		kind = "reference not in the collection pass"
	}
	return fmt.Errorf("template %s: line %s: %s", name, line, kind)
}

// execTmpl runs t with a get implemented by fn. A failing get is reported by its
// own message (it cites the Reference, never the value).
func execTmpl(t *template.Template, w io.Writer, get func(string) (string, error)) error {
	return template.Must(t.Clone()).Funcs(template.FuncMap{"get": get}).Execute(w, nil)
}

func cmdRender(env *Env, args []string) error {
	show, args := takeShow(args)
	if len(args) != 1 {
		return usagef("usage: render [--show] <name>")
	}
	name := args[0]
	if !validName(name) {
		return usagef("invalid template name %q", name)
	}
	if err := env.guardSecretOut(show); err != nil {
		return err
	}
	dir, err := config.Dir(env.Getenv, env.HomeDir)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "templates", name+".tmpl")
	if _, err := os.Stat(path); err == nil {
		if err := config.CheckPrivate(path); err != nil {
			return err
		}
	}
	src, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template %s not found", name)
		}
		return err
	}
	t, refs, seen, err := collectTmpl(name, string(src))
	if err != nil {
		return err
	}

	// Single prompt, only if there is something to resolve.
	var get func(string) (string, error) = func(string) (string, error) { return "", errNewRef }
	if len(refs) > 0 {
		d, err := env.unlock("render " + name + " (" + strings.Join(refs, ", ") + ")")
		if err != nil {
			return err
		}
		get = func(s string) (string, error) {
			r, ok := seen[s]
			if !ok {
				return "", errNewRef
			}
			res, err := ref.Resolve(d, r, env.Now())
			for _, w := range res.Warnings {
				env.warn(w)
			}
			return res.Value, err
		}
	}

	// Pass 2: real run, buffered so a failure writes nothing.
	var out bytes.Buffer
	if err := execTmpl(t, &out, get); err != nil {
		return rewriteTmplErr(name, "execution error", err)
	}
	defer env.hold(out.Bytes())()
	_, err = env.Stdout.Write(out.Bytes())
	return err
}

// collectTmpl parses src and runs the collection pass (no prompt, no Database):
// it returns the template, the distinct References in order and their parsed
// form. Errors are already safe to show (no values).
func collectTmpl(name, src string) (*template.Template, []string, map[string]ref.Ref, error) {
	funcs := template.FuncMap{
		"get": func(string) (string, error) { return "", nil },
		"json": func(s string) (string, error) {
			var b bytes.Buffer
			e := json.NewEncoder(&b)
			e.SetEscapeHTML(false)
			if err := e.Encode(s); err != nil {
				return "", err
			}
			return strings.TrimSuffix(b.String(), "\n"), nil
		},
		"base64": func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) },
	}
	t, err := template.New(name).Funcs(funcs).Parse(src)
	if err != nil {
		return nil, nil, nil, rewriteTmplErr(name, "syntax error", err)
	}

	// Pass 1: collect References, no prompt.
	var refs []string
	seen := map[string]ref.Ref{}
	var badRef error
	err = execTmpl(t, io.Discard, func(s string) (string, error) {
		if _, ok := seen[s]; ok {
			return "", nil
		}
		r, err := ref.Parse(s)
		if err != nil {
			badRef = err
			return "", err
		}
		seen[s] = r
		refs = append(refs, s)
		return "", nil
	})
	if badRef != nil {
		return nil, nil, nil, badRef // cites the Reference, never a value
	}
	if err != nil {
		return nil, nil, nil, rewriteTmplErr(name, "execution error", err)
	}

	return t, refs, seen, nil
}
