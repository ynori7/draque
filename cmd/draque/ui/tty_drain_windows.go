//go:build windows

package ui

// drainStdinInput is a no-op on Windows; the terminal capability query
// response issue only manifests on Unix pty-based terminals.
func drainStdinInput() {}
