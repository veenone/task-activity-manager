// Package importer turns spreadsheet rows into drafts: a column mapping,
// per-row validation with file row numbers, a dry run that only validates,
// and an import that creates every valid row's draft in one transaction.
package importer

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Mapping names the column header for each draft field; empty means
// unmapped. Summary is required.
//
// Key is the odd one out: it names no field of a draft. A row that carries
// an issue key updates that issue instead of creating a new one, which is
// what makes a file exported from Jira round-trip rather than duplicate
// every row it describes.
type Mapping struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Labels      string `json:"labels"`
	Assignee    string `json:"assignee"`
	StoryPoints string `json:"storyPoints"`
	ParentKey   string `json:"parentKey"`
}

// RowError is one row that was skipped. Row is the file row: the header is
// row 1, the first data row is row 2.
type RowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// Result is what a dry run or an import found and did. Created holds the
// draft keys of rows that had no key of their own; Updated holds the keys of
// rows that named an existing issue and actually changed one of its fields.
// A dry run fills neither, since it writes nothing.
type Result struct {
	Rows    int        `json:"rows"`
	Created []string   `json:"created"`
	Updated []string   `json:"updated"`
	Errors  []RowError `json:"errors"`
}

// synonyms are the normalised header names each field accepts, first
// match wins.
var synonyms = map[string][]string{
	"key":         {"key", "issuekey", "issue", "jirakey", "jiraissue", "jiratask", "jiraid", "url", "link"},
	"type":        {"type", "issuetype"},
	"summary":     {"summary", "title"},
	"description": {"description"},
	"priority":    {"priority"},
	"labels":      {"labels", "label"},
	"assignee":    {"assignee"},
	"storyPoints": {"storypoints", "points", "estimate"},
	"parentKey":   {"parent", "parentkey", "epic", "epiclink"},
}

// normalize lowercases a header and drops spaces, underscores, and hyphens.
func normalize(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	return strings.NewReplacer(" ", "", "_", "", "-", "").Replace(h)
}

// AutoMap picks a column for each field by header name; unmatched fields
// stay empty. A header serves one field only, in field order.
func AutoMap(headers []string) Mapping {
	used := map[int]bool{}
	pick := func(field string) string {
		for _, want := range synonyms[field] {
			for i, h := range headers {
				if !used[i] && normalize(h) == want {
					used[i] = true
					return h
				}
			}
		}
		return ""
	}
	return Mapping{
		Key:         pick("key"),
		Type:        pick("type"),
		Summary:     pick("summary"),
		Description: pick("description"),
		Priority:    pick("priority"),
		Labels:      pick("labels"),
		Assignee:    pick("assignee"),
		StoryPoints: pick("storyPoints"),
		ParentKey:   pick("parentKey"),
	}
}

// columns resolves the mapping to header indexes; -1 means unmapped.
type columns struct {
	key, typ, summary, description, priority, labels, assignee, points, parent int
}

func resolve(headers []string, m Mapping) (columns, error) {
	index := func(name string) (int, error) {
		if name == "" {
			return -1, nil
		}
		for i, h := range headers {
			if h == name {
				return i, nil
			}
		}
		return -1, fmt.Errorf("column %q is not in the file", name)
	}
	var c columns
	var err error
	// A file that only updates needs no Summary column: its rows name the
	// issues they change, and a sheet that carries assignees or estimates
	// but not titles is a normal thing to import.
	if m.Summary == "" && m.Key == "" {
		return c, errors.New("a Summary column must be mapped, or a Key column for rows that update an existing issue")
	}
	fields := []struct {
		name string
		dst  *int
	}{
		{m.Key, &c.key}, {m.Type, &c.typ}, {m.Summary, &c.summary}, {m.Description, &c.description}, {m.Priority, &c.priority},
		{m.Labels, &c.labels}, {m.Assignee, &c.assignee}, {m.StoryPoints, &c.points}, {m.ParentKey, &c.parent},
	}
	for _, f := range fields {
		if *f.dst, err = index(f.name); err != nil {
			return c, err
		}
	}
	return c, nil
}

func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// blank reports whether every mapped cell of row is empty, so a row that is
// blank across the columns the mapping actually reads can be skipped rather
// than counted and reported as a missing summary.
func blank(row []string, c columns) bool {
	for _, i := range []int{c.key, c.typ, c.summary, c.description, c.priority, c.labels, c.assignee, c.points, c.parent} {
		if cell(row, i) != "" {
			return false
		}
	}
	return true
}

// logicalType maps a type cell to a creatable logical type. Blank means
// task; the profile's requirement type name counts as requirement.
func logicalType(raw, requirementType string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(raw))
	switch n {
	case "", backend.TypeTask:
		return backend.TypeTask, nil
	case backend.TypeStory, backend.TypeBug, backend.TypeRequirement, backend.TypeEpic:
		return n, nil
	}
	if requirementType != "" && n == strings.ToLower(strings.TrimSpace(requirementType)) {
		return backend.TypeRequirement, nil
	}
	return "", fmt.Errorf("Type %q cannot be created; use Task, Story, Bug, Epic, or %s", strings.TrimSpace(raw), requirementLabel(requirementType))
}

