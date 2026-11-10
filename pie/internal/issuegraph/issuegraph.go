// Package issuegraph fetches an issue and the issues it links to for the Link
// map's issue graph, and keeps what Jira sent so the graph still opens and
// expands when Jira cannot be reached (#464).
package issuegraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"agile-suite/pie/internal/query"
)

// Cached is an issue as PIE last fetched it.
type Cached struct {
	Issue     query.Issue `json:"issue"`
	FetchedAt string      `json:"fetchedAt"`
}

// Neighbourhood is an issue, first, and the issues it links to. Offline means
// Jira could not be reached and these came from the store; Missing are the
// neighbours it holds no copy of.
type Neighbourhood struct {
	Issues  []Cached `json:"issues"`
	Offline bool     `json:"offline"`
	Missing []string `json:"missing"`
}

// Fetch reads issues by key from Jira. A key Jira does not have is left out.
type Fetch func(ctx context.Context, keys []string) ([]query.Issue, error)

// Store keeps the issues fetched for the graph, per profile. It is a cache:
// Jira stays the record, and Refresh or a new expansion replaces an entry.
type Store struct{ db *sql.DB }

// New returns a Store on a database opened with saved.Schema.
func New(db *sql.DB) *Store { return &Store{db: db} }

// Put stores issues as fetched at at, replacing earlier copies.
func (s *Store) Put(profileID string, issues []query.Issue, at time.Time) error {
	when := at.UTC().Format(time.RFC3339)
	for _, is := range issues {
		raw, err := json.Marshal(is)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(`INSERT INTO graph_issue (profile_id, issue_key, issue, fetched_at) VALUES (?, ?, ?, ?)
			ON CONFLICT (profile_id, issue_key) DO UPDATE SET issue = excluded.issue, fetched_at = excluded.fetched_at`,
			profileID, is.Key, string(raw), when); err != nil {
			return err
		}
	}
	return nil
}

// Get returns the stored copies of keys that are held.
func (s *Store) Get(profileID string, keys []string) (map[string]Cached, error) {
	out := map[string]Cached{}
	for _, k := range keys {
		var raw, when string
		err := s.db.QueryRow(`SELECT issue, fetched_at FROM graph_issue WHERE profile_id = ? AND issue_key = ?`, profileID, k).Scan(&raw, &when)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var is query.Issue
		if err := json.Unmarshal([]byte(raw), &is); err != nil {
			continue // a damaged row reads as not stored
		}
		out[k] = Cached{Issue: is, FetchedAt: when}
	}
	return out, nil
}

// offlineError says Jira could not be reached and the store had no copy.
type offlineError struct{ key string }

func (e offlineError) Error() string {
	return fmt.Sprintf("Jira cannot be reached and %s is not stored on this computer", e.key)
}

// IsOffline reports whether err means Jira could not be reached: a network
// failure rather than Jira refusing a request.
func IsOffline(err error) bool {
	var ue *url.Error
	var oe offlineError
	return errors.As(err, &ue) || errors.As(err, &oe)
}

// neighbours are the keys an issue links to, its parent and its sub-tasks,
// each once and in that order.
func neighbours(is query.Issue) []string {
	seen := map[string]bool{is.Key: true}
	var out []string
	add := func(k string) {
		if k != "" && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, l := range is.Links {
		add(l.Key)
	}
	add(is.Parent)
	for _, k := range is.Subtasks {
		add(k)
	}
	return out
}

// Expand returns key and its neighbours, from Jira when it answers, and stores
// them. When Jira cannot be reached it returns what the store holds instead.
func Expand(ctx context.Context, fetch Fetch, s *Store, profileID, key string, now time.Time) (Neighbourhood, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	got, err := fetch(ctx, []string{key})
	if IsOffline(err) {
		return stored(s, profileID, key)
	}
	if err != nil {
		return Neighbourhood{}, err
	}
	if len(got) == 0 {
		return Neighbourhood{}, fmt.Errorf("Jira has no issue %s, or this account cannot see it", key)
	}
	around := neighbours(got[0])
	more, err := fetch(ctx, around)
	if IsOffline(err) {
		return stored(s, profileID, key)
	}
	if err != nil {
		return Neighbourhood{}, err
	}
	all := append(got[:1], more...)
	if err := s.Put(profileID, all, now); err != nil {
		return Neighbourhood{}, err
	}
	when := now.UTC().Format(time.RFC3339)
	n := Neighbourhood{Missing: []string{}}
	found := map[string]bool{}
	for _, is := range all {
		found[is.Key] = true
		n.Issues = append(n.Issues, Cached{Issue: is, FetchedAt: when})
	}
	for _, k := range around {
		if !found[k] {
			n.Missing = append(n.Missing, k)
		}
	}
	return n, nil
}

// stored is Expand's answer from the store alone.
func stored(s *Store, profileID, key string) (Neighbourhood, error) {
	held, err := s.Get(profileID, []string{key})
	if err != nil {
		return Neighbourhood{}, err
	}
	first, ok := held[key]
	if !ok {
		return Neighbourhood{}, offlineError{key}
	}
	around := neighbours(first.Issue)
	near, err := s.Get(profileID, around)
	if err != nil {
		return Neighbourhood{}, err
	}
	n := Neighbourhood{Offline: true, Issues: []Cached{first}, Missing: []string{}}
	for _, k := range around {
		if c, ok := near[k]; ok {
			n.Issues = append(n.Issues, c)
		} else {
			n.Missing = append(n.Missing, k)
		}
	}
	return n, nil
}

// Keep stores a run's issues and fetches and stores the issues one link
// further, for a saved query with Keep links for offline on (#464). A
// network failure while fetching the next level leaves the run stored.
func Keep(ctx context.Context, fetch Fetch, s *Store, profileID string, run []query.Issue, now time.Time) error {
	if err := s.Put(profileID, run, now); err != nil {
		return err
	}
	have := map[string]bool{}
	for _, is := range run {
		have[is.Key] = true
	}
	var next []string
	for _, is := range run {
		for _, k := range neighbours(is) {
			if !have[k] {
				have[k] = true
				next = append(next, k)
			}
		}
	}
	if len(next) == 0 {
		return nil
	}
	more, err := fetch(ctx, next)
	if IsOffline(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.Put(profileID, more, now)
}
