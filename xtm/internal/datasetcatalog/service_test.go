package datasetcatalog

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/testrepo"
)

type fakeBackend struct {
	mu            sync.Mutex
	owned         []backend.Test
	basics        map[string]backend.TestBasic
	containers    []backend.Container
	links         []backend.ContainerLink
	execPlans     map[string][]string
	testDatasets  map[string]backend.Dataset
	planDatasets  map[string]backend.Dataset
	runDatasets   map[string]backend.Dataset
	basicCalls    [][]string
	testCalls     map[string]int
	planCalls     map[string]int
	runCalls      map[string]int
	execPlanCalls map[string]int
	probeErrors   map[string]error
	probeBlock    chan struct{}
	probeStarted  chan struct{}
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		basics: map[string]backend.TestBasic{}, execPlans: map[string][]string{},
		testDatasets: map[string]backend.Dataset{}, planDatasets: map[string]backend.Dataset{},
		runDatasets: map[string]backend.Dataset{}, testCalls: map[string]int{}, planCalls: map[string]int{},
		runCalls: map[string]int{}, execPlanCalls: map[string]int{}, probeErrors: map[string]error{},
	}
}

func (f *fakeBackend) SearchTestsPage(_ context.Context, _, _, _ string, startAt, maxResults int) ([]backend.Test, int, error) {
	end := startAt + maxResults
	if end > len(f.owned) {
		end = len(f.owned)
	}
	if startAt >= len(f.owned) {
		return nil, len(f.owned), nil
	}
	return append([]backend.Test(nil), f.owned[startAt:end]...), len(f.owned), nil
}

func (f *fakeBackend) ListTestsBasic(_ context.Context, keys []string) ([]backend.TestBasic, error) {
	f.mu.Lock()
	f.basicCalls = append(f.basicCalls, append([]string(nil), keys...))
	f.mu.Unlock()
	out := make([]backend.TestBasic, 0, len(keys))
	for _, key := range keys {
		out = append(out, f.basics[key])
	}
	return out, nil
}

func (f *fakeBackend) ListContainers(_ context.Context, _ string, _ func(done, total int)) ([]backend.Container, []backend.ContainerLink, error) {
	return append([]backend.Container(nil), f.containers...), append([]backend.ContainerLink(nil), f.links...), nil
}

func (f *fakeBackend) ExecPlans(_ context.Context, execKey string) ([]string, error) {
	f.mu.Lock()
	f.execPlanCalls[execKey]++
	f.mu.Unlock()
	return append([]string(nil), f.execPlans[execKey]...), nil
}

func (f *fakeBackend) GetDataset(_ context.Context, testKey string) (backend.Dataset, error) {
	f.mu.Lock()
	f.testCalls[testKey]++
	f.mu.Unlock()
	f.waitForProbeRelease()
	return f.testDatasets[testKey], f.probeErrors["test/"+testKey]
}

func (f *fakeBackend) GetPlanDataset(_ context.Context, planKey, testKey string) (backend.Dataset, error) {
	key := planKey + "/" + testKey
	f.mu.Lock()
	f.planCalls[key]++
	f.mu.Unlock()
	f.waitForProbeRelease()
	return f.planDatasets[key], f.probeErrors["plan/"+key]
}

func (f *fakeBackend) GetRunDataset(_ context.Context, execKey, testKey string) (backend.Dataset, error) {
	key := execKey + "/" + testKey
	f.mu.Lock()
	f.runCalls[key]++
	f.mu.Unlock()
	f.waitForProbeRelease()
	return f.runDatasets[key], f.probeErrors["execution/"+key]
}

func (f *fakeBackend) waitForProbeRelease() {
	if f.probeStarted != nil {
		f.probeStarted <- struct{}{}
	}
	if f.probeBlock != nil {
		<-f.probeBlock
	}
}

func catalogRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st)
}

func oneCell(name, value string) backend.Dataset {
	return backend.Dataset{Parameters: []backend.DatasetParameter{{Name: name}}, Rows: [][]string{{value}}}
}

