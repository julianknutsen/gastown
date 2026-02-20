package polecat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/mail"
)

// writeInboxMessages writes JSONL messages to a legacy mailbox inbox.
func writeInboxMessages(t *testing.T, dir string, msgs []*mail.Message) {
	t.Helper()
	inboxPath := filepath.Join(dir, "inbox.jsonl")
	f, err := os.Create(inboxPath)
	if err != nil {
		t.Fatalf("creating inbox: %v", err)
	}
	defer func() { _ = f.Close() }()

	for _, msg := range msgs {
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshaling message: %v", err)
		}
		if _, err := f.WriteString(string(data) + "\n"); err != nil {
			t.Fatalf("writing message: %v", err)
		}
	}
}

// readInboxMessages reads remaining messages from a legacy mailbox.
func readInboxMessages(t *testing.T, mb *mail.Mailbox) []*mail.Message {
	t.Helper()
	msgs, err := mb.List()
	if err != nil {
		t.Fatalf("listing inbox: %v", err)
	}
	return msgs
}

// readArchiveMessages reads messages from the archive file.
func readArchiveMessages(t *testing.T, dir string) []*mail.Message {
	t.Helper()
	archivePath := filepath.Join(dir, "inbox.jsonl.archive")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading archive: %v", err)
	}

	var msgs []*mail.Message
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var msg mail.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("unmarshaling archive message: %v", err)
		}
		msgs = append(msgs, &msg)
	}
	return msgs
}

// makePendingSpawn creates a PendingSpawn with a legacy mailbox for testing.
func makePendingSpawn(mb *mail.Mailbox, session, mailID string, age time.Duration) *PendingSpawn {
	return &PendingSpawn{
		Rig:       "gastown",
		Polecat:   "test-polecat",
		Session:   session,
		Issue:     "gt-test1",
		SpawnedAt: time.Now().Add(-age),
		MailID:    mailID,
		mailbox:   mb,
	}
}

func TestClearPendingFromList_ArchivesSingleMatch(t *testing.T) {
	dir := t.TempDir()

	msgs := []*mail.Message{
		{ID: "msg-1", Subject: "POLECAT_STARTED gastown/max", Body: "Session: gt-gastown-max\nIssue: gt-abc", Timestamp: time.Now()},
		{ID: "msg-2", Subject: "POLECAT_STARTED gastown/fury", Body: "Session: gt-gastown-fury\nIssue: gt-def", Timestamp: time.Now()},
	}
	writeInboxMessages(t, dir, msgs)

	mb := mail.NewMailbox(dir)

	pending := []*PendingSpawn{
		makePendingSpawn(mb, "gt-gastown-max", "msg-1", 0),
		makePendingSpawn(mb, "gt-gastown-fury", "msg-2", 0),
	}

	cleared, err := clearPendingFromList(pending, "gt-gastown-max")
	if err != nil {
		t.Fatalf("clearPendingFromList: %v", err)
	}
	if cleared != 1 {
		t.Fatalf("expected 1 cleared, got %d", cleared)
	}

	// Verify msg-1 was archived
	archived := readArchiveMessages(t, dir)
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived message, got %d", len(archived))
	}
	if archived[0].ID != "msg-1" {
		t.Errorf("expected archived message ID msg-1, got %s", archived[0].ID)
	}

	// Verify msg-2 still in inbox
	remaining := readInboxMessages(t, mb)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(remaining))
	}
	if remaining[0].ID != "msg-2" {
		t.Errorf("expected remaining message ID msg-2, got %s", remaining[0].ID)
	}
}

