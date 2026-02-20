package polecat

import (
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/mail"
)

// setupTestMailbox creates a legacy mailbox in a temp dir and populates it
// with POLECAT_STARTED messages for testing.
func setupTestMailbox(t *testing.T, msgs []*mail.Message) *mail.Mailbox {
	t.Helper()
	tmpDir := t.TempDir()
	mb := mail.NewMailbox(tmpDir)
	for _, msg := range msgs {
		if err := mb.Append(msg); err != nil {
			t.Fatalf("Append error: %v", err)
		}
	}
	return mb
}

func TestCheckMailboxForSpawns_Basic(t *testing.T) {
	mb := setupTestMailbox(t, []*mail.Message{
		{
			ID:        "mail-001",
			Subject:   "POLECAT_STARTED gastown/Toast",
			Body:      "Session: gt-gastown-polecat-Toast\nIssue: gt-abc123",
			Timestamp: time.Now().Add(-1 * time.Minute),
		},
	})

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	if len(pending) != 1 {
		t.Fatalf("expected 1 pending spawn, got %d", len(pending))
	}

	ps := pending[0]
	if ps.Rig != "gastown" {
		t.Errorf("Rig = %q, want %q", ps.Rig, "gastown")
	}
	if ps.Polecat != "Toast" {
		t.Errorf("Polecat = %q, want %q", ps.Polecat, "Toast")
	}
	if ps.Session != "gt-gastown-polecat-Toast" {
		t.Errorf("Session = %q, want %q", ps.Session, "gt-gastown-polecat-Toast")
	}
	if ps.Issue != "gt-abc123" {
		t.Errorf("Issue = %q, want %q", ps.Issue, "gt-abc123")
	}
	if ps.MailID != "mail-001" {
		t.Errorf("MailID = %q, want %q", ps.MailID, "mail-001")
	}
	if ps.mailbox == nil {
		t.Error("mailbox should not be nil")
	}
}

func TestCheckMailboxForSpawns_IgnoresNonStartedMessages(t *testing.T) {
	mb := setupTestMailbox(t, []*mail.Message{
		{
			ID:      "mail-001",
			Subject: "POLECAT_STARTED gastown/Toast",
			Body:    "Session: gt-gastown-polecat-Toast\nIssue: gt-abc123",
		},
		{
			ID:      "mail-002",
			Subject: "HEALTH_CHECK gastown/Toast",
			Body:    "Session: gt-gastown-polecat-Toast",
		},
		{
			ID:      "mail-003",
			Subject: "Some other mail",
			Body:    "Not a spawn notification",
		},
	})

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	if len(pending) != 1 {
		t.Errorf("expected 1 pending spawn, got %d", len(pending))
	}
}

func TestCheckMailboxForSpawns_MultipleSpawns(t *testing.T) {
	mb := setupTestMailbox(t, []*mail.Message{
		{
			ID:        "mail-001",
			Subject:   "POLECAT_STARTED gastown/Toast",
			Body:      "Session: gt-gastown-polecat-Toast\nIssue: gt-abc",
			Timestamp: time.Now().Add(-2 * time.Minute),
		},
		{
			ID:        "mail-002",
			Subject:   "POLECAT_STARTED gastown/Nux",
			Body:      "Session: gt-gastown-polecat-Nux\nIssue: gt-def",
			Timestamp: time.Now().Add(-1 * time.Minute),
		},
	})

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	if len(pending) != 2 {
		t.Fatalf("expected 2 pending spawns, got %d", len(pending))
	}
}

func TestCheckMailboxForSpawns_EmptyMailbox(t *testing.T) {
	mb := setupTestMailbox(t, nil)

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("expected 0 pending spawns, got %d", len(pending))
	}
}

func TestCheckMailboxForSpawns_MalformedSubject(t *testing.T) {
	mb := setupTestMailbox(t, []*mail.Message{
		{
			ID:      "mail-001",
			Subject: "POLECAT_STARTED no-slash-here",
			Body:    "Session: some-session\nIssue: gt-abc",
		},
	})

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("expected 0 pending spawns for malformed subject, got %d", len(pending))
	}
}

