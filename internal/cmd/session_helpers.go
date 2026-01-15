package cmd

import (
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
)

// sessionsProvider is a function that creates a Sessions instance.
// This can be overridden in tests to inject a double.
var sessionsProvider = func() session.Sessions {
	return tmux.NewTmux()
}

// newSessions returns a Sessions instance for terminal operations.
// Uses sessionsProvider which can be overridden in tests.
func newSessions() session.Sessions {
	return sessionsProvider()
}

// newTmux returns a Tmux instance for tmux-specific operations.
// Use this only when tmux-specific features are needed (theming, env vars, etc.).
// For basic session operations, prefer newSessions().
func newTmux() *tmux.Tmux {
	return tmux.NewTmux()
}
