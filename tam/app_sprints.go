package main

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/sprints"
)

// The sprint ceremonies. Unlike the board writes in app_boards.go, which
// journal and take no guard, these two push to Jira the moment they are
// called, so they take the same per-profile lock a sync, a commit and a
// boards refresh take, under their own name so a refusal can say which
// operation is actually running. Anything calling them from the frontend
// goes through the sync reducer, the rule tam/CLAUDE.md records for every
// bound method that takes acquire.

// sprintService builds the lifecycle service over the profile's backend and
// the board cache, the way commitEngine builds the commit engine: the
// backend belongs to a profile, so the service is built per call.
//
// Pending is wired here because the question it asks spans both
// repositories, and app.go is the one place holding them both.
func (a *App) sprintService(p profile.Profile, b backend.IssueBackend) *sprints.Service {
	s := sprints.New(b, a.boards, p.ProjectKey)
	s.Pending = a.pendingInSprint
	return s
}

// StartSprint starts a sprint on Jira with the name, goal and dates the
// dialog collected, and refreshes that board's sprint list afterwards. The
// dates arrive as a date input wrote them and are parsed by the service; a
// value that is not a date, or an end before a start, never reaches Jira.
func (a *App) StartSprint(profileID string, boardID, sprintID int, name, goal, start, end string) error {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return err
	}
	defer a.release(p.ID)

	log.Printf("tam: starting sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	draft := backend.SprintDraft{Name: name, Goal: goal, StartDate: start, EndDate: end}
	if err := a.sprintService(p, b).Start(a.ctx, p.ID, boardID, sprintID, draft); err != nil {
		log.Printf("tam: start sprint %d for %s failed: %v", sprintID, p.Name, err)
		return err
	}
	return nil
}

// CompleteSprint moves the sprint's unfinished issues to moveTo, the
// backlog when it is empty, and then closes the sprint.
//
// A push that failed partway comes back as a Completion carrying its own
// Message, with no Go error: Wails hands the frontend either the value or
// the error and never both, so an error there would deliver the sentence and
// drop the keys it is about. Every other failure is an error, and says
// everything it has to say in its own words.
//
// boardID is not in the plan's one-line signature and is needed all the
// same: "unfinished" is defined by the board's last column, which cannot be
// read without knowing which board is being completed on.
func (a *App) CompleteSprint(profileID string, boardID, sprintID int, moveTo string) (sprints.Completion, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return sprints.Completion{Failed: []string{}}, err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return sprints.Completion{Failed: []string{}}, err
	}
	defer a.release(p.ID)

	log.Printf("tam: completing sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	done, err := a.sprintService(p, b).Complete(a.ctx, p.ID, boardID, sprintID, moveTo)
	done.Failed = backend.NonNil(done.Failed)
	if err != nil {
		log.Printf("tam: complete sprint %d for %s failed after moving %d: %v", sprintID, p.Name, done.Moved, err)
		return done, err
	}
	if done.Message != "" {
		log.Printf("tam: complete sprint %d for %s stopped partway: %s", sprintID, p.Name, done.Message)
		return done, nil
	}
	log.Printf("tam: completed sprint %d for %s: %d issues moved to %s", sprintID, p.Name, done.Moved, done.MovedTo)
	return done, nil
}

// SuggestSprintDates is what the start dialog opens with: today, today plus
// the board's own sprint length, and the name the board's last sprint
// suggests. It reads the cache only, so it answers whether or not Jira can
// be reached.
func (a *App) SuggestSprintDates(profileID string, boardID int) (sprints.Suggestion, error) {
	if err := a.requireStore(); err != nil {
		return sprints.Suggestion{}, err
	}
	length, err := a.boards.SprintLength(a.ctx, profileID, boardID)
	if err != nil {
		return sprints.Suggestion{}, err
	}
	cached, err := a.boards.ListSprints(a.ctx, profileID, boardID)
	if err != nil {
		return sprints.Suggestion{}, err
	}
	return sprints.Suggest(time.Now(), length, lastSprintName(cached)), nil
}

// lastSprintName is the name of the board's most recently created sprint,
// which is the one a new sprint's number follows. It is the highest id
// rather than the latest start date: a future sprint has no dates yet, and
// Jira hands out sprint ids in the order they were created, which is the
// order their names are numbered in.
func lastSprintName(cached []boardrepo.Sprint) string {
	name, best := "", 0
	for _, s := range cached {
		if s.ID >= best {
			name, best = s.Name, s.ID
		}
	}
	return name
}

// PendingInSprint counts the journal rows belonging to cards that sit in the
// sprint and are staying there. The Complete button asks before it opens its
// dialog: a card dragged to Done an hour ago is Done on the board and not in
// Jira, and completing the sprint would move it to the backlog as
// unfinished.
//
// A card journaled out of the sprint is not counted, and needs no case of
// its own: a journaled sprint move writes the destination onto the cached
// row as it is made, so the card has already left the sprint as far as this
// read is concerned. A card journaled into it counts, because it is staying.
func (a *App) PendingInSprint(profileID string, sprintID int) (int, error) {
	if err := a.requireStore(); err != nil {
		return 0, err
	}
	return a.pendingInSprint(a.ctx, profileID, sprintID)
}

func (a *App) pendingInSprint(ctx context.Context, profileID string, sprintID int) (int, error) {
	rows, err := a.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return 0, err
	}
	sid := strconv.Itoa(sprintID)
	// One cache read per key, not per row: an issue with an edit, a
	// transition and a rank against it is three rows and one card.
	staying := map[string]bool{}
	n := 0
	for _, row := range rows {
		key := row.EntityKey
		if _, seen := staying[key]; !seen {
			iss, err := a.repo.GetIssue(ctx, profileID, key)
			if err != nil && !errors.Is(err, issuerepo.ErrNotFound) {
				return 0, err
			}
			staying[key] = err == nil && iss.SprintID == sid
		}
		if staying[key] {
			n++
		}
	}
	return n, nil
}
