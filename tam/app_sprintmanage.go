package main

import (
	"fmt"
	"log"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/sprints"
)

// Managing a sprint from the board, rather than running one: making it,
// changing it, and destroying it. All three are journaled: a new sprint is a
// draft under a negative id, an edit or a delete of a draft rewrites or
// discards that draft, and an edit or a delete of a sprint Jira holds is a
// sprint_edit or sprint_delete row that Commit pushes. None of them needs a
// backend, so all three work offline.
//
// They still take a.acquire(p.ID, "sprint"), the lock the two ceremonies in
// app_sprints.go take, which serialises them against a Commit rewriting the
// same rows. acquire reports the name that is HELD, so a refusal says a
// sprint operation is running rather than which one.
//
// None of the three carries a partial result or a note any more: each
// writes the journal and the cache in one transaction, or refuses with
// nothing written, so a Go error is the whole story.

// SprintCreated is what CreateSprint answers with: the draft sprint, whose
// negative id is what the dialog switches a picker to. Note stays in the
// shape, always empty now that nothing is re-read from Jira, so the
// frontend's one path reads the same field it always has.
type SprintCreated struct {
	Sprint backend.Sprint `json:"sprint"`
	Note   string         `json:"note"`
}

// CreateSprint drafts a new sprint on the board with the dialog's four
// fields. Nothing reaches Jira: the draft is journaled, every picker offers
// it at once, and Commit creates it before any card moved into it is sent.
// The "sprint" lock is still taken, which serialises a draft against a
// Commit rewriting the same rows and keeps the frontend's runQuietLock
// honest.
func (a *App) CreateSprint(profileID string, boardID int, name, goal, start, end string) (SprintCreated, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return SprintCreated{}, err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return SprintCreated{}, err
	}
	defer a.release(p.ID)

	d, err := sprints.DraftSprint(backend.SprintDraft{Name: name, Goal: goal, StartDate: start, EndDate: end})
	if err != nil {
		return SprintCreated{}, err
	}
	boardName, err := a.cachedBoardName(p.ID, boardID)
	if err != nil {
		return SprintCreated{}, err
	}
	made, err := a.repo.CreateDraftSprint(a.ctx, p.ID, issuerepo.DraftSprint{
		BoardID: boardID, BoardName: boardName, Name: d.Name, Goal: d.Goal, StartDate: d.StartDate, EndDate: d.EndDate,
	})
	if err != nil {
		return SprintCreated{}, err
	}
	log.Printf("tam: drafted sprint %d %q on board %d for %s (%s)", made.ID, made.Name, boardID, p.Name, p.ProjectKey)
	return SprintCreated{Sprint: made}, nil
}

// cachedBoardName is the name of a board the cache holds, and a refusal for
// one it does not: a draft sprint on a board nobody synced would be offered
// by no picker, since every one of them joins to the board row.
func (a *App) cachedBoardName(profileID string, boardID int) (string, error) {
	name := a.boardName(profileID, boardID)
	if name == "" {
		return "", fmt.Errorf("board %d is not in the cache; refresh the boards first", boardID)
	}
	return name, nil
}

// EditSprint rewrites a sprint's name, goal and dates. A draft sprint, a
// negative id, is rewritten in its draft; a sprint Jira holds is journaled,
// changed in the cache at once, and edited in Jira on Commit. clearGoal is
// what tells an empty goal box left that way from one asking to remove a
// goal that was there, the distinction sprints.Committed.Edit's doc
// explains; a draft needs no such flag, since its goal is simply what the
// dialog sent.
func (a *App) EditSprint(profileID string, boardID, sprintID int, name, goal, start, end string, clearGoal bool) (string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return "", err
	}
	defer a.release(p.ID)

	d, err := sprints.DraftSprint(backend.SprintDraft{Name: name, Goal: goal, StartDate: start, EndDate: end})
	if err != nil {
		return "", err
	}
	if sprintID < 0 {
		_, err := a.repo.EditDraftSprint(a.ctx, p.ID, sprintID, issuerepo.DraftSprint{Name: d.Name, Goal: d.Goal, StartDate: d.StartDate, EndDate: d.EndDate})
		return "", err
	}
	log.Printf("tam: journaling an edit of sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	return "", a.repo.JournalSprintEdit(a.ctx, p.ID, sprintID, issuerepo.SprintEdit{
		BoardID: boardID, Name: d.Name, Goal: d.Goal, StartDate: d.StartDate, EndDate: d.EndDate, ClearGoal: clearGoal,
	})
}

// DeleteSprint deletes a sprint. A draft sprint is discarded locally, which
// puts every card moved into it back where it was; a sprint Jira holds is
// queued for Commit, which destroys it in Jira and then removes TAM's
// copies. The queue is refused while cards in the sprint have pending
// changes, the same refusal the push itself makes.
func (a *App) DeleteSprint(profileID string, boardID, sprintID int) (string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return "", err
	}
	defer a.release(p.ID)

	if sprintID < 0 {
		return "", a.repo.DiscardDraftSprint(a.ctx, p.ID, sprintID)
	}
	n, err := a.pendingInSprint(a.ctx, p.ID, sprintID)
	if err != nil {
		return "", err
	}
	if err := sprints.RefusePendingDelete(n); err != nil {
		return "", err
	}
	log.Printf("tam: journaling a delete of sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	return "", a.repo.JournalSprintDelete(a.ctx, p.ID, boardID, sprintID)
}

// ListBoardSprintDetails composes the Sprints view's data: one board's
// sprints and its own unassigned work, each carrying its issues and the four
// progress numbers a fill bar draws from. a.repo is the IssueSource, the
// same one GetBoard passes: the cards themselves live in the issue cache,
// not in boardrepo's own tables. Unlike the three writes above, it takes no
// acquire lock: it is a local read, the same as GetBoard and the other
// board reads in app_boards.go.
func (a *App) ListBoardSprintDetails(profileID string, boardID int) ([]boardrepo.SprintDetail, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	details, err := a.boards.BoardSprintDetails(a.ctx, a.repo, profileID, boardID)
	if err != nil {
		return nil, err
	}
	if details == nil {
		details = []boardrepo.SprintDetail{}
	}
	return details, nil
}
