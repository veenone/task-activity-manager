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

// Config is what a pass needs besides its sprints: where the pages live, and
// the clock and zone the templates and timestamps read.
type Config struct {
	SpaceKey string
	RootID   string
	Location *time.Location
	Now      func() time.Time
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
