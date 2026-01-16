package beads

// BeadsStub wraps a BeadsOps and allows injecting errors for testing.
// It delegates all calls to the underlying implementation unless an error
// is set for that method.
type BeadsStub struct {
	BeadsOps

	// Error injection for each method group
	ListErr             error
	ShowErr             error
	CreateErr           error
	UpdateErr           error
	CloseErr               error
	CloseWithOptionsErr    error
	DeleteErr              error
	ReopenErr           error
	ReadyErr            error
	BlockedErr          error
	AddDependencyErr         error
	AddDependencyWithTypeErr error
	RemoveDependencyErr      error
	SyncErr             error
	ConfigGetErr        error
	ConfigSetErr        error
	InitErr             error
	MigrateErr          error
	DaemonStartErr      error
	DaemonStopErr       error
	DaemonStatusErr     error
	DaemonHealthErr     error
	MolSeedErr          error
	MolCurrentErr       error
	MolCatalogErr       error
	WispCreateErr            error
	WispCreateWithOptionsErr error
	WispListErr              error
	WispGCErr           error
	MolBurnErr          error
	MolBondErr          error
	GateShowErr         error
	GateWaitErr         error
	GateListErr         error
	GateResolveErr      error
	GateAddWaiterErr    error
	GateCheckErr        error
	SwarmStatusErr      error
	SwarmCreateErr      error
	SwarmListErr        error
	SwarmValidateErr    error
	FormulaShowErr      error
	FormulaListErr      error
	CookErr             error
	LegAddErr           error
	AgentStateErr       error
	LabelAddErr         error
	LabelRemoveErr      error
	SlotShowErr         error
	SlotSetErr          error
	SlotClearErr        error
	SearchErr           error
	MessageThreadErr    error
	VersionErr          error
	DoctorErr           error
	PrimeErr            error
	StatsErr            error
	FlushErr            error
	BurnErr             error
	CommentErr          error
}

// NewBeadsStub creates a new BeadsStub wrapping the given BeadsOps.
func NewBeadsStub(impl BeadsOps) *BeadsStub {
	return &BeadsStub{BeadsOps: impl}
}

// List delegates to the wrapped implementation unless ListErr is set.
func (s *BeadsStub) List(opts ListOptions) ([]*Issue, error) {
	if s.ListErr != nil {
		return nil, s.ListErr
	}
	return s.BeadsOps.List(opts)
}

// Show delegates to the wrapped implementation unless ShowErr is set.
func (s *BeadsStub) Show(id string) (*Issue, error) {
	if s.ShowErr != nil {
		return nil, s.ShowErr
	}
	return s.BeadsOps.Show(id)
}

// ShowMultiple delegates to the wrapped implementation unless ShowErr is set.
func (s *BeadsStub) ShowMultiple(ids []string) (map[string]*Issue, error) {
	if s.ShowErr != nil {
		return nil, s.ShowErr
	}
	return s.BeadsOps.ShowMultiple(ids)
}

// Create delegates to the wrapped implementation unless CreateErr is set.
func (s *BeadsStub) Create(opts CreateOptions) (*Issue, error) {
	if s.CreateErr != nil {
		return nil, s.CreateErr
	}
	return s.BeadsOps.Create(opts)
}

// CreateWithID delegates to the wrapped implementation unless CreateErr is set.
func (s *BeadsStub) CreateWithID(id string, opts CreateOptions) (*Issue, error) {
	if s.CreateErr != nil {
		return nil, s.CreateErr
	}
	return s.BeadsOps.CreateWithID(id, opts)
}

// Update delegates to the wrapped implementation unless UpdateErr is set.
func (s *BeadsStub) Update(id string, opts UpdateOptions) error {
	if s.UpdateErr != nil {
		return s.UpdateErr
	}
	return s.BeadsOps.Update(id, opts)
}

// Close delegates to the wrapped implementation unless CloseErr is set.
func (s *BeadsStub) Close(ids ...string) error {
	if s.CloseErr != nil {
		return s.CloseErr
	}
	return s.BeadsOps.Close(ids...)
}

// CloseWithReason delegates to the wrapped implementation unless CloseErr is set.
func (s *BeadsStub) CloseWithReason(reason string, ids ...string) error {
	if s.CloseErr != nil {
		return s.CloseErr
	}
	return s.BeadsOps.CloseWithReason(reason, ids...)
}

