// Cycles and executions, which are Zephyr's own entities rather than Jira
// issues, and the one group where its model differs from Xray's in shape and
// not only in naming.
//
// Zephyr nests Project, then Jira Version, then Cycle, then Execution. XTM has
// containers holding tests, so a cycle is a container of the execution kind
// (the only kind Essential has) and an execution is a test's membership of one
// plus its run status. There is no Test Set and no Test Plan.
//
// Writes wait for #371 Phase 5.

package zephyr

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"agile-suite/xtm/internal/backend"
)

// adHocCycleName is what the -1 pseudo-cycle is called in the view. Zephyr
// reports it with no name of its own, and the executions that belong to no
// cycle genuinely live in it, so it is shown rather than hidden.
const adHocCycleName = "Ad hoc executions"

// ListContainers lists a project's cycles, and the tests inside them.
//
// A cycle belongs to a Jira version, and /cycle answers for one version at a
// time, so the project's versions decide how many requests this makes. The -1
// version is asked for too: that is where the executions that belong to no
// release sit.
//
// The cycle list says nothing about membership, so the links come from each
// cycle's executions. That endpoint is documented but was never probed, so a
// failure there logs and leaves the cycle without members rather than failing
// the whole walk: a Containers view listing cycles with no tests is still
// usable, and an empty one over an unconfirmed response shape is not.
func (a *Adapter) ListContainers(ctx context.Context, projectKey string, onProgress func(done, total int)) ([]backend.Container, []backend.ContainerLink, error) {
	projectID, versionIDs, err := a.z.projectVersionIDs(ctx, projectKey)
	if err != nil {
		return nil, nil, err
	}
	var (
		containers []backend.Container
		links      []backend.ContainerLink
		seen       = map[string]bool{}
	)
	for i, versionID := range versionIDs {
		cycles, err := a.z.cycles(ctx, projectID, versionID)
		if err != nil {
			return nil, nil, err
		}
		for _, c := range cycles {
			// The same pseudo-cycle comes back for every version, and a real
			// cycle can be reported under more than one too, so a container
			// is taken once by its id.
			if seen[c.Key] {
				continue
			}
			seen[c.Key] = true
			containers = append(containers, c)

			runs, err := a.cycleRuns(ctx, c.Key)
			if err != nil {
				log.Printf("xtm: zephyr: executions of cycle %s: %v (listing it without members)", c.Key, err)
				continue
			}
			for _, r := range runs {
				links = append(links, backend.ContainerLink{
					ContainerKey: c.Key,
					TestKey:      r.TestKey,
					RunStatus:    r.Status,
				})
			}
		}
		if onProgress != nil {
			onProgress(i+1, len(versionIDs))
		}
	}
	return containers, links, nil
}

// GetTestRuns reports one cycle's executions as run results.
//
// Unlike the list walk this reports its failures, because it answers a view
// the user opened on one cycle: an empty list in place of an error reads as a
// cycle nobody has run yet.
//
// The statuses are Zephyr's own vocabulary, read from the instance, so a run
// can read UNEXECUTED or PASS WARNING where an Xray profile would read one of
// the six in testrepo.RunStatuses. They are the instance's words and are shown
// as they come.
func (a *Adapter) GetTestRuns(ctx context.Context, execKey string) ([]backend.TestRun, error) {
	return a.cycleRuns(ctx, execKey)
}

