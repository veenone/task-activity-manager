package main

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/importer"
	"agile-suite/tam/internal/tamstore"
)

// TestImportFailsWhenTheOpenSprintsReadFails pins the honest half of a
// broken sprint read: ImportIssues has to fail the whole import rather than
// hand back a Result whose Sprint rows all say "no synced boards", which is
// what happened before this fix no matter why the read actually failed.
//
// The seam is the boards repository's own store, swapped for one already
// closed: a.repo stays on the store newTestApp built, so the requirement
// type read above OpenSprints in ImportIssues still succeeds and the
// failure pinned here is OpenSprints' alone.
func TestImportFailsWhenTheOpenSprintsReadFails(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	boardsStore, err := tamstore.Open(filepath.Join(t.TempDir(), "boards.db"))
	if err != nil {
		t.Fatalf("open boards store: %v", err)
	}
	a.boards = boardrepo.New(boardsStore.DB())
	if err := boardsStore.Close(); err != nil {
		t.Fatalf("close boards store: %v", err)
	}

	content := base64.StdEncoding.EncodeToString([]byte("Type,Summary,Sprint\nTask,Into a sprint,Sprint 12\n"))
	m := importer.AutoMap([]string{"Type", "Summary", "Sprint"})
	res, err := a.ImportIssues(p.ID, content, false, "plan.csv", m, true)
	if err == nil {
		t.Fatalf("ImportIssues with a failed sprint read = %+v, nil error; want an error and no rows", res)
	}
	if !strings.Contains(err.Error(), "open sprints") {
		t.Errorf("err = %q, want it to name the open sprints read", err)
	}
	if len(res.Errors) != 0 || len(res.Created) != 0 {
		t.Errorf("a failed read must not hand back rows: %+v", res)
	}
}