// CloseWithOptions delegates to the wrapped implementation unless CloseWithOptionsErr is set.
func (s *BeadsStub) CloseWithOptions(opts CloseOptions, ids ...string) error {
	if s.CloseWithOptionsErr != nil {
		return s.CloseWithOptionsErr
	}
	return s.BeadsOps.CloseWithOptions(opts, ids...)
}

// Delete delegates to the wrapped implementation unless DeleteErr is set.
func (s *BeadsStub) Delete(ids ...string) error {
	if s.DeleteErr != nil {
		return s.DeleteErr
	}
	return s.BeadsOps.Delete(ids...)
}

// Reopen delegates to the wrapped implementation unless ReopenErr is set.
func (s *BeadsStub) Reopen(id string) error {
	if s.ReopenErr != nil {
		return s.ReopenErr
	}
	return s.BeadsOps.Reopen(id)
}

// Ready delegates to the wrapped implementation unless ReadyErr is set.
func (s *BeadsStub) Ready() ([]*Issue, error) {
	if s.ReadyErr != nil {
		return nil, s.ReadyErr
	}
	return s.BeadsOps.Ready()
}

// ReadyWithLabel delegates to the wrapped implementation unless ReadyErr is set.
func (s *BeadsStub) ReadyWithLabel(label string, limit int) ([]*Issue, error) {
	if s.ReadyErr != nil {
		return nil, s.ReadyErr
	}
	return s.BeadsOps.ReadyWithLabel(label, limit)
}

// Blocked delegates to the wrapped implementation unless BlockedErr is set.
func (s *BeadsStub) Blocked() ([]*Issue, error) {
	if s.BlockedErr != nil {
		return nil, s.BlockedErr
	}
	return s.BeadsOps.Blocked()
}

// AddDependency delegates to the wrapped implementation unless AddDependencyErr is set.
func (s *BeadsStub) AddDependency(issue, dependsOn string) error {
	if s.AddDependencyErr != nil {
		return s.AddDependencyErr
	}
	return s.BeadsOps.AddDependency(issue, dependsOn)
}

// AddDependencyWithType delegates to the wrapped implementation unless AddDependencyWithTypeErr is set.
func (s *BeadsStub) AddDependencyWithType(issue, dependsOn, depType string) error {
	if s.AddDependencyWithTypeErr != nil {
		return s.AddDependencyWithTypeErr
	}
	return s.BeadsOps.AddDependencyWithType(issue, dependsOn, depType)
}

// RemoveDependency delegates to the wrapped implementation unless RemoveDependencyErr is set.
func (s *BeadsStub) RemoveDependency(issue, dependsOn string) error {
	if s.RemoveDependencyErr != nil {
		return s.RemoveDependencyErr
	}
	return s.BeadsOps.RemoveDependency(issue, dependsOn)
}

// Sync delegates to the wrapped implementation unless SyncErr is set.
func (s *BeadsStub) Sync() error {
	if s.SyncErr != nil {
		return s.SyncErr
	}
	return s.BeadsOps.Sync()
}

// SyncFromMain delegates to the wrapped implementation unless SyncErr is set.
func (s *BeadsStub) SyncFromMain() error {
	if s.SyncErr != nil {
		return s.SyncErr
	}
	return s.BeadsOps.SyncFromMain()
}

// SyncImportOnly delegates to the wrapped implementation unless SyncErr is set.
func (s *BeadsStub) SyncImportOnly() error {
	if s.SyncErr != nil {
		return s.SyncErr
	}
	return s.BeadsOps.SyncImportOnly()
}

// GetSyncStatus delegates to the wrapped implementation unless SyncErr is set.
func (s *BeadsStub) GetSyncStatus() (*SyncStatus, error) {
	if s.SyncErr != nil {
		return nil, s.SyncErr
	}
	return s.BeadsOps.GetSyncStatus()
}

// ConfigGet delegates to the wrapped implementation unless ConfigGetErr is set.
func (s *BeadsStub) ConfigGet(key string) (string, error) {
	if s.ConfigGetErr != nil {
		return "", s.ConfigGetErr
	}
	return s.BeadsOps.ConfigGet(key)
}

// ConfigSet delegates to the wrapped implementation unless ConfigSetErr is set.
func (s *BeadsStub) ConfigSet(key, value string) error {
	if s.ConfigSetErr != nil {
		return s.ConfigSetErr
	}
	return s.BeadsOps.ConfigSet(key, value)
}