func TestSyncCatalogEnumeratesOwnedAndReferencedTests(t *testing.T) {
	b := newFakeBackend()
	b.owned = []backend.Test{{Key: "QA-1", Summary: "Owned"}}
	b.basics["EXT-2"] = backend.TestBasic{Key: "EXT-2", Summary: "Referenced", ProjectKey: "EXT"}
	b.containers = []backend.Container{{Key: "QA-PLAN-1", Kind: backend.KindTestPlan, Summary: "Plan"}}
	b.links = []backend.ContainerLink{{ContainerKey: "QA-PLAN-1", TestKey: "EXT-2"}}
	b.testDatasets["QA-1"] = oneCell("A", "owned")
	b.planDatasets["QA-PLAN-1/EXT-2"] = oneCell("B", "external")
	repo := catalogRepo(t)

	if err := New(b, repo).Sync(context.Background(), "p1", "QA", nil); err != nil {
		t.Fatalf("sync: %v", err)
	}
	page, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{})
	if err != nil {
		t.Fatal(err)
	}
	gotKeys := []string{page.Tests[0].TestKey, page.Tests[1].TestKey}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, []string{"EXT-2", "QA-1"}) {
		t.Fatalf("catalog Test keys = %v", gotKeys)
	}
	if len(b.basicCalls) != 1 || !reflect.DeepEqual(b.basicCalls[0], []string{"EXT-2"}) {
		t.Fatalf("basic identity calls = %v, want only the referenced external Test", b.basicCalls)
	}
	if page.State.OwnedTestCount != 1 || page.State.ExternalTestCount != 1 || page.State.RelationshipCount != 1 {
		t.Fatalf("state = %+v, want exact discovery counts", page.State)
	}
}

func TestSyncCatalogProbesEachDefinitionOnce(t *testing.T) {
	b := newFakeBackend()
	b.owned = []backend.Test{{Key: "QA-1", Summary: "Owned"}}
	b.containers = []backend.Container{
		{Key: "QA-PLAN-1", Kind: backend.KindTestPlan},
		{Key: "QA-EXEC-1", Kind: backend.KindTestExec},
	}
	b.links = []backend.ContainerLink{
		{ContainerKey: "QA-PLAN-1", TestKey: "QA-1"}, {ContainerKey: "QA-PLAN-1", TestKey: "QA-1"},
		{ContainerKey: "QA-EXEC-1", TestKey: "QA-1"}, {ContainerKey: "QA-EXEC-1", TestKey: "QA-1"},
	}
	b.testDatasets["QA-1"] = oneCell("A", "1")
	b.planDatasets["QA-PLAN-1/QA-1"] = oneCell("A", "2")
	b.runDatasets["QA-EXEC-1/QA-1"] = oneCell("A", "3")
	repo := catalogRepo(t)

	if err := New(b, repo).Sync(context.Background(), "p1", "QA", nil); err != nil {
		t.Fatal(err)
	}
	if b.testCalls["QA-1"] != 1 || b.planCalls["QA-PLAN-1/QA-1"] != 1 || b.runCalls["QA-EXEC-1/QA-1"] != 1 {
		t.Fatalf("probe calls: test=%v plan=%v run=%v", b.testCalls, b.planCalls, b.runCalls)
	}
}

func TestSyncCatalogRecordsExecutionPlanLinks(t *testing.T) {
	b := newFakeBackend()
	b.owned = []backend.Test{{Key: "QA-1", Summary: "Owned"}}
	b.containers = []backend.Container{{Key: "QA-EXEC-1", Kind: backend.KindTestExec}}
	b.links = []backend.ContainerLink{{ContainerKey: "QA-EXEC-1", TestKey: "QA-1"}}
	b.execPlans["QA-EXEC-1"] = []string{"QA-PLAN-1", "QA-PLAN-2"}
	b.testDatasets["QA-1"] = oneCell("A", "1")
	repo := catalogRepo(t)

	if err := New(b, repo).Sync(context.Background(), "p1", "QA", nil); err != nil {
		t.Fatal(err)
	}
	detail, found, err := repo.GetDatasetCatalogDetail("p1", "QA-1")
	if err != nil || !found {
		t.Fatalf("detail found=%t err=%v", found, err)
	}
	if len(detail.ExecPlans) != 2 || b.execPlanCalls["QA-EXEC-1"] != 1 {
		t.Fatalf("links = %+v, ExecPlans calls = %v", detail.ExecPlans, b.execPlanCalls)
	}
}

