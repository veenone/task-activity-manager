package sprints

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
)

// Managing a sprint rather than running one: making it, changing it, and
// destroying it. The package doc carries the argument for why these three
// reach Jira immediately like the two ceremonies beside them; what follows
// is what each of them refuses first.

// errNoIssueCache is what Delete refuses with when the issue cache seam is
// not wired. Create and Edit let the same gap through and only log it,
// because their Issues calls are bookkeeping after Jira has already moved.
// Delete's are not: losing them loses the cache surgery that keeps Jira's
// board tables and issue rows from naming a sprint that no longer exists,
// and the one audit row that will be the only trace of the sprint left once
// Jira has destroyed it. Refusing here, before Jira is asked anything,
// costs nothing; letting the delete through and discovering the gap
// afterwards cannot be undone.
var errNoIssueCache = errors.New("the issue cache is not wired, so this delete could not finish its cache work; nothing was sent to Jira")

// Create makes the sprint on the board and answers with the sprint Jira
// made, which is where its real id comes from and the whole reason this call
// is not journaled.
//
// The dates arrive as a date input produced them, a bare day with no time
// and no zone, and go through the same conversion a start takes: a value
// that is not a date is refused where the message can name it, and so is an
// end before a start. Jira makes a future sprint and offers no other option,
// so there is no state to choose here.
//
// The board's sprint list is re-read afterwards through the refresh that
// refuses an empty answer, which is the right one here: a create proves the
// board has a sprint, so a list with nothing in it is the flaky 400 that
// refusal exists to distrust. The sprint exists whatever the cache makes of
// it afterwards, so a failed re-read comes back as a note beside the new
// sprint rather than as an error, and the sprint itself still travels: Wails
// hands the frontend either the value or the error and never both.
func (s *Service) Create(ctx context.Context, profileID string, boardID int, d backend.SprintDraft) (backend.Sprint, string, error) {
	b, err := s.board()
	if err != nil {
		return backend.Sprint{}, "", err
	}
	if d.StartDate, d.EndDate, err = dates(d.StartDate, d.EndDate); err != nil {
		return backend.Sprint{}, "", err
	}
	made, err := b.CreateSprint(ctx, boardID, d)
	if err != nil {
		return backend.Sprint{}, "", err
	}
	s.audit(ctx, profileID, made.ID, "create", "", "", createdName(made, d))
	return made, note(s.refreshSprints(ctx, b, profileID, boardID)), nil
}

// Edit rewrites a sprint's name, goal and dates. clearGoal is what tells an
// empty goal box that means "leave it alone" from one that means "remove
// it": the partial update that stops a blank field wiping a real goal is
// also what makes a goal impossible to clear, so the two are different
// requests and are sent as different requests.
//
// The dates are converted before the state is read, so a date nobody can
// parse is refused without spending a round trip on a sprint that was never
// going to be written. The state itself comes from Jira and not from the
// cache, and requireEditable carries why. Nothing here confirms editing a
// running sprint: a flag in the answer would arrive after the write, so the
// dialog asks that question from the cached state before it calls.
func (s *Service) Edit(ctx context.Context, profileID string, boardID, sprintID int, d backend.SprintDraft, clearGoal bool) (string, error) {
	b, err := s.board()
	if err != nil {
		return "", err
	}
	if d.StartDate, d.EndDate, err = dates(d.StartDate, d.EndDate); err != nil {
		return "", err
	}
	was, err := s.requireEditable(ctx, b, boardID, sprintID)
	if err != nil {
		return "", err
	}
	if err := b.EditSprint(ctx, sprintID, d, clearGoal); err != nil {
		return "", err
	}
	s.audit(ctx, profileID, sprintID, "edit", editedFields(was, d, clearGoal), was.Name, d.Name)
	return note(s.refreshSprints(ctx, b, profileID, boardID)), nil
}

