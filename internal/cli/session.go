package cli

import (
	"errors"
	"fmt"

	"github.com/racoonlemore/kdbx-secrets/internal/config"
	"github.com/racoonlemore/kdbx-secrets/internal/db"
	"github.com/racoonlemore/kdbx-secrets/internal/prompt"
)

// unlock is the shared "needs the Database" path, in the fixed validation order:
// config, Database file/header, prompt, open. Callers must already have checked
// arguments and syntax. why names what is requested (shown in the prompt).
func (env *Env) unlock(why string) (*db.DB, error) {
	d, term, err := env.unlockTerm(why, nil)
	if err != nil {
		return nil, err
	}
	term.Close()
	return d, nil
}

// unlockTerm is unlock that, on success, hands back the open terminal for
// further prompts (the caller closes it) and, if keep is set, passes it the
// password before the buffer is wiped.
func (env *Env) unlockTerm(why string, keep func(pw []byte)) (_ *db.DB, term Terminal, err error) {
	defer func() {
		if err != nil && term != nil {
			term.Close()
			term = nil
		}
	}()
	cfg, err := config.Load(env.Getenv, env.HomeDir)
	if err != nil {
		return nil, term, err
	}
	kdf, err := db.CheckFile(cfg.Database)
	if err != nil {
		return nil, term, fmt.Errorf("database %s: %w", cfg.Database, err)
	}
	if env.Harden != nil {
		env.Harden(kdf.MemoryBytes) // before any secret enters memory
	}
	if env.OpenTerminal == nil {
		return nil, term, errors.New("no terminal available for the password prompt")
	}
	t, err := env.OpenTerminal()
	if err != nil {
		return nil, nil, fmt.Errorf("no terminal available for the password prompt: %w", err)
	}
	term = t
	who := "unknown caller"
	if env.ParentChain != nil {
		if c := env.ParentChain(); c != "" {
			who = c
		}
	}
	msg := fmt.Sprintf("KeePass password (asked by %s; requested: %s): ", prompt.Sanitize(who), prompt.Sanitize(why))
	buf := make([]byte, prompt.MaxPassword+1)
	defer env.hold(buf)()
	for attempt := 1; ; attempt++ {
		pw, err := prompt.Password(term, msg, buf)
		if err != nil {
			return nil, term, err
		}
		d, err := db.Open(cfg.Database, pw)
		if err == nil && keep != nil {
			keep(pw)
		}
		clear(buf)
		if errors.Is(err, db.ErrWrongPassword) {
			if attempt == maxAttempts {
				return nil, term, fmt.Errorf("incorrect password (%d attempts)", maxAttempts)
			}
			fmt.Fprintln(term, "incorrect password, try again")
			continue
		}
		if err != nil {
			return nil, term, fmt.Errorf("database %s: %w", cfg.Database, err)
		}
		return d, term, nil
	}
}

const maxAttempts = 3
