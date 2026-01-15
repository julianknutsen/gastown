package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMayorSessionName(t *testing.T) {
	// Mayor session name is now fixed (one per machine), uses HQ prefix
	want := "hq-mayor"
	got := MayorSessionName()
	if got != want {
		t.Errorf("MayorSessionName() = %q, want %q", got, want)
	}
}

func TestDeaconSessionName(t *testing.T) {
	// Deacon session name is now fixed (one per machine), uses HQ prefix
	want := "hq-deacon"
	got := DeaconSessionName()
	if got != want {
		t.Errorf("DeaconSessionName() = %q, want %q", got, want)
	}
}

func TestWitnessSessionName(t *testing.T) {
	tests := []struct {
		rig  string
		want string
	}{
		{"gastown", "gt-gastown-witness"},
		{"beads", "gt-beads-witness"},
		{"foo", "gt-foo-witness"},
	}
	for _, tt := range tests {
		t.Run(tt.rig, func(t *testing.T) {
			got := WitnessSessionName(tt.rig)
			if got != tt.want {
				t.Errorf("WitnessSessionName(%q) = %q, want %q", tt.rig, got, tt.want)
			}
		})
	}
}

func TestRefinerySessionName(t *testing.T) {
	tests := []struct {
		rig  string
		want string
	}{
		{"gastown", "gt-gastown-refinery"},
		{"beads", "gt-beads-refinery"},
		{"foo", "gt-foo-refinery"},
	}
	for _, tt := range tests {
		t.Run(tt.rig, func(t *testing.T) {
			got := RefinerySessionName(tt.rig)
			if got != tt.want {
				t.Errorf("RefinerySessionName(%q) = %q, want %q", tt.rig, got, tt.want)
			}
		})
	}
}

func TestCrewSessionName(t *testing.T) {
	tests := []struct {
		rig  string
		name string
		want string
	}{
		{"gastown", "max", "gt-gastown-crew-max"},
		{"beads", "alice", "gt-beads-crew-alice"},
		{"foo", "bar", "gt-foo-crew-bar"},
	}
	for _, tt := range tests {
		t.Run(tt.rig+"/"+tt.name, func(t *testing.T) {
			got := CrewSessionName(tt.rig, tt.name)
			if got != tt.want {
				t.Errorf("CrewSessionName(%q, %q) = %q, want %q", tt.rig, tt.name, got, tt.want)
			}
		})
	}
}

func TestPolecatSessionName(t *testing.T) {
	tests := []struct {
		rig  string
		name string
		want string
	}{
		{"gastown", "Toast", "gt-gastown-Toast"},
		{"gastown", "Furiosa", "gt-gastown-Furiosa"},
		{"beads", "worker1", "gt-beads-worker1"},
	}
	for _, tt := range tests {
		t.Run(tt.rig+"/"+tt.name, func(t *testing.T) {
			got := PolecatSessionName(tt.rig, tt.name)
			if got != tt.want {
				t.Errorf("PolecatSessionName(%q, %q) = %q, want %q", tt.rig, tt.name, got, tt.want)
			}
		})
	}
}

func TestPrefix(t *testing.T) {
	want := "gt-"
	if Prefix != want {
		t.Errorf("Prefix = %q, want %q", Prefix, want)
	}
}

func TestPropulsionNudgeForRole_WithSessionID(t *testing.T) {
	// Create temp directory with session_id file
	tmpDir := t.TempDir()
	runtimeDir := filepath.Join(tmpDir, ".runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		t.Fatalf("creating runtime dir: %v", err)
	}

	sessionID := "test-session-abc123"
	if err := os.WriteFile(filepath.Join(runtimeDir, "session_id"), []byte(sessionID), 0644); err != nil {
		t.Fatalf("writing session_id: %v", err)
	}

	// Test that session ID is appended
	msg := PropulsionNudgeForRole("mayor", tmpDir)
	if !strings.Contains(msg, "[session:test-session-abc123]") {
		t.Errorf("PropulsionNudgeForRole(mayor, tmpDir) = %q, should contain [session:test-session-abc123]", msg)
	}
}

func TestPropulsionNudgeForRole_WithoutSessionID(t *testing.T) {
	// Use nonexistent directory
	msg := PropulsionNudgeForRole("mayor", "/nonexistent-dir-12345")
	if strings.Contains(msg, "[session:") {
		t.Errorf("PropulsionNudgeForRole(mayor, /nonexistent) = %q, should NOT contain session ID", msg)
	}
}

