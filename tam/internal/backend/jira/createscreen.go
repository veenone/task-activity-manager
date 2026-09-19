package jira

import (
	"context"
	"log"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// createScreen is one issue type's create metadata and how the read went.
//
// Only the per-type answer can be read as the screen. The classic call lists
// fields that are not on it, and a read that failed says nothing at all.
// Both of those are "not known", and not knowing a field is absent is a
// different thing from knowing it is, so both let every field through and
// leave Jira's own refusal as the backstop. Reading silence as absence would
// quietly strip the estimate off every create on an instance whose metadata
// call fails, which is the worst outcome available here.
type createScreen struct {
	meta corejira.CreateMeta
	err  error
}

// known says the answer can be read as the create screen at all.
func (s createScreen) known() bool {
	return s.err == nil && s.meta.Source == corejira.MetaPerType
}

// carries says the screen holds id and will let a create set it. A field
// listing add and remove but not set refuses the single value a create
// sends, the same as on an edit screen.
func (s createScreen) carries(id string) bool {
	if !s.known() {
		return true
	}
	f, ok := s.meta.Field(id)
	return ok && settable(f)
}

// applyOwnFields writes the three fields TAM sets itself that an instance can
// leave off a create screen: the estimate, the epic the issue hangs under,
// and an epic's own name. They never went through applyExtras, so nothing
// checked them against the screen and Jira refused the whole create when one
// was missing, exactly as it did on the edit path (issue #52).
//
// It returns what it left out, named, so Commit can say what did not go.
// An epic's name is not among them: it is the summary the user typed, which
// goes as the summary regardless, and applyExtras already states that rule
// for a field TAM sets itself. The estimate and the parent are the user's
// own work and are always named.
//
// pointsID is empty when the draft carries no estimate; resolving it can
// fail for reasons that are not about the screen at all, and CreateIssue
// refuses the create over those before reaching here.
func (b *Backend) applyOwnFields(ctx context.Context, d backend.IssueDraft, ids fieldIDs, pointsID string, screen createScreen, fields map[string]any) []string {
	var leftOut []string
	if d.StoryPoints != nil {
		if screen.carries(pointsID) {
			fields[pointsID] = *d.StoryPoints
		} else {
			log.Printf("tam: the %s create of %q leaves out the estimate: %s is not on the create screen", d.Type, d.Summary, pointsID)
			leftOut = append(leftOut, b.fieldLabel(ctx, pointsID))
		}
	}
	// A sub-task hangs off its parent through Jira's own parent field, which
	// CreateIssue sets and no create screen omits, so only the Epic Link is
	// in question here.
	if d.ParentKey != "" && d.Type != backend.TypeSubtask && d.Type != backend.TypeEpic {
		switch {
		case ids.EpicLink == "":
			log.Printf("tam: %s has no Epic Link field; parent %s dropped from the create of %q", b.c.BaseURL(), d.ParentKey, d.Summary)
			leftOut = append(leftOut, "Epic Link")
		case !screen.carries(ids.EpicLink):
			log.Printf("tam: the %s create of %q leaves out parent %s: %s is not on the create screen", d.Type, d.Summary, d.ParentKey, ids.EpicLink)
			leftOut = append(leftOut, b.fieldLabel(ctx, ids.EpicLink))
		default:
			fields[ids.EpicLink] = d.ParentKey
		}
	}
	return leftOut
}

// applyEpicName gives an epic the name Jira keeps beside its summary, when
// the instance has the field, the screen carries it, and nothing has set it
// already. It runs after the extras, which may have set it themselves.
func (b *Backend) applyEpicName(d backend.IssueDraft, ids fieldIDs, screen createScreen, fields map[string]any) {
	if d.Type != backend.TypeEpic || ids.EpicName == "" {
		return
	}
	if _, set := fields[ids.EpicName]; set {
		return
	}
	if !screen.carries(ids.EpicName) {
		log.Printf("tam: the epic create of %q leaves out %s, which is not on the create screen", d.Summary, ids.EpicName)
		return
	}
	fields[ids.EpicName] = d.Summary
}
