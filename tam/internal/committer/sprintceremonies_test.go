package committer_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/sprints"
)

func startAs(name string) issuerepo.SprintStart {
	return issuerepo.SprintStart{BoardID: 1, Name: name, StartDate: "2026-09-14T09:00:00.000+0000", EndDate: "2026-09-28T09:00:00.000+0000"}
}

// withActive22 adds an active sprint 22 on board 1 to withSprints.
func withActive22(t *testing.T) (harness, *fakeSprints) {
	t.Helper()
	h, f := withSprints(t)
	if _, err := h.db.Exec(`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 22, 1, 'Sprint 22', 'active')`); err != nil {
		t.Fatal(err)
	}
	return h, f
}

// Journalled in the opposite order, pushed edit, start, complete, delete.
func TestSprintChangesArePushedEditsStartsCompletionsThenDeletes(t *testing.T) {
	h, f := withActive22(t)
	ctx := context.Background()
	if err := h.repo.JournalSprintDelete(ctx, "p1", 1, 21); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.JournalSprintComplete(ctx, "p1", 22, issuerepo.SprintComplete{BoardID: 1, MoveTo: "20"}); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.JournalSprintStart(ctx, "p1", 20, startAs("Sprint 20")); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.JournalSprintEdit(ctx, "p1", 20, editTo("Sprint 20b")); err != nil {
		t.Fatal(err)
	}
	f.done[22] = sprints.Completion{Moved: 4, MovedTo: "Sprint 20b"}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, c := range f.calls {
		kinds = append(kinds, strings.SplitN(c, " on ", 2)[0])
	}
	if got := strings.Join(kinds, ", "); got != "edit 20, start 20, complete 22, delete 21" {
		t.Errorf("push order = %s", got)
	}
	want := "Sprint 20b edited, Sprint 20 started, Sprint 22 completed, 4 unfinished cards moved to Sprint 20b, Sprint 21 deleted"
	if got := strings.Join(res.SprintsChanged, ", "); got != want {
		t.Errorf("sprints changed = %q\nwant %q", got, want)
	}
	if len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("commit = %+v", res)
	}
}

// A draft sprint created and started in one Commit is started under the id
// Jira gave it.
func TestADraftSprintIsStartedUnderItsRealIdInTheSameCommit(t *testing.T) {
	h, f := withSprints(t)
	ctx := context.Background()
	draft, err := h.repo.CreateDraftSprint(ctx, "p1", draftSprint15())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.JournalSprintStart(ctx, "p1", draft.ID, startAs("Sprint 15")); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CreatedSprints) != 1 {
		t.Fatalf("commit = %+v", res)
	}
	want := "start " + strconv.Itoa(res.CreatedSprints[0].ID) + " on " + strconv.Itoa(demoBoard) + ": Sprint 15 2026-09-14T09:00:00.000+0000"
	if strings.Join(f.calls, "|") != want {
		t.Errorf("pushes = %v, want %q", f.calls, want)
	}
	if len(res.Failures) != 0 || len(res.Held) != 0 || res.Remaining != 0 || strings.Join(res.SprintsChanged, "") != "Sprint 15 started" {
		t.Errorf("commit = %+v", res)
	}
}

// The draft's create failed, so its start waits for the next Commit.
func TestAStartOfADraftSprintWhoseCreateFailedIsHeld(t *testing.T) {
	h, f := withSprints(t)
	ctx := context.Background()
	draft, err := h.repo.CreateDraftSprint(ctx, "p1", draftSprint15())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.JournalSprintStart(ctx, "p1", draft.ID, startAs("Sprint 15")); err != nil {
		t.Fatal(err)
	}
	h.jira.sprintCreateErr = errors.New("503 Service Unavailable")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 {
		t.Errorf("a start was pushed for a sprint Jira does not hold: %v", f.calls)
	}
	key := strconv.Itoa(draft.ID)
	if len(res.Held) != 1 || res.Held[0].Key != key || res.Held[0].EntityType != issuerepo.EntitySprintStart || !strings.Contains(res.Held[0].Reason, "Sprint 15") {
		t.Errorf("held = %+v", res.Held)
	}
	if res.Remaining != 2 {
		t.Errorf("the create and the start both wait: %d left", res.Remaining)
	}
}

// Cards moved and the close failed: the failure names the moved cards, is
// worth retrying, and keeps the row so the retry closes the sprint.
func TestACompletionThatStoppedAfterMovingCardsKeepsItsRow(t *testing.T) {
	h, f := withActive22(t)
	ctx := context.Background()
	if err := h.repo.JournalSprintComplete(ctx, "p1", 22, issuerepo.SprintComplete{BoardID: 1}); err != nil {
		t.Fatal(err)
	}
	f.done[22] = sprints.Completion{Moved: 2, MovedTo: "the backlog",
		Message: "2 of 2 unfinished issues moved to the backlog (PLAT-1, PLAT-2), but the sprint could not be closed and is open with none of them in it: 403 Forbidden"}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || len(res.SprintsChanged) != 0 || res.Remaining != 1 {
		t.Fatalf("commit = %+v", res)
	}
	fl := res.Failures[0]
	row, _ := h.repo.PendingForKey(ctx, "p1", "22")
	if fl.EntityType != issuerepo.EntitySprintComplete || !fl.Retryable || fl.Key != "Sprint 22" || len(row) != 1 || fl.RowID != row[0].ID ||
		!strings.Contains(fl.Error, "PLAT-1, PLAT-2") || !strings.Contains(fl.Error, "could not be closed") {
		t.Errorf("failure = %+v", fl)
	}

	f.done[22] = sprints.Completion{Moved: 0, MovedTo: "the backlog"}
	res, err = h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.SprintsChanged, "") != "Sprint 22 completed, 0 unfinished cards moved to the backlog" || res.Remaining != 0 {
		t.Errorf("the retry = %+v", res)
	}
}
