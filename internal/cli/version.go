package cli

import "fmt"

func init() { register("version", cmdVersion) }

func cmdVersion(env *Env, args []string) error {
	if len(args) != 0 {
		return usagef("usage: version")
	}
	v := env.Version
	if v == "" {
		v = "dev"
	}
	_, err := fmt.Fprintln(env.Stdout, v)
	return err
}
