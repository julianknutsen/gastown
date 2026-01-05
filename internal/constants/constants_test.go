package constants

import (
	"path/filepath"
	"testing"
)

func TestRigPath(t *testing.T) {
	tests := []struct {
		name     string
		townRoot string
		rigName  string
		want     string
	}{
		{
			name:     "simple paths",
			townRoot: "/home/user/gt",
			rigName:  "myrig",
			want:     filepath.Join("/home/user/gt", "myrig"),
		},
		{
			name:     "nested town root",
			townRoot: "/home/user/projects/gastown",
			rigName:  "project-a",
			want:     filepath.Join("/home/user/projects/gastown", "project-a"),
		},
		{
			name:     "rig name with dashes",
			townRoot: "/gt",
			rigName:  "my-project-rig",
			want:     filepath.Join("/gt", "my-project-rig"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RigPath(tt.townRoot, tt.rigName)
			if got != tt.want {
				t.Errorf("RigPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRigPath_PanicOnEmptyTownRoot(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("RigPath() did not panic on empty townRoot")
		}
	}()
	RigPath("", "myrig")
}

func TestRigPath_PanicOnEmptyRigName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("RigPath() did not panic on empty rigName")
		}
	}()
	RigPath("/home/user/gt", "")
}

func TestTownRootFromRig(t *testing.T) {
	tests := []struct {
		name    string
		rigPath string
		want    string
	}{
		{
			name:    "simple path",
			rigPath: "/home/user/gt/myrig",
			want:    "/home/user/gt",
		},
		{
			name:    "nested path",
			rigPath: "/home/user/projects/gastown/project-a",
			want:    "/home/user/projects/gastown",
		},
		{
			name:    "root level rig",
			rigPath: "/gt/myrig",
			want:    "/gt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TownRootFromRig(tt.rigPath)
			if got != tt.want {
				t.Errorf("TownRootFromRig() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTownRootFromRig_PanicOnEmptyRigPath(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("TownRootFromRig() did not panic on empty rigPath")
		}
	}()
	TownRootFromRig("")
}

func TestAgentClonePath(t *testing.T) {
	tests := []struct {
		name      string
		rigPath   string
		agentDir  string
		agentName string
		want      string
	}{
		{
			name:      "polecat path",
			rigPath:   "/home/user/gt/myrig",
			agentDir:  DirPolecats,
			agentName: "alpha",
			want:      filepath.Join("/home/user/gt/myrig", DirPolecats, "alpha"),
		},
		{
			name:      "crew path",
			rigPath:   "/home/user/gt/myrig",
			agentDir:  DirCrew,
			agentName: "dev",
			want:      filepath.Join("/home/user/gt/myrig", DirCrew, "dev"),
		},
		{
			name:      "polecat with numbers",
			rigPath:   "/gt/project",
			agentDir:  DirPolecats,
			agentName: "worker-01",
			want:      filepath.Join("/gt/project", DirPolecats, "worker-01"),
		},
		{
			name:      "crew with dashes",
			rigPath:   "/gt/project",
			agentDir:  DirCrew,
			agentName: "backend-team",
			want:      filepath.Join("/gt/project", DirCrew, "backend-team"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AgentClonePath(tt.rigPath, tt.agentDir, tt.agentName)
			if got != tt.want {
				t.Errorf("AgentClonePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAgentClonePath_PanicOnEmptyRigPath(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("AgentClonePath() did not panic on empty rigPath")
		}
	}()
	AgentClonePath("", DirPolecats, "alpha")
}

func TestAgentClonePath_PanicOnEmptyAgentDir(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("AgentClonePath() did not panic on empty agentDir")
		}
	}()
	AgentClonePath("/home/user/gt/myrig", "", "alpha")
}

func TestAgentClonePath_PanicOnEmptyAgentName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("AgentClonePath() did not panic on empty agentName")
		}
	}()
	AgentClonePath("/home/user/gt/myrig", DirPolecats, "")
}

// TestRoundTrip verifies that RigPath and TownRootFromRig are inverses.
func TestRoundTrip(t *testing.T) {
	townRoot := "/home/user/gt"
	rigName := "myrig"

	rigPath := RigPath(townRoot, rigName)
	recoveredTownRoot := TownRootFromRig(rigPath)

	if recoveredTownRoot != townRoot {
		t.Errorf("Round trip failed: TownRootFromRig(RigPath(%q, %q)) = %q, want %q",
			townRoot, rigName, recoveredTownRoot, townRoot)
	}
}

