//go:build !windows && !linux
// +build !windows,!linux

package collectors

// StartFileWatcher is a NOP on unsupported platforms.
func StartFileWatcher(_ string, _ int) {}
