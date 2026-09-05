// Package syncer pulls a profile's issues from its backend into the store,
// one page per transaction, and keeps the per-profile sync state. It has
// no Wails dependency: the app forwards Progress as events.
package syncer

import (
	"context"
	"fmt"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Progress is one frame of a running sync. Its JSON shape matches XTM's so
// the shared sync reducer understands it: Done marks the terminal frame.
type Progress struct {
	Phase   string `json:"phase"`
	Fetched int    `json:"fetched"`
	Total   int    `json:"total"`
	Done    bool   `json:"done"`
	Stage   string `json:"stage"`
}

// Summary is what a finished sync reports.
type Summary struct {
	Fetched  int    `json:"fetched"`
	Upserted int    `json:"upserted"`
	Skipped  int    `json:"skipped"`
	Full     bool   `json:"full"`
	Elapsed  string `json:"elapsed"`
}

// PartialSyncError says a sync stopped after some pages had already been
// written. The rows stay; the sync state is not advanced.
type PartialSyncError struct {
	Pages int
	Err   error
}

func (e *PartialSyncError) Error() string {
	return fmt.Sprintf("sync stopped after %d page(s): %v", e.Pages, e.Err)
}

func (e *PartialSyncError) Unwrap() error { return e.Err }

// Engine syncs one backend into one store.
type Engine struct {
	b    backend.IssueBackend
	repo *issuerepo.Repository
	// PageSize is how many issues one search asks for. Fifty keeps a page
	// well under Jira DC's response limits with the fields the grid needs.
	PageSize int
	// Now is the clock, replaceable in tests.
	Now func() time.Time
}

// New builds an engine with the default page size and clock.
func New(b backend.IssueBackend, repo *issuerepo.Repository) *Engine {
	return &Engine{b: b, repo: repo, PageSize: 50, Now: time.Now}
}

// Sync pulls the profile's issues. An incremental sync (full=false) asks
// the backend for issues updated since the last sync; a full sync clears
// the profile's rows inside the first page's transaction and pulls
// everything. The sync's start time becomes the new last_synced so an
// issue updated during the sync is picked up next time.
func (e *Engine) Sync(ctx context.Context, profileID, projectKey, scopeJQL string, full bool, onProgress func(Progress)) (Summary, error) {
	emit := func(p Progress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	start := e.Now().UTC()
	sum := Summary{Full: full}

	emit(Progress{Phase: "issues", Stage: "Checking the connection"})
	state, err := e.repo.SyncState(ctx, profileID)
	if err != nil {
		return sum, err
	}
	if _, err := e.b.TestConnection(ctx); err != nil {
		return e.fail(ctx, profileID, state, 0, sum, err, emit)
	}

	since := ""
	if !full {
		since = state.LastSynced
	}
	pages, startAt, total := 0, 0, -1
	for total < 0 || startAt < total {
		page, n, err := e.b.SearchIssuesPage(ctx, projectKey, scopeJQL, since, backend.AllTypes, startAt, e.PageSize)
		if err != nil {
			return e.fail(ctx, profileID, state, pages, sum, err, emit)
		}
		total = n
		keep := make([]backend.Issue, 0, len(page))
		for _, iss := range page {
			if iss.Type != "" {
				keep = append(keep, iss)
			}
		}
		if err := e.repo.UpsertPage(ctx, profileID, keep, start, full && pages == 0); err != nil {
			return e.fail(ctx, profileID, state, pages, sum, err, emit)
		}
		pages++
		sum.Fetched += len(page)
		sum.Upserted += len(keep)
		sum.Skipped += len(page) - len(keep)
		startAt += len(page)
		emit(Progress{Phase: "issues", Fetched: sum.Fetched, Total: total, Stage: "Fetching issues"})
		if len(page) == 0 {
			break
		}
	}

	next := issuerepo.SyncState{LastSynced: start.Format(time.RFC3339), LastFull: state.LastFull}
	if full {
		next.LastFull = next.LastSynced
	}
	if err := e.repo.SetSyncState(ctx, profileID, next); err != nil {
		return e.fail(ctx, profileID, state, pages, sum, err, emit)
	}
	sum.Elapsed = e.Now().Sub(start).Round(time.Millisecond).String()
	emit(Progress{Phase: "issues", Fetched: sum.Fetched, Total: total, Done: true, Stage: "Done"})
	return sum, nil
}

// fail records the error without advancing last_synced, sends the terminal
// frame, and wraps the cause as a PartialSyncError when pages had landed.
// The state write drops the cancellation, since a cancelled sync is exactly
// the case where the status bar needs to say what went wrong.
func (e *Engine) fail(ctx context.Context, profileID string, state issuerepo.SyncState, pages int, sum Summary, cause error, emit func(Progress)) (Summary, error) {
	state.LastError = cause.Error()
	_ = e.repo.SetSyncState(context.WithoutCancel(ctx), profileID, state)
	emit(Progress{Phase: "issues", Fetched: sum.Fetched, Done: true, Stage: "Failed"})
	if pages > 0 {
		return sum, &PartialSyncError{Pages: pages, Err: cause}
	}
	return sum, cause
}
