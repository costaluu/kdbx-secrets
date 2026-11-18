package cli

import (
	"errors"
	"fmt"

	"github.com/racoonlemore/kdbx-secrets/internal/hardening"
	"github.com/racoonlemore/kdbx-secrets/internal/prompt"
)

// warn is the single warnings printer (sanitized: warnings cite References).
func (env *Env) warn(w string) {
	fmt.Fprintln(env.Stderr, "kdbx-secrets: warning:", prompt.Sanitize(w))
}

// hold locks a secret buffer in RAM; the returned func wipes and unlocks it.
func (env *Env) hold(b []byte) func() {
	unlock := hardening.LockBuffer(b, env.Stderr)
	return func() { clear(b); unlock() }
}

// takeShow removes every --show from args.
func takeShow(args []string) (show bool, rest []string) {
	for _, a := range args {
		if a == "--show" {
			show = true
		} else {
			rest = append(rest, a)
		}
	}
	return
}

// guardSecretOut is the single check that refuses secrets on a terminal stdout
// without --show. Call it before the prompt.
func (env *Env) guardSecretOut(show bool) error {
	if env.StdoutIsTTY && !show {
		return errors.New("refusing to write a secret to a terminal; use --show")
	}
	return nil
}
