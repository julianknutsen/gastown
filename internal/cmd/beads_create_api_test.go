//go:build integration

// Package cmd contains integration tests for the beads create APIs.
//
// These tests verify the new unified Beads constructor and create methods:
// - New(townRoot) and New(townRoot, rigPath) constructors
// - CreateRigAgent, CreateOrReopenRigAgent
// - CreateTownAgent, CreateTownConvoy
// - CreateRigIdentity
// - Create with explicit ID routing
//
// Run with: go test -tags=integration ./internal/cmd -run TestBeadsCreateAPI -v
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// setupTestEnv creates an isolated test environment.
func setupTestEnv(t *testing.T) (townRoot, rigPath string) {
	t.Helper()
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed")
	}
	gtBinary := buildGT(t)
	townRoot = setupCreateAPITestTown(t, gtBinary)
	rigPath = filepath.Join(townRoot, "testrig")
	return
}

// Each test gets its own isolated town. Go limits parallelism to GOMAXPROCS.

func TestBeadsCreateAPI_Constructor_TownOnly(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testNewTownOnlyConstructor(t, townRoot)
}

func TestBeadsCreateAPI_Constructor_WithRig(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testNewWithRigConstructor(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateRigAgent_Named(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateRigAgent_Named(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateRigAgent_Singleton(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateRigAgent_Singleton(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateRigAgent_TownOnlyContext_Error(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testCreateRigAgent_TownOnlyContext(t, townRoot)
}

func TestBeadsCreateAPI_CreateRigAgent_FieldsPersisted(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateRigAgent_FieldsPersisted(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateOrReopenRigAgent_New(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateOrReopenRigAgent_New(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateOrReopenRigAgent_Existing(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateOrReopenRigAgent_Existing(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateOrReopenRigAgent_ReopenClosed(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateOrReopenRigAgent_ReopenClosed(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateOrReopenRigAgent_TownOnlyContext_Error(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testCreateOrReopenRigAgent_TownOnlyContext(t, townRoot)
}

func TestBeadsCreateAPI_CreateRigIdentity_RigContext(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateRigIdentity_RigContext(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateRigIdentity_TownOnlyContext_Error(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testCreateRigIdentity_TownOnlyContext(t, townRoot)
}

func TestBeadsCreateAPI_CreateTownAgent_TownOnlyContext(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testCreateTownAgent_TownOnlyContext(t, townRoot)
}

func TestBeadsCreateAPI_CreateTownAgent_RigContext(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateTownAgent_RigContext(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateTownConvoy_TownOnlyContext(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testCreateTownConvoy_TownOnlyContext(t, townRoot)
}

func TestBeadsCreateAPI_CreateTownConvoy_RigContext(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateTownConvoy_RigContext(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateTownConvoy_FieldsPersisted(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testCreateTownConvoy_FieldsPersisted(t, townRoot)
}

func TestBeadsCreateAPI_CreateWithID_HqFromRig(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateWithID_HqFromRig(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateWithID_RigFromRig(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateWithID_RigFromRig(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateWithID_HqFromTownOnly(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testCreateWithID_HqFromTownOnly(t, townRoot)
}

func TestBeadsCreateAPI_CreateWithID_RigFromTownOnly_Error(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testCreateWithID_RigFromTownOnly_Error(t, townRoot)
}

func TestBeadsCreateAPI_CreateWithID_MismatchedPrefix_Error(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateWithID_MismatchedPrefix_Error(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateWithoutID_RigContext(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testCreateWithoutID_RigContext(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_CreateWithoutID_TownOnlyContext(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testCreateWithoutID_TownOnlyContext(t, townRoot)
}

func TestBeadsCreateAPI_ShowVsGetAgent_ShowOnAgent(t *testing.T) {
	t.Parallel()
	townRoot, rigPath := setupTestEnv(t)
	testShow_OnAgent(t, townRoot, rigPath)
}

func TestBeadsCreateAPI_ShowVsGetAgent_GetAgentOnNonAgent_Error(t *testing.T) {
	t.Parallel()
	townRoot, _ := setupTestEnv(t)
	testGetAgent_OnNonAgent_Error(t, townRoot)
}

// setupCreateAPITestTown creates a test town with one rig for create API tests.
func setupCreateAPITestTown(t *testing.T, gtBinary string) string {
	t.Helper()

	tmpDir := t.TempDir()
	townRoot := filepath.Join(tmpDir, "create-api-town")
	reposDir := filepath.Join(tmpDir, "repos")

	os.MkdirAll(reposDir, 0755)

	// Create source repo for rig
	rigRepo := filepath.Join(reposDir, "testrig-repo")
	createBareGitRepo(t, rigRepo)

	// Install town
	cmd := exec.Command(gtBinary, "install", townRoot, "--name", "create-api-town")
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gt install failed: %v\nOutput: %s", err, output)
	}

	// Add rig with prefix "tr" (testrig)
	cmd = exec.Command(gtBinary, "rig", "add", "testrig", rigRepo, "--prefix", "tr")
	cmd.Dir = townRoot
	cmd.Env = filterOutBeadsDir(os.Environ())
	cmd.Env = append(cmd.Env, "HOME="+tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gt rig add failed: %v\nOutput: %s", err, output)
	}

	return townRoot
}

// === Constructor Tests ===

func testNewTownOnlyConstructor(t *testing.T, townRoot string) {
	bd := beads.New(townRoot)

	if bd.TownRoot() != townRoot {
		t.Errorf("TownRoot() = %q, want %q", bd.TownRoot(), townRoot)
	}
	if bd.RigPath() != "" {
		t.Errorf("RigPath() = %q, want empty", bd.RigPath())
	}
	if bd.RigName() != "" {
		t.Errorf("RigName() = %q, want empty", bd.RigName())
	}
	if bd.RigPrefix() != "" {
		t.Errorf("RigPrefix() = %q, want empty", bd.RigPrefix())
	}
	if bd.IsRig() {
		t.Error("IsRig() = true, want false")
	}
	if !bd.IsTown() {
		t.Error("IsTown() = false, want true")
	}
	if bd.IsBound() {
		t.Error("IsBound() = true, want false for town-only")
	}
}

func testNewWithRigConstructor(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	if bd.TownRoot() != townRoot {
		t.Errorf("TownRoot() = %q, want %q", bd.TownRoot(), townRoot)
	}
	if bd.RigPath() != rigPath {
		t.Errorf("RigPath() = %q, want %q", bd.RigPath(), rigPath)
	}
	if bd.RigName() != "testrig" {
		t.Errorf("RigName() = %q, want %q", bd.RigName(), "testrig")
	}
	if bd.RigPrefix() != "tr" {
		t.Errorf("RigPrefix() = %q, want %q", bd.RigPrefix(), "tr")
	}
	if !bd.IsRig() {
		t.Error("IsRig() = false, want true")
	}
	if !bd.IsTown() {
		t.Error("IsTown() = false, want true")
	}
	if !bd.IsBound() {
		t.Error("IsBound() = false, want true")
	}
}

// === CreateRigAgent Tests ===

func testCreateRigAgent_Named(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	fields := &beads.AgentFields{
		RoleType:   "polecat",
		Rig:        "testrig",
		AgentState: "spawning",
	}
	issue, err := bd.CreateRigAgent("polecat", "NamedCat", "Test Named Polecat", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}

	expectedID := "tr-testrig-polecat-NamedCat"
	if issue.ID != expectedID {
		t.Errorf("ID = %q, want %q", issue.ID, expectedID)
	}

	// Verify with Show
	found, err := bd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show failed: %v", err)
	}
	if found.ID != expectedID {
		t.Errorf("Show returned ID = %q, want %q", found.ID, expectedID)
	}
}

func testCreateRigAgent_Singleton(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	fields := &beads.AgentFields{
		RoleType:   "witness",
		Rig:        "testrig",
		AgentState: "idle",
	}
	// Empty name is valid for singletons like witness/refinery
	// ID format: {prefix}-{rigName}-{roleType}
	issue, err := bd.CreateRigAgent("witness", "", "Test Witness", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent with empty name failed: %v", err)
	}

	// Should create ID without name suffix: tr-testrig-witness
	expectedID := "tr-testrig-witness"
	if issue.ID != expectedID {
		t.Errorf("ID = %q, want %q", issue.ID, expectedID)
	}

	// Verify with Show
	found, err := bd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show failed: %v", err)
	}
	if found.ID != expectedID {
		t.Errorf("Show returned ID = %q, want %q", found.ID, expectedID)
	}
}

func testCreateRigAgent_TownOnlyContext(t *testing.T, townRoot string) {
	bd := beads.New(townRoot) // town-only, no rig

	fields := &beads.AgentFields{
		RoleType:   "polecat",
		AgentState: "spawning",
	}
	_, err := bd.CreateRigAgent("polecat", "ShouldFail", "Test", fields)
	if err == nil {
		t.Error("CreateRigAgent from town-only context should fail")
	}
	if !strings.Contains(err.Error(), "rig context") {
		t.Errorf("Error should mention rig context, got: %v", err)
	}
}

func testCreateRigAgent_FieldsPersisted(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	fields := &beads.AgentFields{
		RoleType:          "polecat",
		Rig:               "testrig",
		AgentState:        "working",
		HookBead:          "tr-hook-123",
		RoleBead:          "tr-role-456",
		CleanupStatus:     "pending",
		ActiveMR:          "tr-mr-789",
		NotificationLevel: "verbose",
	}
	issue, err := bd.CreateRigAgent("polecat", "FieldTest", "Test Fields", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}

	// Verify with GetAgentBead (parses fields)
	foundIssue, foundFields, err := bd.GetAgentBead(issue.ID)
	if err != nil {
		t.Fatalf("GetAgentBead failed: %v", err)
	}
	if foundIssue == nil || foundFields == nil {
		t.Fatal("GetAgentBead returned nil")
	}

	// Verify fields were persisted
	if foundFields.RoleType != "polecat" {
		t.Errorf("RoleType = %q, want %q", foundFields.RoleType, "polecat")
	}
	if foundFields.Rig != "testrig" {
		t.Errorf("Rig = %q, want %q", foundFields.Rig, "testrig")
	}
	if foundFields.AgentState != "working" {
		t.Errorf("AgentState = %q, want %q", foundFields.AgentState, "working")
	}
	if foundFields.HookBead != "tr-hook-123" {
		t.Errorf("HookBead = %q, want %q", foundFields.HookBead, "tr-hook-123")
	}
}

// === CreateOrReopenRigAgent Tests ===

func testCreateOrReopenRigAgent_New(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	fields := &beads.AgentFields{
		RoleType:   "polecat",
		Rig:        "testrig",
		AgentState: "spawning",
	}
	issue, err := bd.CreateOrReopenRigAgentBead("polecat", "ReopenNew", "Test Reopen New", fields)
	if err != nil {
		t.Fatalf("CreateOrReopenRigAgentBead failed: %v", err)
	}

	expectedID := "tr-testrig-polecat-ReopenNew"
	if issue.ID != expectedID {
		t.Errorf("ID = %q, want %q", issue.ID, expectedID)
	}
}

func testCreateOrReopenRigAgent_Existing(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	fields := &beads.AgentFields{
		RoleType:   "polecat",
		Rig:        "testrig",
		AgentState: "spawning",
	}

	// Create first
	issue1, err := bd.CreateOrReopenRigAgentBead("polecat", "ReopenExist", "Test Reopen Existing", fields)
	if err != nil {
		t.Fatalf("First CreateOrReopenRigAgentBead failed: %v", err)
	}

	// Call again - should return existing
	issue2, err := bd.CreateOrReopenRigAgentBead("polecat", "ReopenExist", "Test Reopen Existing", fields)
	if err != nil {
		t.Fatalf("Second CreateOrReopenRigAgentBead failed: %v", err)
	}

	if issue1.ID != issue2.ID {
		t.Errorf("Second call returned different ID: %q vs %q", issue1.ID, issue2.ID)
	}
}

func testCreateOrReopenRigAgent_ReopenClosed(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	fields := &beads.AgentFields{
		RoleType:   "polecat",
		Rig:        "testrig",
		AgentState: "spawning",
	}

	// Create
	issue1, err := bd.CreateOrReopenRigAgentBead("polecat", "ReopenClosed", "Test Reopen Closed", fields)
	if err != nil {
		t.Fatalf("First CreateOrReopenRigAgentBead failed: %v", err)
	}

	// Close it
	if err := bd.Close(issue1.ID); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Call again - should reopen
	issue2, err := bd.CreateOrReopenRigAgentBead("polecat", "ReopenClosed", "Test Reopen Closed", fields)
	if err != nil {
		t.Fatalf("Reopen CreateOrReopenRigAgentBead failed: %v", err)
	}

	if issue1.ID != issue2.ID {
		t.Errorf("Reopen returned different ID: %q vs %q", issue1.ID, issue2.ID)
	}

	// Verify it's open
	found, err := bd.Show(issue2.ID)
	if err != nil {
		t.Fatalf("Show failed: %v", err)
	}
	if found.Status != "open" {
		t.Errorf("Status = %q, want %q", found.Status, "open")
	}
}

func testCreateOrReopenRigAgent_TownOnlyContext(t *testing.T, townRoot string) {
	bd := beads.New(townRoot)

	fields := &beads.AgentFields{
		RoleType:   "polecat",
		AgentState: "spawning",
	}
	_, err := bd.CreateOrReopenRigAgentBead("polecat", "ShouldFail", "Test", fields)
	if err == nil {
		t.Error("CreateOrReopenRigAgentBead from town-only context should fail")
	}
	if !strings.Contains(err.Error(), "rig context") {
		t.Errorf("Error should mention rig context, got: %v", err)
	}
}

// === CreateRigIdentity Tests ===

func testCreateRigIdentity_RigContext(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	fields := &beads.RigFields{
		State:  "active",
		Repo:   "https://github.com/test/repo",
		Prefix: "tr",
	}
	issue, err := bd.CreateRigIdentityBead(fields)
	if err != nil {
		t.Fatalf("CreateRigIdentityBead failed: %v", err)
	}

	// ID format: {prefix}-rig-{rigName}
	expectedID := "tr-rig-testrig"
	if issue.ID != expectedID {
		t.Errorf("ID = %q, want %q", issue.ID, expectedID)
	}

	// Verify with Show
	found, err := bd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show failed: %v", err)
	}
	if found.ID != expectedID {
		t.Errorf("Show returned ID = %q, want %q", found.ID, expectedID)
	}
}

func testCreateRigIdentity_TownOnlyContext(t *testing.T, townRoot string) {
	bd := beads.New(townRoot)

	fields := &beads.RigFields{
		State: "active",
	}
	_, err := bd.CreateRigIdentityBead(fields)
	if err == nil {
		t.Error("CreateRigIdentityBead from town-only context should fail")
	}
	if !strings.Contains(err.Error(), "rig context") {
		t.Errorf("Error should mention rig context, got: %v", err)
	}
}

// === CreateTownAgent Tests ===

func testCreateTownAgent_TownOnlyContext(t *testing.T, townRoot string) {
	bd := beads.New(townRoot)

	fields := &beads.AgentFields{
		RoleType:          "deacon",
		AgentState:        "idle",
		NotificationLevel: "verbose",
	}
	issue, err := bd.CreateTownAgent("deacon", "Test Deacon Agent", fields)
	if err != nil {
		t.Fatalf("CreateTownAgent from town-only context failed: %v", err)
	}

	// ID format: {TownBeadsPrefix}-{roleType}
	expectedID := beads.TownBeadsPrefix + "-deacon"
	if issue.ID != expectedID {
		t.Errorf("ID = %q, want %q", issue.ID, expectedID)
	}

	// Verify it's in town beads and fields are persisted
	foundIssue, foundFields, err := bd.GetAgentBead(issue.ID)
	if err != nil {
		t.Fatalf("GetAgentBead failed: %v", err)
	}
	if foundIssue.ID != expectedID {
		t.Errorf("GetAgentBead returned ID = %q, want %q", foundIssue.ID, expectedID)
	}
	if foundFields.RoleType != "deacon" {
		t.Errorf("RoleType = %q, want %q", foundFields.RoleType, "deacon")
	}
	if foundFields.AgentState != "idle" {
		t.Errorf("AgentState = %q, want %q", foundFields.AgentState, "idle")
	}
}

func testCreateTownAgent_RigContext(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	fields := &beads.AgentFields{
		RoleType:   "mayor",
		AgentState: "idle",
	}
	issue, err := bd.CreateTownAgent("mayor", "Test Mayor Agent", fields)
	if err != nil {
		t.Fatalf("CreateTownAgent from rig context failed: %v", err)
	}

	// Should still create in town beads with hq- prefix
	expectedID := beads.TownBeadsPrefix + "-mayor"
	if issue.ID != expectedID {
		t.Errorf("ID = %q, want %q", issue.ID, expectedID)
	}

	// Verify it's in town beads (readable from town context)
	townBd := beads.New(townRoot)
	found, err := townBd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show from town context failed: %v", err)
	}
	if found.ID != expectedID {
		t.Errorf("Show returned ID = %q, want %q", found.ID, expectedID)
	}
}

// === CreateTownConvoy Tests ===

func testCreateTownConvoy_TownOnlyContext(t *testing.T, townRoot string) {
	bd := beads.New(townRoot)

	fields := &beads.ConvoyFields{
		Notify: "test@example.com",
	}
	issue, err := bd.CreateTownConvoy("Test Convoy Town", 3, fields)
	if err != nil {
		t.Fatalf("CreateTownConvoy from town-only context failed: %v", err)
	}

	// ID format: {TownBeadsPrefix}-cv-{random}
	if !strings.HasPrefix(issue.ID, beads.TownBeadsPrefix+"-cv-") {
		t.Errorf("ID = %q, should have prefix %q", issue.ID, beads.TownBeadsPrefix+"-cv-")
	}
}

func testCreateTownConvoy_RigContext(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	fields := &beads.ConvoyFields{
		Notify: "test@example.com",
	}
	issue, err := bd.CreateTownConvoy("Test Convoy Rig", 3, fields)
	if err != nil {
		t.Fatalf("CreateTownConvoy from rig context failed: %v", err)
	}

	// Should still create in town beads with hq- prefix
	if !strings.HasPrefix(issue.ID, beads.TownBeadsPrefix+"-cv-") {
		t.Errorf("ID = %q, should have prefix %q", issue.ID, beads.TownBeadsPrefix+"-cv-")
	}

	// Verify it's in town beads
	townBd := beads.New(townRoot)
	found, err := townBd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show from town context failed: %v", err)
	}
	if found.ID != issue.ID {
		t.Errorf("Show returned ID = %q, want %q", found.ID, issue.ID)
	}
}

func testCreateTownConvoy_FieldsPersisted(t *testing.T, townRoot string) {
	bd := beads.New(townRoot)

	fields := &beads.ConvoyFields{
		Notify:   "convoy-test@example.com",
		Molecule: "mol-test-123",
	}
	issue, err := bd.CreateTownConvoy("Test Convoy Fields", 5, fields)
	if err != nil {
		t.Fatalf("CreateTownConvoy failed: %v", err)
	}

	// Verify fields by reading issue and parsing
	found, err := bd.Show(issue.ID)
	if err != nil {
		t.Fatalf("Show failed: %v", err)
	}

	parsedFields := beads.ParseConvoyFields(found.Description)
	if parsedFields.Notify != "convoy-test@example.com" {
		t.Errorf("Notify = %q, want %q", parsedFields.Notify, "convoy-test@example.com")
	}
	if parsedFields.Molecule != "mol-test-123" {
		t.Errorf("Molecule = %q, want %q", parsedFields.Molecule, "mol-test-123")
	}
}

// === Create with Explicit ID Tests ===

func testCreateWithID_HqFromRig(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	customID := fmt.Sprintf("hq-test-fromrig-%s", generateShortTestID())
	issue, err := bd.Create(beads.CreateOptions{
		ID:    customID,
		Title: "Test hq from rig",
		Type:  "task",
	})
	if err != nil {
		t.Fatalf("Create with hq- ID from rig context failed: %v", err)
	}

	if issue.ID != customID {
		t.Errorf("ID = %q, want %q", issue.ID, customID)
	}

	// Verify it's in town beads
	townBd := beads.New(townRoot)
	found, err := townBd.Show(customID)
	if err != nil {
		t.Fatalf("Show from town context failed: %v", err)
	}
	if found.ID != customID {
		t.Errorf("Show returned ID = %q, want %q", found.ID, customID)
	}
}

func testCreateWithID_RigFromRig(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	customID := fmt.Sprintf("tr-test-fromrig-%s", generateShortTestID())
	issue, err := bd.Create(beads.CreateOptions{
		ID:    customID,
		Title: "Test tr from rig",
		Type:  "task",
	})
	if err != nil {
		t.Fatalf("Create with tr- ID from rig context failed: %v", err)
	}

	if issue.ID != customID {
		t.Errorf("ID = %q, want %q", issue.ID, customID)
	}

	// Verify it's in rig beads
	found, err := bd.Show(customID)
	if err != nil {
		t.Fatalf("Show failed: %v", err)
	}
	if found.ID != customID {
		t.Errorf("Show returned ID = %q, want %q", found.ID, customID)
	}
}

func testCreateWithID_HqFromTownOnly(t *testing.T, townRoot string) {
	bd := beads.New(townRoot)

	customID := fmt.Sprintf("hq-test-fromtown-%s", generateShortTestID())
	issue, err := bd.Create(beads.CreateOptions{
		ID:    customID,
		Title: "Test hq from town-only",
		Type:  "task",
	})
	if err != nil {
		t.Fatalf("Create with hq- ID from town-only context failed: %v", err)
	}

	if issue.ID != customID {
		t.Errorf("ID = %q, want %q", issue.ID, customID)
	}
}

func testCreateWithID_RigFromTownOnly_Error(t *testing.T, townRoot string) {
	bd := beads.New(townRoot) // town-only, no rig

	customID := fmt.Sprintf("tr-test-shouldfail-%s", generateShortTestID())
	_, err := bd.Create(beads.CreateOptions{
		ID:    customID,
		Title: "Test rig ID from town-only should fail",
		Type:  "task",
	})
	if err == nil {
		t.Error("Create with rig prefix ID from town-only context should fail")
	}
	if !strings.Contains(err.Error(), "rig context") && !strings.Contains(err.Error(), "rig prefix") {
		t.Errorf("Error should mention rig context/prefix, got: %v", err)
	}
}

func testCreateWithID_MismatchedPrefix_Error(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath) // rig has tr- prefix

	// Try to create with a different rig prefix (not tr-, not hq-)
	customID := fmt.Sprintf("xx-test-mismatch-%s", generateShortTestID())
	_, err := bd.Create(beads.CreateOptions{
		ID:    customID,
		Title: "Test mismatched prefix should fail",
		Type:  "task",
	})
	if err == nil {
		t.Error("Create with mismatched prefix should fail")
	}
	if !strings.Contains(err.Error(), "mismatch") && !strings.Contains(err.Error(), "prefix") {
		t.Errorf("Error should mention prefix mismatch, got: %v", err)
	}
}

// === Create without ID Tests ===

func testCreateWithoutID_RigContext(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	issue, err := bd.Create(beads.CreateOptions{
		Title: "Test no ID from rig context",
		Type:  "task",
	})
	if err != nil {
		t.Fatalf("Create without ID from rig context failed: %v", err)
	}

	// Should create in town beads with hq- prefix (default behavior)
	if !strings.HasPrefix(issue.ID, beads.TownBeadsPrefix+"-") {
		t.Errorf("ID = %q, should have prefix %q", issue.ID, beads.TownBeadsPrefix+"-")
	}
}

func testCreateWithoutID_TownOnlyContext(t *testing.T, townRoot string) {
	bd := beads.New(townRoot)

	issue, err := bd.Create(beads.CreateOptions{
		Title: "Test no ID from town-only context",
		Type:  "task",
	})
	if err != nil {
		t.Fatalf("Create without ID from town-only context failed: %v", err)
	}

	// Should create in town beads with hq- prefix
	if !strings.HasPrefix(issue.ID, beads.TownBeadsPrefix+"-") {
		t.Errorf("ID = %q, should have prefix %q", issue.ID, beads.TownBeadsPrefix+"-")
	}
}

// === Show vs GetAgent Tests ===

func testShow_OnAgent(t *testing.T, townRoot, rigPath string) {
	bd := beads.New(townRoot, rigPath)

	// Create an agent bead
	fields := &beads.AgentFields{
		RoleType:   "polecat",
		Rig:        "testrig",
		AgentState: "idle",
	}
	created, err := bd.CreateRigAgent("polecat", "ShowTest", "Test Show", fields)
	if err != nil {
		t.Fatalf("CreateRigAgent failed: %v", err)
	}

	// Show should work on agent bead
	issue, err := bd.Show(created.ID)
	if err != nil {
		t.Fatalf("Show on agent bead failed: %v", err)
	}
	if issue.ID != created.ID {
		t.Errorf("Show returned ID = %q, want %q", issue.ID, created.ID)
	}

	// Verify it has gt:agent label
	if !beads.HasLabel(issue, "gt:agent") {
		t.Error("Agent bead should have gt:agent label")
	}
}

func testGetAgent_OnNonAgent_Error(t *testing.T, townRoot string) {
	bd := beads.New(townRoot)

	// Create a non-agent bead (convoy)
	convoyFields := &beads.ConvoyFields{
		Notify: "test@example.com",
	}
	convoy, err := bd.CreateTownConvoy("Non-Agent Bead", 1, convoyFields)
	if err != nil {
		t.Fatalf("CreateTownConvoy failed: %v", err)
	}

	// GetAgentBead should fail on non-agent
	_, _, err = bd.GetAgentBead(convoy.ID)
	if err == nil {
		t.Error("GetAgentBead on non-agent bead should fail")
	}
	if !strings.Contains(err.Error(), "not an agent bead") && !strings.Contains(err.Error(), "gt:agent") {
		t.Errorf("Error should mention not an agent bead, got: %v", err)
	}
}
