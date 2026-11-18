package cli

import "fmt"

func init() { register("help", cmdHelp) }

const helpText = `usage: <command> [args]

A Reference is Group/Entry#Field (without the Root group). Without #, it is the
Password. Also: uuid:<32 hex>, custom attributes, #TOTP, #attachment:<name>.

commands:
  get [--show] <reference>...
      one value per line, with a single prompt
  render [--show] <name>
      runs the template ~/.kdbx-secrets/templates/<name>.tmpl
  adapter create <name> [--from <file>] [--force] | delete <name> | list
      manages the templates
  encrypt (-r <public-key-file> | --to <reference>)...
      encrypts stdin to OpenPGP (armored) for the recipients
  decrypt --key <reference> [--show]
      decrypts stdin with the private key held in the Database
  run --vault <mount>/<path>=<entry-reference> [--vault ...] -- <command> [args]
      runs the command against a local, read-only Vault KV v2 imitation
  version
      prints the version
  help
      prints this help

--show is required to write secrets to a terminal.
exit codes: 0 ok, 1 error, 2 incorrect usage (run passes through the child's code)
`

func cmdHelp(env *Env, args []string) error {
	if len(args) != 0 {
		return usagef("usage: help")
	}
	_, err := fmt.Fprint(env.Stdout, helpText)
	return err
}
