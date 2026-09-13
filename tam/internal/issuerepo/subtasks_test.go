package issuerepo_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

func TestCreateSubtaskUnderEachStandardIssueType(t *testing.T) {
	for _, typ := range []string{backend.TypeTask, backend.TypeStory, backend.TypeBug, backend.TypeRequirement} {
		t.Run(typ, func(t *testing.T) {
			r := newRepo(t)
			ctx := context.Background()
			if err := r.UpsertPage(ctx, "p1", []backend.Issue{{Key: "PLAT-1", Type: typ}}, time.Now(), false); err != nil {
				t.Fatal(err)
			}
			key, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeSubtask, Summary: "Wire validation", ParentKey: " PLAT-1 "})
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.GetIssue(ctx, "p1", key)
			if err != nil || got.Type != backend.TypeSubtask || got.ParentKey != "PLAT-1" || !got.Draft {
				t.Fatalf("draft = %+v, error = %v", got, err)
			}
			pending, err := r.ListPendingChanges(ctx, "p1")
			if err != nil || len(pending) != 1 {
				t.Fatalf("pending = %+v, error = %v", pending, err)
			}
			var draft backend.IssueDraft
			if err := json.Unmarshal([]byte(pending[0].AfterVal), &draft); err != nil {
				t.Fatal(err)
			}
			if draft.Type != backend.TypeSubtask || draft.ParentKey != "PLAT-1" {
				t.Fatalf("commit payload = %+v", draft)
			}
		})
	}
}

func TestCreateSubtaskRejectsInvalidParents(t *testing.T) {
	for _, tc := range []struct{ name, typ, parent, message string }{
		{"missing", "task", "", "needs a parent"},
		{"whitespace", "task", "  ", "needs a parent"},
		{"uncached", "task", "PLAT-404", "not in the cache"},
		{"epic", "epic", "PLAT-1", "is an epic"},
		{"nested", "subtask", "PLAT-1", "cannot nest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRepo(t)
			ctx := context.Background()
			if err := r.UpsertPage(ctx, "p1", []backend.Issue{{Key: "PLAT-1", Type: tc.typ}}, time.Now(), false); err != nil {
				t.Fatal(err)
			}
			_, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeSubtask, Summary: "Invalid", ParentKey: tc.parent})
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error = %v, want %s", err, tc.message)
			}
			pending, err := r.ListPendingChanges(ctx, "p1")
			if err != nil || len(pending) != 0 {
				t.Fatalf("invalid create left pending work: %+v, %v", pending, err)
			}
		})
	}
}