func TestPropulsionNudgeForRole_EmptyWorkDir(t *testing.T) {
	// Empty workDir should not crash and should not include session ID
	msg := PropulsionNudgeForRole("mayor", "")
	if strings.Contains(msg, "[session:") {
		t.Errorf("PropulsionNudgeForRole(mayor, \"\") = %q, should NOT contain session ID", msg)
	}
}

func TestPropulsionNudgeForRole_AllRoles(t *testing.T) {
	tests := []struct {
		role     string
		contains string
	}{
		{"polecat", "gt hook"},
		{"crew", "gt hook"},
		{"witness", "gt prime"},
		{"refinery", "gt prime"},
		{"deacon", "gt prime"},
		{"mayor", "gt prime"},
		{"unknown", "gt hook"},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			msg := PropulsionNudgeForRole(tt.role, "")
			if !strings.Contains(msg, tt.contains) {
				t.Errorf("PropulsionNudgeForRole(%q, \"\") = %q, should contain %q", tt.role, msg, tt.contains)
			}
		})
	}
}

// --- Town ID tests ---

func TestTownID(t *testing.T) {
	t.Run("generates 6-char hex string", func(t *testing.T) {
		id := TownID("/home/user/gastown")
		if len(id) != TownIDLength {
			t.Errorf("TownID length = %d, want %d", len(id), TownIDLength)
		}
		// Should be valid hex
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("TownID contains non-hex char: %c", c)
			}
		}
	})

	t.Run("returns empty for empty input", func(t *testing.T) {
		id := TownID("")
		if id != "" {
			t.Errorf("TownID(\"\") = %q, want empty", id)
		}
	})

	t.Run("same input produces same ID", func(t *testing.T) {
		id1 := TownID("/home/user/gastown")
		id2 := TownID("/home/user/gastown")
		if id1 != id2 {
			t.Errorf("TownID not deterministic: %q != %q", id1, id2)
		}
	})

	t.Run("different inputs produce different IDs", func(t *testing.T) {
		id1 := TownID("/home/user/town1")
		id2 := TownID("/home/user/town2")
		if id1 == id2 {
			t.Errorf("TownID collision: %q == %q", id1, id2)
		}
	})
}

func TestExtractTownID(t *testing.T) {
	tests := []struct {
		name     string
		session  string
		expected string
	}{
		{"with town ID", "hq-mayor-abc123", "abc123"},
		{"with town ID (rig)", "gt-myrig-witness-def456", "def456"},
		{"legacy (no town ID)", "hq-mayor", ""},
		{"legacy rig session", "gt-myrig-witness", ""},
		{"invalid hex suffix", "hq-mayor-zzzzzz", ""},
		{"short suffix", "hq-mayor-abc", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTownID(tt.session)
			if got != tt.expected {
				t.Errorf("ExtractTownID(%q) = %q, want %q", tt.session, got, tt.expected)
			}
		})
	}
}

func TestStripTownID(t *testing.T) {
	tests := []struct {
		name     string
		session  string
		expected string
	}{
		{"with town ID", "hq-mayor-abc123", "hq-mayor"},
		{"with town ID (rig)", "gt-myrig-witness-def456", "gt-myrig-witness"},
		{"crew with town ID", "gt-myrig-crew-max-abc123", "gt-myrig-crew-max"},
		{"legacy (no town ID)", "hq-mayor", "hq-mayor"},
		{"invalid suffix unchanged", "hq-mayor-zzzzzz", "hq-mayor-zzzzzz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripTownID(tt.session)
			if got != tt.expected {
				t.Errorf("StripTownID(%q) = %q, want %q", tt.session, got, tt.expected)
			}
		})
	}
}

func TestMatchesTown(t *testing.T) {
	townRoot := "/home/user/gastown"
	townID := TownID(townRoot)

	tests := []struct {
		name     string
		session  string
		expected bool
	}{
		{"matching town ID", "hq-mayor-" + townID, true},
		{"different town ID", "hq-mayor-000000", false},
		{"legacy session (no ID)", "hq-mayor", true}, // Backwards compatible
		{"legacy rig session", "gt-myrig-witness", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesTown(tt.session, townRoot)
			if got != tt.expected {
				t.Errorf("MatchesTown(%q, townRoot) = %v, want %v", tt.session, got, tt.expected)
			}
		})
	}
}

