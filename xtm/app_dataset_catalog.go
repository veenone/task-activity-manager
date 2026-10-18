package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/xtm/internal/datasetcatalog"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

// SyncDatasetCatalog rebuilds the local, read-only inventory of datasets and
// the Tests, Plans, and Executions that use them.
func (a *App) SyncDatasetCatalog(profileID string) (err error) {
	defer recoverToError("SyncDatasetCatalog", &err)
	defer a.emitDatasetCatalogProgress(syncer.Progress{Done: true})
	if err := a.requireStore(); err != nil {
		return err
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return err
	}
	token, err := a.creds.Load(profileID)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	client := newBackend(p.Backend, p.JiraURL, token, p.CACert, p.AllowUntrustedTLS)
	service := datasetcatalog.New(client, a.repo)
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	err = service.Sync(ctx, profileID, p.ProjectKey, func(progress datasetcatalog.Progress) {
		a.emitDatasetCatalogProgress(syncer.Progress{
			Stage: progress.Stage, Fetched: progress.Fetched, Total: progress.Total,
		})
	})
	return syncFailure(err)
}

// ListDatasetCatalog returns one page of Test-centred catalog summaries.
func (a *App) ListDatasetCatalog(profileID string, query testrepo.DatasetCatalogQuery) (page testrepo.DatasetCatalogPage, err error) {
	defer recoverToError("ListDatasetCatalog", &err)
	if err := a.requireStore(); err != nil {
		return testrepo.DatasetCatalogPage{}, err
	}
	return a.repo.ListDatasetCatalog(profileID, query)
}

// GetDatasetCatalogDetail returns all dataset definitions and container links
// for one Test in the active catalog generation.
func (a *App) GetDatasetCatalogDetail(profileID, testKey string) (detail testrepo.DatasetCatalogDetail, err error) {
	defer recoverToError("GetDatasetCatalogDetail", &err)
	if err := a.requireStore(); err != nil {
		return testrepo.DatasetCatalogDetail{}, err
	}
	testKey = strings.TrimSpace(testKey)
	if testKey == "" {
		return testrepo.DatasetCatalogDetail{}, fmt.Errorf("dataset catalog Test key is required")
	}
	detail, found, err := a.repo.GetDatasetCatalogDetail(profileID, testKey)
	if err != nil {
		return testrepo.DatasetCatalogDetail{}, err
	}
	if !found {
		return testrepo.DatasetCatalogDetail{}, fmt.Errorf("dataset catalog Test %s was not found", testKey)
	}
	return detail, nil
}

func (a *App) emitDatasetCatalogProgress(progress syncer.Progress) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if a.emitEvent != nil {
		a.emitEvent(ctx, "sync:progress", progress)
		return
	}
	runtime.EventsEmit(ctx, "sync:progress", progress)
}
