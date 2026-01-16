package beads_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// TestTarget represents an implementation to test against.
type TestTarget struct {
	Name    string
	Factory func(t *testing.T, env *TestEnv) beads.BeadsOps
	IsRawBd bool // True for raw bd without workarounds
}

// TestContext represents a testing context (working directory, target prefix).
type TestContext struct {
	Name       string
	IsCrossRig bool   // Whether target bead is in different rig than workDir
	Setup      func(t *testing.T, env *TestEnv, ops beads.BeadsOps) string // Returns target bead ID
}

// TestEnv holds the test environment.
type TestEnv struct {
	TownRoot   string
	GastownDir string
	AIPlatDir  string
	TestHome   string
}

// setupTestEnv creates a test environment with multiple rigs.
func setupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	townRoot := t.TempDir()
	testHome := t.TempDir()

	// Create mayor/town.json marker
	mayorDir := filepath.Join(townRoot, "mayor")
	_ = os.MkdirAll(mayorDir, 0755)
	_ = os.WriteFile(filepath.Join(mayorDir, "town.json"), []byte(`{"name": "test-town"}`), 0644)

	// Create rig directories with .beads
	gastownBeads := filepath.Join(townRoot, "gastown", "mayor", "rig", ".beads")
	aiplatBeads := filepath.Join(townRoot, "ai_platform", "mayor", "rig", ".beads")
	townBeads := filepath.Join(townRoot, ".beads")

	_ = os.MkdirAll(gastownBeads, 0755)
	_ = os.MkdirAll(aiplatBeads, 0755)
	_ = os.MkdirAll(townBeads, 0755)

	// Create redirect files for rig roots
	gastownRigBeads := filepath.Join(townRoot, "gastown", ".beads")
	aiplatRigBeads := filepath.Join(townRoot, "ai_platform", ".beads")
	_ = os.MkdirAll(gastownRigBeads, 0755)
	_ = os.MkdirAll(aiplatRigBeads, 0755)
	_ = os.WriteFile(filepath.Join(gastownRigBeads, "redirect"), []byte("mayor/rig/.beads\n"), 0644)
	_ = os.WriteFile(filepath.Join(aiplatRigBeads, "redirect"), []byte("mayor/rig/.beads\n"), 0644)

	// Create empty issues.jsonl files
	_ = os.WriteFile(filepath.Join(townBeads, "issues.jsonl"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(gastownBeads, "issues.jsonl"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(aiplatBeads, "issues.jsonl"), []byte{}, 0644)

	// Write routes.jsonl
	routes := `{"prefix": "gt-", "path": "gastown/mayor/rig"}
{"prefix": "ap-", "path": "ai_platform/mayor/rig"}
{"prefix": "hq-", "path": "."}
`
	_ = os.WriteFile(filepath.Join(townBeads, "routes.jsonl"), []byte(routes), 0644)

	return &TestEnv{
		TownRoot:   townRoot,
		GastownDir: filepath.Join(townRoot, "gastown", "mayor", "rig"),
		AIPlatDir:  filepath.Join(townRoot, "ai_platform", "mayor", "rig"),
		TestHome:   testHome,
	}
}

// isBdInstalled checks if bd CLI is available.
func isBdInstalled() bool {
	_, err := exec.LookPath("bd")
	return err == nil
}

// getTestTargets returns the implementations to test against.
func getTestTargets(t *testing.T, env *TestEnv) []TestTarget {
	t.Helper()

	targets := []TestTarget{
		{
			Name: "Double",
			Factory: func(t *testing.T, env *TestEnv) beads.BeadsOps {
				d := beads.NewDoubleWithPrefix("gt")
				d.AddDatabase("ap")
				d.AddDatabase("hq")
				return d
			},
			IsRawBd: false,
		},
	}

	// Only add Implementation and RawBd if bd is installed
	if isBdInstalled() {
		targets = append(targets,
			TestTarget{
				Name: "Implementation",
				Factory: func(t *testing.T, env *TestEnv) beads.BeadsOps {
					initRealBd(t, env)
					return beads.NewWithTownRoot(env.GastownDir, env.TownRoot)
				},
				IsRawBd: false,
			},
			TestTarget{
				Name: "RawBd",
				Factory: func(t *testing.T, env *TestEnv) beads.BeadsOps {
					initRealBd(t, env)
					// TrueRawBdOps calls bd directly without BEADS_DIR or routing workarounds
					return &TrueRawBdOps{workDir: env.GastownDir, testHome: env.TestHome}
				},
				IsRawBd: true,
			},
		)
	}

	return targets
}

// initRealBd initializes real bd databases.
func initRealBd(t *testing.T, env *TestEnv) {
	t.Helper()

	// Set HOME to isolate planning database
	t.Setenv("HOME", env.TestHome)

	// Initialize each database with its correct prefix
	// Must match routes.jsonl: gt- -> gastown, ap- -> ai_platform, hq- -> townRoot
	dbConfigs := []struct {
		dir    string
		prefix string
	}{
		{env.TownRoot, "hq"},
		{env.GastownDir, "gt"},
		{env.AIPlatDir, "ap"},
	}

	for _, cfg := range dbConfigs {
		cmd := exec.Command("bd", "init", "--quiet", "--prefix", cfg.prefix)
		cmd.Dir = cfg.dir
		cmd.Env = append(os.Environ(), "HOME="+env.TestHome)
		_ = cmd.Run() // Ignore errors if already initialized
	}
}

// getTestContexts returns the contexts to test in.
func getTestContexts() []TestContext {
	return []TestContext{
		{
			Name:       "same-rig",
			IsCrossRig: false,
			Setup: func(t *testing.T, env *TestEnv, ops beads.BeadsOps) string {
				// Create a bead in the current rig (gt)
				issue, err := ops.Create(beads.CreateOptions{
					Title: "Same-rig target",
					Type:  "task",
				})
				if err != nil {
					t.Fatalf("Setup Create failed: %v", err)
				}
				return issue.ID // Will be gt-X
			},
		},
		{
			Name:       "cross-rig-gt-to-ap",
			IsCrossRig: true,
			Setup: func(t *testing.T, env *TestEnv, ops beads.BeadsOps) string {
				// For Double: create in ap database, but we're working from gt
				if d, ok := ops.(*beads.Double); ok {
					d.SetCurrentPrefix("ap")
					issue, err := d.Create(beads.CreateOptions{
						Title: "Cross-rig target",
						Type:  "task",
					})
					if err != nil {
						t.Fatalf("Setup Create failed: %v", err)
					}
					d.SetCurrentPrefix("gt") // Switch back to gt
					return issue.ID          // Will be ap-X
				}

				// For real bd: create in ai_platform directory
				apOps := beads.New(env.AIPlatDir)
				issue, err := apOps.Create(beads.CreateOptions{
					Title: "Cross-rig target",
					Type:  "task",
				})
				if err != nil {
					t.Fatalf("Setup Create failed: %v", err)
				}
				return issue.ID // Will be ap-X
			},
		},
	}
}

// ConformanceTest defines a conformance test case for ID-based operations.
// The Test function returns an error if the test fails, nil if it passes.
// This allows us to probe for expected failures without failing the test.
type ConformanceTest struct {
	Name      string
	Operation string                                          // Operation name for BdRoutingBugs lookup
	Test      func(ops beads.BeadsOps, targetID string) error // Returns nil on success, error on failure
}

// SimpleConformanceTest defines a test case for non-ID operations.
// These don't need cross-rig testing since they don't route by bead ID.
type SimpleConformanceTest struct {
	Name      string
	Operation string
	Test      func(ops beads.BeadsOps) error // Returns nil on success, error on failure
}

// RunSimpleConformanceTest runs a test across all targets (no cross-rig contexts).
// Use this for operations that don't take a bead ID (Config, Daemon, Version, etc.).
func RunSimpleConformanceTest(t *testing.T, st SimpleConformanceTest) {
	// Get target names first (need an env to get targets)
	tmpEnv := setupTestEnv(t)
	targetTemplates := getTestTargets(t, tmpEnv)

	for _, target := range targetTemplates {
		t.Run(target.Name, func(t *testing.T) {
			// Each target gets a fresh environment to avoid cross-contamination
			env := setupTestEnv(t)
			// Re-get targets with fresh env
			targets := getTestTargets(t, env)
			var ops beads.BeadsOps
			for _, tgt := range targets {
				if tgt.Name == target.Name {
					ops = tgt.Factory(t, env)
					break
				}
			}

			// For RawBd, check if this operation has known bugs
			isKnownBug := target.IsRawBd && !beads.IsBdFixed(st.Operation)

			err := st.Test(ops)

			if isKnownBug {
				if err == nil {
					t.Logf("Note: RawBd %s passed (may have been fixed)", st.Operation)
				} else {
					t.Logf("Verified: RawBd %s bug exists (%v)", st.Operation, err)
				}
			} else {
				if err != nil {
					t.Errorf("%s failed: %v", st.Operation, err)
				}
			}
		})
	}
}

// RunConformanceTest runs a test across all targets and contexts.
func RunConformanceTest(t *testing.T, ct ConformanceTest) {
	// Get target and context names first
	tmpEnv := setupTestEnv(t)
	targetTemplates := getTestTargets(t, tmpEnv)
	contexts := getTestContexts()

	for _, target := range targetTemplates {
		for _, ctx := range contexts {
			name := target.Name + "/" + ctx.Name
			t.Run(name, func(t *testing.T) {
				// Each target/context gets a fresh environment
				env := setupTestEnv(t)
				targets := getTestTargets(t, env)
				var ops beads.BeadsOps
				for _, tgt := range targets {
					if tgt.Name == target.Name {
						ops = tgt.Factory(t, env)
						break
					}
				}
				targetID := ctx.Setup(t, env, ops)

				// Check if this is a known-bug scenario
				// BdRoutingBugs tracks routing bugs that only manifest in cross-rig scenarios.
				// Raw bd could also have same-rig bugs, but those would be tracked separately.
				isKnownBug := target.IsRawBd && !beads.IsBdFixed(ct.Operation)

				err := ct.Test(ops, targetID)

				if isKnownBug {
					// For known bugs, we expect failure (especially in cross-rig scenarios).
					// Log the result but don't fail the test.
					if err == nil {
						// Test passed - if cross-rig, bd may have been fixed!
						if ctx.IsCrossRig {
							t.Logf("WARNING: bd may have fixed %s routing! "+
								"If confirmed, update BdRoutingBugs[\"%s\"] = true and "+
								"remove the workaround from Implementation.", ct.Operation, ct.Operation)
						}
						// Same-rig pass is expected even for operations with routing bugs
					} else {
						if ctx.IsCrossRig {
							// Expected failure - routing bug still exists
							t.Logf("Verified: bd routing bug for %s still exists (%v)", ct.Operation, err)
						} else {
							// Same-rig failure is unexpected - this is a real bug
							t.Errorf("RawBd %s failed in same-rig context: %v", ct.Operation, err)
						}
					}
				} else {
					// Double, Implementation, or same-rig with fixed operation = must pass
					if err != nil {
						t.Errorf("%s failed: %v", ct.Operation, err)
					}
				}
			})
		}
	}
}

// =============================================================================
// TrueRawBdOps - Direct bd CLI calls without workarounds
// =============================================================================

// TrueRawBdOps wraps bd commands WITHOUT setting BEADS_DIR or other env vars.
// This tests bd's native routing behavior.
type TrueRawBdOps struct {
	workDir  string
	testHome string
}

func (r *TrueRawBdOps) run(args ...string) ([]byte, error) {
	cmd := exec.Command("bd", args...)
	cmd.Dir = r.workDir
	// Only set HOME to isolate planning db, but NOT BEADS_DIR
	cmd.Env = append(os.Environ(), "HOME="+r.testHome)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("bd %v: %s", args, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func (r *TrueRawBdOps) Create(opts beads.CreateOptions) (*beads.Issue, error) {
	args := []string{"create", "--json"}
	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}
	if opts.Type != "" {
		args = append(args, "--labels=gt:"+opts.Type)
	}
	if opts.Priority >= 0 {
		args = append(args, fmt.Sprintf("--priority=%d", opts.Priority))
	}
	if opts.Description != "" {
		args = append(args, "--description="+opts.Description)
	}

	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}

	var issue beads.Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing create output: %w", err)
	}
	return &issue, nil
}

