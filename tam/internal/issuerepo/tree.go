package issuerepo

import (
	"context"
	"fmt"
	"strings"

	"agile-suite/tam/internal/backend"
)

// TreeQuery filters the Epics view. Text matches an epic or any child;
// SprintID keeps epics with a child in the sprint; ShowDone keeps done
// children and fully done epics.
type TreeQuery struct {
	Text     string `json:"text"`
	SprintID string `json:"sprintId"`
	ShowDone bool   `json:"showDone"`
}

// EpicNode is one epic with the children the filter kept and the progress
// of the children the query returned.
type EpicNode struct {
	Issue      backend.Issue   `json:"issue"`
	Children   []backend.Issue `json:"children"`
	Total      int             `json:"total"`
	Done       int             `json:"done"`
	Points     float64         `json:"points"`
	DonePoints float64         `json:"donePoints"`
}

// Tree is the Epics view's data: epics in rank order and the non-epic
// issues without a known epic. Truncated is set when the profile has more
// non-epic issues than the tree will hold in one call.
type Tree struct {
	Epics     []EpicNode      `json:"epics"`
	Orphans   []backend.Issue `json:"orphans"`
	Truncated bool            `json:"truncated"`
}

// treeCap bounds how many non-epic rows one EpicTree call reads. A profile
// past it still gets a tree, just a truncated one; Total and Done are
// computed over the rows the cap let through, not the whole cache.
const treeCap = 5000

// ListEpics returns the profile's epics, drafts first, then by rank.
func (r *Repository) ListEpics(ctx context.Context, profileID string) ([]backend.Issue, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+issueColumns+` FROM issue WHERE profile_id = ? AND type = ?`+issueOrder, profileID, backend.TypeEpic)
	if err != nil {
		return nil, fmt.Errorf("list epics: %w", err)
	}
	defer rows.Close()
	out := []backend.Issue{}
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, iss)
	}
	return out, rows.Err()
}

func matches(iss backend.Issue, text string) bool {
	if text == "" {
		return true
	}
	t := strings.ToLower(text)
	return strings.Contains(strings.ToLower(iss.Key), t) || strings.Contains(strings.ToLower(iss.Summary), t)
}

// EpicTree groups the cached issues under their epics. The sprint filter
// runs in SQL, same as issueFilter's; text stays a Go-side check because a
// matching epic must keep its non-matching children, which a WHERE clause
// cannot express.
func (r *Repository) EpicTree(ctx context.Context, profileID string, q TreeQuery) (Tree, error) {
	epics, err := r.ListEpics(ctx, profileID)
	if err != nil {
		return Tree{}, err
	}
	where := []string{"profile_id = ?", "type <> ?"}
	args := []any{profileID, backend.TypeEpic}
	if q.SprintID != "" {
		where = append(where, `(sprint_id = ? OR
			(type = 'subtask' AND parent_key IN (SELECT key FROM issue WHERE profile_id = ? AND sprint_id = ?)) OR
			key IN (SELECT parent_key FROM issue WHERE profile_id = ? AND type = 'subtask' AND sprint_id = ?))`)
		args = append(args, q.SprintID, profileID, q.SprintID, profileID, q.SprintID)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+issueColumns+` FROM issue WHERE `+strings.Join(where, " AND ")+issueOrder+` LIMIT ?`,
		append(args, treeCap+1)...)
	if err != nil {
		return Tree{}, fmt.Errorf("list children: %w", err)
	}
	defer rows.Close()
	byParent := map[string][]backend.Issue{}
	var others []backend.Issue
	n := 0
	truncated := false
	for rows.Next() {
		if n == treeCap {
			truncated = true
			break
		}
		iss, err := scanIssue(rows)
		if err != nil {
			return Tree{}, err
		}
		byParent[iss.ParentKey] = append(byParent[iss.ParentKey], iss)
		others = append(others, iss)
		n++
	}
	if err := rows.Err(); err != nil {
		return Tree{}, err
	}
	text := strings.TrimSpace(q.Text)
	parents := map[string]bool{}
	for _, c := range others {
		if c.Type != backend.TypeSubtask {
			parents[c.Key] = true
		}
	}
	// Keep the parent as context when a subtask matches, even when the
	// parent is done. Only direct epic children contribute to epic totals.
	family := func(c backend.Issue, epicMatches bool) []backend.Issue {
		parentMatches := epicMatches || matches(c, text)
		children := []backend.Issue{}
		if c.Type != backend.TypeSubtask {
			for _, sub := range byParent[c.Key] {
				if sub.Type == backend.TypeSubtask && (q.ShowDone || !backend.IsDone(sub.Status)) && (parentMatches || matches(sub, text)) {
					children = append(children, sub)
				}
			}
		}
		if len(children) == 0 && ((!q.ShowDone && backend.IsDone(c.Status)) || !parentMatches) {
			return nil
		}
		return append([]backend.Issue{c}, children...)
	}
	isEpic := map[string]bool{}
	tree := Tree{Epics: []EpicNode{}, Orphans: []backend.Issue{}, Truncated: truncated}
	for _, e := range epics {
		isEpic[e.Key] = true
		node := EpicNode{Issue: e, Children: []backend.Issue{}}
		all := byParent[e.Key]
		for _, c := range all {
			if c.Type == backend.TypeSubtask {
				continue
			}
			node.Total++
			if c.StoryPoints != nil {
				node.Points += *c.StoryPoints
			}
			if backend.IsDone(c.Status) {
				node.Done++
				if c.StoryPoints != nil {
					node.DonePoints += *c.StoryPoints
				}
			}
		}
		epicMatches := matches(e, text)
		anyChild := false
		for _, c := range all {
			if c.Type == backend.TypeSubtask {
				continue
			}
			visible := family(c, epicMatches)
			node.Children = append(node.Children, visible...)
			anyChild = anyChild || len(visible) > 0
		}
		if text != "" && !epicMatches && !anyChild {
			continue
		}
		if q.SprintID != "" && !anyChild {
			continue
		}
		if !q.ShowDone && node.Total > 0 && node.Done == node.Total && !anyChild {
			continue
		}
		tree.Epics = append(tree.Epics, node)
	}
	for _, c := range others {
		if c.Type == backend.TypeSubtask && parents[c.ParentKey] {
			continue
		}
		if c.Type != backend.TypeSubtask && isEpic[c.ParentKey] {
			continue
		}
		tree.Orphans = append(tree.Orphans, family(c, false)...)
	}
	return tree, nil
}
