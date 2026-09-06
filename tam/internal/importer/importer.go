// Package importer turns spreadsheet rows into drafts: a column mapping,
// per-row validation with file row numbers, a dry run that only validates,
// and an import that creates every valid row's draft in one transaction.
package importer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Mapping names the column header for each draft field; empty means
// unmapped. Summary is required.
type Mapping struct {
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

// Result is what a dry run or an import found and did.
type Result struct {
	Rows    int        `json:"rows"`
	Created []string   `json:"created"`
	Errors  []RowError `json:"errors"`
}

// synonyms are the normalised header names each field accepts, first
// match wins.
var synonyms = map[string][]string{
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
	typ, summary, description, priority, labels, assignee, points, parent int
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
	if m.Summary == "" {
		return c, errors.New("a Summary column must be mapped")
	}
	fields := []struct {
		name string
		dst  *int
	}{
		{m.Type, &c.typ}, {m.Summary, &c.summary}, {m.Description, &c.description}, {m.Priority, &c.priority},
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

// logicalType maps a type cell to a creatable logical type. Blank means
// task; the profile's requirement type name counts as requirement.
func logicalType(cell, requirementType string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(cell))
	switch n {
	case "", backend.TypeTask:
		return backend.TypeTask, nil
	case backend.TypeStory, backend.TypeBug, backend.TypeRequirement:
		return n, nil
	}
	if requirementType != "" && n == strings.ToLower(strings.TrimSpace(requirementType)) {
		return backend.TypeRequirement, nil
	}
	return "", fmt.Errorf("Type %q cannot be created; use Task, Story, Bug, or %s", strings.TrimSpace(cell), requirementLabel(requirementType))
}

func requirementLabel(requirementType string) string {
	if strings.TrimSpace(requirementType) == "" {
		return "Requirement"
	}
	return strings.TrimSpace(requirementType)
}

// Run validates every data row and, unless dryRun, creates the valid ones
// as drafts in one transaction. Rows with errors are skipped and listed.
func Run(ctx context.Context, repo *issuerepo.Repository, profileID, projectKey, requirementType string, records [][]string, m Mapping, fileName string, dryRun bool) (Result, error) {
	if len(records) < 2 {
		return Result{}, errors.New("the file has a header row but no data rows")
	}
	c, err := resolve(records[0], m)
	if err != nil {
		return Result{}, err
	}
	res := Result{Rows: len(records) - 1, Created: []string{}, Errors: []RowError{}}
	var drafts []backend.IssueDraft
	parents := map[string]bool{}
	for i, row := range records[1:] {
		fileRow := i + 2
		fail := func(msg string) { res.Errors = append(res.Errors, RowError{Row: fileRow, Message: msg}) }
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
		points, err := backend.ParsePoints(cell(row, c.points))
		if err != nil {
			fail(fmt.Sprintf("Story points %q is not a number.", cell(row, c.points)))
			continue
		}
		parent := cell(row, c.parent)
		if parent != "" {
			known, seen := parents[parent]
			if !seen {
				_, err := repo.GetIssue(ctx, profileID, parent)
				switch {
				case errors.Is(err, issuerepo.ErrNotFound):
					known = false
				case err != nil:
					return Result{}, err
				default:
					known = true
				}
				parents[parent] = known
			}
			if !known {
				fail(fmt.Sprintf("Parent %s is not in the cache. Sync first or clear the cell.", parent))
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
	}
	if dryRun || len(drafts) == 0 {
		return res, nil
	}
	keys, err := repo.CreateDrafts(ctx, profileID, projectKey, drafts, "imported from "+fileName)
	if err != nil {
		return res, err
	}
	res.Created = keys
	return res, nil
}

// TemplateCSV is a starter file with the supported columns and one example
// row.
func TemplateCSV() []byte {
	return []byte("Type,Summary,Description,Priority,Labels,Assignee,Story Points,Parent\n" +
		"Story,Apply promo code at payment step,As a shopper I can enter a promo code,High,\"checkout, promo\",ranand,5,PLAT-350\n")
}