func (r *TrueRawBdOps) Show(id string) (*beads.Issue, error) {
	out, err := r.run("show", id, "--json")
	if err != nil {
		// bd returns error when issue not found
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Error:") {
			return nil, beads.ErrNotFound
		}
		return nil, err
	}

	// Handle empty output
	if len(out) == 0 || string(out) == "null" || string(out) == "[]" {
		return nil, beads.ErrNotFound
	}

	var issues []*beads.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing show output: %w", err)
	}
	if len(issues) == 0 {
		return nil, beads.ErrNotFound
	}
	return issues[0], nil
}

func (r *TrueRawBdOps) Update(id string, opts beads.UpdateOptions) error {
	args := []string{"update", id}
	if opts.Title != nil {
		args = append(args, "--title="+*opts.Title)
	}
	if opts.Priority != nil {
		args = append(args, fmt.Sprintf("--priority=%d", *opts.Priority))
	}
	if opts.Description != nil {
		args = append(args, "--description="+*opts.Description)
	}
	// Label operations
	for _, label := range opts.AddLabels {
		args = append(args, "--add-label="+label)
	}
	for _, label := range opts.RemoveLabels {
		args = append(args, "--remove-label="+label)
	}
	_, err := r.run(args...)
	return err
}

func (r *TrueRawBdOps) Close(ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	args := append([]string{"close"}, ids...)
	_, err := r.run(args...)
	return err
}

