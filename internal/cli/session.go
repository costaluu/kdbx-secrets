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
	cfg, err := config.Load(env.Getenv, env.HomeDir)
	if err != nil {
		return nil, err
	}
	kdf, err := db.CheckFile(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("database %s: %w", cfg.Database, err)
	}
	if env.Harden != nil {
		env.Harden(kdf.MemoryBytes) // before any secret enters memory
	}
	if env.OpenTerminal == nil {
		return nil, errors.New("no terminal available for the password prompt")
	}
	term, err := env.OpenTerminal()
	if err != nil {
		return nil, fmt.Errorf("no terminal available for the password prompt: %w", err)
	}
	defer term.Close()
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
			return nil, err
		}
		d, err := db.Open(cfg.Database, pw)
		clear(buf)
		if errors.Is(err, db.ErrWrongPassword) {
			if attempt == maxAttempts {
				return nil, fmt.Errorf("incorrect password (%d attempts)", maxAttempts)
			}
			fmt.Fprintln(term, "incorrect password, try again")
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("database %s: %w", cfg.Database, err)
		}
		return d, nil
	}
}

const maxAttempts = 3
