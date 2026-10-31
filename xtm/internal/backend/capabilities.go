// The capability declaration every backend answers with.
//
// It lives beside the interface rather than inside backend.go because it is a
// DTO the frontend reads, not part of the contract a backend implements, and
// because backend.go had reached the 400-line ceiling C2 sets.

package backend

// DefaultRunStatuses is Xray's Test Run result vocabulary, and the fallback
// for a backend that declares none of its own. The frontend mirrors it in
// components/containers/constants.ts for the pre-load default.
var DefaultRunStatuses = []string{"TODO", "EXECUTING", "PASS", "FAIL", "ABORTED", "BLOCKED"}

// Capabilities describes what a concrete backend supports so the app and sync
// engine can gate features. Full capability gating is a later phase; this
// struct is the minimal surface introduced now.
type Capabilities struct {
	// Name identifies the backend for diagnostics (e.g. "xray").
	Name string `json:"name"`

	// IDStyle is how entity ids are shaped ("opaque" for Xray issue keys).
	IDStyle string `json:"idStyle"`

	// SupportsJQLScope reports whether saved-scope queries use JQL.
	SupportsJQLScope bool `json:"supportsJqlScope"`

	// StepModel is how test steps are represented ("objects" for Xray steps).
	StepModel string `json:"stepModel"`

	SupportsTestTypes bool `json:"supportsTestTypes"`
	// SupportsFolders advertises that the backend can REPORT a folder
	// structure. It does not promise the folders can be created, renamed or
	// moved between: Kiwi surfaces its per-product Categories as a flat set of
	// folders it can read but not reshape, so the two questions had to
	// separate. Callers that offer folder editing gate on
	// SupportsFolderWrites; everything that only displays folders gates here.
	SupportsFolders bool `json:"supportsFolders"`
	// SupportsFolderWrites advertises create / rename / delete / move-test.
	// Always false where SupportsFolders is false.
	SupportsFolderWrites        bool `json:"supportsFolderWrites"`
	SupportsPreconditionObjects bool `json:"supportsPreconditionObjects"`
	SupportsRequirementObjects  bool `json:"supportsRequirementObjects"`
	SupportsIssueLinkTypes      bool `json:"supportsIssueLinkTypes"`
	SupportsEnvironments        bool `json:"supportsEnvironments"`
	SupportsContainers          bool `json:"supportsContainers"`

	// ContainerKinds are the container kind identifiers the backend supports.
	ContainerKinds []string `json:"containerKinds"`

	SupportsTestRuns bool `json:"supportsTestRuns"`

	// RunStatuses is the result vocabulary this backend accepts, in the order
	// to offer it in. A backend that declares none is offered
	// DefaultRunStatuses, which is also what the UI shows while a profile's
	// capabilities are still loading.
	//
	// It exists because the vocabulary is not shared. Zephyr configures it per
	// instance and reads it at connect, Kiwi has its own names, and the result
	// dropdown offered Xray's six on every profile: a user could pick a result
	// their backend has no id for and only find out at commit (#371 Phase 5).
	RunStatuses []string `json:"runStatuses"`

	// StatusModel is how statuses transition ("workflow" for Jira).
	StatusModel string `json:"statusModel"`

	SupportsWorkflowTransitions bool `json:"supportsWorkflowTransitions"`
	SupportsBugCreation         bool `json:"supportsBugCreation"`
	SupportsBugLinks            bool `json:"supportsBugLinks"`
	// SupportsBugRouting reports that this profile files its defects into a
	// SEPARATE backend: a Kiwi workspace with a Jira bug connection
	// (RND_P_4TFINT_05-359). It is a property of the profile's configuration,
	// not of the adapter, so no adapter's Capabilities() sets it; app.go's
	// GetCapabilities merges it in.
	//
	// SupportsBugCreation stays whatever the primary adapter reports. Kiwi
	// still cannot create a bug; something else does it on Kiwi's behalf, and
	// flipping the adapter's own flag would make it lie.
	SupportsBugRouting bool `json:"supportsBugRouting"`
	SupportsTags       bool `json:"supportsTags"`

	// SupportsDatasets: the backend models parameterized tests as a dataset
	// (parameters and rows) that GetDataset can read. It reports that the
	// backend HAS the concept, not that this instance permits it: Xray gained
	// the dataset REST endpoints in 5.1 and exposes no version endpoint, so an
	// older instance answers 404 and reads as "no dataset".
	SupportsDatasets bool `json:"supportsDatasets"`

	// SupportsDatasetWrites: the backend can create and replace a test's
	// dataset, not only read it. Separate from SupportsDatasets so a backend
	// that models the concept read-only keeps showing datasets while the
	// editor stays away, and so the commit path can skip a queued dataset
	// change rather than failing it (#277 W1).
	SupportsDatasetWrites bool `json:"supportsDatasetWrites"`

	SupportsParameterizedRunDetail bool `json:"supportsParameterizedRunDetail"`
	SupportsParameterizedRunWrites bool `json:"supportsParameterizedRunWrites"`
	// StepWrites is separate because Xray takes an iteration result but not a
	// step one, and reports each step carrying its iteration's status: the
	// commit path must ask before it compares values or it raises a conflict no
	// resolution can clear (#369).
	SupportsParameterizedRunStepWrites bool `json:"supportsParameterizedRunStepWrites"`

	// SupportsComponentAdmin: the backend can create, edit and delete project
	// components (it implements ComponentManager).
	SupportsComponentAdmin bool `json:"supportsComponentAdmin"`
}
