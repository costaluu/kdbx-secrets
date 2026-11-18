package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/racoonlemore/kdbx-secrets/internal/pgp"
	"github.com/racoonlemore/kdbx-secrets/internal/ref"
)

func init() { register("encrypt", cmdEncrypt) }

func cmdEncrypt(env *Env, args []string) error {
	var files, refs []string
	var parsed []ref.Ref
	for i := 0; i < len(args); i++ {
		if (args[i] != "-r" && args[i] != "--to") || i+1 >= len(args) {
			return usagef("usage: encrypt (-r <public-key-file> | --to <reference>)...")
		}
		if args[i] == "-r" {
			files = append(files, args[i+1])
		} else {
			r, err := ref.Parse(args[i+1])
			if err != nil {
				return usagef("%v", err)
			}
			parsed, refs = append(parsed, r), append(refs, args[i+1])
		}
		i++
	}
	if len(files)+len(parsed) == 0 {
		return usagef("encrypt needs at least one recipient (-r <public-key-file> or --to <reference>)")
	}
	var to openpgp.EntityList
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		el, err := pgp.ReadPublicKey(data)
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		to = append(to, el...)
	}
	if len(parsed) > 0 {
		d, err := env.unlock(strings.Join(refs, " "))
		if err != nil {
			return err
		}
		for i, r := range parsed {
			res, err := ref.Resolve(d, r, env.Now())
			if err != nil {
				return err
			}
			el, err := pgp.ReadPublicKey([]byte(res.Value))
			if err != nil {
				return fmt.Errorf("reference %q: %w", refs[i], err)
			}
			to = append(to, el...)
		}
	}
	return pgp.Encrypt(env.Stdout, env.Stdin, to)
}
