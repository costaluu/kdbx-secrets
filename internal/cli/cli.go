// Package cli is the single, testable entry point of kdbx-secrets: Run takes
// an injected Env and returns the exit code. main only builds the real Env.
//
// Adding a command: create <name>.go with an init() that calls register.
package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/racoonlemore/kdbx-secrets/internal/prompt"
)

// Terminal is the controlling terminal used for the password prompt.
type Terminal = io.ReadWriteCloser

// Env is everything the CLI touches from the outside world.
type Env struct {
	Args         []string                 // without the program name
	Getenv       func(string) string      // nil means "no variables"
	HomeDir      string                   // user home, for ~/.kdbx-secrets and ~ expansion
	Stdin        io.Reader                // nil means empty
	Stdout       io.Writer                //
	Stderr       io.Writer                //
	StdoutIsTTY  bool                     // whether Stdout is a terminal (see guardSecretOut)
	OpenTerminal func() (Terminal, error) // lazy: only called when a prompt is needed
	Now          func() time.Time         // clock (TOTP); nil means time.Now
	ParentChain  func() string            // "aws ← zsh" shown in the prompt; nil means unknown
	Version      string                   // injected at build time; "" prints "dev"
	Harden       func(kdfMemory uint64)   // memory hardening before the prompt; nil in tests
	Environ      func() []string          // process environment inherited by `run` children; nil means empty
}

func (env *Env) environ() []string {
	if env.Environ == nil {
		return nil
	}
	return env.Environ()
}

// Exit codes (spec: 0 ok, 1 error, 2 incorrect usage).
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// usageError makes Run exit with 2 instead of 1.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, a ...any) error { return usageError{fmt.Sprintf(format, a...)} }

type command func(env *Env, args []string) error

var commands = map[string]command{}

func register(name string, c command) { commands[name] = c }

// Run executes one invocation and returns the process exit code.
func Run(env *Env) int {
	if env.Stdout == nil {
		env.Stdout = io.Discard
	}
	if env.Stderr == nil {
		env.Stderr = io.Discard
	}
	if env.Getenv == nil {
		env.Getenv = func(string) string { return "" }
	}
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.Stdin == nil {
		env.Stdin = strings.NewReader("")
	}
	err := dispatch(env)
	if err == nil {
		return ExitOK
	}
	if c, ok := err.(exitCode); ok {
		return int(c)
	}
	fmt.Fprintln(env.Stderr, "kdbx-secrets:", prompt.Sanitize(err.Error()))
	if _, ok := err.(usageError); ok {
		return ExitUsage
	}
	return ExitError
}

func dispatch(env *Env) error {
	if len(env.Args) == 0 {
		return usagef("missing command (one of: %s)", strings.Join(names(), ", "))
	}
	c, ok := commands[env.Args[0]]
	if !ok {
		return usagef("unknown command %q (one of: %s)", env.Args[0], strings.Join(names(), ", "))
	}
	return c(env, env.Args[1:])
}

func names() []string {
	var n []string
	for k := range commands {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}
