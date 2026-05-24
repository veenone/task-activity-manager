// Package syncer pulls Xray data from Jira into the local store (FR-1).
package syncer

import (
	"context"
	"fmt"
	"time"

	"xray-test-manager/internal/jira"
	"xray-test-manager/internal/testrepo"
)

// pageSize is the Jira search page size. Jira DC commonly caps maxResults at
// 100 for /search; the engine pages until the reported total is reached.
const pageSize = 100

// throttle is the pause between pages — keeps a large sync within Jira DC's
// default rate limit (FR-1.8 / Q11).
const throttle = 200 * time.Millisecond

// Progress reports sync advancement to the caller, which forwards it to the UI.
type Progress struct {
	Fetched int  `json:"fetched"`
	Total   int  `json:"total"`
	Done    bool `json:"done"`
}

// Engine runs a pull sync for one profile.
type Engine struct {
	client *jira.Client
	repo   *testrepo.Repository
}

// New returns a sync engine bound to a Jira client and the local repository.
func New(client *jira.Client, repo *testrepo.Repository) *Engine {
	return &Engine{client: client, repo: repo}
}

// FullSync pulls the Test Repository folder tree and every Test for the
// project, upserting them into the local store and calling onProgress after
// each page. Upserts are idempotent, so an interrupted sync is safe to re-run.
func (e *Engine) FullSync(ctx context.Context, profileID, projectKey string, onProgress func(Progress)) error {
	if err := e.syncFolders(ctx, profileID, projectKey); err != nil {
		return err
	}

	fetched := 0
	total := -1

	for total < 0 || fetched < total {
		if err := ctx.Err(); err != nil {
			return err
		}

		tests, pageTotal, err := e.client.SearchTestsPage(ctx, projectKey, fetched, pageSize)
		if err != nil {
			return fmt.Errorf("fetch page at offset %d: %w", fetched, err)
		}
		total = pageTotal

		if err := e.repo.UpsertTests(profileID, toRepoTests(tests)); err != nil {
			return err
		}
		fetched += len(tests)

		if onProgress != nil {
			onProgress(Progress{Fetched: fetched, Total: total})
		}

		if len(tests) == 0 {
			break // defensive: avoid an infinite loop if total is misreported
		}
		if fetched < total {
			time.Sleep(throttle)
		}
	}

	if err := e.repo.SetSyncState(profileID, fetched); err != nil {
		return err
	}
	if onProgress != nil {
		onProgress(Progress{Fetched: fetched, Total: total, Done: true})
	}
	return nil
}

// syncFolders pulls the Test Repository folder tree and upserts it. Empty
// results are tolerated — the real-Jira implementation is currently a no-op
// (FR-13.1), but demo mode populates the tree.
func (e *Engine) syncFolders(ctx context.Context, profileID, projectKey string) error {
	folders, err := e.client.ListFolders(ctx, projectKey)
	if err != nil {
		return fmt.Errorf("list folders: %w", err)
	}
	if len(folders) == 0 {
		return nil
	}
	repoFolders := make([]testrepo.Folder, len(folders))
	for i, f := range folders {
		repoFolders[i] = testrepo.Folder{
			ID:       f.ID,
			ParentID: f.ParentID,
			Name:     f.Name,
		}
	}
	return e.repo.UpsertFolders(profileID, repoFolders)
}

// toRepoTests maps the Jira client's Test type to the repository's TestCase.
func toRepoTests(in []jira.Test) []testrepo.TestCase {
	out := make([]testrepo.TestCase, len(in))
	for i, t := range in {
		out[i] = testrepo.TestCase{
			Key:         t.Key,
			ID:          t.ID,
			Summary:     t.Summary,
			Description: t.Description,
			Status:      t.Status,
			Priority:    t.Priority,
			Labels:      t.Labels,
			Updated:     t.Updated,
			FolderID:    t.FolderID,
		}
	}
	return out
}