// editedFields names which of an edit's fields actually moved, so the audit
// row says more than "an edit happened" when the name was left alone and
// only the dates moved or the goal was cleared: before and after otherwise
// carry nothing but the name, which is identical on both sides of exactly
// that edit.
//
// was is what Jira held before the write; d and clearGoal are what was sent.
// Dates are compared by the moment they name rather than by their text,
// through datesChanged, since Jira's own report and the value this package
// just ran through sprintdate.Format do not have to be byte for byte
// identical to name the same instant.
func editedFields(was backend.Sprint, d backend.SprintDraft, clearGoal bool) string {
	var changed []string
	if was.Name != d.Name {
		changed = append(changed, "name")
	}
	if clearGoal {
		if was.Goal != "" {
			changed = append(changed, "goal")
		}
	} else if was.Goal != d.Goal {
		changed = append(changed, "goal")
	}
	if datesChanged(was.StartDate, was.EndDate, d.StartDate, d.EndDate) {
		changed = append(changed, "dates")
	}
	return strings.Join(changed, ", ")
}

// datesChanged is the moment-based comparison editedFields needs. A pair
// this package cannot parse is taken as changed rather than as equal, since
// an unreadable date is not evidence that nothing moved; a sprint Jira has
// already agreed to hold a name for always carries readable dates by the
// time Edit reaches here; a value that does not, one of Store's own test
// doubles say for instance, has nothing this comparison could trust anyway.
func datesChanged(wasStart, wasEnd, start, end string) bool {
	ws, err := sprintdate.Parse(wasStart)
	if err != nil {
		return true
	}
	we, err := sprintdate.Parse(wasEnd)
	if err != nil {
		return true
	}
	s, err := sprintdate.Parse(start)
	if err != nil {
		return true
	}
	e, err := sprintdate.Parse(end)
	if err != nil {
		return true
	}
	return !ws.Equal(s) || !we.Equal(e)
}

// Delete destroys the sprint in Jira and then removes TAM's copies of it. It
// is the one action in this application that cannot be undone from anywhere,
// so it asks Jira what the sprint is before it touches anything and refuses
// every state but a sprint that has never been started or one that is
// already gone.
//
// It also refuses before Jira is asked anything at all when the issue cache
// is not wired. Unlike Create and Edit, whose Issues calls are a footnote
// logged after Jira has already moved, this delete needs that seam to blank
// the vanished sprint off every cached card and to write the one audit row
// that will be the only trace of the sprint left anywhere once Jira has
// destroyed it; letting the delete through without it would cost both for
// good, where refusing costs nothing.
//
// The string it answers with is the note a cache write that did not land
// leaves, or the sentence a sprint that was already gone leaves, either of
// which arrives beside a success. Jira has done the deleting in both cases;
// what follows is local bookkeeping that may not fail the write it is
// bookkeeping for.
func (s *Service) Delete(ctx context.Context, profileID string, boardID, sprintID int) (string, error) {
	if s.Issues == nil {
		return "", errNoIssueCache
	}
	b, err := s.board()
	if err != nil {
		return "", err
	}
	doomed, gone, err := s.requireDeletable(ctx, b, boardID, sprintID)
	if err != nil {
		return "", err
	}
	if err := s.refusePendingDelete(ctx, profileID, sprintID); err != nil {
		return "", err
	}
	if !gone {
		if err := b.DeleteSprint(ctx, sprintID); err != nil {
			return "", err
		}
	}
	s.audit(ctx, profileID, sprintID, "delete", "", s.doomedName(ctx, profileID, sprintID, doomed, gone), "")
	line := note(s.forget(ctx, b, profileID, boardID, sprintID))
	if gone {
		return strings.TrimSpace(fmt.Sprintf("sprint %d was already gone from Jira, so only TAM's own copy of it was removed. %s", sprintID, line)), nil
	}
	return line, nil
}

// doomedName is the name the delete's audit row carries as its before
// value. requireDeletable hands back a real backend.Sprint, name and all,
// for a sprint it found and is about to delete; for one that was already
// gone it hands back a zero backend.Sprint, since there was nothing left in
// Jira's list to read a name off. That second case is the one branch where
// keeping the name matters most, since Jira no longer has it to ask again,
// so it is read from the board cache instead. A cache miss leaves the row
// carrying an empty name rather than failing the delete over it: Jira has
// already been asked, in both branches, and the delete cannot be undone by
// refusing to finish recording it.
func (s *Service) doomedName(ctx context.Context, profileID string, sprintID int, doomed backend.Sprint, gone bool) string {
	if !gone {
		return doomed.Name
	}
	name, err := s.store.SprintName(ctx, profileID, strconv.Itoa(sprintID))
	if err != nil {
		return ""
	}
	return name
}