func requirementLabel(requirementType string) string {
	if strings.TrimSpace(requirementType) == "" {
		return "Requirement"
	}
	return strings.TrimSpace(requirementType)
}

// jiraKey is a project key (Jira DC allows letters, digits, and underscores
// after the first letter) followed by a hyphen and the issue number. The
// hyphen in the character class is not for Jira's sake but for TAM's own
// draft keys, TAM-NEW-3: they reach here whenever someone pastes one back
// in, and they deserve the "that is a draft" answer rather than being turned
// away as unreadable.
var jiraKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*-[0-9]+$`)

// parseKey reads the Key cell. A spreadsheet exported from Jira, or written
// by hand from the browser, holds either the bare key or the browse URL, and
// Excel turns the second into a hyperlink whose text is the URL; both name
// the same issue, so both are accepted. Anything else is an error rather
// than a silent create, because a row the importer cannot match is exactly
// the row that would otherwise duplicate an issue that already exists.
func parseKey(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	if i := strings.Index(v, "/browse/"); i >= 0 {
		v = v[i+len("/browse/"):]
		v = strings.TrimRight(v, "/")
		if j := strings.IndexAny(v, "?#/"); j >= 0 {
			v = v[:j]
		}
	}
	v = strings.ToUpper(strings.TrimSpace(v))
	if !jiraKey.MatchString(v) {
		return "", fmt.Errorf("Key %q is not an issue key or a browse URL", strings.TrimSpace(raw))
	}
	return v, nil
}

// Run validates every data row and, unless dryRun, applies it in one write
// per kind: the rows that name no issue are created as drafts, and the rows
// whose Key cell names one are journaled as edits to it. Rows with errors
// are skipped and listed; a file of good rows and bad ones still lands its
// good ones.
//
// On an update row the Type cell is ignored. An issue's type is not one of
// the fields the journal can edit, and a file exported from Jira carries the
// type of every row, so reading it would fail rows that only restate what
// the issue already is. An empty cell on an update row means "leave this
// field alone" rather than "clear it": a spreadsheet routinely carries only
// the columns someone cared to fill in, and clearing the rest would be a
// destructive reading of an omission.
//
// A row without a key whose type and summary already match a draft from an
// earlier import, or repeat an earlier row of this same file, is skipped, so
// a second pass over a partly-imported file never duplicates.
//
// A row's Parent cell can name an epic created earlier in the same file:
// epic rows must come before the children that point at them, since a
// child's row is checked against the file's own epics in the order they
// appear, not the order they are typed.
func Run(ctx context.Context, repo *issuerepo.Repository, profileID, projectKey, requirementType string, records [][]string, m Mapping, fileName string, dryRun bool) (Result, error) {
	if len(records) < 2 {
		return Result{}, errors.New("the file has a header row but no data rows")
	}
	c, err := resolve(records[0], m)
	if err != nil {
		return Result{}, err
	}
	drafted, err := repo.DraftIndex(ctx, profileID)
	if err != nil {
		return Result{}, err
	}
	nextNum, err := repo.NextDraftNumber(ctx, profileID)
	if err != nil {
		return Result{}, err
	}
	newEpics := map[string]bool{}
	seen := map[string]int{}
	seenKeys := map[string]int{}
	res := Result{Created: []string{}, Updated: []string{}, Errors: []RowError{}}
	var drafts []backend.IssueDraft
	var edits []issuerepo.Edit

	// parentMsg says why a parent cell cannot be used, or "" when it can, and
	// remembers the answer: a file usually points many rows at the same few
	// epics, and each distinct parent is worth one cache read, not one per
	// row. epicOnly is what every level but a sub-task needs, because for
	// them the cell is the Epic Link rather than Jira's own parent.
	parents := map[string]string{}
	parentMsg := func(parent string, epicOnly bool) (string, error) {
		memo := parent
		if !epicOnly {
			memo = "*" + parent
		}
		if msg, ok := parents[memo]; ok {
			return msg, nil
		}
		msg := ""
		iss, err := repo.GetIssue(ctx, profileID, parent)
		switch {
		case errors.Is(err, issuerepo.ErrNotFound):
			msg = fmt.Sprintf("Parent %s is not in the cache. Sync first or clear the cell.", parent)
		case err != nil:
			return "", err
		case iss.Draft:
			msg = fmt.Sprintf("Parent %s is a draft; commit it first.", parent)
		case epicOnly && iss.Type != backend.TypeEpic:
			msg = fmt.Sprintf("Parent %s is not an epic.", parent)
		}
		parents[memo] = msg
		return msg, nil
	}

	for i, row := range records[1:] {
		fileRow := i + 2
		if blank(row, c) {
			continue
		}
		res.Rows++
		fail := func(msg string) { res.Errors = append(res.Errors, RowError{Row: fileRow, Message: msg}) }

		key, err := parseKey(cell(row, c.key))
		if err != nil {
			fail(err.Error() + ".")
			continue
		}
		if key != "" {
			rowEdits, msg, err := updateRow(ctx, repo, profileID, key, row, c, seenKeys, fileRow, parentMsg)
			if err != nil {
				return Result{}, err
			}
			if msg != "" {
				fail(msg)
				continue
			}
			edits = append(edits, rowEdits...)
			continue
		}

		summary := cell(row, c.summary)
		if summary == "" {
			fail("Summary is empty.")
			continue
		}
		typ, err := logicalType(cell(row, c.typ), requirementType)
		if err != nil {
			fail(err.Error() + ".")
			continue
		}
		dupKey := strings.ToLower(typ + "|" + summary)
		if draftKey, ok := drafted[dupKey]; ok {
			fail(fmt.Sprintf("Already a draft (%s); commit or discard it first.", draftKey))
			continue
		}
		if firstRow, ok := seen[dupKey]; ok {
			fail(fmt.Sprintf("Duplicate of row %d.", firstRow))
			continue
		}
		seen[dupKey] = fileRow
		pointsRaw := cell(row, c.points)
		points, err := backend.ParsePoints(pointsRaw)
		if err != nil {
			fail(fmt.Sprintf("Story points %q is not a number.", pointsRaw))
			continue
		}
		parent := cell(row, c.parent)
		if typ == backend.TypeEpic && parent != "" {
			fail("An epic cannot have a parent.")
			continue
		}
		if parent != "" && !newEpics[parent] {
			msg, err := parentMsg(parent, true)
			if err != nil {
				return Result{}, err
			}
			if msg != "" {
				fail(msg)
				continue
			}
		}
		drafts = append(drafts, backend.IssueDraft{
			Type:        typ,
			Summary:     summary,
			Description: cell(row, c.description),
			Priority:    cell(row, c.priority),
			Labels:      backend.SplitLabels(cell(row, c.labels)),
			Assignee:    cell(row, c.assignee),
			StoryPoints: points,
			ParentKey:   parent,
			Extra:       map[string]string{},
		})
		if typ == backend.TypeEpic {
			newEpics[fmt.Sprintf("%s%d", issuerepo.DraftPrefix, nextNum+len(drafts))] = true
		}
	}
	if dryRun {
		return res, nil
	}
	note := "imported from " + fileName
	if len(drafts) > 0 {
		keys, err := repo.CreateDrafts(ctx, profileID, projectKey, drafts, note)
		if err != nil {
			return res, err
		}
		res.Created = keys
	}
	if len(edits) > 0 {
		keys, err := repo.EditFields(ctx, profileID, edits, note)
		if err != nil {
			return res, err
		}
		res.Updated = keys
	}
	return res, nil
}

// updateRow turns one keyed row into the edits it asks for, or returns the
// message that disqualifies it. Only mapped, non-empty cells become edits;
// a value that already matches is dropped later, by the repository, so a
// re-import of an unchanged file leaves nothing pending.
func updateRow(ctx context.Context, repo *issuerepo.Repository, profileID, key string, row []string, c columns, seenKeys map[string]int, fileRow int, parentMsg func(string, bool) (string, error)) ([]issuerepo.Edit, string, error) {
	if firstRow, ok := seenKeys[key]; ok {
		return nil, fmt.Sprintf("Duplicate of row %d; %s appears twice.", firstRow, key), nil
	}

	iss, err := repo.GetIssue(ctx, profileID, key)
	switch {
	case errors.Is(err, issuerepo.ErrNotFound):
		return nil, fmt.Sprintf("%s is not in the cache. Sync first, or clear the Key cell to create it instead.", key), nil
	case err != nil:
		return nil, "", err
	case iss.Draft:
		return nil, fmt.Sprintf("%s is a draft; commit it before importing edits to it.", key), nil
	}

	var edits []issuerepo.Edit
	add := func(field, value string) { edits = append(edits, issuerepo.Edit{Key: key, Field: field, Value: value}) }
	for _, f := range []struct {
		field string
		col   int
	}{
		{"summary", c.summary}, {"description", c.description}, {"priority", c.priority},
		{"labels", c.labels}, {"assignee", c.assignee},
	} {
		if v := cell(row, f.col); v != "" {
			add(f.field, v)
		}
	}
	if raw := cell(row, c.points); raw != "" {
		points, err := backend.ParsePoints(raw)
		if err != nil {
			return nil, fmt.Sprintf("Story points %q is not a number.", raw), nil
		}
		add("storyPoints", backend.FormatPoints(points))
	}
	if parent := cell(row, c.parent); parent != "" {
		if iss.Type == backend.TypeEpic {
			return nil, "An epic cannot have a parent.", nil
		}
		msg, err := parentMsg(parent, iss.Type != backend.TypeSubtask)
		if err != nil {
			return nil, "", err
		}
		if msg != "" {
			return nil, msg, nil
		}
		add("parentKey", parent)
	}
	// Claimed only now: a row turned away for a reason of its own contributed
	// nothing, so the next row naming the same issue is a first attempt, not
	// a duplicate of a failure.
	seenKeys[key] = fileRow
	return edits, "", nil
}