func (r *TrueRawBdOps) LabelAdd(id, label string) error {
	_, err := r.run("label", "add", id, label)
	return err
}

func (r *TrueRawBdOps) ShowMultiple(ids []string) (map[string]*beads.Issue, error) {
	if len(ids) == 0 {
		return map[string]*beads.Issue{}, nil
	}
	args := append([]string{"show", "--json"}, ids...)
	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}

	var issues []*beads.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing show output: %w", err)
	}

	result := make(map[string]*beads.Issue)
	for _, issue := range issues {
		result[issue.ID] = issue
	}
	return result, nil
}

func (r *TrueRawBdOps) CloseWithReason(reason string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	args := []string{"close", "--reason=" + reason}
	args = append(args, ids...)
	_, err := r.run(args...)
	return err
}

func (r *TrueRawBdOps) CloseWithOptions(opts beads.CloseOptions, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	args := []string{"close"}
	args = append(args, ids...)
	if opts.Reason != "" {
		args = append(args, "--reason="+opts.Reason)
	}
	if opts.Session != "" {
		args = append(args, "--session="+opts.Session)
	}
	_, err := r.run(args...)
	return err
}

func (r *TrueRawBdOps) AgentState(beadID, state string) error {
	_, err := r.run("agent", "state", beadID, state)
	return err
}