func TestClearPendingSpawnFromList_ClearsAllMatching(t *testing.T) {
	// Simulate duplicate POLECAT_STARTED messages for the same session
	tmpDir := t.TempDir()
	mb := mail.NewMailbox(tmpDir)

	msgs := []*mail.Message{
		{
			ID:        "mail-001",
			Subject:   "POLECAT_STARTED gastown/Toast",
			Body:      "Session: gt-gastown-polecat-Toast\nIssue: gt-abc",
			Timestamp: time.Now().Add(-2 * time.Minute),
		},
		{
			ID:        "mail-002",
			Subject:   "POLECAT_STARTED gastown/Toast",
			Body:      "Session: gt-gastown-polecat-Toast\nIssue: gt-abc",
			Timestamp: time.Now().Add(-1 * time.Minute),
		},
		{
			ID:        "mail-003",
			Subject:   "POLECAT_STARTED gastown/Nux",
			Body:      "Session: gt-gastown-polecat-Nux\nIssue: gt-def",
			Timestamp: time.Now(),
		},
	}
	for _, msg := range msgs {
		if err := mb.Append(msg); err != nil {
			t.Fatalf("Append error: %v", err)
		}
	}

	// Build pending list from mailbox
	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending spawns, got %d", len(pending))
	}

	// Clear all for Toast session
	err = clearPendingSpawnFromList(pending, "gt-gastown-polecat-Toast")
	if err != nil {
		t.Fatalf("clearPendingSpawnFromList error: %v", err)
	}

	// Verify: only Nux should remain in inbox
	remaining, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns after clear error: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining spawn, got %d", len(remaining))
	}
	if remaining[0].Session != "gt-gastown-polecat-Nux" {
		t.Errorf("remaining session = %q, want %q", remaining[0].Session, "gt-gastown-polecat-Nux")
	}

	// Verify: both Toast messages should be in archive
	archived, err := mb.ListArchived()
	if err != nil {
		t.Fatalf("ListArchived error: %v", err)
	}
	if len(archived) != 2 {
		t.Fatalf("expected 2 archived messages, got %d", len(archived))
	}
}

func TestClearPendingSpawnFromList_Idempotent(t *testing.T) {
	mb := setupTestMailbox(t, []*mail.Message{
		{
			ID:      "mail-001",
			Subject: "POLECAT_STARTED gastown/Toast",
			Body:    "Session: gt-gastown-polecat-Toast\nIssue: gt-abc",
		},
	})

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	// Clear a non-existent session should succeed (idempotent)
	err = clearPendingSpawnFromList(pending, "nonexistent-session")
	if err != nil {
		t.Errorf("clearPendingSpawnFromList should be idempotent, got error: %v", err)
	}

	// Original message should still be in inbox
	remaining, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns after clear error: %v", err)
	}
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining spawn, got %d", len(remaining))
	}
}

func TestClearPendingSpawnFromList_NilMailboxError(t *testing.T) {
	// Build a pending spawn with nil mailbox
	pending := []*PendingSpawn{
		{
			Session: "gt-gastown-polecat-Toast",
			MailID:  "mail-001",
			mailbox: nil,
		},
	}

	err := clearPendingSpawnFromList(pending, "gt-gastown-polecat-Toast")
	if err == nil {
		t.Fatal("expected error for nil mailbox, got nil")
	}
	if !containsSubstr(err.Error(), "nil mailbox") {
		t.Errorf("error = %q, want containing 'nil mailbox'", err.Error())
	}
}

func TestClearPendingSpawnFromList_ArchiveSideEffect(t *testing.T) {
	tmpDir := t.TempDir()
	mb := mail.NewMailbox(tmpDir)

	if err := mb.Append(&mail.Message{
		ID:      "mail-001",
		Subject: "POLECAT_STARTED gastown/Toast",
		Body:    "Session: gt-gastown-polecat-Toast\nIssue: gt-abc",
	}); err != nil {
		t.Fatalf("Append error: %v", err)
	}

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	// Verify no archive entries before clear
	archived, err := mb.ListArchived()
	if err != nil {
		t.Fatalf("ListArchived error: %v", err)
	}
	if len(archived) != 0 {
		t.Fatalf("expected 0 archived before clear, got %d", len(archived))
	}

	// Clear the spawn
	err = clearPendingSpawnFromList(pending, "gt-gastown-polecat-Toast")
	if err != nil {
		t.Fatalf("clearPendingSpawnFromList error: %v", err)
	}

	// Verify message was archived (not just deleted)
	archived, err = mb.ListArchived()
	if err != nil {
		t.Fatalf("ListArchived error: %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived message after clear, got %d", len(archived))
	}
	if archived[0].ID != "mail-001" {
		t.Errorf("archived message ID = %q, want %q", archived[0].ID, "mail-001")
	}

	// Verify message was removed from inbox
	remaining, err := mb.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	for _, msg := range remaining {
		if msg.ID == "mail-001" {
			t.Error("message should have been removed from inbox after archive")
		}
	}
}

