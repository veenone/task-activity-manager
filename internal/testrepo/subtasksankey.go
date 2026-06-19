package testrepo

import (
	"fmt"
	"sort"
)

// GetSubTaskTraceability builds a Parent issue -> Test Execution -> run status
// flow over sub-task Test Executions only (kind = 'testexec' with a non-empty
// parent_key). Each sub-task execution has exactly one parent, so the parent is
// layer 0, the execution layer 1, the run status layer 2. The flow unit is a
// membership (a test's run in a sub-task execution); each adds 1 across the
// three layers, so the diagram balances. parentFilters narrows to chosen
// parents; empty means all. Computed entirely from the local store.
func (r *Repository) GetSubTaskTraceability(profileID string, parentFilters []string) (Sankey, error) {
	out := Sankey{Nodes: []SankeyNode{}, Links: []SankeyLink{}}

	summaryByKey, err := r.containerSummaries(profileID)
	if err != nil {
		return out, err
	}

	q := `SELECT c.parent_key, l.container_key, l.run_status
		 FROM test_container_test l
		 JOIN test_container c
		   ON c.profile_id = l.profile_id AND c.jira_key = l.container_key
		 WHERE l.profile_id = ? AND c.kind = 'testexec' AND c.parent_key != ''`
	args := []any{profileID}
	if parents := nonEmptyKeys(parentFilters); len(parents) > 0 {
		q += " AND c.parent_key IN (" + sqlPlaceholders(len(parents)) + ")"
		for _, p := range parents {
			args = append(args, p)
		}
	}
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return out, fmt.Errorf("read sub-task runs: %w", err)
	}
	defer rows.Close()

	parentExec := map[[2]string]int{}
	execStatus := map[[2]string]int{}
	value := map[string]int{}
	label := map[string]string{}
	layer := map[string]int{}
	note := func(id, lbl string, lyr, add int) {
		value[id] += add
		label[id] = lbl
		layer[id] = lyr
	}

	for rows.Next() {
		var parentKey, execKey, runStatus string
		if err := rows.Scan(&parentKey, &execKey, &runStatus); err != nil {
			return out, err
		}
		status := runStatus
		if status == "" {
			status = "(none)"
		}
		parentID := "parent:" + parentKey
		execID := "exec:" + execKey
		statusID := "status:" + status

		note(parentID, parentKey, 0, 1)
		note(execID, orKey(summaryByKey[execKey], execKey), 1, 1)
		note(statusID, status, 2, 1)
		parentExec[[2]string{parentID, execID}]++
		execStatus[[2]string{execID, statusID}]++
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	for id, lbl := range label {
		out.Nodes = append(out.Nodes, SankeyNode{ID: id, Label: lbl, Layer: layer[id], Value: value[id]})
	}
	sort.Slice(out.Nodes, func(i, j int) bool {
		if out.Nodes[i].Layer != out.Nodes[j].Layer {
			return out.Nodes[i].Layer < out.Nodes[j].Layer
		}
		if out.Nodes[i].Value != out.Nodes[j].Value {
			return out.Nodes[i].Value > out.Nodes[j].Value
		}
		return out.Nodes[i].ID < out.Nodes[j].ID
	})
	out.Links = append(out.Links, flatten(parentExec)...)
	out.Links = append(out.Links, flatten(execStatus)...)
	return out, nil
}
