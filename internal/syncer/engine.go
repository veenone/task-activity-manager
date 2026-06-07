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

// Sync pulls the Test Repository folder tree, the project's Tests, and the
// project's Preconditions into the local store, calling onProgress after each
// Test page. If `since` is empty, this is a full sync; otherwise it is an
// incremental sync that only fetches Tests updated since the watermark
// (FR-1.2). Upserts are idempotent, so an interrupted sync is safe to re-run.
func (e *Engine) Sync(ctx context.Context, profileID, projectKey, scopeJQL, since string, onProgress func(Progress)) error {
	if err := e.syncFolders(ctx, profileID, projectKey); err != nil {
		return err
	}

	fetched := 0
	total := -1

	for total < 0 || fetched < total {
		if err := ctx.Err(); err != nil {
			return err
		}

		tests, pageTotal, err := e.client.SearchTestsPage(ctx, projectKey, scopeJQL, since, fetched, pageSize)
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

	if err := e.syncPreconditions(ctx, profileID, projectKey); err != nil {
		return err
	}

	if err := e.syncContainers(ctx, profileID, projectKey); err != nil {
		return err
	}

	if err := e.syncCustomFields(ctx, profileID, projectKey); err != nil {
		return err
	}

	if err := e.repo.SetSyncState(profileID); err != nil {
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

// syncPreconditions pulls the Preconditions for a project and reconciles the
// Test-to-Precondition links. An empty result is tolerated — the real-Jira
// implementation is currently a no-op pending live verification (FR-13.4),
// but demo mode populates them.
func (e *Engine) syncPreconditions(ctx context.Context, profileID, projectKey string) error {
	preconditions, links, err := e.client.ListPreconditions(ctx, projectKey)
	if err != nil {
		return fmt.Errorf("list preconditions: %w", err)
	}
	if len(preconditions) == 0 && len(links) == 0 {
		return nil
	}
	repoPre := make([]testrepo.Precondition, len(preconditions))
	for i, p := range preconditions {
		repoPre[i] = testrepo.Precondition{
			Key:         p.Key,
			Summary:     p.Summary,
			Type:        p.Type,
			Description: p.Description,
		}
	}
	if err := e.repo.UpsertPreconditions(profileID, repoPre); err != nil {
		return err
	}
	return e.repo.ReplaceAllTestPreconditions(profileID, links)
}

// syncContainers pulls the project's Test Sets, Test Plans and Test
// Executions and reconciles their Test memberships (FR-1.3). An empty result
// is tolerated — the real-Jira implementation is currently a no-op pending
// live verification, but demo mode populates them.
func (e *Engine) syncContainers(ctx context.Context, profileID, projectKey string) error {
	containers, links, err := e.client.ListContainers(ctx, projectKey)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	if len(containers) == 0 && len(links) == 0 {
		return nil
	}
	repoContainers := make([]testrepo.Container, len(containers))
	for i, c := range containers {
		repoContainers[i] = testrepo.Container{
			Key:     c.Key,
			Kind:    c.Kind,
			Summary: c.Summary,
			Status:  c.Status,
		}
	}
	if err := e.repo.UpsertContainers(profileID, repoContainers); err != nil {
		return err
	}
	repoLinks := make([]testrepo.ContainerLink, len(links))
	for i, l := range links {
		repoLinks[i] = testrepo.ContainerLink{
			ContainerKey: l.ContainerKey,
			TestKey:      l.TestKey,
			RunStatus:    l.RunStatus,
		}
	}
	return e.repo.ReplaceAllContainerLinks(profileID, repoLinks)
}

// syncCustomFields pulls the custom field definitions configured for the
// project's Test issue type (FR-2.6) and caches them. An empty result is
// tolerated — the real-Jira implementation is currently a no-op pending live
// verification, but demo mode populates them.
func (e *Engine) syncCustomFields(ctx context.Context, profileID, projectKey string) error {
	defs, err := e.client.ListCustomFields(ctx, projectKey)
	if err != nil {
		return fmt.Errorf("list custom fields: %w", err)
	}
	if len(defs) == 0 {
		return nil
	}
	repoDefs := make([]testrepo.CustomFieldDef, len(defs))
	for i, d := range defs {
		repoDefs[i] = testrepo.CustomFieldDef{FieldID: d.ID, Name: d.Name, Type: d.Type}
	}
	return e.repo.UpsertCustomFields(profileID, repoDefs)
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