func TestClearPendingFromList_ArchivesAllMatchingSessions(t *testing.T) {
	dir := t.TempDir()

	// Two messages for the same session (duplicate spawns)
	msgs := []*mail.Message{
		{ID: "msg-1", Subject: "POLECAT_STARTED gastown/max", Body: "Session: gt-gastown-max\nIssue: gt-abc", Timestamp: time.Now().Add(-time.Minute)},
		{ID: "msg-2", Subject: "POLECAT_STARTED gastown/max", Body: "Session: gt-gastown-max\nIssue: gt-abc", Timestamp: time.Now()},
		{ID: "msg-3", Subject: "POLECAT_STARTED gastown/fury", Body: "Session: gt-gastown-fury\nIssue: gt-def", Timestamp: time.Now()},
	}
	writeInboxMessages(t, dir, msgs)

	mb := mail.NewMailbox(dir)

	pending := []*PendingSpawn{
		makePendingSpawn(mb, "gt-gastown-max", "msg-1", time.Minute),
		makePendingSpawn(mb, "gt-gastown-max", "msg-2", 0),
		makePendingSpawn(mb, "gt-gastown-fury", "msg-3", 0),
	}

	cleared, err := clearPendingFromList(pending, "gt-gastown-max")
	if err != nil {
		t.Fatalf("clearPendingFromList: %v", err)
	}
	if cleared != 2 {
		t.Fatalf("expected 2 cleared, got %d", cleared)
	}

	// Both msg-1 and msg-2 should be archived
	archived := readArchiveMessages(t, dir)
	if len(archived) != 2 {
		t.Fatalf("expected 2 archived messages, got %d", len(archived))
	}

	// msg-3 should remain
	remaining := readInboxMessages(t, mb)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(remaining))
	}
	if remaining[0].ID != "msg-3" {
		t.Errorf("expected remaining message ID msg-3, got %s", remaining[0].ID)
	}
}

func TestClearPendingFromList_IdempotentWhenNotFound(t *testing.T) {
	dir := t.TempDir()

	msgs := []*mail.Message{
		{ID: "msg-1", Subject: "POLECAT_STARTED gastown/max", Body: "Session: gt-gastown-max", Timestamp: time.Now()},
	}
	writeInboxMessages(t, dir, msgs)

	mb := mail.NewMailbox(dir)

	pending := []*PendingSpawn{
		makePendingSpawn(mb, "gt-gastown-max", "msg-1", 0),
	}

	// Clear a non-existent session - should succeed (idempotent)
	cleared, err := clearPendingFromList(pending, "gt-gastown-nonexistent")
	if err != nil {
		t.Fatalf("expected nil error for non-existent session, got: %v", err)
	}
	if cleared != 0 {
		t.Fatalf("expected 0 cleared, got %d", cleared)
	}

	// Nothing should be archived
	archived := readArchiveMessages(t, dir)
	if len(archived) != 0 {
		t.Fatalf("expected 0 archived messages, got %d", len(archived))
	}
}

func TestClearPendingFromList_NilMailboxReturnsError(t *testing.T) {
	pending := []*PendingSpawn{
		{
			Session: "gt-gastown-max",
			MailID:  "msg-1",
			mailbox: nil, // nil mailbox
		},
	}

	_, err := clearPendingFromList(pending, "gt-gastown-max")
	if err == nil {
		t.Fatal("expected error for nil mailbox, got nil")
	}
}

func TestClearPendingFromList_AlreadyArchivedIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	msgs := []*mail.Message{
		{ID: "msg-1", Subject: "POLECAT_STARTED gastown/max", Body: "Session: gt-gastown-max", Timestamp: time.Now()},
	}
	writeInboxMessages(t, dir, msgs)

	mb := mail.NewMailbox(dir)

	pending := []*PendingSpawn{
		makePendingSpawn(mb, "gt-gastown-max", "msg-1", 0),
	}

	// First clear succeeds
	cleared, err := clearPendingFromList(pending, "gt-gastown-max")
	if err != nil {
		t.Fatalf("first clear: %v", err)
	}
	if cleared != 1 {
		t.Fatalf("expected 1 cleared on first pass, got %d", cleared)
	}

	// Second clear on same pending list (simulates race: another process archived
	// between our list and archive calls). Should not error — ErrMessageNotFound
	// is treated as non-fatal.
	cleared, err = clearPendingFromList(pending, "gt-gastown-max")
	if err != nil {
		t.Fatalf("second clear (race simulation) should not error, got: %v", err)
	}
	if cleared != 0 {
		t.Fatalf("expected 0 cleared on second pass, got %d", cleared)
	}
}

