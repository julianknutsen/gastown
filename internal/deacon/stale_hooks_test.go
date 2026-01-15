package deacon

import (
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/session"
)

// mockSessionChecker is a test implementation of SessionChecker.
type mockSessionChecker struct {
	sessions map[string]bool
}

func (m *mockSessionChecker) Exists(id session.SessionID) (bool, error) {
	return m.sessions[string(id)], nil
}

func TestAssigneeToSessionName(t *testing.T) {
	tests := []struct {
		name     string
		assignee string
		want     string
	}{
		{
			name:     "deacon",
			assignee: "deacon",
			want:     "hq-deacon",
		},
		{
			name:     "mayor",
			assignee: "mayor",
			want:     "hq-mayor",
		},
		{
			name:     "witness",
			assignee: "gastown/witness",
			want:     "gt-gastown-witness",
		},
		{
			name:     "refinery",
			assignee: "gastown/refinery",
			want:     "gt-gastown-refinery",
		},
		{
			name:     "polecat",
			assignee: "gastown/polecats/max",
			want:     "gt-gastown-max",
		},
		{
			name:     "crew",
			assignee: "gastown/crew/joe",
			want:     "gt-gastown-crew-joe",
		},
		{
			name:     "unknown single part",
			assignee: "unknown",
			want:     "",
		},
		{
			name:     "unknown two parts",
			assignee: "gastown/unknown",
			want:     "",
		},
		{
			name:     "unknown three parts",
			assignee: "gastown/unknown/foo",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assigneeToSessionName(tt.assignee)
			if got != tt.want {
				t.Errorf("assigneeToSessionName(%q) = %q, want %q", tt.assignee, got, tt.want)
			}
		})
	}
}

func TestStaleHookConfig_SessionChecker(t *testing.T) {
	// Verify that SessionChecker can be injected into the config
	mock := &mockSessionChecker{
		sessions: map[string]bool{
			"gt-gastown-witness": true,
			"gt-gastown-max":     false,
		},
	}

	cfg := &StaleHookConfig{
		MaxAge:         1 * time.Hour,
		DryRun:         true,
		SessionChecker: mock,
	}

	// Verify the mock works as expected
	alive, _ := cfg.SessionChecker.Exists(session.SessionID("gt-gastown-witness"))
	if !alive {
		t.Error("expected witness session to be alive")
	}

	alive, _ = cfg.SessionChecker.Exists(session.SessionID("gt-gastown-max"))
	if alive {
		t.Error("expected max session to be dead")
	}

	alive, _ = cfg.SessionChecker.Exists(session.SessionID("nonexistent"))
	if alive {
		t.Error("expected nonexistent session to return false")
	}
}

func TestDefaultStaleHookConfig(t *testing.T) {
	cfg := DefaultStaleHookConfig()

	if cfg.MaxAge != 1*time.Hour {
		t.Errorf("MaxAge = %v, want 1h", cfg.MaxAge)
	}

	if cfg.DryRun {
		t.Error("DryRun should be false by default")
	}

	if cfg.SessionChecker != nil {
		t.Error("SessionChecker should be nil by default")
	}
}
