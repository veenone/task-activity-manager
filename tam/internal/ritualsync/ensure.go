// Package ritualsync keeps a board's ritual pages in step with Confluence.
// Ensure writes a sprint's pages locally from templates, with no network,
// and Run is the Sync pass: create, adopt, pull, push, and record conflicts.
// internal/ritualtemplate stays pure and internal/ritualrepo stays storage;
// every decision about what a Sync does to a page lives here.
package ritualsync

import (
	"context"
	"time"

	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

// Config is what a pass needs besides its sprints: where the pages live, the
// project a missing root's suggested title names, and the clock and zone the
// templates and timestamps read.
type Config struct {
	SpaceKey   string
	RootID     string
	ProjectKey string
	Location   *time.Location
	Now        func() time.Time
}

// Sprint is one sprint a pass covers.
type Sprint struct {
	Info  ritualtemplate.SprintInfo
	State string
}

// Info is a cached sprint in the shape the templates read.
func Info(s boardrepo.Sprint, boardName string) ritualtemplate.SprintInfo {
	return ritualtemplate.SprintInfo{ID: s.ID, Name: s.Name, Goal: s.Goal, StartDate: s.StartDate, EndDate: s.EndDate, BoardName: boardName}
}

// Sprints picks the sprints a pass covers, in the cache's own order: every
// active or future sprint, and a closed one only when it already holds
// documents. A closed sprint never gets new pages from a template, and a
// first Sync must not write five pages for every sprint in a board's history.
func Sprints(cached []boardrepo.Sprint, boardName string, withRows []int) []Sprint {
	has := map[int]bool{}
	for _, id := range withRows {
		has[id] = true
	}
	out := []Sprint{}
	for _, s := range cached {
		// A draft sprint is not in Jira and may never be, so it gets no
		// pages until Commit creates it.
		if s.Draft {
			continue
		}
		if s.State == "closed" && !has[s.ID] {
			continue
		}
		out = append(out, Sprint{Info: Info(s, boardName), State: s.State})
	}
	return out
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// Ensure writes whichever of a sprint's five documents are missing, each
// rendered from its template, locally and with no network call. A row the
// retired wizard left with a remark gets that remark carried onto its page.
// A closed sprint is left as it is.
func Ensure(ctx context.Context, docs *ritualrepo.Repository, profileID string, boardID int, s Sprint, loc *time.Location, now time.Time) error {
	if s.State == "closed" {
		return nil
	}
	need, err := docs.NeedsTemplate(ctx, profileID, boardID, s.Info.ID, ritualtemplate.Types)
	if err != nil {
		return err
	}
	for _, l := range need {
		notes := make([]ritualtemplate.Note, len(l.Issues))
		for i, issue := range l.Issues {
			notes[i] = ritualtemplate.Note{Key: issue.Key, Remark: issue.Remark}
		}
		body := ritualtemplate.Render(l.RitualType, s.Info, loc) + ritualtemplate.EarlierNotes(l.Remark, notes)
		k := ritualrepo.Key{ProfileID: profileID, BoardID: boardID, SprintID: s.Info.ID, RitualType: l.RitualType}
		if err := docs.WriteTemplate(ctx, k, ritualtemplate.Title(l.RitualType, s.Info), body, stamp(now)); err != nil {
			return err
		}
	}
	return nil
}

// EnsureAgreement writes one done agreement document from its template if
// there is none yet, locally and with no network call, and leaves a document
// somebody has written in exactly as it is.
//
// It is separate from Ensure, and its only caller is the binding behind the
// button that asks for the document, because a done agreement is not a page
// every sprint gets: a board has one standing agreement and a sprint has
// additions only when it needs them. info carries which one, through its
// sprint id: zero for the board's own document, a sprint's id for that
// sprint's additions.
func EnsureAgreement(ctx context.Context, docs *ritualrepo.Repository, profileID string, boardID int, info ritualtemplate.SprintInfo, now time.Time) error {
	need, err := docs.NeedsTemplate(ctx, profileID, boardID, info.ID, []string{ritualtemplate.DoneAgreement})
	if err != nil || len(need) == 0 {
		return err
	}
	k := ritualrepo.Key{ProfileID: profileID, BoardID: boardID, SprintID: info.ID, RitualType: ritualtemplate.DoneAgreement}
	// nil for the zone: the done agreement's render reads no date and no
	// clock, so the bytes this writes are the bytes ritualsync's adoption
	// check renders to compare against, whatever zone that pass runs in.
	body := ritualtemplate.Render(ritualtemplate.DoneAgreement, info, nil)
	return docs.WriteTemplate(ctx, k, ritualtemplate.Title(ritualtemplate.DoneAgreement, info), body, stamp(now))
}
