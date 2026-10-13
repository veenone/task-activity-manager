package testrepo_test

import (
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

func statusRows(t *testing.T, repo *testrepo.Repository) []testrepo.PendingChange {
	t.Helper()
	all, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	var out []testrepo.PendingChange
	for _, c := range all {
		if c.Field == "status" {
			out = append(out, c)
		}
	}
	return out
}

func TestTransitionFieldsRideInTheTransitionsOwnRow(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", Summary: "a", Status: "Open"}}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := repo.TransitionTestWithFields("p1", "QA-1", "Done",
		map[string]any{"resolution": map[string]any{"id": "1"}}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	rows := statusRows(t, repo)
	if len(rows) != 1 || rows[0].AfterVal != "Done" || rows[0].TransitionFields != `{"resolution":{"id":"1"}}` {
		t.Fatalf("rows %+v", rows)
	}

	if err := repo.DiscardPendingChange("p1", rows[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if rows := statusRows(t, repo); len(rows) != 0 {
		t.Fatalf("discard left %+v", rows)
	}
}

func TestTransitionWithoutFieldsStoresNone(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", Summary: "a", Status: "Open"}}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.TransitionTest("p1", "QA-1", "In Progress"); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if rows := statusRows(t, repo); len(rows) != 1 || rows[0].TransitionFields != "" {
		t.Fatalf("rows %+v", rows)
	}
}