// TestRigPathFromAgentClone verifies extracting rig path from agent clone paths.
func TestRigPathFromAgentClone(t *testing.T) {
	rigPath := "/home/user/gt/myrig"

	// Test with polecat clone path
	polecatClonePath := AgentClonePath(rigPath, DirPolecats, "alpha")
	got := RigPathFromAgentClone(polecatClonePath)
	if got != rigPath {
		t.Errorf("RigPathFromAgentClone(polecat) = %q, want %q", got, rigPath)
	}

	// Test with crew clone path
	crewClonePath := AgentClonePath(rigPath, DirCrew, "dev")
	got = RigPathFromAgentClone(crewClonePath)
	if got != rigPath {
		t.Errorf("RigPathFromAgentClone(crew) = %q, want %q", got, rigPath)
	}
}

func TestRigPathFromAgentClone_PanicOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("RigPathFromAgentClone() did not panic on empty input")
		}
	}()
	RigPathFromAgentClone("")
}

// TestPathStructure verifies the expected directory structure.
func TestPathStructure(t *testing.T) {
	rigPath := "/home/user/gt/myrig"

	// Verify polecat path structure: rigPath/polecats/name
	polecatPath := AgentClonePath(rigPath, DirPolecats, "alpha")
	expectedPolecat := rigPath + "/" + DirPolecats + "/alpha"
	if polecatPath != expectedPolecat {
		t.Errorf("Polecat path structure wrong: got %q, want %q", polecatPath, expectedPolecat)
	}

	// Verify crew path structure: rigPath/crew/name
	crewPath := AgentClonePath(rigPath, DirCrew, "dev")
	expectedCrew := rigPath + "/" + DirCrew + "/dev"
	if crewPath != expectedCrew {
		t.Errorf("Crew path structure wrong: got %q, want %q", crewPath, expectedCrew)
	}
}

// TestMayorPaths verifies mayor-related path helpers.
func TestMayorPaths(t *testing.T) {
	townRoot := "/home/user/gt"
	rigPath := "/home/user/gt/myrig"

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"MayorRigsPath", MayorRigsPath(townRoot), townRoot + "/" + DirMayor + "/" + FileRigsJSON},
		{"MayorTownPath", MayorTownPath(townRoot), townRoot + "/" + DirMayor + "/" + FileTownJSON},
		{"MayorConfigPath", MayorConfigPath(townRoot), townRoot + "/" + DirMayor + "/" + FileConfigJSON},
		{"MayorAccountsPath", MayorAccountsPath(townRoot), townRoot + "/" + DirMayor + "/" + FileAccountsJSON},
		{"RigMayorPath", RigMayorPath(rigPath), rigPath + "/" + DirMayor + "/" + DirRig},
		{"RigBeadsPath", RigBeadsPath(rigPath), rigPath + "/" + DirMayor + "/" + DirRig + "/" + DirBeads},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestRigSubdirPaths verifies rig subdirectory path helpers.
func TestRigSubdirPaths(t *testing.T) {
	rigPath := "/home/user/gt/myrig"

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"RigPolecatsPath", RigPolecatsPath(rigPath), rigPath + "/" + DirPolecats},
		{"RigCrewPath", RigCrewPath(rigPath), rigPath + "/" + DirCrew},
		{"RigRuntimePath", RigRuntimePath(rigPath), rigPath + "/" + DirRuntime},
		{"RigSettingsPath", RigSettingsPath(rigPath), rigPath + "/" + DirSettings},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestTownRuntimePath verifies TownRuntimePath.
func TestTownRuntimePath(t *testing.T) {
	townRoot := "/home/user/gt"
	got := TownRuntimePath(townRoot)
	want := townRoot + "/" + DirRuntime
	if got != want {
		t.Errorf("TownRuntimePath() = %q, want %q", got, want)
	}
}

// TestPathHelpersPanicOnEmptyInput verifies all helpers panic on empty input.
func TestPathHelpersPanicOnEmptyInput(t *testing.T) {
	panicTests := []struct {
		name string
		fn   func()
	}{
		{"MayorRigsPath", func() { MayorRigsPath("") }},
		{"MayorTownPath", func() { MayorTownPath("") }},
		{"MayorConfigPath", func() { MayorConfigPath("") }},
		{"MayorAccountsPath", func() { MayorAccountsPath("") }},
		{"RigMayorPath", func() { RigMayorPath("") }},
		{"RigBeadsPath", func() { RigBeadsPath("") }},
		{"RigPolecatsPath", func() { RigPolecatsPath("") }},
		{"RigCrewPath", func() { RigCrewPath("") }},
		{"RigRuntimePath", func() { RigRuntimePath("") }},
		{"RigSettingsPath", func() { RigSettingsPath("") }},
		{"TownRuntimePath", func() { TownRuntimePath("") }},
	}

	for _, tt := range panicTests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("%s() did not panic on empty input", tt.name)
				}
			}()
			tt.fn()
		})
	}
}
