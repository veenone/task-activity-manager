package syncer_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

// countingBackend is the demo Xray backend with the two writes a large commit
// makes most (field edits and run results) counted per key, so a test can see
// a row pushed twice or not at all. failUpdate keys answer UpdateIssue with an
// error, the way a Jira validation failure would.
type countingBackend struct {
	backend.Backend
	mu         sync.Mutex
	updates    map[string]int
	runs       map[string]int
	failUpdate map[string]bool
}

func newCountingBackend(fail ...string) *countingBackend {
	b := &countingBackend{
		Backend:    xray.New(jira.NewClient("demo", "tok")),
		updates:    map[string]int{},
		runs:       map[string]int{},
		failUpdate: map[string]bool{},
	}
	for _, k := range fail {
		b.failUpdate[k] = true
	}
	return b
}

func (b *countingBackend) RemoteVersion(context.Context, string, string) (backend.VersionToken, error) {
	return "", nil
}

func (b *countingBackend) UpdateIssue(_ context.Context, key string, _ map[string]any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.updates[key]++
	if b.failUpdate[key] {
		return errors.New("Jira rejected the edit")
	}
	return nil
}

func (b *countingBackend) SetTestRunStatus(_ context.Context, execKey, testKey, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runs[execKey+":"+testKey]++
	return nil
}

// seedLargeCommit journals 60 summary edits (QA-1..QA-60) and 40 run results
// (QA-1..QA-40 in QA-TE-1): 100 pending rows.
func seedLargeCommit(t *testing.T) *testrepo.Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	repo := testrepo.NewRepository(st)

	tests := make([]testrepo.TestCase, 0, 60)
	links := make([]testrepo.ContainerLink, 0, 40)
	for i := 1; i <= 60; i++ {
		key := fmt.Sprintf("QA-%d", i)
		tests = append(tests, testrepo.TestCase{Key: key, ID: fmt.Sprint(i), Summary: "old", Status: "Open"})
		if i <= 40 {
			links = append(links, testrepo.ContainerLink{ContainerKey: "QA-TE-1", TestKey: key, RunStatus: "EXECUTING"})
		}
	}
	if err := repo.UpsertTests("p1", tests); err != nil {
		t.Fatalf("seed tests: %v", err)
	}
	if err := repo.UpsertContainers("p1", []testrepo.Container{{Key: "QA-TE-1", Kind: "testexec", Summary: "Cycle 1"}}); err != nil {
		t.Fatalf("seed exec: %v", err)
	}
	if err := repo.ReplaceAllContainerLinks("p1", links); err != nil {
		t.Fatalf("seed links: %v", err)
	}
	for _, tc := range tests {
		if err := repo.EditTestField("p1", tc.Key, "summary", "new "+tc.Key); err != nil {
			t.Fatalf("edit %s: %v", tc.Key, err)
		}
	}
	for _, l := range links {
		if err := repo.SetTestRunStatus("p1", "QA-TE-1", l.TestKey, "PASS"); err != nil {
			t.Fatalf("run %s: %v", l.TestKey, err)
		}
	}
	pending, err := repo.ListPendingChanges("p1")
	if err != nil || len(pending) != 100 {
		t.Fatalf("seeded %d pending rows (err %v), want 100", len(pending), err)
	}
	return repo
}

// progressLog collects commit progress events in arrival order.
type progressLog struct {
	mu     sync.Mutex
	events []syncer.Progress
}

func (l *progressLog) add(p syncer.Progress) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, p)
}

// checkCountsUp asserts every event carries the same total, done never goes
// backwards, and the last event reports done == total.
func (l *progressLog) checkCountsUp(t *testing.T, total int) {
	t.Helper()
	if len(l.events) == 0 {
		t.Fatal("commit emitted no progress events")
	}
	prev := 0
	for i, p := range l.events {
		if p.Total != total {
			t.Fatalf("event %d total = %d, want %d", i, p.Total, total)
		}
		if p.Fetched < prev || p.Fetched > total {
			t.Fatalf("event %d done = %d after %d (total %d)", i, p.Fetched, prev, total)
		}
		prev = p.Fetched
	}
	if last := l.events[len(l.events)-1]; last.Fetched != total {
		t.Fatalf("last event done = %d, want %d", last.Fetched, total)
	}
}

func TestCommitOfAHundredRowsPushesEachOnceAndReportsProgress(t *testing.T) {
	repo := seedLargeCommit(t)
	b := newCountingBackend("QA-7", "QA-17")
	log := &progressLog{}
	eng := syncer.New(b, repo, syncer.WithCommitProgress(log.add))

	result, err := eng.CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}

	for i := 1; i <= 60; i++ {
		key := fmt.Sprintf("QA-%d", i)
		if b.updates[key] != 1 {
			t.Errorf("%s field edit pushed %d times, want 1", key, b.updates[key])
		}
		if i <= 40 && b.runs["QA-TE-1:"+key] != 1 {
			t.Errorf("%s run result pushed %d times, want 1", key, b.runs["QA-TE-1:"+key])
		}
	}
	failed := make([]string, 0, len(result.Failed))
	for _, f := range result.Failed {
		failed = append(failed, f.TestKey)
	}
	sort.Strings(failed)
	if fmt.Sprint(failed) != "[QA-17 QA-7]" {
		t.Errorf("failed = %v, want [QA-17 QA-7]", failed)
	}

	left, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	kept := make([]string, 0, len(left))
	for _, c := range left {
		kept = append(kept, c.EntityKey)
	}
	sort.Strings(kept)
	if fmt.Sprint(kept) != "[QA-17 QA-7]" {
		t.Errorf("pending after commit = %v, want only the two failed edits", kept)
	}

	log.checkCountsUp(t, 100)
}

func TestSelectiveCommitProgressCountsOnlyTheSelection(t *testing.T) {
	repo := seedLargeCommit(t)
	pending, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	ids := make([]int64, 0, 30)
	for _, c := range pending[:30] {
		ids = append(ids, c.ID)
	}
	log := &progressLog{}
	eng := syncer.New(newCountingBackend(), repo, syncer.WithCommitProgress(log.add))

	if _, err := eng.CommitChangesForIDs(context.Background(), "p1", "QA", ids); err != nil {
		t.Fatalf("CommitChangesForIDs: %v", err)
	}

	log.checkCountsUp(t, 30)
	left, _ := repo.ListPendingChanges("p1")
	if len(left) != 70 {
		t.Errorf("pending after selective commit = %d, want 70", len(left))
	}
}