func (r *TrueRawBdOps) List(opts beads.ListOptions) ([]*beads.Issue, error) {
	args := []string{"list", "--json"}
	if opts.Status != "" {
		args = append(args, "--status="+opts.Status)
	}
	if opts.Label != "" {
		args = append(args, "--label="+opts.Label)
	}
	if opts.Priority >= 0 {
		args = append(args, fmt.Sprintf("--priority=%d", opts.Priority))
	}
	if opts.Limit > 0 {
		args = append(args, fmt.Sprintf("--limit=%d", opts.Limit))
	}
	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}
	var issues []*beads.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing list output: %w", err)
	}
	return issues, nil
}

func (r *TrueRawBdOps) CreateWithID(id string, opts beads.CreateOptions) (*beads.Issue, error) {
	args := []string{"create", "--json", "--id=" + id}
	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}
	if opts.Type != "" {
		args = append(args, "--labels=gt:"+opts.Type)
	}
	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}
	var issue beads.Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing create output: %w", err)
	}
	return &issue, nil
}

func (r *TrueRawBdOps) Delete(ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	// Use --hard to create invisible tombstones (so Show returns ErrNotFound)
	// Note: Due to a bd bug, --hard still creates tombstones instead of truly
	// deleting, but at least they're invisible to bd show/reopen.
	args := []string{"delete", "--force", "--hard"}
	args = append(args, ids...)
	_, err := r.run(args...)
	return err
}

func (r *TrueRawBdOps) Reopen(id string) error {
	_, err := r.run("reopen", id)
	return err
}

func (r *TrueRawBdOps) AddDependency(issue, dependsOn string) error {
	_, err := r.run("dep", "add", issue, dependsOn)
	return err
}

