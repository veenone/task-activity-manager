package reports

import (
	"sort"
	"strings"

	"agile-suite/tam/internal/backend"
)

// roster gathers report rows per person, for the sprint's People section and
// the board's allocation. A row is keyed by username, since a display name is
// neither unique nor stable, and by display name when there is no username.
type roster map[string]*Person

// person is the row for someone, made on first sight.
func (r roster) person(username, name, email string) *Person {
	key := username
	if key == "" {
		key = name
	}
	p, ok := r[key]
	if !ok {
		p = &Person{Username: username, Name: name, Issues: []PersonCard{}}
		r[key] = p
	}
	if p.Email == "" {
		p.Email = email
	}
	return p
}

// assign counts a card against its assignee's row: one more card, its points,
// its own estimate, remaining and time spent. It returns the row and the
// card's breakdown entry, for the caller to add the figures only it knows.
func (r roster) assign(iss backend.Issue) (*Person, PersonCard) {
	p := r.person(iss.AssigneeName, iss.Assignee, iss.AssigneeEmail)
	p.Cards++
	if iss.StoryPoints != nil {
		p.Points += *iss.StoryPoints
	}
	p.EstimateSeconds += seconds(iss.OriginalEstimateSeconds)
	p.RemainingSeconds += seconds(iss.RemainingEstimateSeconds)
	p.SpentSeconds += seconds(iss.TimeSpentSeconds)
	return p, PersonCard{
		Key: iss.Key, Summary: iss.Summary, Status: iss.Status, StatusCategory: iss.StatusCategory,
		Points: iss.StoryPoints, EstimateSeconds: seconds(iss.OriginalEstimateSeconds),
		RemainingSeconds: seconds(iss.RemainingEstimateSeconds), SpentSeconds: seconds(iss.TimeSpentSeconds),
	}
}

// list is the rows sorted by display name ignoring case, the unassigned row
// last.
//
// A row that has a display name but no username is the same person as a row
// with that name and a username, when there is exactly one such row: an
// assignee cached before schema 14 carries no username, and a second row for
// them would split their cards in two.
func (r roster) list() []Person {
	named := map[string][]*Person{}
	for _, p := range r {
		if p.Username != "" {
			k := strings.ToLower(p.Name)
			named[k] = append(named[k], p)
		}
	}
	for key, p := range r {
		if p.Username != "" || p.Name == "" {
			continue
		}
		if same := named[strings.ToLower(p.Name)]; len(same) == 1 {
			same[0].absorb(*p)
			delete(r, key)
		}
	}
	out := make([]Person, 0, len(r))
	for _, p := range r {
		out = append(out, *p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Name == "") != (b.Name == "") {
			return b.Name == ""
		}
		if la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name); la != lb {
			return la < lb
		}
		return a.Username < b.Username
	})
	return out
}

// Allocation is the board's open work per assignee (#288): every card on the
// board that is not finished, one row each person, with the cards behind
// each row in the order they were given. Finished is Jira's done category,
// or the status name by backend.IsDone for a row cached before categories
// were. Logged and in-progress time need a changelog, which the board's
// cache does not hold, so those two figures stay zero.
func Allocation(issues []backend.Issue) []Person {
	r := roster{}
	for _, iss := range issues {
		if iss.StatusCategory == "done" || (iss.StatusCategory == "" && backend.IsDone(iss.Status)) {
			continue
		}
		p, card := r.assign(iss)
		p.Issues = append(p.Issues, card)
	}
	return r.list()
}
