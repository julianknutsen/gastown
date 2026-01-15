package session

import (
	"testing"
)

// Helper to create a test double with sessions already added.
func newTestDouble() *Double {
	return NewDouble()
}

// addSession is a helper to add a session to a test double.
func addSession(d *Double, name string) {
	_, _ = d.Start(name, "", "sleep 60")
}

// Note: TownSessions does NOT pass Session conformance tests because it intentionally
// transforms session names (appends town suffix). Code using TownSessions must use it
// consistently - don't mix TownSessions and raw Session access to the same sessions.

func TestTownSessions_Start(t *testing.T) {
	townRoot := "/home/user/gastown"
	townID := TownID(townRoot)

	t.Run("appends town suffix", func(t *testing.T) {
		double := newTestDouble()
		ts := NewTownSessions(double, townRoot)

		id, err := ts.Start("hq-mayor", "/work", "sleep 60")
		if err != nil {
			t.Fatalf("Start error: %v", err)
		}

		expected := "hq-mayor-" + townID
		if string(id) != expected {
			t.Errorf("expected %q, got %q", expected, id)
		}

		// Verify session was created with suffixed name
		exists, _ := double.Exists(SessionID(expected))
		if !exists {
			t.Error("session should exist with town suffix")
		}
	})

	t.Run("no suffix without town root", func(t *testing.T) {
		double := newTestDouble()
		ts := NewTownSessions(double, "")

		id, err := ts.Start("hq-mayor", "/work", "sleep 60")
		if err != nil {
			t.Fatalf("Start error: %v", err)
		}

		if string(id) != "hq-mayor" {
			t.Errorf("expected legacy name, got %q", id)
		}
	})
}

func TestTownSessions_Exists(t *testing.T) {
	townRoot := "/home/user/gastown"
	townID := TownID(townRoot)

	t.Run("finds new format session", func(t *testing.T) {
		double := newTestDouble()
		addSession(double, "hq-mayor-"+townID)
		ts := NewTownSessions(double, townRoot)

		exists, err := ts.Exists("hq-mayor")
		if err != nil {
			t.Fatalf("Exists error: %v", err)
		}
		if !exists {
			t.Error("should find new format session")
		}
	})

	t.Run("finds legacy session on retry", func(t *testing.T) {
		double := newTestDouble()
		addSession(double, "hq-mayor") // legacy
		ts := NewTownSessions(double, townRoot)

		exists, err := ts.Exists("hq-mayor")
		if err != nil {
			t.Fatalf("Exists error: %v", err)
		}
		if !exists {
			t.Error("should find legacy session on retry")
		}
	})

	t.Run("returns false when not found", func(t *testing.T) {
		double := newTestDouble()
		ts := NewTownSessions(double, townRoot)

		exists, err := ts.Exists("hq-mayor")
		if err != nil {
			t.Fatalf("Exists error: %v", err)
		}
		if exists {
			t.Error("should return false when not found")
		}
	})

	t.Run("does not find other town's session", func(t *testing.T) {
		otherTownID := TownID("/home/user/other")
		double := newTestDouble()
		addSession(double, "hq-mayor-"+otherTownID)
		ts := NewTownSessions(double, townRoot)

		exists, err := ts.Exists("hq-mayor")
		if err != nil {
			t.Fatalf("Exists error: %v", err)
		}
		if exists {
			t.Error("should not find other town's session")
		}
	})

	t.Run("handles pre-resolved ID with suffix", func(t *testing.T) {
		double := newTestDouble()
		addSession(double, "hq-mayor-"+townID)
		ts := NewTownSessions(double, townRoot)

		// Pass already-suffixed ID
		exists, err := ts.Exists(SessionID("hq-mayor-" + townID))
		if err != nil {
			t.Fatalf("Exists error: %v", err)
		}
		if !exists {
			t.Error("should find pre-resolved session")
		}
	})
}

