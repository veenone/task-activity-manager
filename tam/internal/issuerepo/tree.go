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
//
// Types, Statuses and Assignees read the same as IssueQuery's: empty is no
// filter, several entries are an OR inside the field, and the fields are an
// AND. They narrow the children, and an epic the filter leaves no child is
// dropped, the rule SprintID already follows. The tree is read for the work
// under an epic, so an epic's own type, status and assignee are not matched.
type TreeQuery struct {
	Text      string   `json:"text"`
	SprintID  string   `json:"sprintId"`
	ShowDone  bool     `json:"showDone"`
	Types     []string `json:"types"`
	Statuses  []string `json:"statuses"`
	Assignees []string `json:"assignees"`
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

// Tree is the epic half of the Epics view: epics in rank order with the
// work that hangs off them. The issues under no epic are the other half and
// do not come through here at all. They are a flat list far larger than a
// tree can draw, so the view pages them through ListIssues with
// IssueQuery.NoEpic instead (#341).
//
// IssueTotal and OrphanTotal are the two sides' totals, counted past the
// cap the read stops at so each tab can show a number it can stand behind.
// They partition the profile's non-epic issues between them. Truncated says
// the epic read itself hit the cap, which is the only state in which an
// epic's own counts are short.
type Tree struct {
	Epics       []EpicNode `json:"epics"`
	Truncated   bool       `json:"truncated"`
	IssueTotal  int        `json:"issueTotal"`
	OrphanTotal int        `json:"orphanTotal"`
}

// treeCap bounds how many rows one read of the tree returns. It applies to
// each half on its own: before #338 a single capped read in rank order
// served both, so a cache full of issues with no epic pushed real epic
// children past the cap and left their epics drawn as "0 of 0".
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

// underEpic matches the rows the epic tree is built from: the children of
// an epic, and the subtasks hanging off those children. Its negation is the
// rest of the cache, which is what the view lists under "No epic".
//
// The epic keys are a subquery rather than a Go-side list because the list
// would have to be spliced into the SQL as placeholders, and a profile's
// epics are already one indexed read away.
func underEpic(profileID string) (string, []any) {
	epicKeys := `SELECT key FROM issue WHERE profile_id = ? AND type = ?`
	clause := `(parent_key IN (` + epicKeys + `) OR parent_key IN (` +
		`SELECT key FROM issue WHERE profile_id = ? AND type <> ? AND parent_key IN (` + epicKeys + `)))`
	return clause, []any{profileID, backend.TypeEpic, profileID, backend.TypeEpic, profileID, backend.TypeEpic}
}

// readTreeRows runs one capped read and says whether the cap cut it short.
func (r *Repository) readTreeRows(ctx context.Context, where []string, args []any) ([]backend.Issue, bool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+issueColumns+` FROM issue WHERE `+strings.Join(where, " AND ")+issueOrder+` LIMIT ?`,
		append(append([]any{}, args...), treeCap+1)...)
	if err != nil {
		return nil, false, fmt.Errorf("list children: %w", err)
	}
	defer rows.Close()
	out := []backend.Issue{}
	truncated := false
	for rows.Next() {
		if len(out) == treeCap {
			truncated = true
			break
		}
		iss, err := scanIssue(rows)
		if err != nil {
			return nil, false, err
		}
		out = append(out, iss)
	}
	return out, truncated, rows.Err()
}

// countTreeRows counts what one side of the split holds, past the cap the
// read itself stops at. It is what lets each tab show a total it can stand
// behind rather than the number of rows that happened to fit.
func (r *Repository) countTreeRows(ctx context.Context, where []string, args []any) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM issue WHERE `+strings.Join(where, " AND "), args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count tree rows: %w", err)
	}
	return n, nil
}

// EpicTree groups the cached issues under their epics. The sprint filter
// runs in SQL, same as issueFilter's; text stays a Go-side check because a
// matching epic must keep its non-matching children, which a WHERE clause
// cannot express.
//
// The epic tree and the issues with no epic are read separately so one
// cannot starve the other of rows (#338).
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
	// Type, status and assignee narrow the children in SQL, beside the
	// sprint clause and for the same reason: the reads below stop at
	// treeCap, so a Go-side filter would narrow whichever rows the cap let
	// through and the counts would describe a different set than the tree.
	if narrow, narrowArgs := narrowRows(q.Types, q.Statuses, q.Assignees); narrow != "" {
		// A row that matched, or the parent of a subtask that matched. The
		// parent is read as context so that the match is drawn at all:
		// family below hangs the kept children off their parent, and with
		// the parent filtered out the child had nothing to sit under and
		// went missing with it (#353). Its siblings do not come along.
		where = append(where, "(("+narrow+") OR key IN (SELECT parent_key FROM issue WHERE profile_id = ? AND type = ? AND "+narrow+"))")
		args = append(args, narrowArgs...)
		args = append(args, profileID, backend.TypeSubtask)
		args = append(args, narrowArgs...)
	}
	under, underArgs := underEpic(profileID)
	// Each read gets its own slice: appending to a shared one would hand
	// the second its backing array.
	epicWhere := append(append([]string{}, where...), under)
	epicArgs := append(append([]any{}, args...), underArgs...)

	children, truncated, err := r.readTreeRows(ctx, epicWhere, epicArgs)
	if err != nil {
		return Tree{}, err
	}
	// The two totals label the tabs, so they measure how big each side of
	// the profile is and ignore q entirely. Each tab narrows within its own
	// side and says so in its own way: the tree by what it draws, the
	// no-epic list through its pager. A badge that moved with the filter
	// would be a third count disagreeing with both.
	sideArgs := append([]any{profileID, backend.TypeEpic}, underArgs...)
	side := []string{"profile_id = ?", "type <> ?"}
	issueTotal, err := r.countTreeRows(ctx, append(append([]string{}, side...), under), sideArgs)
	if err != nil {
		return Tree{}, err
	}
	orphanTotal, err := r.countTreeRows(ctx, append(append([]string{}, side...), "NOT "+under), sideArgs)
	if err != nil {
		return Tree{}, err
	}

	byParent := map[string][]backend.Issue{}
	for _, iss := range children {
		byParent[iss.ParentKey] = append(byParent[iss.ParentKey], iss)
	}
	text := strings.TrimSpace(q.Text)
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
	// narrowed is true when the user asked for a subset of the children. An
	// epic the filter left nothing under is then dropped rather than drawn
	// empty, because an empty epic row under a filter reads as an epic with
	// no work in it.
	narrowed := q.SprintID != "" || len(q.Types) > 0 || len(q.Statuses) > 0 || len(q.Assignees) > 0
	tree := Tree{
		Epics: []EpicNode{}, Truncated: truncated,
		IssueTotal: issueTotal, OrphanTotal: orphanTotal,
	}
	for _, e := range epics {
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
		if narrowed && !anyChild {
			continue
		}
		if !q.ShowDone && node.Total > 0 && node.Done == node.Total && !anyChild {
			continue
		}
		tree.Epics = append(tree.Epics, node)
	}
	return tree, nil
}
