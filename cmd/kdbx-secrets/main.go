// Command kdbx-secrets: main only assembles the real environment.
package main

import (
	"os"

	"golang.org/x/term"

	"github.com/racoonlemore/kdbx-secrets/internal/cli"
	"github.com/racoonlemore/kdbx-secrets/internal/hardening"
	"github.com/racoonlemore/kdbx-secrets/internal/prompt"
)

// version is injected at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	home, _ := os.UserHomeDir()
	os.Exit(cli.Run(&cli.Env{
		Args:        os.Args[1:],
		Getenv:      os.Getenv,
		HomeDir:     home,
		Environ:     os.Environ,
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		StdoutIsTTY: term.IsTerminal(int(os.Stdout.Fd())),
		OpenTerminal: func() (cli.Terminal, error) {
			c, err := prompt.OpenConsole()
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		Harden:      func(kdfMem uint64) { hardening.Apply(kdfMem, os.Stderr) },
		ParentChain: prompt.ParentChain,
		Version:     version,
	}))
}