func (r *TrueRawBdOps) AddDependencyWithType(issue, dependsOn, depType string) error {
	_, err := r.run("dep", "add", issue, dependsOn, "--type="+depType)
	return err
}

func (r *TrueRawBdOps) RemoveDependency(issue, dependsOn string) error {
	_, err := r.run("dep", "remove", issue, dependsOn)
	return err
}

func (r *TrueRawBdOps) Ready() ([]*beads.Issue, error) {
	out, err := r.run("ready", "--json")
	if err != nil {
		return nil, err
	}
	var issues []*beads.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing ready output: %w", err)
	}
	return issues, nil
}

func (r *TrueRawBdOps) ReadyWithLabel(label string, limit int) ([]*beads.Issue, error) {
	args := []string{"ready", "--json", "--label=" + label}
	if limit > 0 {
		args = append(args, fmt.Sprintf("--limit=%d", limit))
	}
	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}
	var issues []*beads.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing ready output: %w", err)
	}
	return issues, nil
}

func (r *TrueRawBdOps) Blocked() ([]*beads.Issue, error) {
	out, err := r.run("blocked", "--json")
	if err != nil {
		return nil, err
	}
	var issues []*beads.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing blocked output: %w", err)
	}
	return issues, nil
}

func (r *TrueRawBdOps) Sync() error {
	_, err := r.run("sync")
	return err
}

func (r *TrueRawBdOps) SyncFromMain() error {
	_, err := r.run("sync", "--from-main")
	return err
}

func (r *TrueRawBdOps) SyncImportOnly() error {
	_, err := r.run("sync", "--import-only")
	return err
}

func (r *TrueRawBdOps) GetSyncStatus() (*beads.SyncStatus, error) {
	// bd sync --status returns sync status
	out, err := r.run("sync", "--status", "--json")
	if err != nil {
		// If sync status fails, return empty status (not an error)
		return &beads.SyncStatus{}, nil
	}
	var status beads.SyncStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return &beads.SyncStatus{}, nil
	}
	return &status, nil
}

func (r *TrueRawBdOps) Init(opts beads.InitOptions) error {
	args := []string{"init"}
	if opts.Prefix != "" {
		args = append(args, "--prefix="+opts.Prefix)
	}
	if opts.Quiet {
		args = append(args, "--quiet")
	}
	_, err := r.run(args...)
	return err
}

func (r *TrueRawBdOps) Migrate(opts beads.MigrateOptions) error {
	args := []string{"migrate"}
	if opts.UpdateRepoID {
		args = append(args, "--update-repo-id")
	}
	if opts.Yes {
		args = append(args, "--yes")
	}
	_, err := r.run(args...)
	return err
}

func (r *TrueRawBdOps) ConfigGet(key string) (string, error) {
	out, err := r.run("config", "get", key)
	if err != nil {
		return "", nil // Return empty for missing keys
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *TrueRawBdOps) ConfigSet(key, value string) error {
	_, err := r.run("config", "set", key, value)
	return err
}

func (r *TrueRawBdOps) DaemonStart() error {
	_, err := r.run("daemon", "start")
	return err
}

func (r *TrueRawBdOps) DaemonStop() error {
	_, err := r.run("daemon", "stop")
	return err
}

func (r *TrueRawBdOps) DaemonStatus() (*beads.DaemonStatus, error) {
	out, err := r.run("daemon", "status", "--json")
	if err != nil {
		// Daemon not running is not an error
		return &beads.DaemonStatus{Running: false}, nil
	}
	var status beads.DaemonStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return &beads.DaemonStatus{Running: false}, nil
	}
	return &status, nil
}

func (r *TrueRawBdOps) DaemonHealth() (*beads.DaemonHealth, error) {
	out, err := r.run("daemon", "health", "--json")
	if err != nil {
		return &beads.DaemonHealth{}, nil
	}
	var health beads.DaemonHealth
	if err := json.Unmarshal(out, &health); err != nil {
		return &beads.DaemonHealth{}, nil
	}
	return &health, nil
}

