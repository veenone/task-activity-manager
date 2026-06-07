package testrepo_test

import (
	"path/filepath"
	"testing"

	"xray-test-manager/internal/store"
	"xray-test-manager/internal/testrepo"
)

func newSyncLogRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st)
}

func TestRecordAndListSyncLog(t *testing.T) {
	repo := newSyncLogRepo(t)

	if err := repo.RecordSyncLog("p1", "2026-06-07T10:00:00Z", "2026-06-07T10:00:05Z", "success", 4812, ""); err != nil {
		t.Fatalf("record success: %v", err)
	}
	if err := repo.RecordSyncLog("p1", "2026-06-07T11:00:00Z", "2026-06-07T11:00:01Z", "error", 0, "connection refused"); err != nil {
		t.Fatalf("record error: %v", err)
	}

	entries, err := repo.ListSyncLog("p1", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	// Newest first.
	if entries[0].Outcome != "error" || entries[0].Error != "connection refused" {
		t.Errorf("newest entry = %+v, want the error run first", entries[0])
	}
	if entries[1].Outcome != "success" || entries[1].Fetched != 4812 {
		t.Errorf("oldest entry = %+v, want the success run with 4812 fetched", entries[1])
	}
}

func TestListSyncLogIsProfileScoped(t *testing.T) {
	repo := newSyncLogRepo(t)
	if err := repo.RecordSyncLog("p1", "2026-06-07T10:00:00Z", "2026-06-07T10:00:05Z", "success", 1, ""); err != nil {
		t.Fatalf("record: %v", err)
	}

	entries, _ := repo.ListSyncLog("p2", 10)
	if len(entries) != 0 {
		t.Errorf("p2 should see no sync history from p1, got %d", len(entries))
	}
}
