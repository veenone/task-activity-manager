package jira

import (
	"context"
	"log"
	"sort"

	corejira "agile-suite/core/jira"
)

// editFieldLabels is how each of TAM's own editable field names reads to a
// person. They match the labels the detail panel draws, so a refusal names
// the field the user typed into rather than the key the journal holds.
var editFieldLabels = map[string]string{
	"summary":     "Summary",
	"description": "Description",
	"priority":    "Priority",
	"labels":      "Labels",
	"storyPoints": "Story points",
	"assignee":    "Assignee",
	"parentKey":   "Epic",
}

// EditableFields is the part of TAM's own editable set that this issue's edit
// screen actually carries, by TAM's names, sorted. Jira decides it per
// project and issue type, so every issue sharing those two answers the same
// and the caller caches it against them rather than against the key.
//
// A field on the screen that TAM does not edit is not named: Reporter is
// editable on some screens and TAM offers no control for it, and naming it
// here would only invite a caller to draw one.
func (b *Backend) EditableFields(ctx context.Context, key string) ([]string, error) {
	meta, err := b.c.EditMeta(ctx, key)
	if err != nil {
		return nil, err
	}
	return editableNames(meta, b.discover(ctx)), nil
}

// editableNames maps the ids on a screen back to TAM's own names. The two
// custom ones come from discovery, so an instance that numbers Story Points
// differently still matches; an instance where discovery found neither
// simply reports neither, which is the same answer a screen without them
// gives.
func editableNames(meta corejira.MetaFields, ids fieldIDs) []string {
	names := map[string]string{
		"summary":     "summary",
		"description": "description",
		"priority":    "priority",
		"labels":      "labels",
		"assignee":    "assignee",
	}
	if ids.Points != "" {
		names[ids.Points] = "storyPoints"
	}
	if ids.EpicLink != "" {
		names[ids.EpicLink] = "parentKey"
	}
	out := []string{}
	for _, f := range meta {
		name, ok := names[f.ID]
		if ok && settable(f) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// settable says a write may set the field. Jira lists what each field on the
// screen accepts, and a TAM edit always sets: it sends one value for the
// whole field rather than adding to or removing from it. A field listing
// only add and remove, or nothing at all, would refuse that write, so
// offering it would be drawing an enabled control over a field Jira will not
// take.
//
// No operations array at all is a different thing from an empty one. An
// older Data Center payload can omit it, and reading silence as a refusal
// would empty the form on those instances, so an absent array leaves the
// field editable and Jira's own answer stays the backstop.
func settable(f corejira.MetaField) bool {
	if f.Operations == nil {
		return true
	}
	for _, op := range f.Operations {
		if op == "set" {
			return true
		}
	}
	return false
}

// editScreen is the set UpdateIssue guards a write with, nil when Jira could
// not be asked. Nil means "unknown", never "empty": an empty set would refuse
// every field, and a screen nothing could read says nothing about what is on
// it, so the write goes and Jira's own refusal stays the backstop.
func (b *Backend) editScreen(ctx context.Context, key string) map[string]bool {
	names, err := b.EditableFields(ctx, key)
	if err != nil {
		log.Printf("tam: the edit screen of %s could not be read, so the edit is sent and Jira's own answer decides: %v", key, err)
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}