// TestMultiTownFiltering tests the filtering behavior for multi-town deployments.
// This simulates how callers should filter List() results by town.
func TestMultiTownFiltering(t *testing.T) {
	town1Root := "/home/user/town1"
	town2Root := "/home/user/town2"
	town1ID := TownID(town1Root)
	town2ID := TownID(town2Root)

	// Simulate sessions from multiple towns plus legacy sessions
	allSessions := []string{
		// Town 1 sessions (new format)
		"hq-mayor-" + town1ID,
		"hq-deacon-" + town1ID,
		"gt-rig1-witness-" + town1ID,
		"gt-rig1-crew-max-" + town1ID,
		// Town 2 sessions (new format)
		"hq-mayor-" + town2ID,
		"hq-deacon-" + town2ID,
		"gt-rig2-witness-" + town2ID,
		// Legacy sessions (no suffix - backwards compatible)
		"hq-mayor",
		"gt-oldrig-witness",
	}

	t.Run("filter for town1", func(t *testing.T) {
		var filtered []string
		for _, s := range allSessions {
			if MatchesTown(s, town1Root) {
				filtered = append(filtered, s)
			}
		}

		// Should include: town1 sessions + legacy sessions
		// Should exclude: town2 sessions
		expected := 6 // 4 town1 + 2 legacy
		if len(filtered) != expected {
			t.Errorf("expected %d sessions for town1, got %d: %v", expected, len(filtered), filtered)
		}

		// Verify town2 sessions are excluded
		for _, s := range filtered {
			if ExtractTownID(s) == town2ID {
				t.Errorf("town2 session %q should not be in town1 filter", s)
			}
		}
	})

	t.Run("filter for town2", func(t *testing.T) {
		var filtered []string
		for _, s := range allSessions {
			if MatchesTown(s, town2Root) {
				filtered = append(filtered, s)
			}
		}

		// Should include: town2 sessions + legacy sessions
		// Should exclude: town1 sessions
		expected := 5 // 3 town2 + 2 legacy
		if len(filtered) != expected {
			t.Errorf("expected %d sessions for town2, got %d: %v", expected, len(filtered), filtered)
		}

		// Verify town1 sessions are excluded
		for _, s := range filtered {
			if ExtractTownID(s) == town1ID {
				t.Errorf("town1 session %q should not be in town2 filter", s)
			}
		}
	})

	t.Run("legacy sessions visible to all towns", func(t *testing.T) {
		legacySessions := []string{"hq-mayor", "gt-oldrig-witness", "gt-rig-crew-bob"}

		for _, legacy := range legacySessions {
			if !MatchesTown(legacy, town1Root) {
				t.Errorf("legacy session %q should match town1", legacy)
			}
			if !MatchesTown(legacy, town2Root) {
				t.Errorf("legacy session %q should match town2", legacy)
			}
		}
	})
}

// TestFilterSessionsByTown tests the FilterSessionsByTown helper function.
func TestFilterSessionsByTown(t *testing.T) {
	town1Root := "/home/user/town1"
	town2Root := "/home/user/town2"
	town1ID := TownID(town1Root)
	town2ID := TownID(town2Root)

	allSessions := []string{
		"hq-mayor-" + town1ID,
		"hq-mayor-" + town2ID,
		"hq-mayor", // legacy
		"gt-rig1-witness-" + town1ID,
		"gt-rig2-witness-" + town2ID,
	}

	t.Run("filters correctly for town1", func(t *testing.T) {
		filtered := FilterSessionsByTown(allSessions, town1Root)

		// Should get: town1 mayor, legacy mayor, town1 witness
		if len(filtered) != 3 {
			t.Errorf("expected 3 sessions, got %d: %v", len(filtered), filtered)
		}

		// Verify no town2 sessions
		for _, s := range filtered {
			tid := ExtractTownID(s)
			if tid != "" && tid != town1ID {
				t.Errorf("unexpected session %q in town1 filter", s)
			}
		}
	})

	t.Run("filters correctly for town2", func(t *testing.T) {
		filtered := FilterSessionsByTown(allSessions, town2Root)

		// Should get: town2 mayor, legacy mayor, town2 witness
		if len(filtered) != 3 {
			t.Errorf("expected 3 sessions, got %d: %v", len(filtered), filtered)
		}
	})

	t.Run("empty town root returns all", func(t *testing.T) {
		filtered := FilterSessionsByTown(allSessions, "")
		if len(filtered) != len(allSessions) {
			t.Errorf("empty town root should return all sessions, got %d", len(filtered))
		}
	})
}

