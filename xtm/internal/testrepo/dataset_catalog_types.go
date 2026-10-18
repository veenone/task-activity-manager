package testrepo

const (
	DatasetScopeTest      = "test"
	DatasetScopePlan      = "plan"
	DatasetScopeExecution = "execution"
)

type DatasetCatalogSnapshot struct {
	OwnedTestCount    int                        `json:"ownedTestCount"`
	ExternalTestCount int                        `json:"externalTestCount"`
	RelationshipCount int                        `json:"relationshipCount"`
	Tests             []DatasetCatalogTest       `json:"tests"`
	Definitions       []DatasetCatalogDefinition `json:"definitions"`
	Usages            []DatasetCatalogUsage      `json:"usages"`
	ExecPlans         []DatasetCatalogExecPlan   `json:"execPlans"`
}

type DatasetCatalogTest struct {
	TestKey          string `json:"testKey"`
	Summary          string `json:"summary"`
	ProjectKey       string `json:"projectKey"`
	External         bool   `json:"external"`
	InternalOpenable bool   `json:"internalOpenable"`
}

type DatasetCatalogDefinition struct {
	TestKey          string             `json:"testKey"`
	ScopeKind        string             `json:"scopeKind"`
	ContextKey       string             `json:"contextKey"`
	ContextSummary   string             `json:"contextSummary"`
	Parameters       []DatasetParameter `json:"parameters"`
	Rows             [][]string         `json:"rows"`
	MetadataComplete bool               `json:"metadataComplete"`
}

type DatasetCatalogUsage struct {
	TestKey          string `json:"testKey"`
	ContainerKind    string `json:"containerKind"`
	ContainerKey     string `json:"containerKey"`
	ContainerSummary string `json:"containerSummary"`
}

type DatasetCatalogExecPlan struct {
	ExecKey string `json:"execKey"`
	PlanKey string `json:"planKey"`
}

type DatasetCatalogState struct {
	ActiveGeneration  int    `json:"activeGeneration"`
	LastSyncedAt      string `json:"lastSyncedAt"`
	OwnedTestCount    int    `json:"ownedTestCount"`
	ExternalTestCount int    `json:"externalTestCount"`
	RelationshipCount int    `json:"relationshipCount"`
	TestGroupCount    int    `json:"testGroupCount"`
	DefinitionCount   int    `json:"definitionCount"`
}

type DatasetCatalogQuery struct {
	Search    string `json:"search"`
	Scope     string `json:"scope"`
	Ownership string `json:"ownership"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

type DatasetCatalogTestSummary struct {
	TestKey               string `json:"testKey"`
	Summary               string `json:"summary"`
	ProjectKey            string `json:"projectKey"`
	External              bool   `json:"external"`
	InternalOpenable      bool   `json:"internalOpenable"`
	DefaultParameterCount int    `json:"defaultParameterCount"`
	DefaultRowCount       int    `json:"defaultRowCount"`
	PlanCount             int    `json:"planCount"`
	ExecutionCount        int    `json:"executionCount"`
	OverrideCount         int    `json:"overrideCount"`
}

type DatasetCatalogPage struct {
	State *DatasetCatalogState        `json:"state"`
	Tests []DatasetCatalogTestSummary `json:"tests"`
	Total int                         `json:"total"`
}

type DatasetCatalogDetail struct {
	Test        DatasetCatalogTestSummary  `json:"test"`
	Definitions []DatasetCatalogDefinition `json:"definitions"`
	Usages      []DatasetCatalogUsage      `json:"usages"`
	ExecPlans   []DatasetCatalogExecPlan   `json:"execPlans"`
}
