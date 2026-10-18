package datasetcatalog

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

const (
	discoveryPageSize = 100
	maxProbeWorkers   = 8
)

type Progress struct {
	Stage   string `json:"stage"`
	Fetched int    `json:"fetched"`
	Total   int    `json:"total"`
}

type DiscoveryBackend interface {
	SearchTestsPage(ctx context.Context, projectKey, scopeJQL, since string, startAt, maxResults int) ([]backend.Test, int, error)
	ListTestsBasic(ctx context.Context, keys []string) ([]backend.TestBasic, error)
	ListContainers(ctx context.Context, projectKey string, onProgress func(done, total int)) ([]backend.Container, []backend.ContainerLink, error)
	ExecPlans(ctx context.Context, execKey string) ([]string, error)
	GetDataset(ctx context.Context, testKey string) (backend.Dataset, error)
	GetRunDataset(ctx context.Context, execKey, testKey string) (backend.Dataset, error)
}

type Service struct {
	backend DiscoveryBackend
	repo    *testrepo.Repository
}

func New(b DiscoveryBackend, repo *testrepo.Repository) *Service {
	return &Service{backend: b, repo: repo}
}

type candidate struct {
	key, summary, projectKey string
	external, openable       bool
}

type probeJob struct {
	testKey, scope, contextKey, contextSummary string
}

type probeResult struct {
	job     probeJob
	dataset backend.Dataset
	err     error
}

func (s *Service) Sync(ctx context.Context, profileID, projectKey string, onProgress func(Progress)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	planReader, ok := s.backend.(backend.PlanDatasetReader)
	if !ok {
		return fmt.Errorf("dataset catalog Plan reader is unavailable: %w", backend.ErrUnsupported)
	}
	emitProgress(onProgress, Progress{Stage: "Finding tests"})
	candidates, ownedCount, err := s.enumerateTests(ctx, projectKey)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	emitProgress(onProgress, Progress{Stage: "Finding containers"})
	containers, links, err := s.backend.ListContainers(ctx, projectKey, nil)
	if err != nil {
		return fmt.Errorf("list dataset catalog containers: %w", err)
	}
	containerByKey := make(map[string]backend.Container, len(containers))
	for _, container := range containers {
		if container.Kind == backend.KindTestPlan || container.Kind == backend.KindTestExec {
			containerByKey[container.Key] = container
		}
	}
	uniqueLinks := make(map[string]backend.ContainerLink)
	for _, link := range links {
		container, exists := containerByKey[link.ContainerKey]
		if !exists || link.TestKey == "" {
			continue
		}
		uniqueLinks[linkKey(container.Kind, link.ContainerKey, link.TestKey)] = link
		if _, exists := candidates[link.TestKey]; !exists {
			candidates[link.TestKey] = candidate{key: link.TestKey, external: true}
		}
	}
	if err := s.hydrateExternalTests(ctx, candidates); err != nil {
		return err
	}
	execPlans, err := s.executionPlans(ctx, containerByKey)
	if err != nil {
		return err
	}
	jobs := make([]probeJob, 0, len(candidates)+len(uniqueLinks))
	for key := range candidates {
		jobs = append(jobs, probeJob{testKey: key, scope: testrepo.DatasetScopeTest})
	}
	usages := make([]testrepo.DatasetCatalogUsage, 0, len(uniqueLinks))
	for _, link := range uniqueLinks {
		container := containerByKey[link.ContainerKey]
		scope := testrepo.DatasetScopePlan
		if container.Kind == backend.KindTestExec {
			scope = testrepo.DatasetScopeExecution
		}
		jobs = append(jobs, probeJob{testKey: link.TestKey, scope: scope, contextKey: container.Key, contextSummary: container.Summary})
		usages = append(usages, testrepo.DatasetCatalogUsage{
			TestKey: link.TestKey, ContainerKind: container.Kind, ContainerKey: container.Key, ContainerSummary: container.Summary,
		})
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].testKey+jobs[i].scope+jobs[i].contextKey < jobs[j].testKey+jobs[j].scope+jobs[j].contextKey
	})
	emitProgress(onProgress, Progress{Stage: "Reading datasets", Total: len(jobs)})
	definitions, err := s.runProbes(ctx, planReader, jobs, onProgress)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tests := make([]testrepo.DatasetCatalogTest, 0, len(candidates))
	for _, item := range candidates {
		tests = append(tests, testrepo.DatasetCatalogTest{
			TestKey: item.key, Summary: item.summary, ProjectKey: item.projectKey,
			External: item.external, InternalOpenable: item.openable,
		})
	}
	snapshot := testrepo.DatasetCatalogSnapshot{
		OwnedTestCount: ownedCount, ExternalTestCount: len(candidates) - ownedCount,
		RelationshipCount: len(uniqueLinks), Tests: tests, Definitions: definitions,
		Usages: usages, ExecPlans: execPlans,
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.repo.PublishDatasetCatalog(profileID, snapshot, time.Now().UTC()); err != nil {
		return fmt.Errorf("publish dataset catalog: %w", err)
	}
	return nil
}