// TestOptimisticLookup tests the optimistic-with-retry lookup pattern.
// This demonstrates how TownTerminal resolves sessions by trying
// new format (with suffix) first, then falling back to legacy.
func TestOptimisticLookup(t *testing.T) {
	townRoot := "/home/user/gastown"
	townID := TownID(townRoot)

	// Construct session names manually (TownTerminal does this automatically)
	newName := MayorSessionName() + "-" + townID
	legacyName := MayorSessionName()

	t.Run("finds new format session", func(t *testing.T) {
		existingSessions := map[string]bool{
			newName: true,
		}

		// Optimistic lookup: try new format first
		found := ""
		if existingSessions[newName] {
			found = newName
		} else if existingSessions[legacyName] {
			found = legacyName
		}

		if found != newName {
			t.Errorf("should find new format %q, got %q", newName, found)
		}
	})

	t.Run("falls back to legacy format", func(t *testing.T) {
		existingSessions := map[string]bool{
			legacyName: true,
		}

		found := ""
		if existingSessions[newName] {
			found = newName
		} else if existingSessions[legacyName] {
			found = legacyName
		}

		if found != legacyName {
			t.Errorf("should fall back to legacy %q, got %q", legacyName, found)
		}
	})

	t.Run("returns empty when neither exists", func(t *testing.T) {
		existingSessions := map[string]bool{}

		found := ""
		if existingSessions[newName] {
			found = newName
		} else if existingSessions[legacyName] {
			found = legacyName
		}

		if found != "" {
			t.Errorf("should return empty, got %q", found)
		}
	})
}

// TestCrossTownIsolation verifies that sessions from one town are never
// incorrectly matched to another town. This is critical for multi-town deployments.
func TestCrossTownIsolation(t *testing.T) {
	town1Root := "/home/user/production"
	town2Root := "/home/user/staging"
	town1ID := TownID(town1Root)
	town2ID := TownID(town2Root)

	// Verify the town IDs are different
	if town1ID == town2ID {
		t.Fatalf("test setup error: town IDs should be different, both are %q", town1ID)
	}

	t.Run("mayor sessions isolated by town", func(t *testing.T) {
		town1Mayor := "hq-mayor-" + town1ID
		town2Mayor := "hq-mayor-" + town2ID

		// Town1 should see town1Mayor, not town2Mayor
		if !MatchesTown(town1Mayor, town1Root) {
			t.Error("town1 mayor should match town1")
		}
		if MatchesTown(town2Mayor, town1Root) {
			t.Error("town2 mayor should NOT match town1")
		}

		// Town2 should see town2Mayor, not town1Mayor
		if !MatchesTown(town2Mayor, town2Root) {
			t.Error("town2 mayor should match town2")
		}
		if MatchesTown(town1Mayor, town2Root) {
			t.Error("town1 mayor should NOT match town2")
		}
	})

	t.Run("witness sessions isolated by town", func(t *testing.T) {
		// Same rig name, different towns
		town1Witness := "gt-myrig-witness-" + town1ID
		town2Witness := "gt-myrig-witness-" + town2ID

		if !MatchesTown(town1Witness, town1Root) {
			t.Error("town1 witness should match town1")
		}
		if MatchesTown(town2Witness, town1Root) {
			t.Error("town2 witness should NOT match town1 (same rig name!)")
		}
	})

	t.Run("crew sessions isolated by town", func(t *testing.T) {
		// Same rig and crew name, different towns
		town1Crew := "gt-myrig-crew-max-" + town1ID
		town2Crew := "gt-myrig-crew-max-" + town2ID

		if !MatchesTown(town1Crew, town1Root) {
			t.Error("town1 crew should match town1")
		}
		if MatchesTown(town2Crew, town1Root) {
			t.Error("town2 crew should NOT match town1 (same rig/name!)")
		}
	})

	t.Run("FilterSessionsByTown excludes other towns", func(t *testing.T) {
		allSessions := []string{
			"hq-mayor-" + town1ID,
			"hq-mayor-" + town2ID,
			"gt-rig-witness-" + town1ID,
			"gt-rig-witness-" + town2ID,
			"gt-rig-crew-max-" + town1ID,
			"gt-rig-crew-max-" + town2ID,
		}

		town1Sessions := FilterSessionsByTown(allSessions, town1Root)
		town2Sessions := FilterSessionsByTown(allSessions, town2Root)

		// Each should get exactly 3 (their own town's sessions)
		if len(town1Sessions) != 3 {
			t.Errorf("town1 should have 3 sessions, got %d: %v", len(town1Sessions), town1Sessions)
		}
		if len(town2Sessions) != 3 {
			t.Errorf("town2 should have 3 sessions, got %d: %v", len(town2Sessions), town2Sessions)
		}

		// Verify no cross-contamination
		for _, s := range town1Sessions {
			if ExtractTownID(s) == town2ID {
				t.Errorf("town1 filter returned town2 session: %s", s)
			}
		}
		for _, s := range town2Sessions {
			if ExtractTownID(s) == town1ID {
				t.Errorf("town2 filter returned town1 session: %s", s)
			}
		}
	})

	t.Run("optimistic lookup never returns wrong town", func(t *testing.T) {
		// Construct session names manually (TownTerminal does this automatically)
		town2Mayor := MayorSessionName() + "-" + town2ID

		// Simulate: only town2 has a mayor session
		existingSessions := map[string]bool{
			town2Mayor: true,
		}

		// Town1 lookup should NOT find town2's session
		town1NewName := MayorSessionName() + "-" + town1ID
		legacyName := MayorSessionName()

		found := ""
		if existingSessions[town1NewName] {
			found = town1NewName
		} else if existingSessions[legacyName] {
			found = legacyName
		}

		// Should find nothing (not town2's session)
		if found != "" {
			t.Errorf("town1 lookup should not find town2 session, got %q", found)
		}

		// But town2 lookup should find it
		if !existingSessions[town2Mayor] {
			t.Error("town2 lookup should find its own session")
		}
	})

	t.Run("explicit town ID comparison prevents collision", func(t *testing.T) {
		// Even if two towns have identical session base names,
		// the town ID suffix keeps them separate
		session1 := WitnessSessionName("shared-rig") + "-" + town1ID
		session2 := WitnessSessionName("shared-rig") + "-" + town2ID

		if session1 == session2 {
			t.Errorf("sessions should be different: %q vs %q", session1, session2)
		}

		// Extract and verify different town IDs
		tid1 := ExtractTownID(session1)
		tid2 := ExtractTownID(session2)

		if tid1 == tid2 {
			t.Errorf("town IDs should be different: %q vs %q", tid1, tid2)
		}
		if tid1 != town1ID {
			t.Errorf("session1 should have town1ID %q, got %q", town1ID, tid1)
		}
		if tid2 != town2ID {
			t.Errorf("session2 should have town2ID %q, got %q", town2ID, tid2)
		}
	})
}