// Init delegates to the wrapped implementation unless InitErr is set.
func (s *BeadsStub) Init(opts InitOptions) error {
	if s.InitErr != nil {
		return s.InitErr
	}
	return s.BeadsOps.Init(opts)
}

// Migrate delegates to the wrapped implementation unless MigrateErr is set.
func (s *BeadsStub) Migrate(opts MigrateOptions) error {
	if s.MigrateErr != nil {
		return s.MigrateErr
	}
	return s.BeadsOps.Migrate(opts)
}

// DaemonStart delegates to the wrapped implementation unless DaemonStartErr is set.
func (s *BeadsStub) DaemonStart() error {
	if s.DaemonStartErr != nil {
		return s.DaemonStartErr
	}
	return s.BeadsOps.DaemonStart()
}

// DaemonStop delegates to the wrapped implementation unless DaemonStopErr is set.
func (s *BeadsStub) DaemonStop() error {
	if s.DaemonStopErr != nil {
		return s.DaemonStopErr
	}
	return s.BeadsOps.DaemonStop()
}

// DaemonStatus delegates to the wrapped implementation unless DaemonStatusErr is set.
func (s *BeadsStub) DaemonStatus() (*DaemonStatus, error) {
	if s.DaemonStatusErr != nil {
		return nil, s.DaemonStatusErr
	}
	return s.BeadsOps.DaemonStatus()
}

// DaemonHealth delegates to the wrapped implementation unless DaemonHealthErr is set.
func (s *BeadsStub) DaemonHealth() (*DaemonHealth, error) {
	if s.DaemonHealthErr != nil {
		return nil, s.DaemonHealthErr
	}
	return s.BeadsOps.DaemonHealth()
}

// MolSeed delegates to the wrapped implementation unless MolSeedErr is set.
func (s *BeadsStub) MolSeed(opts MolSeedOptions) error {
	if s.MolSeedErr != nil {
		return s.MolSeedErr
	}
	return s.BeadsOps.MolSeed(opts)
}

// MolCurrent delegates to the wrapped implementation unless MolCurrentErr is set.
func (s *BeadsStub) MolCurrent(moleculeID string) (*MolCurrentOutput, error) {
	if s.MolCurrentErr != nil {
		return nil, s.MolCurrentErr
	}
	return s.BeadsOps.MolCurrent(moleculeID)
}

// MolCatalog delegates to the wrapped implementation unless MolCatalogErr is set.
func (s *BeadsStub) MolCatalog() ([]*MoleculeProto, error) {
	if s.MolCatalogErr != nil {
		return nil, s.MolCatalogErr
	}
	return s.BeadsOps.MolCatalog()
}

// WispCreate delegates to the wrapped implementation unless WispCreateErr is set.
func (s *BeadsStub) WispCreate(protoID, actor string) (*Issue, error) {
	if s.WispCreateErr != nil {
		return nil, s.WispCreateErr
	}
	return s.BeadsOps.WispCreate(protoID, actor)
}

// WispCreateWithOptions delegates to the wrapped implementation unless WispCreateWithOptionsErr is set.
func (s *BeadsStub) WispCreateWithOptions(opts WispCreateOptions) (*Issue, error) {
	if s.WispCreateWithOptionsErr != nil {
		return nil, s.WispCreateWithOptionsErr
	}
	return s.BeadsOps.WispCreateWithOptions(opts)
}

// WispList delegates to the wrapped implementation unless WispListErr is set.
func (s *BeadsStub) WispList(all bool) ([]*Issue, error) {
	if s.WispListErr != nil {
		return nil, s.WispListErr
	}
	return s.BeadsOps.WispList(all)
}

// WispGC delegates to the wrapped implementation unless WispGCErr is set.
func (s *BeadsStub) WispGC() error {
	if s.WispGCErr != nil {
		return s.WispGCErr
	}
	return s.BeadsOps.WispGC()
}

// MolBurn delegates to the wrapped implementation unless MolBurnErr is set.
func (s *BeadsStub) MolBurn(ids ...string) error {
	if s.MolBurnErr != nil {
		return s.MolBurnErr
	}
	return s.BeadsOps.MolBurn(ids...)
}

// MolBond delegates to the wrapped implementation unless MolBondErr is set.
func (s *BeadsStub) MolBond(wispID, beadID string) (*Issue, error) {
	if s.MolBondErr != nil {
		return nil, s.MolBondErr
	}
	return s.BeadsOps.MolBond(wispID, beadID)
}

