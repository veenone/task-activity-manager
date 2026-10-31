package zephyr

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// StatusVocabulary is one instance's execution or step statuses, name to
// numeric id and back.
//
// It exists because Zephyr statuses are configured per instance and written by
// id. The probed instance defines six where SmartBear documents five, the
// extra one being PASS WARNING, so a compiled-in map would have sent the wrong
// id and recorded the wrong result without failing. Reading the vocabulary at
// connect time is the most load-bearing constraint in this backend.
type StatusVocabulary struct {
	byName map[string]int
	byID   map[int]string
}

// IDFor resolves a status name to the numeric id this instance uses for it.
// Matching ignores case and surrounding space, because the name comes from a
// display string. A name the instance never declared is an error: there is no
// id to send, and picking a default would write a result the user did not ask
// for.
func (v *StatusVocabulary) IDFor(name string) (int, error) {
	if v == nil {
		return 0, errors.New("zephyr: no status vocabulary has been read from this instance")
	}
	if id, ok := v.byName[normalizeStatus(name)]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("zephyr: this instance defines no status named %q; it has %s",
		name, strings.Join(v.Names(), ", "))
}

// NameFor is the reverse, for showing what came back from the instance. An id
// the vocabulary does not know is returned as its number, so an unexpected
// status shows as itself rather than as nothing.
func (v *StatusVocabulary) NameFor(id int) string {
	if v == nil {
		return ""
	}
	if name, ok := v.byID[id]; ok {
		return name
	}
	return fmt.Sprintf("%d", id)
}

// Names lists the statuses in the instance's own id order, which is the order
// Zephyr's own UI offers them in.
func (v *StatusVocabulary) Names() []string {
	if v == nil {
		return nil
	}
	ids := make([]int, 0, len(v.byID))
	for id := range v.byID {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, v.byID[id])
	}
	return names
}

func normalizeStatus(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}

// ExecutionStatuses are the statuses an execution can carry on this instance.
func (c *Client) ExecutionStatuses(ctx context.Context) (*StatusVocabulary, error) {
	return c.execStatuses.get(func() (*StatusVocabulary, error) {
		return c.loadStatuses(ctx, "/util/testExecutionStatus")
	})
}

// StepStatuses are the statuses a step result can carry. Zephyr keeps them on
// their own endpoint, and they answered the same six as the execution ones on
// the probed instance, which is exactly the coincidence that would hide a
// single read serving both.
func (c *Client) StepStatuses(ctx context.Context) (*StatusVocabulary, error) {
	return c.stepStatuses.get(func() (*StatusVocabulary, error) {
		return c.loadStatuses(ctx, "/util/teststepExecutionStatus")
	})
}

// loadStatuses reads one vocabulary.
//
// The array envelope is the one shape in this package taken from SmartBear's
// documentation rather than from a probe: the ids and the names were measured,
// the wrapper around them was not. An instance that answers some other shape
// therefore fails here, at the connection test, with the endpoint named,
// instead of leaving every status lookup to fail separately later.
func (c *Client) loadStatuses(ctx context.Context, path string) (*StatusVocabulary, error) {
	var raw []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := c.getJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	v := &StatusVocabulary{
		byName: make(map[string]int, len(raw)),
		byID:   make(map[int]string, len(raw)),
	}
	for _, s := range raw {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		v.byName[normalizeStatus(name)] = s.ID
		v.byID[s.ID] = name
	}
	if len(v.byID) == 0 {
		return nil, fmt.Errorf("zephyr: %s returned no statuses, so no result could be written to this instance", path)
	}
	return v, nil
}
