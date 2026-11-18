//go:build !linux && !windows

package cli

import "os/exec"

// No parent-death link on macOS and other Unixes: if the run dies, the child
// survives and its calls to the Vault fail (documented in the spec).
func bind(*exec.Cmd) {}

func adopt(*exec.Cmd) error { return nil }
