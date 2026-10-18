package testrepo_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"agile-suite/xtm/internal/testrepo"
)

var catalogSyncedAt = time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC)

func TestDatasetCatalogDTOsUseFrontendJSONNames(t *testing.T) {
	body, err := json.Marshal(testrepo.DatasetCatalogTestSummary{TestKey: "QA-1", InternalOpenable: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, `"testKey":"QA-1"`) || !strings.Contains(text, `"internalOpenable":true`) {
		t.Fatalf("summary JSON = %s, want frontend field names", text)
	}
	if strings.Contains(text, `"TestKey"`) || strings.Contains(text, `"InternalOpenable"`) {
		t.Fatalf("summary JSON = %s, contains Go field names", text)
	}
}

func catalogSnapshot() testrepo.DatasetCatalogSnapshot {
	return testrepo.DatasetCatalogSnapshot{
		OwnedTestCount:    2,
		ExternalTestCount: 1,
		RelationshipCount: 3,
		Tests: []testrepo.DatasetCatalogTest{
			{TestKey: "QA-1", Summary: "Login matrix", ProjectKey: "QA", InternalOpenable: true},
		},
		Definitions: []testrepo.DatasetCatalogDefinition{
			{
				TestKey: "QA-1", ScopeKind: testrepo.DatasetScopeTest,
				Parameters: []testrepo.DatasetParameter{{Name: "USER"}}, Rows: [][]string{{"alice"}},
				MetadataComplete: true,
			},
		},
	}
}

func publishCatalog(t *testing.T, repo *testrepo.Repository, snapshot testrepo.DatasetCatalogSnapshot) {
	t.Helper()
	if err := repo.PublishDatasetCatalog("p1", snapshot, catalogSyncedAt); err != nil {
		t.Fatalf("publish catalog: %v", err)
	}
}

func TestPublishDatasetCatalogListsOneTestWithManyUsages(t *testing.T) {
	repo := newRepo(t)
	snapshot := catalogSnapshot()
	snapshot.Usages = []testrepo.DatasetCatalogUsage{
		{TestKey: "QA-1", ContainerKind: "testplan", ContainerKey: "QA-PLAN-1", ContainerSummary: "Release plan"},
		{TestKey: "QA-1", ContainerKind: "testexec", ContainerKey: "QA-EXEC-1", ContainerSummary: "Nightly run"},
	}
	publishCatalog(t, repo, snapshot)

	page, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{})
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	if page.Total != 1 || len(page.Tests) != 1 {
		t.Fatalf("page = %+v, want one Test group", page)
	}
	got := page.Tests[0]
	if got.PlanCount != 1 || got.ExecutionCount != 1 || got.DefaultParameterCount != 1 || got.DefaultRowCount != 1 {
		t.Fatalf("summary = %+v, want both usages and default counts", got)
	}
	if page.State == nil || page.State.RelationshipCount != 3 || page.State.TestGroupCount != 1 || page.State.DefinitionCount != 1 {
		t.Fatalf("state = %+v, want published counts", page.State)
	}
}

func TestPublishDatasetCatalogKeepsAnOverrideOnlyTest(t *testing.T) {
	repo := newRepo(t)
	snapshot := catalogSnapshot()
	snapshot.Tests = []testrepo.DatasetCatalogTest{
		{TestKey: "EXT-2", Summary: "Partner matrix", ProjectKey: "EXT", External: true},
	}
	snapshot.Definitions = []testrepo.DatasetCatalogDefinition{
		{
			TestKey: "EXT-2", ScopeKind: testrepo.DatasetScopePlan, ContextKey: "QA-PLAN-2",
			ContextSummary: "Partner plan", Parameters: []testrepo.DatasetParameter{{Name: "REGION"}},
			Rows: [][]string{{"apac"}}, MetadataComplete: false,
		},
	}
	snapshot.Usages = []testrepo.DatasetCatalogUsage{
		{TestKey: "EXT-2", ContainerKind: "testplan", ContainerKey: "QA-PLAN-2", ContainerSummary: "Partner plan"},
	}
	publishCatalog(t, repo, snapshot)

	page, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Tests[0].TestKey != "EXT-2" || page.Tests[0].OverrideCount != 1 {
		t.Fatalf("page = %+v, want the override-only Test", page)
	}
	if page.Tests[0].DefaultParameterCount != 0 || !page.Tests[0].External || page.Tests[0].InternalOpenable {
		t.Fatalf("summary = %+v, want an external override-only Test", page.Tests[0])
	}
}

