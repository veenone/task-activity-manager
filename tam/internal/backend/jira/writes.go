package jira

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
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
// The second result names the extras left out, for Commit to report.
func (b *Backend) CreateIssue(ctx context.Context, projectKey string, d backend.IssueDraft) (string, []string, error) {
	ids := b.discover(ctx)
	pt := b.typesOrEmpty(ctx, projectKey)
	typeName := typeNameIn(d.Type, b.requirementType, pt)
	if typeName == "" {
		if d.Type == backend.TypeSubtask {
			return "", nil, fmt.Errorf("%s has no sub-task issue type", projectKey)
		}
		return "", nil, fmt.Errorf("%s has no issue type %q", projectKey, d.Type)
	}
	fields := map[string]any{
		"project":   map[string]string{"key": projectKey},
		"issuetype": map[string]string{"name": typeName},
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
	// An estimate TAM cannot place is refused rather than guessed at or
	// dropped, which is a different thing from one the screen does not
	// carry: there the user has nothing to decide.
	pointsID := ""
	if d.StoryPoints != nil {
		var err error
		if pointsID, err = b.pointsField(projectKey, ids); err != nil {
			return "", nil, err
		}
	}
	// A sub-task hangs off its parent through Jira's own parent field, not
	// through the Epic Link, and cannot exist without one.
	if d.Type == backend.TypeSubtask {
		if d.ParentKey == "" {
			return "", nil, errors.New("a sub-task needs a parent issue")
		}
		fields["parent"] = map[string]string{"key": d.ParentKey}
	}
	// The create screen is read once, and only when the create carries
	// something it could refuse: one of the three fields TAM sets itself
	// that an instance can leave off, or an extra. Everything else on the
	// form is on every create screen there is, so a plain create still makes
	// the requests it always made, which is what a long import notices.
	screen := createScreen{}
	if len(d.Extra) > 0 || d.StoryPoints != nil || d.Type == backend.TypeEpic ||
		(d.ParentKey != "" && d.Type != backend.TypeSubtask) {
		screen.meta, screen.err = b.createMeta(ctx, projectKey, d.Type)
	}
	leftOut := b.applyOwnFields(ctx, d, ids, pointsID, screen, fields)
	if len(d.Extra) > 0 {
		leftOut = append(leftOut, b.applyExtras(ctx, typeName, d, ids, screen, fields)...)
	}
	b.applyEpicName(d, ids, screen, fields)
	var resp struct {
		Key string `json:"key"`
	}
	if err := b.c.WriteJSONReturning(ctx, http.MethodPost, "/rest/api/2/issue", map[string]any{"fields": fields}, &resp); err != nil {
		return "", nil, b.humanizeFieldError(ctx, err, true)
	}
	if resp.Key == "" {
		return "", nil, errors.New("Jira created the issue but returned no key")
	}
	return resp.Key, leftOut, nil
}

// applyExtras writes the draft's extra fields into the payload, shaped from
// the type's create metadata. Five rules keep an extra out, each logged:
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
//     lists the id, so the field is not on the screen today;
//   - the metadata cannot be read at all, so nothing can say the field
//     belongs on the screen. A draft's own ScreenFields is not an answer
//     here: it records a screen read from an earlier session, and a create
//     refused today is exactly the case where that reading went stale.
//
// It returns the fields it left out, named, so Commit can say what did not
// go rather than dropping a typed value in silence. A field TAM sets itself
// is not among them: the form's own value is what Jira gets, so nothing the
// user typed is lost.
func (b *Backend) applyExtras(ctx context.Context, typeName string, d backend.IssueDraft, ids fieldIDs, cs createScreen, fields map[string]any) []string {
	meta, metaErr := cs.meta, cs.err
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
	leftOut := []string{}
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
			leftOut = append(leftOut, b.fieldLabel(ctx, id))
			continue
		}
		if metaErr != nil {
			log.Printf("tam: the %s create of %q leaves out %s: the create fields could not be read, so nothing can say the field is on the screen (%v)", typeName, d.Summary, id, metaErr)
			leftOut = append(leftOut, b.fieldLabel(ctx, id))
			continue
		}
		if cs.known() && !cs.carries(id) {
			log.Printf("tam: the %s create of %q leaves out %s, which the %s create screen does not carry or will not let a create set", typeName, d.Summary, id, typeName)
			leftOut = append(leftOut, b.fieldLabel(ctx, id))
			continue
		}
		if meta.Source == corejira.MetaClassic && !(known && f.Required) {
			log.Printf("tam: the %s create of %q leaves out %s, because this Jira only reports its create fields through the classic call, which does not say what is on the screen", typeName, d.Summary, id)
			leftOut = append(leftOut, b.fieldLabel(ctx, id))
			continue
		}
		fields[id] = corejira.ShapeValue(f, v)
	}
	return leftOut
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
	name := typeNameIn(logicalType, b.requirementType, pt)
	if name == "" {
		return corejira.CreateMeta{}, fmt.Errorf("%s has no issue type %q", projectKey, logicalType)
	}
	return b.c.CreateMeta(ctx, projectKey, pt.ids[strings.ToLower(name)], name)
}

// typeNameIn is the Jira name a draft's type means in this project: one of
// TAM's own six mapped through jiraTypeNames, or, for a type TAM has no
// logical type for, the project's own name for it carried verbatim. The
// dialog offers the types the project really has, so a draft can name one
// TAM has never heard of (issue #65 item 2); it is only accepted when this
// project's type list carries that name, so an unknown one is refused by
// name rather than falling back to the task level, which would create a
// type nobody chose under a summary written for another. "" is no such type.
func typeNameIn(draftType, requirementType string, pt projectTypes) string {
	if names := jiraTypeNames([]string{draftType}, requirementType, pt); len(names) > 0 {
		return names[0]
	}
	if _, ok := pt.ids[strings.ToLower(strings.TrimSpace(draftType))]; ok {
		return draftType
	}
	return ""
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
