package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/racoonlemore/kdbx-secrets/internal/ref"
	"github.com/racoonlemore/kdbx-secrets/internal/vault"
)

func init() { register("run", cmdRun) }

// exitCode makes Run exit with the child's code, printing nothing.
type exitCode int

func (e exitCode) Error() string { return fmt.Sprintf("exit %d", int(e)) }

var reservedMounts = map[string]bool{"sys": true, "auth": true, "identity": true, "cubbyhole": true}

type mapping struct {
	target string // "<mount>/<path>"
	raw    string
	ref    ref.Ref
}

// parseVault splits at the first '=' and applies the usage rules (all before
// the prompt).
func parseVault(spec string, seen map[string]bool) (mapping, error) {
	target, raw, ok := strings.Cut(spec, "=")
	if !ok {
		return mapping{}, usagef("--vault %q: expected <mount>/<path>=<reference>", spec)
	}
	segs := strings.Split(target, "/")
	if len(segs) < 2 {
		return mapping{}, usagef("--vault %q: expected <mount>/<path>", target)
	}
	for _, s := range segs {
		if s == "" || s == "." || s == ".." {
			return mapping{}, usagef("--vault %q: invalid path segment %q", target, s)
		}
	}
	if reservedMounts[segs[0]] {
		return mapping{}, usagef("--vault %q: mount %q is reserved", target, segs[0])
	}
	if seen[target] {
		return mapping{}, usagef("--vault %q: repeated path", target)
	}
	if hasField(raw) {
		return mapping{}, usagef("--vault %q: the reference must name an entry, without #field", target)
	}
	r, err := ref.Parse(raw)
	if err != nil {
		return mapping{}, usagef("%v", err)
	}
	return mapping{target, raw, r}, nil
}

// hasField reports an unescaped '#'.
func hasField(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
		} else if s[i] == '#' {
			return true
		}
	}
	return false
}

// supervise runs the child: SIGINT is swallowed (Ctrl-C reaches the child via
// the terminal's process group), SIGTERM and SIGHUP are forwarded, and the
// child is tied to this process. The server keeps answering until Wait returns.
// Notify (not Ignore) so the child does not inherit an ignored SIGINT.
// started runs right after Start (the parent closes its PTY slaves there).
func supervise(c *exec.Cmd, started func()) error {
	runtime.LockOSThread() // Pdeathsig fires when the spawning thread dies
	defer runtime.UnlockOSThread()
	bind(c)
	sigs := make(chan os.Signal, 8)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)
	if err := c.Start(); err != nil {
		return err
	}
	started()
	if err := adopt(c); err != nil {
		c.Process.Kill()
		c.Wait()
		return err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case s := <-sigs:
				if s != os.Interrupt {
					c.Process.Signal(s)
				}
			case <-done:
				return
			}
		}
	}()
	return c.Wait()
}

const runUsage = "usage: run --vault <mount>/<path>=<reference>... [--disable-redaction] (--command '<shell>' | -- <command> [args])"

func cmdRun(env *Env, args []string) error {
	var maps []mapping
	seen := map[string]bool{}
	command, redact := "", true
	i := 0
	for ; i < len(args) && args[i] != "--"; i++ {
		switch {
		case args[i] == "--disable-redaction":
			redact = false
		case args[i] == "--command" && i+1 < len(args) && command == "" && args[i+1] != "":
			i++
			command = args[i]
		case args[i] == "--vault" && i+1 < len(args):
			i++
			m, err := parseVault(args[i], seen)
			if err != nil {
				return err
			}
			seen[m.target] = true
			maps = append(maps, m)
		default:
			return usagef(runUsage)
		}
	}
	var cmdline []string
	switch {
	case len(maps) == 0 || (command != "") == (i < len(args)):
		return usagef(runUsage)
	case command != "":
		cmdline = []string{"sh", "-c", command}
		if runtime.GOOS == "windows" {
			cmdline = []string{"cmd", "/C", command}
		}
	case i+1 < len(args):
		cmdline = args[i+1:]
	default:
		return usagef(runUsage)
	}

	var raws []string
	for _, m := range maps {
		raws = append(raws, m.raw)
	}
	d, err := env.unlock(strings.Join(raws, " "))
	if err != nil {
		return err
	}
	scope := vault.Scope{}
	var fields []map[string]string
	for _, m := range maps {
		f, seed, err := ref.Snapshot(d, m.ref)
		if err != nil {
			return err
		}
		scope[m.target] = vault.Secret{Fields: f, Seed: seed}
		fields = append(fields, f)
	}
	var vals []string
	if redact {
		vals = redactValues(fields)
	}
	d = nil // the rest of the Database is garbage from here on

	srv, err := vault.Start(scope, env.Now)
	if err != nil {
		return err
	}
	defer srv.Close()

	c := exec.Command(cmdline[0], cmdline[1:]...)
	c.Env = append(env.environ(), "VAULT_ADDR=http://"+srv.Addr, "VAULT_TOKEN="+srv.Token)
	c.Stdin = env.Stdin
	var startedOut, startedErr, finishOut, finishErr func()
	c.Stdout, startedOut, finishOut = redactStream(env.Stdout, vals)
	c.Stderr, startedErr, finishErr = redactStream(env.Stderr, vals)
	err = supervise(c, func() { startedOut(); startedErr() })
	startedOut()
	startedErr()
	finishOut()
	finishErr()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ee) && ee.ExitCode() >= 0:
		return exitCode(ee.ExitCode())
	case errors.As(err, &ee):
		// killed by a signal: mirror the shell convention (128+signal)
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return exitCode(128 + int(ws.Signal()))
		}
	}
	return fmt.Errorf("command %q: %w", cmdline[0], err)
}

// redactStream gives the child a stream to dst that hides vals. A terminal
// gets a PTY so the child still sees one. started closes the parent's copy of
// the PTY slave (call after Start, and again if Start failed); finish runs
// after Wait and writes what the redactor held back.
// ponytail: like a plain pipe, finish waits for every holder of the stream
// (a daemonized grandchild keeps run alive until it closes its output).
func redactStream(dst io.Writer, vals []string) (child io.Writer, started, finish func()) {
	if len(vals) == 0 {
		return dst, func() {}, func() {}
	}
	r := newRedactor(dst, vals)
	if f, ok := dst.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if master, slave, err := openPty(f); err == nil {
			done := make(chan struct{})
			go func() { io.Copy(r, master); close(done) }() // EIO once every slave is closed
			return slave, func() { slave.Close() }, func() { <-done; master.Close(); r.Close() }
		}
	}
	return r, func() {}, func() { r.Close() } // exec copies into r; Wait waits for EOF
}
