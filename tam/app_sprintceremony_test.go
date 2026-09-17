package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

// ceremonyBackend answers what a start and a completion push: the sprint's
// issues from Jira, the move of the unfinished ones, the close and the
// start, recording each so a test can tell a write made at once from one
// made on Commit.
type ceremonyBackend struct {
	manageLifecycleBackend
	issues    []backend.Issue
	moves     []string
	completed []int
	started   []string
}

func (b *ceremonyBackend) SearchIssuesPage(_ context.Context, _, scope, _ string, _ []string, startAt, _ int) ([]backend.Issue, int, error) {
	var out []backend.Issue
	for _, iss := range b.issues {
		if scope == "sprint = "+iss.SprintID {
			out = append(out, iss)
		}
	}
	if startAt >= len(out) {
		return []backend.Issue{}, len(out), nil
	}
	return out[startAt:], len(out), nil
}

func (b *ceremonyBackend) MoveIssuesToSprint(_ context.Context, sprintID string, keys []string) error {
	b.moves = append(b.moves, sprintID+": "+strings.Join(keys, ","))
	return nil
}

func (b *ceremonyBackend) CompleteSprint(_ context.Context, sprintID int) error {
	b.completed = append(b.completed, sprintID)
	return nil
}

func (b *ceremonyBackend) StartSprint(_ context.Context, sprintID int, d backend.SprintDraft) error {
	b.started = append(b.started, fmt.Sprintf("%d %s %s", sprintID, d.Name, d.StartDate[:10]))
	return nil
}

var _ backend.BoardBackend = (*ceremonyBackend)(nil)

func card(key, statusID, sprintID string) backend.Issue {
	return backend.Issue{Key: key, ID: key, Project: "PLAT", Type: backend.TypeTask, Summary: key, Status: "s" + statusID,
		StatusID: statusID, SprintID: sprintID, SprintName: "Sprint " + sprintID, Updated: "2026-09-01T00:00:00Z"}
}

// withCeremonies caches board 1 with To Do and Done, sprint 12 active and
// sprint 13 future, and PLAT-1 unfinished and PLAT-2 done in sprint 12.
func withCeremonies(t *testing.T) (*App, string, *ceremonyBackend) {
	t.Helper()
	a := newTestApp(t)
	p := newTestProfile(t, a)
	sprints := []backend.Sprint{
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"},
	}
	cards := []backend.Issue{card("PLAT-1", "1", "12"), card("PLAT-2", "5", "12")}
	fake := &ceremonyBackend{
		manageLifecycleBackend: manageLifecycleBackend{simpleBoardBackend: *twoBoards("PLAT Scrum", "PLAT Kanban", "To Do"), sprints: sprints},
		issues:                 cards,
	}
	a.backends[p.ID] = fake
	cols := []backend.BoardColumn{{Name: "To Do", StatusIDs: []string{"1"}}, {Name: "Done", StatusIDs: []string{"5"}}}
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}, cols, sprints,
		map[string][]string{"12": {"PLAT-1", "PLAT-2"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.repo.UpsertPage(a.ctx, p.ID, cards, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	return a, p.ID, fake
}

// The preview said one card was unfinished. Before Commit a done card was
// reopened and a new one joined the sprint, so Commit moves three, and the
// result reports the three it moved without calling that a failure.
func TestACompletionMovesWhatJiraHoldsAtCommitNotWhatThePreviewSaid(t *testing.T) {
	a, pid, fake := withCeremonies(t)
	if err := a.CompleteSprint(pid, 1, 12, "13", 1); err != nil {
		t.Fatal(err)
	}
	if len(fake.completed) != 0 || len(fake.moves) != 0 {
		t.Fatalf("Jira was written before Commit: moves %v, closed %v", fake.moves, fake.completed)
	}
	fake.issues = []backend.Issue{card("PLAT-1", "1", "12"), card("PLAT-2", "1", "12"), card("PLAT-3", "1", "12")}

	res, err := a.CommitPendingChanges(pid)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(res.SprintsChanged, "; "); got != "Sprint 12 completed, 3 unfinished cards moved to Sprint 13" {
		t.Errorf("sprints changed = %q", got)
	}
	if len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("commit = %+v, want no failure and nothing left", res)
	}
	if strings.Join(fake.moves, "|") != "13: PLAT-1,PLAT-2,PLAT-3" || len(fake.completed) != 1 || fake.completed[0] != 12 {
		t.Errorf("pushed moves %v, closed %v", fake.moves, fake.completed)
	}
}

// A start needs no backend: it is journaled, nothing reaches Jira, and
// Commit starts the sprint with the dialog's dates.
func TestStartSprintWaitsForCommitAndWorksOffline(t *testing.T) {
	a, pid, fake := withCeremonies(t)
	delete(a.backends, pid)
	if note, err := a.StartSprint(pid, 1, 13, "Sprint 13", "Ship", "2026-09-14", "2026-09-28"); err != nil || note != "" {
		t.Fatalf("StartSprint = %q, %v", note, err)
	}
	if _, err := a.StartSprint(pid, 1, 12, "Sprint 12", "", "2026-09-14", "2026-09-28"); err == nil {
		t.Error("an active sprint was queued to start")
	}
	if _, err := a.StartSprint(pid, 1, 13, "Sprint 13", "", "2026-09-28", "2026-09-14"); err == nil {
		t.Error("an end before the start was journaled")
	}
	if listed, _ := a.ListBoardSprints(pid, 1); len(listed) != 2 || listed[1].State != "future" {
		t.Errorf("board sprints = %+v, want sprint 13 still future", listed)
	}

	a.backends[pid] = fake
	res, err := a.CommitPendingChanges(pid)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fake.started, "|") != "13 Sprint 13 2026-09-14" || strings.Join(res.SprintsChanged, "") != "Sprint 13 started" || res.Remaining != 0 {
		t.Errorf("started %v, commit %+v", fake.started, res)
	}
}