func (s *Service) enumerateTests(ctx context.Context, projectKey string) (map[string]candidate, int, error) {
	out := map[string]candidate{}
	for offset := 0; ; offset += discoveryPageSize {
		page, total, err := s.backend.SearchTestsPage(ctx, projectKey, "", "", offset, discoveryPageSize)
		if err != nil {
			return nil, 0, fmt.Errorf("list dataset catalog Tests: %w", err)
		}
		for _, test := range page {
			out[test.Key] = candidate{key: test.Key, summary: test.Summary, projectKey: projectKey, openable: true}
		}
		if offset+len(page) >= total || len(page) == 0 {
			break
		}
	}
	return out, len(out), nil
}

func (s *Service) hydrateExternalTests(ctx context.Context, candidates map[string]candidate) error {
	keys := make([]string, 0)
	for key, item := range candidates {
		if item.external {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	basics, err := s.backend.ListTestsBasic(ctx, keys)
	if err != nil {
		return fmt.Errorf("read referenced dataset catalog Tests: %w", err)
	}
	for _, test := range basics {
		if item, ok := candidates[test.Key]; ok {
			item.summary, item.projectKey = test.Summary, test.ProjectKey
			candidates[test.Key] = item
		}
	}
	return nil
}

func (s *Service) executionPlans(ctx context.Context, containers map[string]backend.Container) ([]testrepo.DatasetCatalogExecPlan, error) {
	keys := make([]string, 0)
	for key, container := range containers {
		if container.Kind == backend.KindTestExec {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	links := make([]testrepo.DatasetCatalogExecPlan, 0)
	for _, execKey := range keys {
		plans, err := s.backend.ExecPlans(ctx, execKey)
		if err != nil {
			return nil, fmt.Errorf("read Plans for Execution %s: %w", execKey, err)
		}
		seen := map[string]bool{}
		for _, planKey := range plans {
			if planKey != "" && !seen[planKey] {
				seen[planKey] = true
				links = append(links, testrepo.DatasetCatalogExecPlan{ExecKey: execKey, PlanKey: planKey})
			}
		}
	}
	return links, nil
}

func (s *Service) runProbes(ctx context.Context, planReader backend.PlanDatasetReader, jobs []probeJob, onProgress func(Progress)) ([]testrepo.DatasetCatalogDefinition, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	jobCh := make(chan probeJob)
	resultCh := make(chan probeResult, len(jobs))
	workers := maxProbeWorkers
	if len(jobs) < workers {
		workers = len(jobs)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				dataset, err := s.probe(ctx, planReader, job)
				resultCh <- probeResult{job: job, dataset: dataset, err: err}
			}
		}()
	}
	go func() {
		for _, job := range jobs {
			if ctx.Err() != nil {
				close(jobCh)
				wg.Wait()
				close(resultCh)
				return
			}
			select {
			case <-ctx.Done():
				close(jobCh)
				wg.Wait()
				close(resultCh)
				return
			case jobCh <- job:
			}
		}
		close(jobCh)
		wg.Wait()
		close(resultCh)
	}()
	definitions := make([]testrepo.DatasetCatalogDefinition, 0)
	fetched := 0
	var firstErr error
	for result := range resultCh {
		if result.err != nil && firstErr == nil {
			firstErr = fmt.Errorf("read %s dataset for %s: %w", result.job.scope, result.job.testKey, result.err)
			continue
		}
		fetched++
		emitProgress(onProgress, Progress{Stage: "Reading datasets", Fetched: fetched, Total: len(jobs)})
		if len(result.dataset.Parameters) == 0 {
			continue
		}
		definitions = append(definitions, catalogDefinition(result.job, result.dataset))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return definitions, nil
}

func (s *Service) probe(ctx context.Context, planReader backend.PlanDatasetReader, job probeJob) (backend.Dataset, error) {
	switch job.scope {
	case testrepo.DatasetScopeTest:
		return s.backend.GetDataset(ctx, job.testKey)
	case testrepo.DatasetScopePlan:
		return planReader.GetPlanDataset(ctx, job.contextKey, job.testKey)
	default:
		return s.backend.GetRunDataset(ctx, job.contextKey, job.testKey)
	}
}

func catalogDefinition(job probeJob, dataset backend.Dataset) testrepo.DatasetCatalogDefinition {
	parameters := make([]testrepo.DatasetParameter, len(dataset.Parameters))
	for i, parameter := range dataset.Parameters {
		parameters[i] = testrepo.DatasetParameter{
			Name: parameter.Name, Combinatorial: parameter.Combinatorial, ID: parameter.ID,
			Type: parameter.Type, Options: parameter.Options, ListScope: parameter.ListScope, ListID: parameter.ListID,
		}
	}
	return testrepo.DatasetCatalogDefinition{
		TestKey: job.testKey, ScopeKind: job.scope, ContextKey: job.contextKey,
		ContextSummary: job.contextSummary, Parameters: parameters, Rows: dataset.Rows,
		MetadataComplete: job.scope != testrepo.DatasetScopePlan,
	}
}

func linkKey(kind, containerKey, testKey string) string {
	return kind + "\x00" + containerKey + "\x00" + testKey
}

func emitProgress(fn func(Progress), progress Progress) {
	if fn != nil {
		fn(progress)
	}
}
