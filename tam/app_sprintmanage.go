package main

import (
	"log"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

// Managing a sprint from the board, rather than running one: making it,
// changing it, and destroying it. These three bindings are the exercisable
// surface over internal/sprints' Create, Edit, and Delete, laid out the same
// way app_sprints.go lays out the two ceremonies: requireProfile by way of
// backendForProfile, a.acquire(p.ID, "sprint") under the same name a start
// and a completion take, the service call, and ceremonyError to reduce
// whatever Jira said to one readable line.
//
// All five sprint bindings share the one lock name on purpose. acquire
// reports the name that is HELD, so sharing it means a refusal says a sprint
// operation is running rather than which one, and that is the trade: what a
// user needs to know is that this profile is busy with a sprint and not with
// a sync, a commit, an import, a boards refresh or a report, which are the
// other names in play.
//
// Nothing here is journaled, for the reasons internal/sprints' package doc
// gives: a sprint id has to be real before anything can point at it, and
// there is nothing to defer.
//
// None of the three carries a partial result the way CompleteSprint's does.
// A guard refuses before Jira is ever asked anything, and once Create, Edit
// or Delete has reached Jira it either finishes, with a note beside its
// success when the board's own re-read did not land, or fails outright with
// nothing moved that the caller needs named. So a Go error is the whole
// story here, unlike the ceremony that can fail with cards already gone from
// the sprint.

// SprintCreated is what CreateSprint answers with. Wails fills in either a
// bound method's value or its error and never both, and it only ever fills
// in one value beside that error, the same limit sprints.Completion is
// built around; the sprint Jira made and the note its re-read left, the two
// separate values the service hands back, travel here as one struct for
// exactly that reason.
type SprintCreated struct {
	Sprint backend.Sprint `json:"sprint"`
	Note   string         `json:"note"`
}

// CreateSprint makes a new sprint on the board with the dialog's four
// fields, and answers with the sprint Jira made, since that is where the
// sprint's real id comes from.
func (a *App) CreateSprint(profileID string, boardID int, name, goal, start, end string) (SprintCreated, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return SprintCreated{}, err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return SprintCreated{}, err
	}
	defer a.release(p.ID)

	log.Printf("tam: creating a sprint on board %d for %s (%s)", boardID, p.Name, p.ProjectKey)
	draft := backend.SprintDraft{Name: name, Goal: goal, StartDate: start, EndDate: end}
	made, note, err := a.sprintService(p, b).Create(a.ctx, p.ID, boardID, draft)
	if err != nil {
		log.Printf("tam: create a sprint on board %d for %s failed: %v", boardID, p.Name, err)
		return SprintCreated{}, ceremonyError(err)
	}
	if note != "" {
		log.Printf("tam: sprint %d for %s created, with a note: %s", made.ID, p.Name, note)
	}
	return SprintCreated{Sprint: made, Note: note}, nil
}

// EditSprint rewrites a sprint's name, goal and dates. clearGoal is what
// tells an empty goal box left that way from one asking to remove a goal
// that was there, the same distinction internal/sprints.Service.Edit's own
// doc explains; it travels as its own argument because the draft's blank
// Goal field cannot say which of the two was meant.
func (a *App) EditSprint(profileID string, boardID, sprintID int, name, goal, start, end string, clearGoal bool) (string, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return "", err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return "", err
	}
	defer a.release(p.ID)

	log.Printf("tam: editing sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	draft := backend.SprintDraft{Name: name, Goal: goal, StartDate: start, EndDate: end}
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

// DeleteSprint destroys the sprint in Jira and then removes TAM's own
// copies of it. It is the one action of the three that cannot be undone
// from anywhere, which is why internal/sprints.Service.Delete refuses
// before Jira is asked anything when the issue cache is not wired; that
// refusal reaches here as an ordinary error, the same as every other guard
// this binding takes.
func (a *App) DeleteSprint(profileID string, boardID, sprintID int) (string, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return "", err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return "", err
	}
	defer a.release(p.ID)

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