func TestPublishDatasetCatalogDeduplicatesLinks(t *testing.T) {
	repo := newRepo(t)
	snapshot := catalogSnapshot()
	usage := testrepo.DatasetCatalogUsage{TestKey: "QA-1", ContainerKind: "testexec", ContainerKey: "QA-EXEC-1"}
	definition := snapshot.Definitions[0]
	link := testrepo.DatasetCatalogExecPlan{ExecKey: "QA-EXEC-1", PlanKey: "QA-PLAN-1"}
	snapshot.Usages = []testrepo.DatasetCatalogUsage{usage, usage}
	snapshot.Definitions = []testrepo.DatasetCatalogDefinition{definition, definition}
	snapshot.ExecPlans = []testrepo.DatasetCatalogExecPlan{link, link}
	publishCatalog(t, repo, snapshot)

	detail, found, err := repo.GetDatasetCatalogDetail("p1", "QA-1")
	if err != nil || !found {
		t.Fatalf("detail found=%t err=%v", found, err)
	}
	if len(detail.Definitions) != 1 || len(detail.Usages) != 1 || len(detail.ExecPlans) != 1 {
		t.Fatalf("detail = %+v, want each repeated link once", detail)
	}
}

func TestPublishDatasetCatalogRejectsMalformedDefinitions(t *testing.T) {
	repo := newRepo(t)
	snapshot := catalogSnapshot()
	snapshot.Definitions[0].Parameters = []testrepo.DatasetParameter{{Name: "A"}, {Name: "B"}}
	snapshot.Definitions[0].Rows = [][]string{{"only-one-cell"}}

	err := repo.PublishDatasetCatalog("p1", snapshot, catalogSyncedAt)
	if err == nil || !strings.Contains(err.Error(), "row") {
		t.Fatalf("publish malformed definition error = %v, want row-width rejection", err)
	}
	page, listErr := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{})
	if listErr != nil || page.State != nil || page.Total != 0 {
		t.Fatalf("catalog after rejected publish = %+v, err=%v", page, listErr)
	}
}

func TestListDatasetCatalogRejectsInvalidQueryBeforeFirstSync(t *testing.T) {
	repo := newRepo(t)
	_, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{Scope: "project"})
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("invalid query error = %v, want scope rejection before any snapshot exists", err)
	}
}