// A draft sprint drafted and started offline is created and then started
// under its real id by one Commit.
func TestADraftSprintCanBeStartedBeforeItExists(t *testing.T) {
	a, pid, fake := withCeremonies(t)
	fake.made = backend.Sprint{ID: 14, BoardID: 1, Name: "Sprint 14", State: "future"}
	made, err := a.CreateSprint(pid, 1, "Sprint 14", "", "2026-09-14", "2026-09-28")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartSprint(pid, 1, made.Sprint.ID, "Sprint 14", "", "2026-09-14", "2026-09-28"); err != nil {
		t.Fatal(err)
	}
	res, err := a.CommitPendingChanges(pid)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fake.started, "|") != "14 Sprint 14 2026-09-14" || res.Remaining != 0 || len(res.Failures) != 0 {
		t.Errorf("started %v, commit %+v", fake.started, res)
	}
}

// Each refusal the push would make from the cache is made before anything
// is journaled.
func TestCompleteSprintRefusesLocallyWithNothingJournaled(t *testing.T) {
	a, pid, _ := withCeremonies(t)
	for _, c := range []struct {
		sprintID int
		moveTo   string
		want     string
	}{
		{-1, "", "draft"},
		{13, "", "has not been started"},
		{12, "-1", "draft sprint"},
		{12, "12", "into itself"},
	} {
		if err := a.CompleteSprint(pid, 1, c.sprintID, c.moveTo, 1); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("complete %d to %q = %v, want %q", c.sprintID, c.moveTo, err, c.want)
		}
	}
	if err := a.EditIssue(pid, "PLAT-2", "summary", "edited"); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteSprint(pid, 1, 12, "", 1); err == nil || !strings.Contains(err.Error(), "commit them before completing") {
		t.Errorf("complete with a pending card change = %v", err)
	}
	pending, _ := a.ListPendingChanges(pid)
	if len(pending) != 1 || pending[0].EntityKey != "PLAT-2" {
		t.Errorf("pending = %+v, want only the card edit", pending)
	}
}
