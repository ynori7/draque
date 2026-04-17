//go:build !windows

package ui

// drainStdinInput is no longer needed — the application runs as a single
// persistent Bubbletea program (tea.WithAltScreen) and never re-enters a
// REPL loop, so terminal response bytes cannot leak into a separate reader.
func drainStdinInput() {}