func TestTownSessions_Stop(t *testing.T) {
	townRoot := "/home/user/gastown"
	townID := TownID(townRoot)

	t.Run("stops new format session", func(t *testing.T) {
		double := newTestDouble()
		addSession(double, "hq-mayor-"+townID)
		ts := NewTownSessions(double, townRoot)

		err := ts.Stop("hq-mayor")
		if err != nil {
			t.Fatalf("Stop error: %v", err)
		}

		exists, _ := double.Exists(SessionID("hq-mayor-" + townID))
		if exists {
			t.Error("session should be stopped")
		}
	})

	t.Run("stops legacy session on retry", func(t *testing.T) {
		double := newTestDouble()
		addSession(double, "hq-mayor")
		ts := NewTownSessions(double, townRoot)

		err := ts.Stop("hq-mayor")
		if err != nil {
			t.Fatalf("Stop error: %v", err)
		}

		exists, _ := double.Exists(SessionID("hq-mayor"))
		if exists {
			t.Error("legacy session should be stopped")
		}
	})

	t.Run("returns nil when not found (idempotent)", func(t *testing.T) {
		double := newTestDouble()
		ts := NewTownSessions(double, townRoot)

		err := ts.Stop("hq-mayor")
		if err != nil {
			t.Errorf("Stop should be idempotent, got error: %v", err)
		}
	})
}

func TestTownSessions_List(t *testing.T) {
	town1Root := "/home/user/town1"
	town2Root := "/home/user/town2"
	town1ID := TownID(town1Root)
	town2ID := TownID(town2Root)

	t.Run("filters by town", func(t *testing.T) {
		double := newTestDouble()
		addSession(double, "hq-mayor-"+town1ID)
		addSession(double, "hq-mayor-"+town2ID)
		addSession(double, "hq-deacon-"+town1ID)
		addSession(double, "hq-mayor") // legacy

		ts := NewTownSessions(double, town1Root)
		sessions, err := ts.List()
		if err != nil {
			t.Fatalf("List error: %v", err)
		}

		// Should include: town1 mayor, town1 deacon, legacy mayor
		// Should exclude: town2 mayor
		if len(sessions) != 3 {
			t.Errorf("expected 3 sessions, got %d", len(sessions))
		}

		for _, id := range sessions {
			tid := ExtractTownID(string(id))
			if tid != "" && tid != town1ID {
				t.Errorf("should not include other town's session: %s", id)
			}
		}
	})

	t.Run("returns all without town root", func(t *testing.T) {
		double := newTestDouble()
		addSession(double, "hq-mayor-"+town1ID)
		addSession(double, "hq-mayor-"+town2ID)
		addSession(double, "hq-mayor")

		ts := NewTownSessions(double, "")
		sessions, err := ts.List()
		if err != nil {
			t.Fatalf("List error: %v", err)
		}

		if len(sessions) != 3 {
			t.Errorf("expected all 3 sessions, got %d", len(sessions))
		}
	})
}

func TestTownSessions_ListAll(t *testing.T) {
	town1Root := "/home/user/town1"
	town2Root := "/home/user/town2"
	town1ID := TownID(town1Root)
	town2ID := TownID(town2Root)

	double := newTestDouble()
	addSession(double, "hq-mayor-"+town1ID)
	addSession(double, "hq-mayor-"+town2ID)
	addSession(double, "hq-deacon-"+town1ID)

	ts := NewTownSessions(double, town1Root)

	t.Run("List filters by town", func(t *testing.T) {
		sessions, err := ts.List()
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		// Should only have town1 sessions
		if len(sessions) != 2 {
			t.Errorf("expected 2 town1 sessions, got %d", len(sessions))
		}
	})

	t.Run("ListAll returns all sessions", func(t *testing.T) {
		sessions, err := ts.ListAll()
		if err != nil {
			t.Fatalf("ListAll error: %v", err)
		}
		// Should have all 3 sessions
		if len(sessions) != 3 {
			t.Errorf("expected 3 total sessions, got %d", len(sessions))
		}
	})
}

func TestTownSessions_Send(t *testing.T) {
	townRoot := "/home/user/gastown"
	townID := TownID(townRoot)

	t.Run("resolves session before send", func(t *testing.T) {
		double := newTestDouble()
		addSession(double, "hq-mayor-"+townID)
		ts := NewTownSessions(double, townRoot)

		err := ts.Send("hq-mayor", "hello")
		if err != nil {
			t.Fatalf("Send error: %v", err)
		}
	})

	t.Run("errors when session not found", func(t *testing.T) {
		double := newTestDouble()
		ts := NewTownSessions(double, townRoot)

		err := ts.Send("hq-mayor", "hello")
		if err == nil {
			t.Error("expected error when session not found")
		}
	})
}