// TestExecutionsForTest finds every cycle one test has been executed in, in
// any project, which is the cross-project discovery the syncer needs for a
// test whose bugs it is refreshing.
//
// Traceability names the cycle but the containers XTM stores are keyed by
// cycle id, so an execution that does not carry one is read in full to get it.
// Both endpoints were probed.
//
// Failures degrade rather than propagate, as the Xray adapter's own version of
// this method does: the syncer calls it per affected test, and one unreadable
// test should not end the pass.
func (a *Adapter) TestExecutionsForTest(ctx context.Context, testKey string) ([]backend.Container, []backend.ContainerLink, error) {
	q := url.Values{}
	q.Set("testIdOrKey", testKey)
	var resp struct {
		Executions []struct {
			Execution traceabilityExecution `json:"execution"`
		} `json:"executions"`
	}
	if err := a.z.getJSON(ctx, "/traceability/executionsByTest?"+q.Encode(), &resp); err != nil {
		log.Printf("xtm: zephyr: executions of %s: %v (skipping)", testKey, err)
		return nil, nil, nil
	}

	var (
		containers []backend.Container
		links      []backend.ContainerLink
	)
	for _, row := range resp.Executions {
		e := row.Execution
		cycleID, cycleName := jsonString(e.CycleID), strings.TrimSpace(e.TestCycle)
		if cycleID == "" {
			full, err := a.z.execution(ctx, jsonString(e.ID))
			if err != nil {
				log.Printf("xtm: zephyr: execution %s of %s: %v (skipping)", jsonString(e.ID), testKey, err)
				continue
			}
			cycleID = jsonString(full.CycleID)
			if cycleName == "" {
				cycleName = strings.TrimSpace(full.CycleName)
			}
		}
		if cycleID == "" {
			log.Printf("xtm: zephyr: execution %s of %s names no cycle (skipping)", jsonString(e.ID), testKey)
			continue
		}
		key := strings.TrimSpace(e.IssueKey)
		if key == "" {
			key = testKey
		}
		containers = append(containers, backend.Container{
			Key:         cycleID,
			Kind:        backend.KindTestExec,
			Summary:     orAdHoc(cycleName),
			FixVersions: nonEmpty(e.VersionName),
		})
		links = append(links, backend.ContainerLink{
			ContainerKey: cycleID,
			TestKey:      key,
			RunStatus:    a.statusName(ctx, e.Status),
		})
	}
	return containers, links, nil
}

// traceabilityExecution is one row of the traceability response. Its ids
// arrive quoted here and bare from the execution read, which is why they are
// raw.
type traceabilityExecution struct {
	ID          json.RawMessage `json:"id"`
	IssueKey    string          `json:"issueKey"`
	Status      json.RawMessage `json:"status"`
	TestCycle   string          `json:"testCycle"`
	CycleID     json.RawMessage `json:"cycleId"`
	VersionName string          `json:"versionName"`
}

// zephyrExecution is one execution as the execution endpoints answer it.
type zephyrExecution struct {
	ID         json.RawMessage   `json:"id"`
	IssueKey   string            `json:"issueKey"`
	Status     json.RawMessage   `json:"status"`
	CycleID    json.RawMessage   `json:"cycleId"`
	CycleName  string            `json:"cycleName"`
	Comment    string            `json:"comment"`
	ExecutedBy string            `json:"executedBy"`
	ExecutedOn string            `json:"executedOn"`
	Defects    []json.RawMessage `json:"defects"`
}

// cycleRuns reads one cycle's executions and reports them as run results.
func (a *Adapter) cycleRuns(ctx context.Context, cycleID string) ([]backend.TestRun, error) {
	q := url.Values{}
	q.Set("cycleId", cycleID)
	var resp struct {
		Executions []zephyrExecution `json:"executions"`
	}
	if err := a.z.getJSON(ctx, "/execution?"+q.Encode(), &resp); err != nil {
		return nil, fmt.Errorf("read the executions in cycle %s: %w", cycleID, err)
	}
	runs := make([]backend.TestRun, 0, len(resp.Executions))
	for _, e := range resp.Executions {
		runs = append(runs, backend.TestRun{
			TestKey:    strings.TrimSpace(e.IssueKey),
			Status:     a.statusName(ctx, e.Status),
			ExecutedBy: e.ExecutedBy,
			FinishedAt: e.ExecutedOn,
			Comment:    e.Comment,
			Defects:    defectKeys(e.Defects),
		})
	}
	return runs, nil
}

// execution reads one execution by id, which is the measured way to learn
// which cycle it belongs to. The response wraps it in an execution field.
func (c *Client) execution(ctx context.Context, id string) (zephyrExecution, error) {
	if strings.TrimSpace(id) == "" {
		return zephyrExecution{}, fmt.Errorf("zephyr: no execution id to read")
	}
	var resp struct {
		Execution zephyrExecution `json:"execution"`
	}
	if err := c.getJSON(ctx, "/execution/"+url.PathEscape(id), &resp); err != nil {
		return zephyrExecution{}, err
	}
	return resp.Execution, nil
}