// GateShow delegates to the wrapped implementation unless GateShowErr is set.
func (s *BeadsStub) GateShow(gateID string) (*Gate, error) {
	if s.GateShowErr != nil {
		return nil, s.GateShowErr
	}
	return s.BeadsOps.GateShow(gateID)
}

// GateWait delegates to the wrapped implementation unless GateWaitErr is set.
func (s *BeadsStub) GateWait(gateID, notifyAgent string) error {
	if s.GateWaitErr != nil {
		return s.GateWaitErr
	}
	return s.BeadsOps.GateWait(gateID, notifyAgent)
}

// GateList delegates to the wrapped implementation unless GateListErr is set.
func (s *BeadsStub) GateList(all bool) ([]*Gate, error) {
	if s.GateListErr != nil {
		return nil, s.GateListErr
	}
	return s.BeadsOps.GateList(all)
}

// GateResolve delegates to the wrapped implementation unless GateResolveErr is set.
func (s *BeadsStub) GateResolve(gateID string) error {
	if s.GateResolveErr != nil {
		return s.GateResolveErr
	}
	return s.BeadsOps.GateResolve(gateID)
}

// GateAddWaiter delegates to the wrapped implementation unless GateAddWaiterErr is set.
func (s *BeadsStub) GateAddWaiter(gateID, waiterID string) error {
	if s.GateAddWaiterErr != nil {
		return s.GateAddWaiterErr
	}
	return s.BeadsOps.GateAddWaiter(gateID, waiterID)
}

// GateCheck delegates to the wrapped implementation unless GateCheckErr is set.
func (s *BeadsStub) GateCheck() error {
	if s.GateCheckErr != nil {
		return s.GateCheckErr
	}
	return s.BeadsOps.GateCheck()
}

// SwarmStatus delegates to the wrapped implementation unless SwarmStatusErr is set.
func (s *BeadsStub) SwarmStatus(swarmID string) (*SwarmStatus, error) {
	if s.SwarmStatusErr != nil {
		return nil, s.SwarmStatusErr
	}
	return s.BeadsOps.SwarmStatus(swarmID)
}

// SwarmCreate delegates to the wrapped implementation unless SwarmCreateErr is set.
func (s *BeadsStub) SwarmCreate(epicID string) (*Issue, error) {
	if s.SwarmCreateErr != nil {
		return nil, s.SwarmCreateErr
	}
	return s.BeadsOps.SwarmCreate(epicID)
}

// SwarmList delegates to the wrapped implementation unless SwarmListErr is set.
func (s *BeadsStub) SwarmList() ([]*Issue, error) {
	if s.SwarmListErr != nil {
		return nil, s.SwarmListErr
	}
	return s.BeadsOps.SwarmList()
}

// SwarmValidate delegates to the wrapped implementation unless SwarmValidateErr is set.
func (s *BeadsStub) SwarmValidate(epicID string) error {
	if s.SwarmValidateErr != nil {
		return s.SwarmValidateErr
	}
	return s.BeadsOps.SwarmValidate(epicID)
}

// FormulaShow delegates to the wrapped implementation unless FormulaShowErr is set.
func (s *BeadsStub) FormulaShow(name string) (*FormulaDetails, error) {
	if s.FormulaShowErr != nil {
		return nil, s.FormulaShowErr
	}
	return s.BeadsOps.FormulaShow(name)
}

// FormulaList delegates to the wrapped implementation unless FormulaListErr is set.
func (s *BeadsStub) FormulaList() ([]*FormulaListEntry, error) {
	if s.FormulaListErr != nil {
		return nil, s.FormulaListErr
	}
	return s.BeadsOps.FormulaList()
}

// Cook delegates to the wrapped implementation unless CookErr is set.
func (s *BeadsStub) Cook(formulaName string) (*Issue, error) {
	if s.CookErr != nil {
		return nil, s.CookErr
	}
	return s.BeadsOps.Cook(formulaName)
}

// LegAdd delegates to the wrapped implementation unless LegAddErr is set.
func (s *BeadsStub) LegAdd(formulaID, stepName string) error {
	if s.LegAddErr != nil {
		return s.LegAddErr
	}
	return s.BeadsOps.LegAdd(formulaID, stepName)
}

// AgentState delegates to the wrapped implementation unless AgentStateErr is set.
func (s *BeadsStub) AgentState(beadID, state string) error {
	if s.AgentStateErr != nil {
		return s.AgentStateErr
	}
	return s.BeadsOps.AgentState(beadID, state)
}