func TestSyncCatalogPublishesOneTestGroupForInheritedUsages(t *testing.T) {
	b := newFakeBackend()
	b.owned = []backend.Test{{Key: "QA-1", Summary: "Owned"}}
	b.containers = []backend.Container{
		{Key: "QA-PLAN-1", Kind: backend.KindTestPlan, Summary: "Plan"},
		{Key: "QA-EXEC-1", Kind: backend.KindTestExec, Summary: "Execution"},
	}
	b.links = []backend.ContainerLink{
		{ContainerKey: "QA-PLAN-1", TestKey: "QA-1"},
		{ContainerKey: "QA-EXEC-1", TestKey: "QA-1"},
	}
	b.testDatasets["QA-1"] = oneCell("A", "default")
	repo := catalogRepo(t)

	if err := New(b, repo).Sync(context.Background(), "p1", "QA", nil); err != nil {
		t.Fatal(err)
	}
	page, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{})
	if err != nil || page.Total != 1 {
		t.Fatalf("page = %+v err=%v", page, err)
	}
	detail, found, err := repo.GetDatasetCatalogDetail("p1", "QA-1")
	if err != nil || !found || len(detail.Definitions) != 1 || len(detail.Usages) != 2 {
		t.Fatalf("detail = %+v found=%t err=%v", detail, found, err)
	}
	if page.Tests[0].OverrideCount != 0 || page.Tests[0].PlanCount != 1 || page.Tests[0].ExecutionCount != 1 {
		t.Fatalf("summary = %+v, want inherited usages without overrides", page.Tests[0])
	}
}

func TestSyncCatalogFailureDoesNotPublish(t *testing.T) {
	b := newFakeBackend()
	b.owned = []backend.Test{{Key: "QA-1"}}
	b.probeErrors["test/QA-1"] = errors.New("planned read failure")
	repo := catalogRepo(t)

	err := New(b, repo).Sync(context.Background(), "p1", "QA", nil)
	if err == nil || !strings.Contains(err.Error(), "planned read failure") {
		t.Fatalf("sync error = %v, want contextualized probe failure", err)
	}
	page, listErr := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{})
	if listErr != nil || page.State != nil || page.Total != 0 {
		t.Fatalf("catalog after failed sync = %+v, err=%v", page, listErr)
	}
}

func TestSyncCatalogCancellationDoesNotPublish(t *testing.T) {
	b := newFakeBackend()
	for i := 1; i <= 10; i++ {
		key := fmt.Sprintf("QA-%d", i)
		b.owned = append(b.owned, backend.Test{Key: key})
		b.testDatasets[key] = oneCell("A", key)
	}
	b.probeBlock = make(chan struct{})
	b.probeStarted = make(chan struct{}, 10)
	repo := catalogRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- New(b, repo).Sync(ctx, "p1", "QA", nil) }()
	for i := 0; i < 8; i++ {
		<-b.probeStarted
	}
	cancel()
	close(b.probeBlock)
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("sync error = %v, want context cancellation after late probe results", err)
	}
	page, listErr := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{})
	if listErr != nil || page.State != nil {
		t.Fatalf("catalog published after cancellation: %+v err=%v", page, listErr)
	}
}

type backendWithoutPlan struct{ source *fakeBackend }