func TestPublishDatasetCatalogFailureKeepsActiveGeneration(t *testing.T) {
	repo, st := newRepoAndStore(t)
	first := catalogSnapshot()
	publishCatalog(t, repo, first)
	if _, err := st.DB().Exec(`CREATE TRIGGER fail_catalog_generation_two BEFORE INSERT ON dataset_catalog_test
		WHEN NEW.generation = 2 BEGIN SELECT RAISE(ABORT, 'planned catalog failure'); END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	second := catalogSnapshot()
	second.Tests[0].Summary = "Must not publish"
	if err := repo.PublishDatasetCatalog("p1", second, catalogSyncedAt.Add(time.Hour)); err == nil {
		t.Fatal("second publish = nil error, want planned transaction failure")
	}

	page, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Tests[0].Summary != "Login matrix" || page.State.ActiveGeneration != 1 {
		t.Fatalf("page after failed publish = %+v, want generation one", page)
	}
}

func TestListDatasetCatalogFiltersAndPagesWholeTestGroups(t *testing.T) {
	repo := newRepo(t)
	snapshot := catalogSnapshot()
	snapshot.Tests = []testrepo.DatasetCatalogTest{
		{TestKey: "QA-1", Summary: "Alpha", ProjectKey: "QA", InternalOpenable: true},
		{TestKey: "QA-2", Summary: "Beta", ProjectKey: "QA", InternalOpenable: true},
		{TestKey: "EXT-3", Summary: "Gamma", ProjectKey: "EXT", External: true},
	}
	snapshot.Definitions = []testrepo.DatasetCatalogDefinition{
		{TestKey: "QA-1", ScopeKind: testrepo.DatasetScopeTest, Parameters: []testrepo.DatasetParameter{{Name: "A"}}, Rows: [][]string{{"1"}}, MetadataComplete: true},
		{TestKey: "QA-2", ScopeKind: testrepo.DatasetScopePlan, ContextKey: "QA-PLAN-2", ContextSummary: "Release beta", Parameters: []testrepo.DatasetParameter{{Name: "B"}}, Rows: [][]string{{"2"}}},
		{TestKey: "EXT-3", ScopeKind: testrepo.DatasetScopePlan, ContextKey: "QA-PLAN-3", ContextSummary: "Partner release", Parameters: []testrepo.DatasetParameter{{Name: "C"}}, Rows: [][]string{{"3"}}},
	}
	snapshot.Usages = []testrepo.DatasetCatalogUsage{
		{TestKey: "QA-2", ContainerKind: "testplan", ContainerKey: "QA-PLAN-2", ContainerSummary: "Release beta"},
		{TestKey: "EXT-3", ContainerKind: "testplan", ContainerKey: "QA-PLAN-3", ContainerSummary: "Partner release"},
	}
	publishCatalog(t, repo, snapshot)

	planPage, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{Scope: testrepo.DatasetScopePlan, Limit: 1, Offset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if planPage.Total != 2 || len(planPage.Tests) != 1 || planPage.Tests[0].TestKey != "QA-2" {
		t.Fatalf("paged Plan results = %+v, want second whole Test group", planPage)
	}
	external, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{Ownership: "external"})
	if err != nil || external.Total != 1 || external.Tests[0].TestKey != "EXT-3" {
		t.Fatalf("external results = %+v, err=%v", external, err)
	}
	searched, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{Search: "PLAN-2"})
	if err != nil || searched.Total != 1 || searched.Tests[0].TestKey != "QA-2" {
		t.Fatalf("container search = %+v, err=%v", searched, err)
	}
}

func TestDatasetCatalogListsAmbiguousPlanLinksWithoutAnEffectiveClaim(t *testing.T) {
	repo := newRepo(t)
	snapshot := catalogSnapshot()
	snapshot.Definitions = append(snapshot.Definitions,
		testrepo.DatasetCatalogDefinition{TestKey: "QA-1", ScopeKind: testrepo.DatasetScopePlan, ContextKey: "QA-PLAN-1", Parameters: []testrepo.DatasetParameter{{Name: "A"}}, Rows: [][]string{{"one"}}},
		testrepo.DatasetCatalogDefinition{TestKey: "QA-1", ScopeKind: testrepo.DatasetScopePlan, ContextKey: "QA-PLAN-2", Parameters: []testrepo.DatasetParameter{{Name: "A"}}, Rows: [][]string{{"two"}}},
	)
	snapshot.Usages = []testrepo.DatasetCatalogUsage{{TestKey: "QA-1", ContainerKind: "testexec", ContainerKey: "QA-EXEC-1"}}
	snapshot.ExecPlans = []testrepo.DatasetCatalogExecPlan{
		{ExecKey: "QA-EXEC-1", PlanKey: "QA-PLAN-1"},
		{ExecKey: "QA-EXEC-1", PlanKey: "QA-PLAN-2"},
	}
	publishCatalog(t, repo, snapshot)

	detail, found, err := repo.GetDatasetCatalogDetail("p1", "QA-1")
	if err != nil || !found {
		t.Fatalf("detail found=%t err=%v", found, err)
	}
	if len(detail.ExecPlans) != 2 || detail.ExecPlans[0].PlanKey == detail.ExecPlans[1].PlanKey {
		t.Fatalf("execution Plan links = %+v, want both distinct candidates", detail.ExecPlans)
	}
}

func TestPublishDatasetCatalogLeavesEditorAndJournalUntouched(t *testing.T) {
	repo := newRepo(t)
	original := testrepo.Dataset{Parameters: []testrepo.DatasetParameter{{Name: "LOCAL"}}, Rows: [][]string{{"before"}}}
	if err := repo.SaveTestDataset("p1", "QA-1", "", original); err != nil {
		t.Fatal(err)
	}
	edited := testrepo.Dataset{Parameters: []testrepo.DatasetParameter{{Name: "LOCAL"}}, Rows: [][]string{{"edited"}}}
	if err := repo.EditTestDataset("p1", "QA-1", edited); err != nil {
		t.Fatal(err)
	}
	pendingBefore, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatal(err)
	}
	publishCatalog(t, repo, catalogSnapshot())

	cached, found, err := repo.GetTestDataset("p1", "QA-1", "")
	if err != nil || !found || !reflect.DeepEqual(cached, edited) {
		t.Fatalf("editor cache = %+v found=%t err=%v, want local edit", cached, found, err)
	}
	pendingAfter, err := repo.ListPendingChanges("p1")
	if err != nil || !reflect.DeepEqual(pendingAfter, pendingBefore) {
		t.Fatalf("pending after publish = %+v, want %+v (err=%v)", pendingAfter, pendingBefore, err)
	}
}