func (r *TrueRawBdOps) Version() (string, error) {
	out, err := r.run("version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *TrueRawBdOps) Stats() (string, error) {
	out, err := r.run("stats")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (r *TrueRawBdOps) StatsJSON() (*beads.RepoStats, error) {
	out, err := r.run("stats", "--json")
	if err != nil {
		return nil, err
	}
	var stats beads.RepoStats
	if err := json.Unmarshal(out, &stats); err != nil {
		return nil, fmt.Errorf("parsing stats output: %w", err)
	}
	return &stats, nil
}

func (r *TrueRawBdOps) LabelRemove(id, label string) error {
	_, err := r.run("label", "remove", id, label)
	return err
}

func (r *TrueRawBdOps) Comment(id, message string) error {
	_, err := r.run("comment", id, message)
	return err
}

// --- Stub implementations for less commonly tested operations ---
func (r *TrueRawBdOps) MolSeed(opts beads.MolSeedOptions) error                               { return nil }
func (r *TrueRawBdOps) MolCurrent(moleculeID string) (*beads.MolCurrentOutput, error)         { return nil, nil }
func (r *TrueRawBdOps) MolCatalog() ([]*beads.MoleculeProto, error)                           { return []*beads.MoleculeProto{}, nil }
func (r *TrueRawBdOps) WispCreate(protoID, actor string) (*beads.Issue, error)                { return nil, nil }
func (r *TrueRawBdOps) WispCreateWithOptions(opts beads.WispCreateOptions) (*beads.Issue, error) { return nil, nil }
func (r *TrueRawBdOps) WispList(all bool) ([]*beads.Issue, error)                             { return []*beads.Issue{}, nil }
func (r *TrueRawBdOps) WispGC() error                                                         { return nil }
func (r *TrueRawBdOps) MolBond(wispID, beadID string) (*beads.Issue, error)                   { return nil, nil }
func (r *TrueRawBdOps) GateShow(gateID string) (*beads.Gate, error)                           { return nil, nil }
func (r *TrueRawBdOps) GateWait(gateID, notifyAgent string) error                             { return nil }
func (r *TrueRawBdOps) GateList(all bool) ([]*beads.Gate, error)                              { return []*beads.Gate{}, nil }
func (r *TrueRawBdOps) GateResolve(gateID string) error                                       { return nil }
func (r *TrueRawBdOps) GateAddWaiter(gateID, waiterID string) error                           { return nil }
func (r *TrueRawBdOps) GateCheck() error                                                      { return nil }
func (r *TrueRawBdOps) SwarmStatus(swarmID string) (*beads.SwarmStatus, error)                { return nil, nil }
func (r *TrueRawBdOps) SwarmCreate(epicID string) (*beads.Issue, error)                       { return nil, nil }
func (r *TrueRawBdOps) SwarmList() ([]*beads.Issue, error)                                    { return []*beads.Issue{}, nil }
func (r *TrueRawBdOps) SwarmValidate(epicID string) error                                     { return nil }
func (r *TrueRawBdOps) FormulaShow(name string) (*beads.Formula, error)                       { return nil, nil }
func (r *TrueRawBdOps) FormulaList() ([]*beads.Formula, error)                                { return []*beads.Formula{}, nil }
func (r *TrueRawBdOps) Cook(formulaName string) (*beads.Issue, error)                         { return nil, nil }
func (r *TrueRawBdOps) LegAdd(formulaID, stepName string) error                               { return nil }
func (r *TrueRawBdOps) SlotShow(id string) (*beads.Slot, error)                               { return nil, nil }
func (r *TrueRawBdOps) SlotSet(agentID, slotName, beadID string) error                        { return nil }
func (r *TrueRawBdOps) SlotClear(agentID, slotName string) error                              { return nil }
func (r *TrueRawBdOps) Search(query string, opts beads.SearchOptions) ([]*beads.Issue, error) { return []*beads.Issue{}, nil }
func (r *TrueRawBdOps) Doctor() (*beads.DoctorReport, error)                                  { return &beads.DoctorReport{Status: "ok"}, nil }
func (r *TrueRawBdOps) Prime() (string, error)                                                { return "", nil }
func (r *TrueRawBdOps) Flush() error                                                          { return nil }
func (r *TrueRawBdOps) Burn(opts beads.BurnOptions) error                                     { return nil }
func (r *TrueRawBdOps) IsBeadsRepo() bool                                                     { return true }
func (r *TrueRawBdOps) Run(args ...string) ([]byte, error)                                    { return r.run(args...) }
