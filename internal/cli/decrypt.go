package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/racoonlemore/kdbx-secrets/internal/pgp"
	"github.com/racoonlemore/kdbx-secrets/internal/ref"
)

func init() { register("decrypt", cmdDecrypt) }

// cmdDecrypt decrypts stdin with a private key held in the Database. The key's
// passphrase is the Password of the same Entry, read only if the key is
// encrypted. Nothing is written unless integrity was verified.
func cmdDecrypt(env *Env, args []string) error {
	show, args := takeShow(args)
	if len(args) != 2 || args[0] != "--key" {
		return usagef("usage: decrypt --key <reference> [--show]")
	}
	keyArg := args[1]
	if keyArg == "" {
		return usagef("usage: decrypt --key <reference> [--show]")
	}
	r, err := ref.Parse(keyArg)
	if err != nil {
		return usagef("%v", err)
	}
	if err := env.guardSecretOut(show); err != nil {
		return err
	}
	// armor adds about a third; anything past this cannot hold <= 64 MiB of plaintext
	msg, err := io.ReadAll(io.LimitReader(env.Stdin, 2*pgp.MaxPlain+1))
	defer clear(msg)
	if err != nil {
		return err
	}
	if len(msg) > 2*pgp.MaxPlain {
		return errors.New("message is larger than 64 MiB")
	}
	d, err := env.unlock(keyArg)
	if err != nil {
		return err
	}
	res, err := ref.Resolve(d, r, env.Now())
	if err != nil {
		return err
	}
	keys, err := pgp.ReadPrivateKey([]byte(res.Value))
	if err != nil {
		return fmt.Errorf("reference %q: %w", keyArg, err)
	}
	var pass []byte
	if pgp.KeyEncrypted(keys) {
		pr := r
		pr.Field = "Password"
		p, err := ref.Resolve(d, pr, env.Now())
		if err != nil {
			return err
		}
		pass = []byte(p.Value)
		defer clear(pass)
	}
	plain, err := pgp.Decrypt(msg, keys, pass)
	if err != nil {
		return err
	}
	defer env.hold(plain)()
	_, err = env.Stdout.Write(plain)
	return err
}
