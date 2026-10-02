// Package dashboardrepo is the store layer for the Dashboards view: the
// saved Jira filters a profile has pinned, and the last figures each one
// answered with.
//
// A dashboard holds a snapshot rather than a live query for one reason: it
// has to say what it counts while Jira is unreachable, and it has to say
// the same thing a week later when somebody reopens it. What is stored is
// the figures the panels draw, not the rows they were counted from. The
// rows are not drawn anywhere, and keeping thousands of them per dashboard
// to recount later would be storage nothing reads.
//
// Every method takes the profile id, because the table is scoped by it.
package dashboardrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"agile-suite/tam/internal/backend"
)

// Unassigned is the bucket an issue with nobody on it is counted under.
// Dropping it would have a panel about where the work sits stay silent
// about the work sitting nowhere.
const Unassigned = "Unassigned"

// pageSize is how many issues one search asks for. A filter can span
// projects and years, so the fetch pages rather than hoping.
const pageSize = 100

// maxPages caps a refresh. A filter matching a hundred thousand issues is
// a filter nobody dashboards deliberately, and counting it would hold the
// per-profile lock for minutes.
//
// ponytail: a flat cap, not a resumable fetch. The snapshot says how many
// it counted, so a truncated one is visible rather than silent.
const maxPages = 50

// Bucket is one row of a count panel: what it counts and how many.
type Bucket struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Snapshot is what a dashboard draws: the totals and the three rankings,
// as of the moment Refresh last ran.
type Snapshot struct {
	Total           int      `json:"total"`
	Points          float64  `json:"points"`
	EstimateSeconds int      `json:"estimateSeconds"`
	SpentSeconds    int      `json:"spentSeconds"`
	ByStatus        []Bucket `json:"byStatus"`
	ByType          []Bucket `json:"byType"`
	ByAssignee      []Bucket `json:"byAssignee"`
	// Capped says the filter matched more than one refresh counts, so the
	// figures describe the first maxPages pages and not the whole filter.
	Capped bool `json:"capped"`
}

// Dashboard is one pinned filter and its last answer.
type Dashboard struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FilterID string `json:"filterId"`
	JQL      string `json:"jql"`
	// RefreshedAt is when the snapshot was taken, empty for a dashboard
	// nobody has refreshed yet. The view prints it beside the figures,
	// because a number with no date on it reads as today's.
	RefreshedAt string   `json:"refreshedAt"`
	Snapshot    Snapshot `json:"snapshot"`
}

// Searcher is the one thing a refresh needs from a backend: a page of
// whatever a JQL matches. It is declared here rather than taken as a whole
// backend so a test can answer it in ten lines.
type Searcher interface {
	SearchByJQL(ctx context.Context, jql string, startAt, maxResults int) ([]backend.Issue, int, error)
}

// Repository runs the queries. It holds no state beyond the handle.
type Repository struct {
	db *sql.DB
}

// New wraps an open tam.db handle.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

