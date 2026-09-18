package jira

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// GetIssue reads one issue with the same fields the search asks for, so it
// parses to the same row.
func (b *Backend) GetIssue(ctx context.Context, key string) (backend.Issue, error) {
	ids := b.discover(ctx)
	fields := append(append([]string{}, baseFields...), ids.list()...)
	raw, err := b.c.GetIssue(ctx, key, fields)
	if err != nil {
		return backend.Issue{}, err
	}
	return parseIssue(raw, ids, b.requirementType, b.typesOrEmpty(ctx, projectOf(key))), nil
}

// jiraFields turns the journal's text values into Jira's field shapes. An
// empty priority, assignee, or points clears the field with null.
func jiraFields(fields map[string]string, ids fieldIDs) (map[string]any, error) {
	out := map[string]any{}
	for name, v := range fields {
		switch name {
		case "summary":
			out["summary"] = v
		case "description":
			out["description"] = v
		case "priority":
			out["priority"] = nameOrNull(v)
		case "assignee":
			out["assignee"] = nameOrNull(v)
		case "labels":
			out["labels"] = backend.SplitLabels(v)
		case "storyPoints":
			if ids.Points == "" {
				return nil, errors.New("this Jira has no Story Points field, so points cannot be pushed")
			}
			p, err := backend.ParsePoints(v)
			if err != nil {
				return nil, err
			}
			if p == nil {
				out[ids.Points] = nil
			} else {
				out[ids.Points] = *p
			}
		case "parentKey":
			if ids.EpicLink == "" {
				return nil, errors.New("this Jira has no Epic Link field, so the epic cannot be pushed")
			}
			if v == "" {
				out[ids.EpicLink] = nil
			} else {
				out[ids.EpicLink] = v
			}
		default:
			return nil, fmt.Errorf("field %q cannot be sent to Jira", name)
		}
	}
	return out, nil
}

func nameOrNull(v string) any {
	if v == "" {
		return nil
	}
	return map[string]string{"name": v}
}

// UpdateIssue PUTs the edited fields. Jira answers 204 on success and a
// 400 with a per-field message otherwise; the client's error carries it.
func (b *Backend) UpdateIssue(ctx context.Context, key string, fields map[string]string) error {
	ids := b.discover(ctx)
	jf, err := jiraFields(fields, ids)
	if err != nil {
		return err
	}
	return b.c.Put(ctx, "/rest/api/2/issue/"+url.PathEscape(key), map[string]any{"fields": jf})
}

// projectOf is the project key an issue key belongs to: everything before the
// last hyphen, since a project key may itself contain hyphens.
func projectOf(issueKey string) string {
	if i := strings.LastIndex(issueKey, "-"); i > 0 {
		return issueKey[:i]
	}
	return issueKey
}

// CreateIssue POSTs the draft. TAM's own fields are set first and are never
// overwritten; extras are then shaped from the type's create metadata and
// filtered by applyExtras, so a field off the create screen is never sent.
func (b *Backend) CreateIssue(ctx context.Context, projectKey string, d backend.IssueDraft) (string, error) {
	ids := b.discover(ctx)
	names := jiraTypeNames([]string{d.Type}, b.requirementType, b.typesOrEmpty(ctx, projectKey))
	if len(names) == 0 {
		if d.Type == backend.TypeSubtask {
			return "", fmt.Errorf("%s has no sub-task issue type", projectKey)
		}
		return "", fmt.Errorf("unknown issue type %q", d.Type)
	}
	fields := map[string]any{
		"project":   map[string]string{"key": projectKey},
		"issuetype": map[string]string{"name": names[0]},
		"summary":   d.Summary,
	}
	if d.Description != "" {
		fields["description"] = d.Description
	}
	if d.Priority != "" {
		fields["priority"] = map[string]string{"name": d.Priority}
	}
	if d.Assignee != "" {
		fields["assignee"] = map[string]string{"name": d.Assignee}
	}
	if len(d.Labels) > 0 {
		fields["labels"] = d.Labels
	}
	if d.StoryPoints != nil && ids.Points != "" {
		fields[ids.Points] = *d.StoryPoints
	}
	// A sub-task hangs off its parent through Jira's own parent field, not
	// through the Epic Link, and cannot exist without one.
	if d.Type == backend.TypeSubtask {
		if d.ParentKey == "" {
			return "", errors.New("a sub-task needs a parent issue")
		}
		fields["parent"] = map[string]string{"key": d.ParentKey}
	} else if d.ParentKey != "" && d.Type != backend.TypeEpic {
		if ids.EpicLink != "" {
			fields[ids.EpicLink] = d.ParentKey
		} else {
			log.Printf("tam: %s has no Epic Link field; parent %s dropped from the create of %q", b.c.BaseURL(), d.ParentKey, d.Summary)
		}
	}
	if len(d.Extra) > 0 {
		b.applyExtras(ctx, projectKey, names[0], d, ids, fields)
	}
	if d.Type == backend.TypeEpic && ids.EpicName != "" {
		if _, set := fields[ids.EpicName]; !set {
			fields[ids.EpicName] = d.Summary
		}
	}
	var resp struct {
		Key string `json:"key"`
	}
	if err := b.c.WriteJSONReturning(ctx, http.MethodPost, "/rest/api/2/issue", map[string]any{"fields": fields}, &resp); err != nil {
		return "", err
	}
	if resp.Key == "" {
		return "", errors.New("Jira created the issue but returned no key")
	}
	return resp.Key, nil
}

