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
// changing it, and destroying it. These three bindings are the surface over
// a sprint's creation, which is journaled, and internal/sprints' Edit and
// Delete, laid out the same way app_sprints.go lays out the two ceremonies:
// requireProfile, a.acquire(p.ID, "sprint") under the same name a start and a
// completion take, the service call for a sprint Jira already holds (past
// backendFor, which a draft never reaches), and ceremonyError to reduce
// whatever Jira said to one readable line.
//
// All five sprint bindings share the one lock name on purpose. acquire
// reports the name that is HELD, so sharing it means a refusal says a sprint
// operation is running rather than which one, and that is the trade: what a
// user needs to know is that this profile is busy with a sprint and not with
// a sync, a commit, an import, a boards refresh or a report, which are the
// other names in play.
//
// A new sprint is journaled as a draft under a negative id, and an edit or a
// delete of such a draft stays local; only a sprint Jira already holds is
// edited or deleted in Jira at once.
//
// None of the three carries a partial result the way CompleteSprint's does.
// Create never reaches Jira at all now: it writes the journal and the draft
// row, or refuses before either is touched. A guard on Edit or Delete
// refuses before Jira is asked anything, and once either has reached Jira it
// finishes, with a note beside its success when the board's own re-read did
// not land, or fails outright with nothing moved that the caller needs
// named. So a Go error is the whole story here, unlike the ceremony that can
// fail with cards already gone from the sprint.

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
// negative id, is rewritten locally; a sprint Jira holds is edited in Jira at
// once. clearGoal is what tells an empty goal box left that way from one
// asking to remove a goal that was there, the same distinction
// internal/sprints.Service.Edit's own doc explains; a draft needs no such
// flag, since its goal is simply what the dialog sent.
func (a *App) EditSprint(profileID string, boardID, sprintID int, name, goal, start, end string, clearGoal bool) (string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return "", err
	}
	defer a.release(p.ID)

	draft := backend.SprintDraft{Name: name, Goal: goal, StartDate: start, EndDate: end}
	if sprintID < 0 {
		d, err := sprints.DraftSprint(draft)
		if err != nil {
			return "", err
		}
		if _, err := a.repo.EditDraftSprint(a.ctx, p.ID, sprintID, issuerepo.DraftSprint{Name: d.Name, Goal: d.Goal, StartDate: d.StartDate, EndDate: d.EndDate}); err != nil {
			return "", err
		}
		return "", nil
	}
	b, err := a.backendFor(p)
	if err != nil {
		return "", err
	}
	log.Printf("tam: editing sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	note, err := a.sprintService(p, b).Edit(a.ctx, p.ID, boardID, sprintID, draft, clearGoal)
	if err != nil {
		log.Printf("tam: edit sprint %d for %s failed: %v", sprintID, p.Name, err)
		return "", ceremonyError(err)
	}
	if note != "" {
		log.Printf("tam: sprint %d for %s edited, with a note: %s", sprintID, p.Name, note)
	}
	return note, nil
}

// DeleteSprint deletes a sprint. A draft sprint is discarded locally, which
// puts every card moved into it back where it was; a sprint Jira holds is
// destroyed in Jira and then removed from TAM's copies, which is why
// internal/sprints.Service.Delete refuses before Jira is asked anything when
// the issue cache is not wired.
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
	b, err := a.backendFor(p)
	if err != nil {
		return "", err
	}
	log.Printf("tam: deleting sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	line, err := a.sprintService(p, b).Delete(a.ctx, p.ID, boardID, sprintID)
	if err != nil {
		log.Printf("tam: delete sprint %d for %s failed: %v", sprintID, p.Name, err)
		return "", ceremonyError(err)
	}
	if line != "" {
		log.Printf("tam: sprint %d for %s deleted, with a note: %s", sprintID, p.Name, line)
	}
	return line, nil
}

// ListBoardSprintDetails composes the Sprints view's data: one board's
// sprints and its own unassigned work, each carrying its issues and the four
// progress numbers a fill bar draws from. a.repo is the IssueSource, the
// same one GetBoard passes: the cards themselves live in the issue cache,
// not in boardrepo's own tables. Unlike the three writes above, it takes no
// acquire lock: it is a local read that never reaches Jira, the same as
// GetBoard and the other board reads in app_boards.go.
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