func TestPruneStalePendingFromList_ArchivesOldSpawns(t *testing.T) {
	dir := t.TempDir()

	now := time.Now()
	msgs := []*mail.Message{
		{ID: "msg-old", Subject: "POLECAT_STARTED gastown/max", Body: "Session: gt-gastown-max", Timestamp: now.Add(-10 * time.Minute)},
		{ID: "msg-new", Subject: "POLECAT_STARTED gastown/fury", Body: "Session: gt-gastown-fury", Timestamp: now},
	}
	writeInboxMessages(t, dir, msgs)

	mb := mail.NewMailbox(dir)

	pending := []*PendingSpawn{
		makePendingSpawn(mb, "gt-gastown-max", "msg-old", 10*time.Minute),
		makePendingSpawn(mb, "gt-gastown-fury", "msg-new", 0),
	}

	pruned, err := pruneStalePendingFromList(pending, 5*time.Minute)
	if err != nil {
		t.Fatalf("pruneStalePendingFromList: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned, got %d", pruned)
	}

	// msg-old should be archived
	archived := readArchiveMessages(t, dir)
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived message, got %d", len(archived))
	}
	if archived[0].ID != "msg-old" {
		t.Errorf("expected archived message ID msg-old, got %s", archived[0].ID)
	}

	// msg-new should remain
	remaining := readInboxMessages(t, mb)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(remaining))
	}
	if remaining[0].ID != "msg-new" {
		t.Errorf("expected remaining message ID msg-new, got %s", remaining[0].ID)
	}
}

func TestPruneStalePendingFromList_NoPruneWhenAllFresh(t *testing.T) {
	dir := t.TempDir()

	msgs := []*mail.Message{
		{ID: "msg-1", Subject: "POLECAT_STARTED gastown/max", Body: "Session: gt-gastown-max", Timestamp: time.Now()},
	}
	writeInboxMessages(t, dir, msgs)

	mb := mail.NewMailbox(dir)

	pending := []*PendingSpawn{
		makePendingSpawn(mb, "gt-gastown-max", "msg-1", 0),
	}

	pruned, err := pruneStalePendingFromList(pending, 5*time.Minute)
	if err != nil {
		t.Fatalf("pruneStalePendingFromList: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("expected 0 pruned, got %d", pruned)
	}

	// Nothing should be archived
	archived := readArchiveMessages(t, dir)
	if len(archived) != 0 {
		t.Fatalf("expected 0 archived messages, got %d", len(archived))
	}
}

func TestPruneStalePendingFromList_NilMailboxReturnsError(t *testing.T) {
	pending := []*PendingSpawn{
		{
			SpawnedAt: time.Now().Add(-10 * time.Minute),
			MailID:    "msg-1",
			mailbox:   nil,
		},
	}

	_, err := pruneStalePendingFromList(pending, 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for nil mailbox, got nil")
	}
}

func TestPruneStalePendingFromList_AlreadyArchivedIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	msgs := []*mail.Message{
		{ID: "msg-old", Subject: "POLECAT_STARTED gastown/max", Body: "Session: gt-gastown-max", Timestamp: time.Now().Add(-10 * time.Minute)},
	}
	writeInboxMessages(t, dir, msgs)

	mb := mail.NewMailbox(dir)

	pending := []*PendingSpawn{
		makePendingSpawn(mb, "gt-gastown-max", "msg-old", 10*time.Minute),
	}

	// First prune succeeds
	pruned, err := pruneStalePendingFromList(pending, 5*time.Minute)
	if err != nil {
		t.Fatalf("first prune: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned on first pass, got %d", pruned)
	}

	// Second prune on same list (simulates race). Should not error.
	pruned, err = pruneStalePendingFromList(pending, 5*time.Minute)
	if err != nil {
		t.Fatalf("second prune (race simulation) should not error, got: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("expected 0 pruned on second pass, got %d", pruned)
	}
}