// TestSessionNameRoundTrip tests that session names can be generated and parsed correctly.
// Session names with town suffix are constructed by TownTerminal (base + "-" + townID).
func TestSessionNameRoundTrip(t *testing.T) {
	townRoot := "/home/user/gastown"
	townID := TownID(townRoot)

	// Helper to add town suffix (mimics what TownTerminal does)
	withTown := func(base string) string {
		return base + "-" + townID
	}

	tests := []struct {
		name        string
		sessionName string
		wantRole    Role
		wantRig     string
		wantName    string
		wantTownID  string
	}{
		{
			name:        "mayor with town",
			sessionName: withTown(MayorSessionName()),
			wantRole:    RoleMayor,
			wantTownID:  townID,
		},
		{
			name:        "deacon with town",
			sessionName: withTown(DeaconSessionName()),
			wantRole:    RoleDeacon,
			wantTownID:  townID,
		},
		{
			name:        "witness with town",
			sessionName: withTown(WitnessSessionName("myrig")),
			wantRole:    RoleWitness,
			wantRig:     "myrig",
			wantTownID:  townID,
		},
		{
			name:        "refinery with town",
			sessionName: withTown(RefinerySessionName("myrig")),
			wantRole:    RoleRefinery,
			wantRig:     "myrig",
			wantTownID:  townID,
		},
		{
			name:        "crew with town",
			sessionName: withTown(CrewSessionName("myrig", "max")),
			wantRole:    RoleCrew,
			wantRig:     "myrig",
			wantName:    "max",
			wantTownID:  townID,
		},
		{
			name:        "polecat with town",
			sessionName: withTown(PolecatSessionName("myrig", "Toast")),
			wantRole:    RolePolecat,
			wantRig:     "myrig",
			wantName:    "Toast",
			wantTownID:  townID,
		},
		// Legacy format (no town ID)
		{
			name:        "legacy mayor",
			sessionName: MayorSessionName(),
			wantRole:    RoleMayor,
			wantTownID:  "",
		},
		{
			name:        "legacy witness",
			sessionName: WitnessSessionName("oldrig"),
			wantRole:    RoleWitness,
			wantRig:     "oldrig",
			wantTownID:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, err := ParseSessionName(tt.sessionName)
			if err != nil {
				t.Fatalf("ParseSessionName(%q) error: %v", tt.sessionName, err)
			}

			if identity.Role != tt.wantRole {
				t.Errorf("Role = %q, want %q", identity.Role, tt.wantRole)
			}
			if identity.Rig != tt.wantRig {
				t.Errorf("Rig = %q, want %q", identity.Rig, tt.wantRig)
			}
			if identity.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", identity.Name, tt.wantName)
			}
			if identity.TownID != tt.wantTownID {
				t.Errorf("TownID = %q, want %q", identity.TownID, tt.wantTownID)
			}
		})
	}
}
