package sprints

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"agile-suite/tam/internal/sprintdate"
)

// The checks a ceremony makes before it touches Jira, gathered off
// sprints.go so the two ceremonies read as what they do rather than as what
// they refuse. Every one of them exists because the bound method is
// reachable without the dialog that would have asked the same question.

// destinationID reads the destination the dialog sent: a sprint id, or the
// empty string for the backlog, which is a destination and not an absence.
//
// The id ends up in a URL path, so only a plain positive number is a sprint
// id here. strconv.Atoi on its own takes "+13", "-1", "0" and "012", and
// "012" then walked past a string comparison with the sprint being completed
// and went to Jira as it was typed. Comparing the numbers is what catches
// that, and refusing everything but the canonical form is what keeps the
// value that reaches Jira the one that was checked.
func destinationID(moveTo string, sprintID int) (string, error) {
	moveTo = strings.TrimSpace(moveTo)
	if moveTo == "" {
		return "", nil
	}
	n, err := strconv.Atoi(moveTo)
	if err != nil || n <= 0 || strconv.Itoa(n) != moveTo {
		return "", fmt.Errorf("sprint id %q is not a sprint id", moveTo)
	}
	if n == sprintID {
		return "", errors.New("a sprint cannot be completed into itself")
	}
	return moveTo, nil
}

// requireCompletable is what the cache has to say about the sprint before a
// completion touches Jira: the board holds it, and it is not one that has
// never been started.
//
// The pair first. The completion judges "finished" against the board's last
// column while the cards come from the sprint, so a mismatched pair would
// decide where somebody's work goes on a definition borrowed from a board
// the sprint was never on, and close the sprint anyway.
//
// Then the state, because a completion moves the cards out before it finds
// out Jira will not close the sprint. Aimed at a sprint that never started,
// it empties that sprint in Jira and then fails, and TAM cannot undo either
// half. The toolbar only offers Complete on the active sprint, which is the
// same argument the other guards here already refuse to rest on: the bound
// method is reachable without the dialog.
//
// Only "future" is refused, and deliberately not "anything that is not
// active". A sprint someone started on the web an hour ago still reads as
// future in a cache nobody has refreshed since, and refusing that costs a
// Refresh, where emptying it costs the sprint. A state the cache does not
// recognise is left to Jira to answer for.
func (s *Service) requireCompletable(ctx context.Context, profileID string, boardID int, sprintID string) error {
	state, ok, err := s.store.BoardSprintState(ctx, profileID, boardID, sprintID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("sprint %s is not on board %d, so that board's last column cannot say which of the sprint's cards finished; complete the sprint from the board it belongs to", sprintID, boardID)
	}
	if state == "future" {
		return fmt.Errorf("sprint %s has not been started, and completing it would move its cards out and then fail to close it; start it first, or press Refresh if it was started somewhere else", sprintID)
	}
	return nil
}

// destination is what the completion reports as the place the cards went.
func (s *Service) destination(ctx context.Context, profileID, sprintID string) (string, error) {
	if sprintID == "" {
		return "the backlog", nil
	}
	name, err := s.store.SprintName(ctx, profileID, sprintID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		return "sprint " + sprintID, nil
	}
	return name, nil
}

// refusePending stops a completion while the journal still holds changes for
// cards staying in this sprint. Committing them first is what makes the
// board and Jira agree about which cards finished.
func (s *Service) refusePending(ctx context.Context, profileID string, sprintID int) error {
	if s.Pending == nil {
		return nil
	}
	n, err := s.Pending(ctx, profileID, sprintID)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%d pending change(s) belong to cards in this sprint; commit them before completing it, or Jira will be asked which cards finished before it has been told", n)
	}
	return nil
}

// completeStatuses is the set of status ids that count as finished: the ones
// the board's last column collects. A board whose columns are not cached, or
// whose last column collects nothing, cannot answer the question this action
// turns on, and guessing is not an option for a move nobody can undo from
// TAM.
func (s *Service) completeStatuses(ctx context.Context, profileID string, boardID int) (map[string]bool, error) {
	cols, err := s.store.Columns(ctx, profileID, boardID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("board %d has no cached columns, so TAM cannot tell which issues finished; refresh the boards first", boardID)
	}
	last := cols[len(cols)-1]
	if len(last.StatusIDs) == 0 {
		return nil, fmt.Errorf("the last column of board %d (%q) collects no statuses, so TAM cannot tell which issues finished", boardID, last.Name)
	}
	set := make(map[string]bool, len(last.StatusIDs))
	for _, id := range last.StatusIDs {
		set[id] = true
	}
	return set, nil
}

// dates turns the two the dialog collected into the pair Jira is sent,
// refusing rather than guessing. An empty date is refused because a sprint
// with one end is not a sprint, and an end before a start is refused here
// rather than only in the dialog.
func dates(start, end string) (string, string, error) {
	from, err := sprintdate.Parse(start)
	if err != nil {
		return "", "", fmt.Errorf("start date: %w", err)
	}
	to, err := sprintdate.Parse(end)
	if err != nil {
		return "", "", fmt.Errorf("end date: %w", err)
	}
	if to.Before(from) {
		return "", "", errors.New("the sprint ends before it starts")
	}
	return sprintdate.Format(from), sprintdate.Format(to), nil
}
