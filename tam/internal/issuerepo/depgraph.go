package issuerepo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"agile-suite/tam/internal/backend"
)

// DependencyLink is one link as the Dependencies view draws it (#612): from
// its outward issue to its inward one, whichever end it was read from, so
// "A blocks B" is one arrow A to B. Owner and Direction are the issue the
// link is cached or journaled under and its direction from there, which a
// removal or a retype names.
type DependencyLink struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Outward   string `json:"outward"`
	Inward    string `json:"inward"`
	Owner     string `json:"owner"`
	Direction string `json:"direction"`
	// Pending marks a link the journal holds: an add, or with Removal a
	// cached link whose removal waits for Commit. PendingID is the row.
	Pending   bool  `json:"pending"`
	PendingID int64 `json:"pendingId"`
	Removal   bool  `json:"removal"`
}

// ExternalIssue is a link's far end that is not cached, such as an issue in
// another project, known only by what the link row says about it.
type ExternalIssue struct {
	Key       string `json:"key"`
	Summary   string `json:"summary"`
	IssueType string `json:"issueType"`
}

// DependencyGraph is every link of the profile, cached or pending, with the
// cached issues they join and the far ends that are not cached.
type DependencyGraph struct {
	Issues   []backend.Issue  `json:"issues"`
	External []ExternalIssue  `json:"external"`
	Links    []DependencyLink `json:"links"`
}

// ends turns a link seen from owner into its outward and inward issues.
func ends(owner, direction, other string) (string, string) {
	if direction == "inward" {
		return other, owner
	}
	return owner, other
}

// DependencyGraph reads the Dependencies view's data from the cache and the
// journal. Every link type comes back; the view filters by type.
func (r *Repository) DependencyGraph(ctx context.Context, profileID string) (DependencyGraph, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT from_key, to_key, link_type, direction, to_summary, to_type, link_id FROM issue_link WHERE profile_id = ?`, profileID)
	if err != nil {
		return DependencyGraph{}, fmt.Errorf("read links: %w", err)
	}
	var links []DependencyLink
	seen := map[string]int{}
	far := map[string]ExternalIssue{}
	add := func(l DependencyLink) {
		k := l.ID
		if k == "" {
			k = l.Type + "|" + l.Outward + "|" + l.Inward
		}
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = len(links)
		links = append(links, l)
	}
	for rows.Next() {
		var from, to, typ, direction, summary, issueType, id string
		if err := rows.Scan(&from, &to, &typ, &direction, &summary, &issueType, &id); err != nil {
			rows.Close()
			return DependencyGraph{}, err
		}
		out, in := ends(from, direction, to)
		add(DependencyLink{ID: id, Type: typ, Outward: out, Inward: in, Owner: from, Direction: direction})
		far[to] = ExternalIssue{Key: to, Summary: summary, IssueType: issueType}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return DependencyGraph{}, err
	}

	pending, err := r.ListPendingChanges(ctx, profileID)
	if err != nil {
		return DependencyGraph{}, err
	}
	for _, p := range pending {
		if p.EntityType != EntityLink {
			continue
		}
		var d backend.LinkDraft
		if json.Unmarshal([]byte(p.AfterVal), &d) != nil {
			continue
		}
		if d.RemoveID != "" {
			if i, ok := seen[d.RemoveID]; ok {
				links[i].Pending, links[i].Removal, links[i].PendingID = true, true, p.ID
			}
			continue
		}
		out, in := ends(p.EntityKey, d.Direction, d.ToKey)
		add(DependencyLink{Type: d.Type, Outward: out, Inward: in, Owner: p.EntityKey, Direction: d.Direction, Pending: true, PendingID: p.ID})
		if _, ok := far[d.ToKey]; !ok {
			far[d.ToKey] = ExternalIssue{Key: d.ToKey, Summary: d.ToSummary, IssueType: d.ToType}
		}
	}

	keys := []string{}
	inGraph := map[string]bool{}
	for _, l := range links {
		for _, k := range []string{l.Outward, l.Inward} {
			if !inGraph[k] {
				inGraph[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	issues, err := r.IssuesByKeys(ctx, r.db, profileID, keys)
	if err != nil {
		return DependencyGraph{}, err
	}
	cached := map[string]bool{}
	for _, iss := range issues {
		cached[iss.Key] = true
	}
	external := []ExternalIssue{}
	for _, k := range keys {
		if !cached[k] {
			e := far[k]
			e.Key = k
			external = append(external, e)
		}
	}
	sort.Slice(links, func(i, j int) bool {
		a, b := links[i], links[j]
		if a.Outward != b.Outward {
			return a.Outward < b.Outward
		}
		if a.Inward != b.Inward {
			return a.Inward < b.Inward
		}
		return a.Type < b.Type
	})
	if links == nil {
		links = []DependencyLink{}
	}
	return DependencyGraph{Issues: issues, External: external, Links: links}, nil
}
