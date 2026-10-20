package main

import (
	"context"
	"strings"
	"testing"

	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

func TestListDatasetCatalogRejectsUnknownFilters(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	_, err := a.ListDatasetCatalog(profileID, testrepo.DatasetCatalogQuery{Scope: "project"})
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("unknown scope error = %v, want a useful validation error", err)
	}

	_, err = a.ListDatasetCatalog(profileID, testrepo.DatasetCatalogQuery{Ownership: "shared"})
	if err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatalf("unknown ownership error = %v, want a useful validation error", err)
	}
}

func TestGetDatasetCatalogDetailRejectsEmptyKey(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)

	_, err := a.GetDatasetCatalogDetail(profileID, "  ")
	if err == nil || !strings.Contains(err.Error(), "Test key") {
		t.Fatalf("empty key error = %v, want a useful validation error", err)
	}
}

func TestSyncDatasetCatalogEmitsProgressAndPublishesDemoSnapshot(t *testing.T) {
	a, profileID := newTestAppWithDemoProfile(t)
	a.ctx = context.Background()
	var progress []syncer.Progress
	a.emitEvent = func(_ context.Context, name string, data ...interface{}) {
		if name != "sync:progress" || len(data) != 1 {
			return
		}
		frame, ok := data[0].(syncer.Progress)
		if !ok {
			t.Fatalf("progress payload type = %T, want syncer.Progress", data[0])
		}
		progress = append(progress, frame)
	}

	if err := a.SyncDatasetCatalog(profileID); err != nil {
		t.Fatalf("sync dataset catalog: %v", err)
	}
	if len(progress) < 4 {
		t.Fatalf("progress frames = %+v, want discovery, reads, counts, and terminal state", progress)
	}
	if progress[0].Stage != "Finding tests" {
		t.Errorf("first stage = %q, want Finding tests", progress[0].Stage)
	}
	if last := progress[len(progress)-1]; !last.Done {
		t.Errorf("terminal progress = %+v, want Done", last)
	}
	foundCountedRead := false
	for _, frame := range progress {
		if frame.Stage == "Reading datasets" && frame.Total > 0 && frame.Fetched > 0 {
			foundCountedRead = true
			break
		}
	}
	if !foundCountedRead {
		t.Errorf("progress = %+v, want a counted Reading datasets frame", progress)
	}

	page, err := a.repo.ListDatasetCatalog(profileID, testrepo.DatasetCatalogQuery{})
	if err != nil {
		t.Fatalf("list published catalog: %v", err)
	}
	if page.State == nil || page.State.ActiveGeneration < 1 {
		t.Fatalf("catalog state = %+v, want a published generation", page.State)
	}
	if page.Total == 0 || len(page.Tests) == 0 {
		t.Fatalf("catalog page = %+v, want demo Tests", page)
	}
}
