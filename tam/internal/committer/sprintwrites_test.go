package committer_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/sprints"
)

// fakeSprints is the SprintWriter seam: every push it was handed, in order,
// the error each sprint id answers with, and the completion each sprint id
// reports.
type fakeSprints struct {
	calls []string
	errs  map[int]error
	done  map[int]sprints.Completion
}

func (f *fakeSprints) Start(_ context.Context, _ string, boardID, sprintID int, d backend.SprintDraft) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("start %d on %d: %s %s", sprintID, boardID, d.Name, d.StartDate))
	return "", f.errs[sprintID]
}

func (f *fakeSprints) Complete(_ context.Context, _ string, boardID, sprintID int, moveTo string) (sprints.Completion, error) {
	f.calls = append(f.calls, fmt.Sprintf("complete %d on %d to %q", sprintID, boardID, moveTo))
	return f.done[sprintID], f.errs[sprintID]
}

func (f *fakeSprints) Edit(_ context.Context, _ string, boardID, sprintID int, d backend.SprintDraft, clearGoal bool) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("edit %d on %d: %s %q %s %s clear=%v", sprintID, boardID, d.Name, d.Goal, d.StartDate, d.EndDate, clearGoal))
	return "", f.errs[sprintID]
}

func (f *fakeSprints) Delete(_ context.Context, _ string, boardID, sprintID int) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("delete %d on %d", sprintID, boardID))
	return "the board could not be re-read", f.errs[sprintID]
}

// withSprints caches future sprints 20 and 21 on board 1 and wires the seam.
func withSprints(t *testing.T) (harness, *fakeSprints) {
	t.Helper()
	h := newHarness(t)
	for _, id := range []int{20, 21} {
		if _, err := h.db.Exec(`INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date, goal)
			VALUES ('p1', ?, 1, ?, 'future', 's', 'e', 'g')`, id, fmt.Sprintf("Sprint %d", id)); err != nil {
			t.Fatal(err)
		}
	}
	f := &fakeSprints{errs: map[int]error{}, done: map[int]sprints.Completion{}}
	h.eng.Sprints = f
	return h, f
}

func editTo(name string) issuerepo.SprintEdit {
	return issuerepo.SprintEdit{BoardID: 1, Name: name, StartDate: "2026-09-02T09:00:00.000+0000", EndDate: "2026-09-16T09:00:00.000+0000", ClearGoal: true}
}

func TestCommitPushesSprintEditsThenDeletesAndClearsTheirRows(t *testing.T) {
	h, f := withSprints(t)
	ctx := context.Background()
	// The delete is journaled first; edits still go first.
	if err := h.repo.JournalSprintDelete(ctx, "p1", 1, 21); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.JournalSprintEdit(ctx, "p1", 20, editTo("Sprint 20b")); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`edit 20 on 1: Sprint 20b "" 2026-09-02T09:00:00.000+0000 2026-09-16T09:00:00.000+0000 clear=true`,
		"delete 21 on 1",
	}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("pushes =\n%s\nwant\n%s", strings.Join(f.calls, "\n"), strings.Join(want, "\n"))
	}
	if strings.Join(res.SprintsChanged, ", ") != "Sprint 20b edited, Sprint 21 deleted" {
		t.Errorf("sprints changed = %v", res.SprintsChanged)
	}
	if len(res.Failures) != 0 || res.Remaining != 0 || len(res.Committed) != 0 {
		t.Errorf("nothing left, and no issue counted: %+v", res)
	}
}

func TestASprintWriteTheSprintRefusesFailsForGoodAndKeepsTheRow(t *testing.T) {
	h, f := withSprints(t)
	ctx := context.Background()
	if err := h.repo.JournalSprintEdit(ctx, "p1", 20, editTo("Sprint 20b")); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.JournalSprintDelete(ctx, "p1", 1, 21); err != nil {
		t.Fatal(err)
	}
	f.errs[20] = fmt.Errorf("sprint 20 is closed: %w", sprints.ErrRefused)
	f.errs[21] = errors.New("502 Bad Gateway")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 2 || res.Remaining != 2 || len(res.SprintsChanged) != 0 {
		t.Fatalf("both fail and stay: %+v", res)
	}
	edit, del := res.Failures[0], res.Failures[1]
	editRow, _ := h.repo.PendingForKey(ctx, "p1", "20")
	if edit.EntityType != issuerepo.EntitySprintEdit || edit.Retryable || edit.RowID != editRow[0].ID || edit.Key != "Sprint 20b" || !strings.Contains(edit.Error, "closed") {
		t.Errorf("the refused edit = %+v", edit)
	}
	if del.EntityType != issuerepo.EntitySprintDelete || !del.Retryable || del.RowID == 0 || !strings.Contains(del.Error, "502") {
		t.Errorf("the failed delete is worth retrying: %+v", del)
	}
}

func TestSprintWritesFailWithoutTheSeam(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.db.Exec(`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 20, 1, 'Sprint 20', 'future')`); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.JournalSprintDelete(ctx, "p1", 1, 20); err != nil {
		t.Fatal(err)
	}
	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Error != "this connection cannot manage sprints" || res.Failures[0].Retryable || res.Remaining != 1 {
		t.Errorf("no seam = %+v", res)
	}
}
