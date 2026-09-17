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

// The sprint ceremonies. Like every other sprint write they are journal rows
// that Commit pushes, so neither needs a backend and both work offline. They
// still take a.acquire(p.ID, "sprint"), the lock the management bindings in
// app_sprintmanage.go take, which serialises them against a Commit rewriting
// the same rows.

// sprintService builds the service Commit pushes sprint writes through, over
// the profile's backend and the board cache, the way commitEngine builds the
// commit engine: the backend belongs to a profile, so the service is built
// per Commit.
//
// Pending and Issues are wired here because the service's store is the board
// cache and both of them are the issue cache, and app.go is the one place
// holding both repositories. Issues is what a delete blanks a vanished
// sprint's name through, and where a pushed edit or delete leaves its audit
// row.
func (a *App) sprintService(p profile.Profile, b backend.IssueBackend) *sprints.Service {
	s := sprints.New(b, a.boards, p.ProjectKey)
	s.Pending = a.pendingInSprint
	s.Issues = a.repo
	return s
}

// StartSprint queues a start of the sprint for Commit, with the name, goal
// and dates the dialog collected. The dates arrive as a date input wrote
// them and are converted here; a value that is not a date, or an end before
// a start, is refused with nothing journaled. A draft sprint can be started
// too, and Commit starts it once it has created it.
func (a *App) StartSprint(profileID string, boardID, sprintID int, name, goal, start, end string) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return err
	}
	defer a.release(p.ID)

	d, err := sprints.DraftSprint(backend.SprintDraft{Name: name, Goal: goal, StartDate: start, EndDate: end})
	if err != nil {
		return err
	}
	log.Printf("tam: journaling a start of sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	return a.repo.JournalSprintStart(a.ctx, p.ID, sprintID, issuerepo.SprintStart{
		BoardID: boardID, Name: d.Name, Goal: d.Goal, StartDate: d.StartDate, EndDate: d.EndDate,
	})
}

// CompleteSprint queues a completion of the sprint for Commit, moving its
// unfinished cards to moveTo, the backlog when it is empty. Commit works the
// unfinished set out from Jira and reports what it really moved.
//
// The refusals the push would make without Jira are made here first, by the
// same check (sprints.Service.CheckComplete).
//
// boardID is needed because "unfinished" is defined by the board's last
// column, which cannot be read without knowing which board is being
// completed on.
func (a *App) CompleteSprint(profileID string, boardID, sprintID int, moveTo string) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return err
	}
	defer a.release(p.ID)

	if moveTo, err = a.sprintService(p, nil).CheckComplete(a.ctx, p.ID, boardID, sprintID, moveTo); err != nil {
		return err
	}
	log.Printf("tam: journaling a completion of sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	return a.repo.JournalSprintComplete(a.ctx, p.ID, sprintID, issuerepo.SprintComplete{
		BoardID: boardID, MoveTo: moveTo,
	})
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