// Create pins a filter as a dashboard. The name is what the person will
// look for it by and the JQL is what it counts, so neither may be blank;
// the filter id may be, which is a dashboard built from typed JQL rather
// than from a saved filter.
func (r *Repository) Create(ctx context.Context, profileID, name, filterID, jql string) (Dashboard, error) {
	d := Dashboard{
		ID:       uuid.NewString(),
		Name:     strings.TrimSpace(name),
		FilterID: strings.TrimSpace(filterID),
		JQL:      strings.TrimSpace(jql),
		Snapshot: emptySnapshot(),
	}
	if d.Name == "" {
		return Dashboard{}, errors.New("a dashboard needs a name to be found by")
	}
	if d.JQL == "" {
		return Dashboard{}, errors.New("a dashboard needs a filter or some JQL to count")
	}
	snapshot, err := json.Marshal(d.Snapshot)
	if err != nil {
		return Dashboard{}, err
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO dashboard (profile_id, id, name, filter_id, jql, refreshed_at, snapshot_json)
		 VALUES (?, ?, ?, ?, ?, '', ?)`,
		profileID, d.ID, d.Name, d.FilterID, d.JQL, string(snapshot)); err != nil {
		return Dashboard{}, fmt.Errorf("create dashboard %s: %w", d.Name, err)
	}
	return d, nil
}

// List is the profile's dashboards, in the order they were made, which is
// the order the person put them in.
func (r *Repository) List(ctx context.Context, profileID string) ([]Dashboard, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, filter_id, jql, refreshed_at, snapshot_json FROM dashboard
		 WHERE profile_id = ? ORDER BY rowid`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list dashboards: %w", err)
	}
	defer rows.Close()
	out := []Dashboard{}
	for rows.Next() {
		var d Dashboard
		var snapshot string
		if err := rows.Scan(&d.ID, &d.Name, &d.FilterID, &d.JQL, &d.RefreshedAt, &snapshot); err != nil {
			return nil, err
		}
		// A snapshot nothing can read is an empty one rather than a failed
		// list: the dashboard is still a dashboard, and the next refresh
		// writes over it.
		d.Snapshot = emptySnapshot()
		if err := json.Unmarshal([]byte(snapshot), &d.Snapshot); err != nil {
			d.Snapshot = emptySnapshot()
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Delete removes one dashboard. Deleting one that is not there is not a
// failure: the list the click came from may be a moment old.
func (r *Repository) Delete(ctx context.Context, profileID, id string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM dashboard WHERE profile_id = ? AND id = ?`, profileID, id); err != nil {
		return fmt.Errorf("delete dashboard: %w", err)
	}
	return nil
}

// SaveSnapshot writes the figures and the moment they were taken.
func (r *Repository) SaveSnapshot(ctx context.Context, profileID, id string, s Snapshot, at string) error {
	encoded, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE dashboard SET snapshot_json = ?, refreshed_at = ? WHERE profile_id = ? AND id = ?`,
		string(encoded), at, profileID, id); err != nil {
		return fmt.Errorf("save dashboard snapshot: %w", err)
	}
	return nil
}

// Refresh runs the dashboard's JQL and stores what came back.
//
// Nothing is written until the whole fetch has succeeded. A refresh that
// cannot reach Jira leaves the last snapshot exactly where it was, because
// a dashboard that blanked itself the moment the network dropped would be
// worse than one saying how old its numbers are.
func (r *Repository) Refresh(ctx context.Context, b Searcher, profileID, id, at string) (Dashboard, error) {
	d, err := r.get(ctx, profileID, id)
	if err != nil {
		return Dashboard{}, err
	}
	var all []backend.Issue
	capped := false
	for page := 0; ; page++ {
		if page >= maxPages {
			capped = true
			break
		}
		issues, total, err := b.SearchByJQL(ctx, d.JQL, len(all), pageSize)
		if err != nil {
			return Dashboard{}, fmt.Errorf("refresh %s: %w", d.Name, err)
		}
		all = append(all, issues...)
		if len(issues) == 0 || len(all) >= total {
			break
		}
	}
	snapshot := Summarise(all)
	snapshot.Capped = capped
	if err := r.SaveSnapshot(ctx, profileID, id, snapshot, at); err != nil {
		return Dashboard{}, err
	}
	d.Snapshot, d.RefreshedAt = snapshot, at
	return d, nil
}

func (r *Repository) get(ctx context.Context, profileID, id string) (Dashboard, error) {
	list, err := r.List(ctx, profileID)
	if err != nil {
		return Dashboard{}, err
	}
	for _, d := range list {
		if d.ID == id {
			return d, nil
		}
	}
	return Dashboard{}, fmt.Errorf("no dashboard %s on this profile", id)
}

// Summarise counts one filter's rows into the figures the panels draw.
//
// Every row counts, whatever project it is in: a saved filter is not bound
// to the profile's own project, and a dashboard that silently dropped the
// rows from elsewhere would answer a different question from the one the
// filter asks.
func Summarise(issues []backend.Issue) Snapshot {
	s := emptySnapshot()
	s.Total = len(issues)
	status, types, people := map[string]int{}, map[string]int{}, map[string]int{}
	for _, iss := range issues {
		if iss.StoryPoints != nil {
			s.Points += *iss.StoryPoints
		}
		if iss.OriginalEstimateSeconds != nil {
			s.EstimateSeconds += *iss.OriginalEstimateSeconds
		}
		if iss.TimeSpentSeconds != nil {
			s.SpentSeconds += *iss.TimeSpentSeconds
		}
		count(status, iss.Status)
		count(types, iss.Type)
		count(people, assigneeOf(iss))
	}
	s.ByStatus, s.ByType, s.ByAssignee = ranked(status), ranked(types), ranked(people)
	return s
}

func assigneeOf(iss backend.Issue) string {
	if strings.TrimSpace(iss.Assignee) == "" {
		return Unassigned
	}
	return iss.Assignee
}

// count adds one to a bucket, skipping a blank name, which is a field no
// sync has filled rather than a thing to rank.
func count(into map[string]int, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	into[name]++
}

// ranked is a count map as a panel reads it: biggest first, and by name
// among equals so two refreshes of the same figures do not reorder the
// rows under the reader.
func ranked(counts map[string]int) []Bucket {
	out := make([]Bucket, 0, len(counts))
	for name, n := range counts {
		out = append(out, Bucket{Name: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// emptySnapshot is the zero a dashboard starts at, with its lists empty
// rather than nil so the wire carries [] and the view maps over them
// without guarding each one.
func emptySnapshot() Snapshot {
	return Snapshot{ByStatus: []Bucket{}, ByType: []Bucket{}, ByAssignee: []Bucket{}}
}
