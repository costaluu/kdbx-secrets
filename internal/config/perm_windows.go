//go:build windows

package config

// CheckPrivate is a no-op on Windows (ACL check deferred).
func CheckPrivate(string) error { return nil }