// applyExtras writes the draft's extra fields into the payload, shaped from
// the type's create metadata. Four rules keep an extra out, each logged:
//
//   - the payload already holds the id: what the form set is never
//     overwritten, which is how Extra["parent"] once replaced the
//     {"key": ...} object with a string;
//   - the field is one of TAM's own, set by the form or by nothing. This is
//     the same isBaseField the dialog filters with, so it also catches a
//     parent Jira reports under a custom id and an Agile field discovery
//     missed, neither of which the id alone would name;
//   - the draft carries the ids its dialog offered and this one is not
//     among them, which is the screen check Commit makes with no network;
//   - the metadata read now came from the per-type endpoint and no longer
//     lists the id, so the field is not on the screen today.
//
// An unreadable metadata read shapes every surviving extra as text, and
// Jira's own validation decides.
func (b *Backend) applyExtras(ctx context.Context, projectKey, typeName string, d backend.IssueDraft, ids fieldIDs, fields map[string]any) {
	meta, metaErr := b.createMeta(ctx, projectKey, d.Type)
	var screen map[string]bool
	if d.ScreenFields != nil {
		screen = make(map[string]bool, len(d.ScreenFields))
		for _, id := range d.ScreenFields {
			screen[id] = true
		}
	}
	extraIDs := make([]string, 0, len(d.Extra))
	for id := range d.Extra {
		extraIDs = append(extraIDs, id)
	}
	sort.Strings(extraIDs)
	for _, id := range extraIDs {
		v := d.Extra[id]
		if strings.TrimSpace(v) == "" {
			continue
		}
		// The metadata is read before the base-field check, not after it,
		// because the schema is half of what says a field is TAM's own: a
		// metadata read that failed leaves the id to answer on its own.
		f, known := meta.Field(id)
		if !known {
			f = corejira.MetaField{ID: id, Schema: corejira.MetaSchema{Type: "string"}}
		}
		if _, set := fields[id]; set || isBaseField(f, ids) {
			log.Printf("tam: the %s create of %q ignores extra %s, which is one of TAM's own fields", typeName, d.Summary, id)
			continue
		}
		if screen != nil && !screen[id] {
			log.Printf("tam: the %s create of %q leaves out %s, which was not on the screen it was drafted against", typeName, d.Summary, id)
			continue
		}
		if metaErr == nil && meta.Source == corejira.MetaPerType && !known {
			log.Printf("tam: the %s create of %q leaves out %s, which is not on the %s create screen", typeName, d.Summary, id, typeName)
			continue
		}
		if metaErr == nil && meta.Source == corejira.MetaClassic && !(known && f.Required) {
			log.Printf("tam: the %s create of %q leaves out %s, because this Jira only reports its create fields through the classic call, which does not say what is on the screen", typeName, d.Summary, id)
			continue
		}
		fields[id] = corejira.ShapeValue(f, v)
	}
}

