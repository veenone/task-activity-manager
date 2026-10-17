package reports

import (
	"fmt"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
)

// Person is one row of a sprint report's People section: what was assigned
// to somebody, and the time they spent on the sprint (#287).
//
// Name is the Jira display name and is empty only on the row for cards with
// no assignee. Username is the key rows are merged by, since a display name
// is neither unique nor stable. Email is blank where Jira hides it.
type Person struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	// Cards and Points are the sprint's cards assigned to this person now,
	// and the points those cards carry now.
	Cards  int     `json:"cards"`
	Points float64 `json:"points"`
	// EstimateSeconds and RemainingSeconds add up each card's own original
	// and remaining estimate. The family aggregates are left out: a sub-task
	// in the sprint is a card of its own, and counting its parent's
	// aggregate as well would count its hours twice.
	EstimateSeconds  int `json:"estimateSeconds"`
	RemainingSeconds int `json:"remainingSeconds"`
	// LoggedSeconds is what this person logged, by worklog author, on the
	// sprint's cards and on sub-tasks outside it, between the sprint's start
	// and its close (or now).
	LoggedSeconds int `json:"loggedSeconds"`
	// InProgressSeconds is how long this person's cards sat in a status of
	// Jira's In Progress category ("indeterminate") inside the same window.
	// A card is charged to whoever holds it now: TAM keeps no assignee
	// changes, so a card that changed hands counts for its current holder.
	InProgressSeconds int `json:"inProgressSeconds"`
	// SpentSeconds is the actual: every hour logged on this person's cards
	// so far, by anyone and at any time, which is what an estimate is
	// measured against. LoggedSeconds above is the sprint's share of work.
	SpentSeconds int `json:"spentSeconds"`
	// Issues is the cards behind Cards, in the order the search returned
	// them, for the People tab's breakdown.
	Issues []PersonCard `json:"issues"`
}

// PersonCard is one card in a person's breakdown. LoggedSeconds is the time
// logged on this card inside the sprint by every author, since a card's own
// row answers for the card rather than for one person.
type PersonCard struct {
	Key               string   `json:"key"`
	Summary           string   `json:"summary"`
	Status            string   `json:"status"`
	StatusCategory    string   `json:"statusCategory"`
	Points            *float64 `json:"points"`
	EstimateSeconds   int      `json:"estimateSeconds"`
	RemainingSeconds  int      `json:"remainingSeconds"`
	SpentSeconds      int      `json:"spentSeconds"`
	LoggedSeconds     int      `json:"loggedSeconds"`
	InProgressSeconds int      `json:"inProgressSeconds"`
}

// StatusCategory names the Jira bucket ("new", "indeterminate", "done") of a
// status, by its id, or by its name for a changelog entry that carried no id.
// It answers "" for a status it does not know.
type StatusCategory func(id, name string) string

// People is the sprint's People section, one row per person, sorted by
// display name ignoring case, with the unassigned row last.
//
// It reads the same issues Build replays and the same window, so the hours
// here and the hours on the burndown are counted over one stretch of time.
func People(sprint backend.Sprint, issues []backend.IssueHistory, category StatusCategory, now time.Time) ([]Person, error) {
	start, _, last, err := window(sprint, now)
	if err != nil {
		return nil, err
	}
	r := roster{}
	for _, h := range issues {
		iss := h.Issue
		p, card := r.assign(iss)
		inProgress, err := timeInProgress(h, category, start, last)
		if err != nil {
			return nil, err
		}
		p.InProgressSeconds += inProgress

		logged := 0
		for _, w := range append(append([]backend.Worklog{}, h.Worklogs...), h.SubtaskWorklogs...) {
			at, err := sprintdate.Parse(w.Started)
			if err != nil {
				return nil, fmt.Errorf("%s has a worklog TAM cannot date (%q): %w", iss.Key, w.Started, err)
			}
			if at.Before(start) || at.After(last) {
				continue
			}
			logged += w.Seconds
			r.person(w.Author, w.AuthorName, w.AuthorEmail).LoggedSeconds += w.Seconds
		}
		card.LoggedSeconds, card.InProgressSeconds = logged, inProgress
		p.Issues = append(p.Issues, card)
	}
	return r.list(), nil
}

// timeInProgress replays one card's status changes and adds up the time it
// spent in the In Progress category between start and last.
//
// The status before the first change is that change's "from" side; a card
// with no status change held its current status throughout.
func timeInProgress(h backend.IssueHistory, category StatusCategory, start, last time.Time) (int, error) {
	type stint struct {
		from     time.Time
		id, name string
	}
	var stints []stint
	for _, c := range h.Changes {
		if c.Field != "status" {
			continue
		}
		at, err := sprintdate.Parse(c.At)
		if err != nil {
			return 0, fmt.Errorf("%s has a status change TAM cannot date (%q): %w", h.Issue.Key, c.At, err)
		}
		if len(stints) == 0 {
			stints = append(stints, stint{id: c.FromID, name: c.From})
		}
		stints = append(stints, stint{from: at, id: c.ToID, name: c.To})
	}
	if len(stints) == 0 {
		stints = []stint{{id: h.Issue.StatusID, name: h.Issue.Status}}
	}

	total := time.Duration(0)
	for i, s := range stints {
		if category(s.id, s.name) != "indeterminate" {
			continue
		}
		from, to := s.from, last
		if i+1 < len(stints) {
			to = stints[i+1].from
		}
		if from.Before(start) {
			from = start
		}
		if to.After(last) {
			to = last
		}
		if to.After(from) {
			total += to.Sub(from)
		}
	}
	return int(total / time.Second), nil
}

// absorb adds another row's figures for the same person into this one.
func (p *Person) absorb(o Person) {
	if p.Email == "" {
		p.Email = o.Email
	}
	p.Cards += o.Cards
	p.Points += o.Points
	p.EstimateSeconds += o.EstimateSeconds
	p.RemainingSeconds += o.RemainingSeconds
	p.LoggedSeconds += o.LoggedSeconds
	p.InProgressSeconds += o.InProgressSeconds
	p.SpentSeconds += o.SpentSeconds
	p.Issues = append(p.Issues, o.Issues...)
}

func seconds(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