// LabelAdd delegates to the wrapped implementation unless LabelAddErr is set.
func (s *BeadsStub) LabelAdd(id, label string) error {
	if s.LabelAddErr != nil {
		return s.LabelAddErr
	}
	return s.BeadsOps.LabelAdd(id, label)
}

// LabelRemove delegates to the wrapped implementation unless LabelRemoveErr is set.
func (s *BeadsStub) LabelRemove(id, label string) error {
	if s.LabelRemoveErr != nil {
		return s.LabelRemoveErr
	}
	return s.BeadsOps.LabelRemove(id, label)
}

// SlotShow delegates to the wrapped implementation unless SlotShowErr is set.
func (s *BeadsStub) SlotShow(id string) (*Slot, error) {
	if s.SlotShowErr != nil {
		return nil, s.SlotShowErr
	}
	return s.BeadsOps.SlotShow(id)
}

// SlotSet delegates to the wrapped implementation unless SlotSetErr is set.
func (s *BeadsStub) SlotSet(agentID, slotName, beadID string) error {
	if s.SlotSetErr != nil {
		return s.SlotSetErr
	}
	return s.BeadsOps.SlotSet(agentID, slotName, beadID)
}

// SlotClear delegates to the wrapped implementation unless SlotClearErr is set.
func (s *BeadsStub) SlotClear(agentID, slotName string) error {
	if s.SlotClearErr != nil {
		return s.SlotClearErr
	}
	return s.BeadsOps.SlotClear(agentID, slotName)
}

// Search delegates to the wrapped implementation unless SearchErr is set.
func (s *BeadsStub) Search(query string, opts SearchOptions) ([]*Issue, error) {
	if s.SearchErr != nil {
		return nil, s.SearchErr
	}
	return s.BeadsOps.Search(query, opts)
}

// MessageThread delegates to the wrapped implementation unless MessageThreadErr is set.
func (s *BeadsStub) MessageThread(threadID string) ([]*Issue, error) {
	if s.MessageThreadErr != nil {
		return nil, s.MessageThreadErr
	}
	return s.BeadsOps.MessageThread(threadID)
}

// Version delegates to the wrapped implementation unless VersionErr is set.
func (s *BeadsStub) Version() (string, error) {
	if s.VersionErr != nil {
		return "", s.VersionErr
	}
	return s.BeadsOps.Version()
}

// Doctor delegates to the wrapped implementation unless DoctorErr is set.
func (s *BeadsStub) Doctor() (*DoctorReport, error) {
	if s.DoctorErr != nil {
		return nil, s.DoctorErr
	}
	return s.BeadsOps.Doctor()
}

// Prime delegates to the wrapped implementation unless PrimeErr is set.
func (s *BeadsStub) Prime() (string, error) {
	if s.PrimeErr != nil {
		return "", s.PrimeErr
	}
	return s.BeadsOps.Prime()
}

// Stats delegates to the wrapped implementation unless StatsErr is set.
func (s *BeadsStub) Stats() (string, error) {
	if s.StatsErr != nil {
		return "", s.StatsErr
	}
	return s.BeadsOps.Stats()
}

// StatsJSON delegates to the wrapped implementation unless StatsErr is set.
func (s *BeadsStub) StatsJSON() (*RepoStats, error) {
	if s.StatsErr != nil {
		return nil, s.StatsErr
	}
	return s.BeadsOps.StatsJSON()
}

// Flush delegates to the wrapped implementation unless FlushErr is set.
func (s *BeadsStub) Flush() error {
	if s.FlushErr != nil {
		return s.FlushErr
	}
	return s.BeadsOps.Flush()
}

// Burn delegates to the wrapped implementation unless BurnErr is set.
func (s *BeadsStub) Burn(opts BurnOptions) error {
	if s.BurnErr != nil {
		return s.BurnErr
	}
	return s.BeadsOps.Burn(opts)
}

// Comment delegates to the wrapped implementation unless CommentErr is set.
func (s *BeadsStub) Comment(id, message string) error {
	if s.CommentErr != nil {
		return s.CommentErr
	}
	return s.BeadsOps.Comment(id, message)
}

// IsBeadsRepo delegates to the wrapped implementation.
func (s *BeadsStub) IsBeadsRepo() bool {
	return s.BeadsOps.IsBeadsRepo()
}

// Compile-time check that BeadsStub implements BeadsOps.
var _ BeadsOps = (*BeadsStub)(nil)