// baseFieldIDs are the create-meta ids TAM's own form carries or sets itself.
// An extra may never name one: the dialog does not offer them, and a create
// never lets an extra overwrite what the form set.
var baseFieldIDs = map[string]bool{
	"project": true, "issuetype": true, "summary": true, "description": true,
	"priority": true, "assignee": true, "labels": true, "reporter": true, "parent": true,
}

// agileBaseTypes are the custom field types behind the Agile fields TAM owns.
// They are matched by type as well as by the discovered ids, because
// discovery can fail on an instance where the field still sits on the screen.
var agileBaseTypes = []string{":gh-epic-link", ":gh-epic-label", ":gh-sprint", ":gh-lexo-rank"}

// isBaseFieldID says the id is one of TAM's own fields: a fixed system id,
// or the Story Points, Epic Link, Epic Name, Sprint, or Rank this instance
// discovered.
func isBaseFieldID(id string, ids fieldIDs) bool {
	if baseFieldIDs[id] {
		return true
	}
	for _, own := range ids.list() {
		if id == own {
			return true
		}
	}
	return false
}

// isBaseField is isBaseFieldID plus the schema, which also catches a parent
// field reported under another id and an Agile field discovery missed.
func isBaseField(f corejira.MetaField, ids fieldIDs) bool {
	if isBaseFieldID(f.ID, ids) || f.Schema.System == "parent" {
		return true
	}
	for _, suffix := range agileBaseTypes {
		if strings.HasSuffix(f.Schema.Custom, suffix) {
			return true
		}
	}
	return false
}

// createMeta reads the create fields of one logical type in the project. The
// type's id comes from the project's own type list, so the per-type
// endpoint can be asked; a type the list was read and does not name is asked
// for by name through the classic call. A list that cannot be read is an
// error, not a reason to fall back: the classic call lists fields that are
// not on the screen.
func (b *Backend) createMeta(ctx context.Context, projectKey, logicalType string) (corejira.CreateMeta, error) {
	pt, err := b.resolveTypes(ctx, projectKey)
	if err != nil {
		return corejira.CreateMeta{}, fmt.Errorf("read the issue types of %s: %w", projectKey, err)
	}
	names := jiraTypeNames([]string{logicalType}, b.requirementType, pt)
	if len(names) == 0 {
		return corejira.CreateMeta{}, fmt.Errorf("unknown issue type %q", logicalType)
	}
	return b.c.CreateMeta(ctx, projectKey, pt.ids[strings.ToLower(names[0])], names[0])
}

// CreateFields returns the create-screen fields of the type beyond the
// form's own, required and optional, sorted by name, with their options when
// they have any. An optional field no text form can fill is left out; a
// required one is still offered as text, because leaving it out would only
// move the failure to a Jira 400 at Commit.
func (b *Backend) CreateFields(ctx context.Context, projectKey, logicalType string) (backend.CreateFieldSet, error) {
	meta, err := b.createMeta(ctx, projectKey, logicalType)
	if err != nil {
		return backend.CreateFieldSet{}, err
	}
	ids := b.discover(ctx)
	out := []backend.FieldSpec{}
	for _, f := range meta.Fields {
		if isBaseField(f, ids) {
			continue
		}
		// A classic answer is not the create screen: on some Data Center
		// versions it lists fields the screen does not carry, and sending one
		// of those fails the whole create. Only what Jira marks required is
		// offered there, for the same reason an unfillable required field
		// still is: leaving a required field out would only move the failure
		// to a Jira 400 at Commit.
		if meta.Source == corejira.MetaClassic && !f.Required {
			continue
		}
		kind := f.Kind()
		if kind == corejira.KindOther {
			if !f.Required {
				continue
			}
			kind = corejira.KindString
		}
		spec := backend.FieldSpec{ID: f.ID, Name: f.Name, Type: kind, Required: f.Required, AllowedValues: []backend.FieldOption{}}
		for _, av := range f.AllowedValues {
			spec.AllowedValues = append(spec.AllowedValues, backend.FieldOption{ID: av.ID, Value: av.Label()})
		}
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return backend.CreateFieldSet{Fields: out, ScreenKnown: meta.Source == corejira.MetaPerType}, nil
}