func (b backendWithoutPlan) SearchTestsPage(ctx context.Context, projectKey, scopeJQL, since string, startAt, maxResults int) ([]backend.Test, int, error) {
	return b.source.SearchTestsPage(ctx, projectKey, scopeJQL, since, startAt, maxResults)
}
func (b backendWithoutPlan) ListTestsBasic(ctx context.Context, keys []string) ([]backend.TestBasic, error) {
	return b.source.ListTestsBasic(ctx, keys)
}
func (b backendWithoutPlan) ListContainers(ctx context.Context, projectKey string, progress func(int, int)) ([]backend.Container, []backend.ContainerLink, error) {
	return b.source.ListContainers(ctx, projectKey, progress)
}
func (b backendWithoutPlan) ExecPlans(ctx context.Context, execKey string) ([]string, error) {
	return b.source.ExecPlans(ctx, execKey)
}
func (b backendWithoutPlan) GetDataset(ctx context.Context, testKey string) (backend.Dataset, error) {
	return b.source.GetDataset(ctx, testKey)
}
func (b backendWithoutPlan) GetRunDataset(ctx context.Context, execKey, testKey string) (backend.Dataset, error) {
	return b.source.GetRunDataset(ctx, execKey, testKey)
}

func TestSyncCatalogRejectsMissingPlanReader(t *testing.T) {
	b := newFakeBackend()
	err := New(backendWithoutPlan{source: b}, catalogRepo(t)).Sync(context.Background(), "p1", "QA", nil)
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("sync error = %v, want backend.ErrUnsupported", err)
	}
}

func TestSyncCatalogDoesNotTouchPendingDatasetEdits(t *testing.T) {
	b := newFakeBackend()
	b.owned = []backend.Test{{Key: "QA-1", Summary: "Owned"}}
	b.testDatasets["QA-1"] = oneCell("REMOTE", "remote")
	repo := catalogRepo(t)
	before := testrepo.Dataset{Parameters: []testrepo.DatasetParameter{{Name: "LOCAL"}}, Rows: [][]string{{"before"}}}
	if err := repo.SaveTestDataset("p1", "QA-1", "", before); err != nil {
		t.Fatal(err)
	}
	edited := testrepo.Dataset{Parameters: []testrepo.DatasetParameter{{Name: "LOCAL"}}, Rows: [][]string{{"edited"}}}
	if err := repo.EditTestDataset("p1", "QA-1", edited); err != nil {
		t.Fatal(err)
	}
	pendingBefore, _ := repo.ListPendingChanges("p1")

	if err := New(b, repo).Sync(context.Background(), "p1", "QA", nil); err != nil {
		t.Fatal(err)
	}
	cached, found, err := repo.GetTestDataset("p1", "QA-1", "")
	if err != nil || !found || !reflect.DeepEqual(cached, edited) {
		t.Fatalf("editor dataset = %+v found=%t err=%v", cached, found, err)
	}
	pendingAfter, err := repo.ListPendingChanges("p1")
	if err != nil || !reflect.DeepEqual(pendingAfter, pendingBefore) {
		t.Fatalf("pending after sync = %+v, want %+v (err=%v)", pendingAfter, pendingBefore, err)
	}
}

func TestSyncCatalogProgressCountsUniqueProbes(t *testing.T) {
	b := newFakeBackend()
	b.owned = []backend.Test{{Key: "QA-1"}}
	b.containers = []backend.Container{{Key: "QA-PLAN-1", Kind: backend.KindTestPlan}, {Key: "QA-EXEC-1", Kind: backend.KindTestExec}}
	b.links = []backend.ContainerLink{
		{ContainerKey: "QA-PLAN-1", TestKey: "QA-1"}, {ContainerKey: "QA-PLAN-1", TestKey: "QA-1"},
		{ContainerKey: "QA-EXEC-1", TestKey: "QA-1"}, {ContainerKey: "QA-EXEC-1", TestKey: "QA-1"},
	}
	var progress []Progress
	if err := New(b, catalogRepo(t)).Sync(context.Background(), "p1", "QA", func(p Progress) {
		progress = append(progress, p)
	}); err != nil {
		t.Fatal(err)
	}
	if len(progress) < 4 || progress[0].Stage != "Finding tests" || progress[1].Stage != "Finding containers" {
		t.Fatalf("progress stages = %+v", progress)
	}
	last := progress[len(progress)-1]
	if last.Stage != "Reading datasets" || last.Total != 3 || last.Fetched != 3 {
		t.Fatalf("final progress = %+v, want three unique probes", last)
	}
}