func TestPruneStalePendingFromList_PrunesOldSpawns(t *testing.T) {
	tmpDir := t.TempDir()
	mb := mail.NewMailbox(tmpDir)

	msgs := []*mail.Message{
		{
			ID:        "mail-old",
			Subject:   "POLECAT_STARTED gastown/OldToast",
			Body:      "Session: gt-gastown-polecat-OldToast\nIssue: gt-old",
			Timestamp: time.Now().Add(-10 * time.Minute),
		},
		{
			ID:        "mail-new",
			Subject:   "POLECAT_STARTED gastown/NewToast",
			Body:      "Session: gt-gastown-polecat-NewToast\nIssue: gt-new",
			Timestamp: time.Now().Add(-1 * time.Minute),
		},
	}
	for _, msg := range msgs {
		if err := mb.Append(msg); err != nil {
			t.Fatalf("Append error: %v", err)
		}
	}

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	// Prune spawns older than 5 minutes
	pruned, err := pruneStalePendingFromList(pending, 5*time.Minute)
	if err != nil {
		t.Fatalf("pruneStalePendingFromList error: %v", err)
	}

	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	// Verify only new spawn remains
	remaining, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns after prune error: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining spawn, got %d", len(remaining))
	}
	if remaining[0].Session != "gt-gastown-polecat-NewToast" {
		t.Errorf("remaining session = %q, want %q", remaining[0].Session, "gt-gastown-polecat-NewToast")
	}

	// Verify old spawn was archived
	archived, err := mb.ListArchived()
	if err != nil {
		t.Fatalf("ListArchived error: %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived message, got %d", len(archived))
	}
	if archived[0].ID != "mail-old" {
		t.Errorf("archived message ID = %q, want %q", archived[0].ID, "mail-old")
	}
}

func TestPruneStalePendingFromList_NothingToprune(t *testing.T) {
	mb := setupTestMailbox(t, []*mail.Message{
		{
			ID:        "mail-001",
			Subject:   "POLECAT_STARTED gastown/Toast",
			Body:      "Session: gt-gastown-polecat-Toast\nIssue: gt-abc",
			Timestamp: time.Now(),
		},
	})

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	pruned, err := pruneStalePendingFromList(pending, 5*time.Minute)
	if err != nil {
		t.Fatalf("pruneStalePendingFromList error: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned for fresh spawn, got %d", pruned)
	}
}

func TestPruneStalePendingFromList_NilMailboxError(t *testing.T) {
	pending := []*PendingSpawn{
		{
			Session:   "gt-gastown-polecat-Toast",
			MailID:    "mail-001",
			SpawnedAt: time.Now().Add(-10 * time.Minute),
			mailbox:   nil,
		},
	}

	_, err := pruneStalePendingFromList(pending, 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for nil mailbox, got nil")
	}
	if !containsSubstr(err.Error(), "nil mailbox") {
		t.Errorf("error = %q, want containing 'nil mailbox'", err.Error())
	}
}

func TestPruneStalePendingFromList_ArchiveSideEffect(t *testing.T) {
	tmpDir := t.TempDir()
	mb := mail.NewMailbox(tmpDir)

	if err := mb.Append(&mail.Message{
		ID:        "mail-stale",
		Subject:   "POLECAT_STARTED gastown/Toast",
		Body:      "Session: gt-gastown-polecat-Toast\nIssue: gt-abc",
		Timestamp: time.Now().Add(-10 * time.Minute),
	}); err != nil {
		t.Fatalf("Append error: %v", err)
	}

	pending, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}

	pruned, err := pruneStalePendingFromList(pending, 5*time.Minute)
	if err != nil {
		t.Fatalf("pruneStalePendingFromList error: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	// Verify archive side effect: message moved to archive
	archived, err := mb.ListArchived()
	if err != nil {
		t.Fatalf("ListArchived error: %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived, got %d", len(archived))
	}
	if archived[0].ID != "mail-stale" {
		t.Errorf("archived ID = %q, want %q", archived[0].ID, "mail-stale")
	}

	// Verify removed from inbox
	remaining, err := checkMailboxForSpawns(mb)
	if err != nil {
		t.Fatalf("checkMailboxForSpawns error: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining after prune, got %d", len(remaining))
	}
}

// containsSubstr checks if s contains substr.
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
