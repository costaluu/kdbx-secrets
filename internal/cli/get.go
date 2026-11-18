package cli

import (
	"fmt"
	"strings"

	"github.com/racoonlemore/kdbx-secrets/internal/ref"
)

func init() { register("get", cmdGet) }

// cmdGet resolves one or more References with a single prompt and writes one
// value per line, in the requested order.
func cmdGet(env *Env, args []string) error {
	show, args := takeShow(args)
	var refs []ref.Ref
	var raw []string
	for _, a := range args {
		r, err := ref.Parse(a)
		if err != nil {
			return usagef("%v", err)
		}
		refs, raw = append(refs, r), append(raw, a)
	}
	if len(refs) == 0 {
		return usagef("usage: get [--show] <reference>...")
	}
	if err := env.guardSecretOut(show); err != nil {
		return err
	}
	d, err := env.unlock(strings.Join(raw, " "))
	if err != nil {
		return err
	}
	var out []byte
	defer func() { clear(out) }() // best effort; the gokeepasslib strings cannot be wiped
	for i, r := range refs {
		res, err := ref.Resolve(d, r, env.Now())
		if err != nil {
			return err
		}
		for _, w := range res.Warnings {
			env.warn(w)
		}
		v := res.Value
		if len(refs) > 1 && strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("reference %q: value has a line break, cannot print one value per line", raw[i])
		}
		out = append(out, v...)
		if len(refs) > 1 {
			out = append(out, '\n')
		}
	}
	defer env.hold(out)()
	_, err = env.Stdout.Write(out)
	return err
}