func TestTownSessions_CrossTownIsolation(t *testing.T) {
	town1Root := "/home/user/production"
	town2Root := "/home/user/staging"
	town1ID := TownID(town1Root)
	town2ID := TownID(town2Root)

	double := newTestDouble()
	addSession(double, "hq-mayor-"+town1ID)
	addSession(double, "hq-mayor-"+town2ID)
	addSession(double, "gt-rig-witness-"+town1ID)
	addSession(double, "gt-rig-witness-"+town2ID)

	ts1 := NewTownSessions(double, town1Root)
	ts2 := NewTownSessions(double, town2Root)

	t.Run("town1 only sees town1 sessions", func(t *testing.T) {
		sessions, _ := ts1.List()
		for _, id := range sessions {
			tid := ExtractTownID(string(id))
			if tid == town2ID {
				t.Errorf("town1 should not see town2 session: %s", id)
			}
		}
	})

	t.Run("town2 only sees town2 sessions", func(t *testing.T) {
		sessions, _ := ts2.List()
		for _, id := range sessions {
			tid := ExtractTownID(string(id))
			if tid == town1ID {
				t.Errorf("town2 should not see town1 session: %s", id)
			}
		}
	})

	t.Run("town1 cannot stop town2 session", func(t *testing.T) {
		// town1 tries to stop "hq-mayor" but only town2's exists with that base
		// This should fail to find it (town1's doesn't exist, legacy doesn't exist)
		double2 := newTestDouble()
		addSession(double2, "hq-mayor-"+town2ID) // only town2

		ts := NewTownSessions(double2, town1Root)
		err := ts.Stop("hq-mayor")
		// Should be idempotent (returns nil) but session should still exist
		if err != nil {
			t.Errorf("Stop should be idempotent: %v", err)
		}
		exists, _ := double2.Exists(SessionID("hq-mayor-" + town2ID))
		if !exists {
			t.Error("town2 session should still exist")
		}
	})
}

func TestTownSessions_ImplementsInterface(t *testing.T) {
	// Compile-time check that TownSessions implements Sessions
	var _ Sessions = (*TownSessions)(nil)
}

func TestTownSessions_StartThenFind(t *testing.T) {
	townRoot := "/home/user/gastown"
	double := newTestDouble()
	ts := NewTownSessions(double, townRoot)

	// Start a session
	id, err := ts.Start("hq-mayor", "/work", "sleep 60")
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Should be able to find it with logical name
	exists, err := ts.Exists("hq-mayor")
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Error("should find started session")
	}

	// Should also be able to find it with returned ID
	exists, err = ts.Exists(id)
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Error("should find session by returned ID")
	}
}

// TestTownSessions_WithSessionNames verifies TownSessions works with session name functions.
// This is the intended usage pattern: callers use session.XxxSessionName() to get logical names.
func TestTownSessions_WithSessionNames(t *testing.T) {
	townRoot := "/home/user/gastown"
	double := newTestDouble()
	ts := NewTownSessions(double, townRoot)

	t.Run("create and find mayor session", func(t *testing.T) {
		// Caller uses session name function to get logical name
		mayorName := MayorSessionName()
		id, err := ts.Start(mayorName, "/work", "sleep 60")
		if err != nil {
			t.Fatalf("Start error: %v", err)
		}

		// Can find using logical name
		exists, err := ts.Exists(SessionID(mayorName))
		if err != nil {
			t.Fatalf("Exists error: %v", err)
		}
		if !exists {
			t.Error("should find mayor session by logical name")
		}

		// Can stop using logical name
		err = ts.Stop(SessionID(mayorName))
		if err != nil {
			t.Fatalf("Stop error: %v", err)
		}

		// Verify stopped
		exists, _ = ts.Exists(id)
		if exists {
			t.Error("session should be stopped")
		}
	})

	t.Run("create and find witness session", func(t *testing.T) {
		rig := "testrig"
		witnessName := WitnessSessionName(rig)
		_, err := ts.Start(witnessName, "/work", "sleep 60")
		if err != nil {
			t.Fatalf("Start error: %v", err)
		}

		exists, err := ts.Exists(SessionID(witnessName))
		if err != nil {
			t.Fatalf("Exists error: %v", err)
		}
		if !exists {
			t.Error("should find witness session")
		}
	})

	t.Run("create and find crew session", func(t *testing.T) {
		rig := "testrig"
		name := "worker1"
		crewName := CrewSessionName(rig, name)
		_, err := ts.Start(crewName, "/work", "sleep 60")
		if err != nil {
			t.Fatalf("Start error: %v", err)
		}

		exists, err := ts.Exists(SessionID(crewName))
		if err != nil {
			t.Fatalf("Exists error: %v", err)
		}
		if !exists {
			t.Error("should find crew session")
		}
	})
}