// projectVersionIDs resolves the Jira project id and the version ids whose
// cycles have to be asked for, the -1 bucket included. One Jira project read
// answers both.
func (c *Client) projectVersionIDs(ctx context.Context, projectKey string) (string, []string, error) {
	var project struct {
		ID       string `json:"id"`
		Versions []struct {
			ID string `json:"id"`
		} `json:"versions"`
	}
	path := "/rest/api/2/project/" + url.PathEscape(projectKey)
	if err := c.jira.Get(ctx, path, &project); err != nil {
		return "", nil, fmt.Errorf("zephyr: read the Jira project %s: %w", projectKey, err)
	}
	if strings.TrimSpace(project.ID) == "" {
		return "", nil, fmt.Errorf("zephyr: Jira returned no project id for %s", projectKey)
	}
	ids := make([]string, 0, len(project.Versions)+1)
	for _, v := range project.Versions {
		if id := strings.TrimSpace(v.ID); id != "" {
			ids = append(ids, id)
		}
	}
	// -1 is the unscheduled version, where the cycles of no release live. It
	// is asked for last so a real release's cycles are listed first.
	return project.ID, append(ids, "-1"), nil
}

// cycles reads one version's cycles as containers.
//
// The response is an object keyed by cycle id rather than an array, which is
// the shape trap here. Anything whose key is not a number is not a cycle:
// Zephyr mixes counters into the same object, and reading one as a cycle would
// put a container named after a record count in the view.
func (c *Client) cycles(ctx context.Context, projectID, versionID string) ([]backend.Container, error) {
	q := url.Values{}
	q.Set("projectId", projectID)
	q.Set("versionId", versionID)
	var raw map[string]json.RawMessage
	if err := c.getJSON(ctx, "/cycle?"+q.Encode(), &raw); err != nil {
		return nil, fmt.Errorf("zephyr: read the cycles of version %s: %w", versionID, err)
	}
	out := make([]backend.Container, 0, len(raw))
	for key, value := range raw {
		if _, err := strconv.Atoi(key); err != nil {
			continue
		}
		var cycle struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Environment string `json:"environment"`
			VersionName string `json:"versionName"`
		}
		if err := json.Unmarshal(value, &cycle); err != nil {
			continue
		}
		out = append(out, backend.Container{
			Key:         key,
			Kind:        backend.KindTestExec,
			Summary:     orAdHoc(cycle.Name),
			Description: cycle.Description,
			// A cycle carries one environment name, not a set, so this is the
			// whole of it. SupportsEnvironments stays false because there is
			// no set to edit; the name is still worth showing.
			Environments: nonEmpty(cycle.Environment),
			// The Jira version the cycle tests. Zephyr nests cycles under a
			// version, and without it every release's cycles read alike.
			FixVersions: nonEmpty(cycle.VersionName),
		})
	}
	return out, nil
}

// statusName reads an execution's status, which arrives in whichever of three
// shapes the instance uses: an object, a name, or the bare numeric id. Only
// the last needs this instance's vocabulary, and an id it does not know is
// shown as itself rather than as nothing.
func (a *Adapter) statusName(ctx context.Context, raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if strings.HasPrefix(s, "{") {
		var obj struct {
			ID   json.RawMessage `json:"id"`
			Name string          `json:"name"`
		}
		if json.Unmarshal(raw, &obj) == nil {
			if name := strings.TrimSpace(obj.Name); name != "" {
				return name
			}
			return a.statusName(ctx, obj.ID)
		}
		return ""
	}
	unquoted := strings.Trim(s, `"`)
	id, err := strconv.Atoi(unquoted)
	if err != nil {
		return unquoted
	}
	v, err := a.z.ExecutionStatuses(ctx)
	if err != nil {
		log.Printf("xtm: zephyr: read this instance's execution statuses: %v (showing status %d as its id)", err, id)
		return unquoted
	}
	return v.NameFor(id)
}

// defectKeys reads the linked issue keys off an execution. Zephyr answers them
// as objects on the builds that were probed; a plain key is accepted because
// the field is documented both ways.
func defectKeys(raw []json.RawMessage) []string {
	var keys []string
	for _, d := range raw {
		s := strings.TrimSpace(string(d))
		if strings.HasPrefix(s, "{") {
			var obj struct {
				Key string `json:"key"`
			}
			if json.Unmarshal(d, &obj) == nil && strings.TrimSpace(obj.Key) != "" {
				keys = append(keys, strings.TrimSpace(obj.Key))
			}
			continue
		}
		if k := jsonString(d); k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

func orAdHoc(name string) string {
	if n := strings.TrimSpace(name); n != "" {
		return n
	}
	return adHocCycleName
}

func nonEmpty(v string) []string {
	if s := strings.TrimSpace(v); s != "" {
		return []string{s}
	}
	return nil
}
