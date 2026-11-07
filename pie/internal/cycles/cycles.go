// Package cycles keeps an analyst's reporting cycles. A cycle is a copy of
// every case with its projects and verdicts, taken when the analyst asks, so
// later verdicts do not change what a cycle reported. Each cycle is compared
// with the last published one before it. Its table is in saved's schema.
package cycles

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"agile-suite/pie/internal/cases"
)

// Cycle is one reporting period. PublishedAt is "" until it is published.
type Cycle struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	TakenAt     string `json:"takenAt"`
	PublishedAt string `json:"publishedAt"`
}

// Change is one project whose verdict on a case differs from the last
// published cycle.
type Change struct {
	Case    string `json:"case"`
	Project string `json:"project"`
	From    string `json:"from"`
	To      string `json:"to"`
}

// Changes is what differs from the cycle named Since, or from nothing when
// Since is "": then every case is New.
type Changes struct {
	Since    string   `json:"since"`
	New      []string `json:"new"`
	Gone     []string `json:"gone"`
	Verdicts []Change `json:"verdicts"`
}

// Detail is a cycle with its copy of the cases and what changed.
type Detail struct {
	Cycle
	Cases   []cases.Detail `json:"cases"`
	Changes Changes        `json:"changes"`
}

var (
	errNoCycle   = errors.New("that cycle no longer exists")
	errPublished = errors.New("that cycle is published, so it stays as it was")
)

// Store reads and writes cycles.
type Store struct {
	db    *sql.DB
	cases *cases.Store
}

// New returns a Store on a database opened with saved.Schema.
func New(db *sql.DB, cs *cases.Store) *Store { return &Store{db: db, cases: cs} }

// List returns the profile's cycles, newest first.
func (s *Store) List(profileID string) ([]Cycle, error) {
	rows, err := s.db.Query(`SELECT id, name, taken_at, published_at FROM report_cycle
		WHERE profile_id = ? ORDER BY id DESC`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Cycle{}
	for rows.Next() {
		var c Cycle
		if err := rows.Scan(&c.ID, &c.Name, &c.TakenAt, &c.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Take stores a new cycle holding the profile's cases as they are now.
func (s *Store) Take(profileID, name string, at time.Time) (Cycle, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Cycle{}, errors.New("give the cycle a name")
	}
	copied, err := s.copy(profileID)
	if err != nil {
		return Cycle{}, err
	}
	when := at.UTC().Format(time.RFC3339)
	r, err := s.db.Exec(`INSERT INTO report_cycle (profile_id, name, taken_at, cases) VALUES (?, ?, ?, ?)
		ON CONFLICT (profile_id, name) DO NOTHING`, profileID, name, when, copied)
	if err != nil {
		return Cycle{}, err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return Cycle{}, errors.New("a cycle with that name already exists")
	}
	id, err := r.LastInsertId()
	return Cycle{ID: id, Name: name, TakenAt: when}, err
}

// Retake replaces an unpublished cycle's copy with the cases as they are now.
func (s *Store) Retake(profileID string, id int64, at time.Time) error {
	copied, err := s.copy(profileID)
	if err != nil {
		return err
	}
	r, err := s.db.Exec(`UPDATE report_cycle SET cases = ?, taken_at = ? WHERE profile_id = ? AND id = ? AND published_at = ''`,
		copied, at.UTC().Format(time.RFC3339), profileID, id)
	return s.draftRow(profileID, id, r, err)
}

// Delete removes an unpublished cycle.
func (s *Store) Delete(profileID string, id int64) error {
	r, err := s.db.Exec(`DELETE FROM report_cycle WHERE profile_id = ? AND id = ? AND published_at = ''`, profileID, id)
	return s.draftRow(profileID, id, r, err)
}

// Get returns one cycle with its cases and what changed since the last cycle
// published before it.
func (s *Store) Get(profileID string, id int64) (Detail, error) {
	var d Detail
	var copied string
	err := s.db.QueryRow(`SELECT id, name, taken_at, published_at, cases FROM report_cycle WHERE profile_id = ? AND id = ?`, profileID, id).
		Scan(&d.ID, &d.Name, &d.TakenAt, &d.PublishedAt, &copied)
	if errors.Is(err, sql.ErrNoRows) {
		return Detail{}, errNoCycle
	}
	if err != nil {
		return Detail{}, err
	}
	if err := json.Unmarshal([]byte(copied), &d.Cases); err != nil {
		return Detail{}, err
	}
	var before []cases.Detail
	err = s.db.QueryRow(`SELECT name, cases FROM report_cycle WHERE profile_id = ? AND id < ? AND published_at != ''
		ORDER BY id DESC LIMIT 1`, profileID, id).Scan(&d.Changes.Since, &copied)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return Detail{}, err
	default:
		if err := json.Unmarshal([]byte(copied), &before); err != nil {
			return Detail{}, err
		}
	}
	since := d.Changes.Since
	d.Changes = diff(before, d.Cases)
	d.Changes.Since = since
	return d, nil
}

func (s *Store) copy(profileID string) (string, error) {
	grid, err := s.cases.Grid(profileID)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(grid)
	return string(b), err
}

// draftRow turns an update that touched no row into the reason: the cycle is
// gone, or it is published.
func (s *Store) draftRow(profileID string, id int64, r sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n > 0 {
		return nil
	}
	var one int
	if err := s.db.QueryRow(`SELECT 1 FROM report_cycle WHERE profile_id = ? AND id = ?`, profileID, id).Scan(&one); errors.Is(err, sql.ErrNoRows) {
		return errNoCycle
	}
	return errPublished
}

// diff lists the cases new in now, the cases gone from before, and every
// project whose verdict moved. A project new on a case counts as moved from
// not assessed, so one that is still not assessed is no change.
func diff(before, now []cases.Detail) Changes {
	was := map[string]map[string]string{}
	for _, c := range before {
		was[c.Key] = map[string]string{}
		for _, p := range c.Projects {
			was[c.Key][p.Key] = p.Verdict
		}
	}
	ch := Changes{New: []string{}, Gone: []string{}, Verdicts: []Change{}}
	seen := map[string]bool{}
	for _, c := range now {
		seen[c.Key] = true
		old, ok := was[c.Key]
		if !ok {
			ch.New = append(ch.New, c.Key)
			continue
		}
		for _, p := range c.Projects {
			from := old[p.Key]
			if from == "" {
				from = cases.NotAssessed
			}
			if from != p.Verdict {
				ch.Verdicts = append(ch.Verdicts, Change{Case: c.Key, Project: p.Key, From: from, To: p.Verdict})
			}
		}
	}
	for _, c := range before {
		if !seen[c.Key] {
			ch.Gone = append(ch.Gone, c.Key)
		}
	}
	return ch
}