// forget removes the sprint from the places TAM keeps it, in the order
// boardrepo.DeleteSprintEverywhere fixes: the board rows first, the issue
// columns second.
//
// That order is what a crash between the two decides, since they are
// separate transactions in separate repositories on purpose. Board rows
// first leaves issues whose sprint columns name a sprint that is gone, stale
// text on the cards that already held it. The other way round leaves the
// sprint alive in OpenSprints, where the New issue dialog, the detail panel
// and the importer's Sprint column all offer it as somewhere to put work.
// Only a full sync repairs either.
//
// The board's list is re-read last, through the one refresh that permits an
// empty answer, because a board whose only sprint has just been deleted
// really does have none left to report. That is the emptiness the sibling
// refusal exists to distrust, and this is the single caller that can make it
// true.
//
// Only the first failure travels back. All of it is cache work after Jira
// has already destroyed the sprint, so none of it may fail the delete, and
// one sentence with one thing to do is all a user can act on anyway.
func (s *Service) forget(ctx context.Context, b lifecycle, profileID string, boardID, sprintID int) error {
	first := s.store.DeleteSprintEverywhere(ctx, profileID, sprintID)
	if first != nil {
		log.Printf("tam: sprint %d was deleted in Jira but its cached board rows could not be removed: %v", sprintID, first)
		first = fmt.Errorf("the sprint could not be removed from the board cache: %w", first)
	}
	if err := s.clearIssues(ctx, profileID, sprintID); err != nil && first == nil {
		first = err
	}
	if err := s.refreshSprintsAllowEmpty(ctx, b, profileID, boardID); err != nil && first == nil {
		first = err
	}
	return first
}

// clearIssues blanks the deleted sprint's name off every cached issue that
// still carries it, so the Backlog and Epics sprint filters and the detail
// panel's Sprint field stop offering it the moment the delete lands rather
// than after the next full sync.
//
// It does not check s.Issues for nil: Delete already refused before Jira was
// touched at all if that field was unset, so by the time forget calls this,
// the only way in here is with the seam wired.
func (s *Service) clearIssues(ctx context.Context, profileID string, sprintID int) error {
	if err := s.Issues.ClearSprint(ctx, profileID, strconv.Itoa(sprintID)); err != nil {
		log.Printf("tam: sprint %d was deleted in Jira but the cards carrying its name could not be cleared: %v", sprintID, err)
		return fmt.Errorf("the cards that were in the sprint could not be updated: %w", err)
	}
	return nil
}

// audit records one of these three writes in the local audit trail.
//
// Nothing on screen reads that row. The Activity tab asks for one issue's
// key, and no view passes a sprint's id, so a sprint's rows are reachable
// only by reading the database with another tool. It is written anyway,
// because once Jira no longer has the sprint this row is the only trace of
// it left on the machine, and it is deliberately never described to the user
// as somewhere they can go and look: the delete confirmation says nothing
// about an activity log. A profile level activity view would change that and
// is not in this plan.
//
// A row that cannot be written is logged and nothing more. Jira has already
// been changed by the time it is attempted, and failing the write over its
// own footnote would report a sprint that exists as one that does not.
// Delete's own seam check means this path is reachable with s.Issues nil
// only from Create and Edit, where that is still the right answer: an unset
// seam there is a missed footnote, not a missed refusal.
func (s *Service) audit(ctx context.Context, profileID string, sprintID int, action, field, before, after string) {
	if s.Issues == nil {
		log.Printf("tam: the %s of sprint %d was not recorded in the audit trail: no issue cache is wired", action, sprintID)
		return
	}
	if err := s.Issues.AuditSprint(ctx, profileID, sprintID, action, field, before, after); err != nil {
		log.Printf("tam: the %s of sprint %d could not be recorded in the audit trail: %v", action, sprintID, err)
	}
}

// createdName is what a new sprint is called: Jira's answer, or the name
// that was asked for when the create came back without a body, which the
// transport tolerates because the refresh is what finds the sprint then.
func createdName(made backend.Sprint, d backend.SprintDraft) string {
	if strings.TrimSpace(made.Name) != "" {
		return made.Name
	}
	return d.Name
}
