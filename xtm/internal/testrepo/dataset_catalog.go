package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) PublishDatasetCatalog(profileID string, snapshot DatasetCatalogSnapshot, syncedAt time.Time) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return errors.New("dataset catalog profile id is required")
	}
	tests, definitions, usages, execPlans, err := validateCatalogSnapshot(snapshot)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin dataset catalog publication: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var generation int
	if err := tx.QueryRow(`SELECT COALESCE(active_generation, 0) + 1 FROM dataset_catalog_state WHERE profile_id = ?`, profileID).Scan(&generation); errors.Is(err, sql.ErrNoRows) {
		generation = 1
	} else if err != nil {
		return fmt.Errorf("choose dataset catalog generation: %w", err)
	}
	for _, test := range tests {
		if _, err := tx.Exec(`INSERT INTO dataset_catalog_test
			(profile_id, generation, test_key, summary, project_key, is_external, internal_openable)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, profileID, generation, test.TestKey, test.Summary,
			test.ProjectKey, boolInt(test.External), boolInt(test.InternalOpenable)); err != nil {
			return fmt.Errorf("insert dataset catalog Test %s: %w", test.TestKey, err)
		}
	}
	for _, definition := range definitions {
		parametersJSON, _ := json.Marshal(nonNilParameters(definition.Parameters))
		rowsJSON, _ := json.Marshal(nonNilRows(definition.Rows))
		if _, err := tx.Exec(`INSERT INTO dataset_catalog_definition
			(profile_id, generation, test_key, scope_kind, context_key, context_summary, parameters_json, rows_json, metadata_complete)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, profileID, generation, definition.TestKey,
			definition.ScopeKind, definition.ContextKey, definition.ContextSummary, string(parametersJSON),
			string(rowsJSON), boolInt(definition.MetadataComplete)); err != nil {
			return fmt.Errorf("insert dataset catalog definition for %s: %w", definition.TestKey, err)
		}
	}
	for _, usage := range usages {
		if _, err := tx.Exec(`INSERT INTO dataset_catalog_usage
			(profile_id, generation, test_key, container_kind, container_key, container_summary)
			VALUES (?, ?, ?, ?, ?, ?)`, profileID, generation, usage.TestKey, usage.ContainerKind,
			usage.ContainerKey, usage.ContainerSummary); err != nil {
			return fmt.Errorf("insert dataset catalog usage for %s: %w", usage.TestKey, err)
		}
	}
	for _, link := range execPlans {
		if _, err := tx.Exec(`INSERT INTO dataset_catalog_exec_plan
			(profile_id, generation, exec_key, plan_key) VALUES (?, ?, ?, ?)`, profileID, generation,
			link.ExecKey, link.PlanKey); err != nil {
			return fmt.Errorf("insert dataset catalog Execution Plan link: %w", err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO dataset_catalog_state
		(profile_id, active_generation, last_synced_at, owned_test_count, external_test_count,
		relationship_count, test_group_count, definition_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET active_generation=excluded.active_generation,
		last_synced_at=excluded.last_synced_at, owned_test_count=excluded.owned_test_count,
		external_test_count=excluded.external_test_count, relationship_count=excluded.relationship_count,
		test_group_count=excluded.test_group_count, definition_count=excluded.definition_count`, profileID,
		generation, syncedAt.UTC().Format(time.RFC3339), snapshot.OwnedTestCount, snapshot.ExternalTestCount,
		snapshot.RelationshipCount, len(tests), len(definitions)); err != nil {
		return fmt.Errorf("activate dataset catalog generation: %w", err)
	}
	for _, table := range []string{"dataset_catalog_test", "dataset_catalog_definition", "dataset_catalog_usage", "dataset_catalog_exec_plan"} {
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE profile_id = ? AND generation <> ?", profileID, generation); err != nil {
			return fmt.Errorf("remove old %s generations: %w", table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dataset catalog publication: %w", err)
	}
	return nil
}

func validateCatalogSnapshot(snapshot DatasetCatalogSnapshot) ([]DatasetCatalogTest, []DatasetCatalogDefinition, []DatasetCatalogUsage, []DatasetCatalogExecPlan, error) {
	testByKey := map[string]DatasetCatalogTest{}
	for _, test := range snapshot.Tests {
		if strings.TrimSpace(test.TestKey) == "" {
			return nil, nil, nil, nil, errors.New("dataset catalog Test key is required")
		}
		testByKey[test.TestKey] = test
	}
	definitionByKey := map[string]DatasetCatalogDefinition{}
	definedTests := map[string]bool{}
	for _, definition := range snapshot.Definitions {
		if _, ok := testByKey[definition.TestKey]; !ok {
			return nil, nil, nil, nil, fmt.Errorf("dataset definition references unknown Test %q", definition.TestKey)
		}
		if err := validateCatalogDefinition(definition); err != nil {
			return nil, nil, nil, nil, err
		}
		key := strings.Join([]string{definition.TestKey, definition.ScopeKind, definition.ContextKey}, "\x00")
		definitionByKey[key], definedTests[definition.TestKey] = definition, true
	}
	tests := make([]DatasetCatalogTest, 0, len(definedTests))
	for key, test := range testByKey {
		if definedTests[key] {
			tests = append(tests, test)
		}
	}
	definitions := mapValues(definitionByKey)
	usageByKey := map[string]DatasetCatalogUsage{}
	for _, usage := range snapshot.Usages {
		if usage.TestKey == "" || usage.ContainerKey == "" || (usage.ContainerKind != "testplan" && usage.ContainerKind != "testexec") {
			return nil, nil, nil, nil, fmt.Errorf("invalid dataset catalog usage for Test %q", usage.TestKey)
		}
		if definedTests[usage.TestKey] {
			usageByKey[strings.Join([]string{usage.TestKey, usage.ContainerKind, usage.ContainerKey}, "\x00")] = usage
		}
	}
	linkByKey := map[string]DatasetCatalogExecPlan{}
	for _, link := range snapshot.ExecPlans {
		if link.ExecKey == "" || link.PlanKey == "" {
			return nil, nil, nil, nil, errors.New("dataset catalog Execution and Plan keys are required")
		}
		linkByKey[link.ExecKey+"\x00"+link.PlanKey] = link
	}
	return tests, definitions, mapValues(usageByKey), mapValues(linkByKey), nil
}

func validateCatalogDefinition(definition DatasetCatalogDefinition) error {
	if definition.TestKey == "" {
		return errors.New("dataset definition Test key is required")
	}
	switch definition.ScopeKind {
	case DatasetScopeTest:
		if definition.ContextKey != "" {
			return errors.New("Test dataset definition cannot have a context key")
		}
	case DatasetScopePlan, DatasetScopeExecution:
		if definition.ContextKey == "" {
			return fmt.Errorf("%s dataset definition needs a context key", definition.ScopeKind)
		}
	default:
		return fmt.Errorf("unknown dataset scope %q", definition.ScopeKind)
	}
	if err := ValidateDataset(Dataset{Parameters: definition.Parameters, Rows: definition.Rows}); err != nil {
		return fmt.Errorf("invalid dataset definition for %s: %w", definition.TestKey, err)
	}
	for i, row := range definition.Rows {
		if len(row) != len(definition.Parameters) {
			return fmt.Errorf("dataset definition for %s row %d has %d cells, want %d", definition.TestKey, i+1, len(row), len(definition.Parameters))
		}
	}
	return nil
}

func (r *Repository) ListDatasetCatalog(profileID string, query DatasetCatalogQuery) (DatasetCatalogPage, error) {
	if err := validateCatalogQuery(profileID, query); err != nil {
		return DatasetCatalogPage{}, err
	}
	state, found, err := r.datasetCatalogState(profileID)
	if err != nil || !found {
		return DatasetCatalogPage{}, err
	}
	where, args, err := catalogWhere(profileID, state.ActiveGeneration, query)
	if err != nil {
		return DatasetCatalogPage{}, err
	}
	var total int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM dataset_catalog_test t WHERE "+where, args...).Scan(&total); err != nil {
		return DatasetCatalogPage{}, fmt.Errorf("count dataset catalog: %w", err)
	}
	limit := query.Limit
	if limit == 0 {
		limit = 50
	}
	rows, err := r.db.Query(`SELECT t.test_key, t.summary, t.project_key, t.is_external, t.internal_openable,
		COALESCE(d.parameters_json, '[]'), COALESCE(d.rows_json, '[]'),
		(SELECT COUNT(*) FROM dataset_catalog_usage u WHERE u.profile_id=t.profile_id AND u.generation=t.generation AND u.test_key=t.test_key AND u.container_kind='testplan'),
		(SELECT COUNT(*) FROM dataset_catalog_usage u WHERE u.profile_id=t.profile_id AND u.generation=t.generation AND u.test_key=t.test_key AND u.container_kind='testexec'),
		(SELECT COUNT(*) FROM dataset_catalog_definition x WHERE x.profile_id=t.profile_id AND x.generation=t.generation AND x.test_key=t.test_key AND x.scope_kind<>'test')
		FROM dataset_catalog_test t LEFT JOIN dataset_catalog_definition d ON d.profile_id=t.profile_id AND d.generation=t.generation AND d.test_key=t.test_key AND d.scope_kind='test'
		WHERE `+where+` ORDER BY t.test_key LIMIT ? OFFSET ?`, append(args, limit, query.Offset)...)
	if err != nil {
		return DatasetCatalogPage{}, fmt.Errorf("list dataset catalog: %w", err)
	}
	defer rows.Close()
	tests := []DatasetCatalogTestSummary{}
	for rows.Next() {
		var summary DatasetCatalogTestSummary
		var external, openable int
		var parametersJSON, rowsJSON string
		if err := rows.Scan(&summary.TestKey, &summary.Summary, &summary.ProjectKey, &external, &openable,
			&parametersJSON, &rowsJSON, &summary.PlanCount, &summary.ExecutionCount, &summary.OverrideCount); err != nil {
			return DatasetCatalogPage{}, err
		}
		summary.External, summary.InternalOpenable = external != 0, openable != 0
		var parameters []DatasetParameter
		var values [][]string
		if err := json.Unmarshal([]byte(parametersJSON), &parameters); err != nil {
			return DatasetCatalogPage{}, fmt.Errorf("decode catalog parameters for %s: %w", summary.TestKey, err)
		}
		if err := json.Unmarshal([]byte(rowsJSON), &values); err != nil {
			return DatasetCatalogPage{}, fmt.Errorf("decode catalog rows for %s: %w", summary.TestKey, err)
		}
		summary.DefaultParameterCount, summary.DefaultRowCount = len(parameters), len(values)
		tests = append(tests, summary)
	}
	if err := rows.Err(); err != nil {
		return DatasetCatalogPage{}, err
	}
	return DatasetCatalogPage{State: &state, Tests: tests, Total: total}, nil
}

func catalogWhere(profileID string, generation int, query DatasetCatalogQuery) (string, []any, error) {
	if err := validateCatalogQuery(profileID, query); err != nil {
		return "", nil, err
	}
	parts := []string{"t.profile_id = ?", "t.generation = ?"}
	args := []any{profileID, generation}
	if query.Scope != "" {
		parts = append(parts, `EXISTS (SELECT 1 FROM dataset_catalog_definition f WHERE f.profile_id=t.profile_id AND f.generation=t.generation AND f.test_key=t.test_key AND f.scope_kind=?)`)
		args = append(args, query.Scope)
	}
	if query.Ownership != "" {
		parts = append(parts, "t.is_external = ?")
		args = append(args, boolInt(query.Ownership == "external"))
	}
	if search := strings.ToLower(strings.TrimSpace(query.Search)); search != "" {
		like := "%" + search + "%"
		parts = append(parts, `(lower(t.test_key) LIKE ? OR lower(t.summary) LIKE ? OR EXISTS
			(SELECT 1 FROM dataset_catalog_usage u WHERE u.profile_id=t.profile_id AND u.generation=t.generation AND u.test_key=t.test_key AND (lower(u.container_key) LIKE ? OR lower(u.container_summary) LIKE ?)))`)
		args = append(args, like, like, like, like)
	}
	return strings.Join(parts, " AND "), args, nil
}

func validateCatalogQuery(profileID string, query DatasetCatalogQuery) error {
	if strings.TrimSpace(profileID) == "" || query.Offset < 0 || query.Limit < 0 {
		return errors.New("invalid dataset catalog query")
	}
	if query.Scope != "" && query.Scope != DatasetScopeTest && query.Scope != DatasetScopePlan && query.Scope != DatasetScopeExecution {
		return fmt.Errorf("unknown dataset scope %q", query.Scope)
	}
	if query.Ownership != "" && query.Ownership != "owned" && query.Ownership != "external" {
		return fmt.Errorf("unknown dataset ownership %q", query.Ownership)
	}
	return nil
}

func (r *Repository) GetDatasetCatalogDetail(profileID, testKey string) (DatasetCatalogDetail, bool, error) {
	if strings.TrimSpace(testKey) == "" {
		return DatasetCatalogDetail{}, false, errors.New("dataset catalog Test key is required")
	}
	page, err := r.ListDatasetCatalog(profileID, DatasetCatalogQuery{Search: testKey, Limit: 100})
	if err != nil || page.State == nil {
		return DatasetCatalogDetail{}, false, err
	}
	var summary DatasetCatalogTestSummary
	found := false
	for _, test := range page.Tests {
		if test.TestKey == testKey {
			summary, found = test, true
			break
		}
	}
	if !found {
		return DatasetCatalogDetail{}, false, nil
	}
	detail := DatasetCatalogDetail{Test: summary, Definitions: []DatasetCatalogDefinition{}, Usages: []DatasetCatalogUsage{}, ExecPlans: []DatasetCatalogExecPlan{}}
	generation := page.State.ActiveGeneration
	definitionRows, err := r.db.Query(`SELECT scope_kind, context_key, context_summary, parameters_json, rows_json, metadata_complete
		FROM dataset_catalog_definition WHERE profile_id=? AND generation=? AND test_key=? ORDER BY scope_kind, context_key`, profileID, generation, testKey)
	if err != nil {
		return DatasetCatalogDetail{}, false, err
	}
	for definitionRows.Next() {
		definition := DatasetCatalogDefinition{TestKey: testKey}
		var parametersJSON, rowsJSON string
		var complete int
		if err := definitionRows.Scan(&definition.ScopeKind, &definition.ContextKey, &definition.ContextSummary, &parametersJSON, &rowsJSON, &complete); err != nil {
			_ = definitionRows.Close()
			return DatasetCatalogDetail{}, false, err
		}
		definition.MetadataComplete = complete != 0
		if err := json.Unmarshal([]byte(parametersJSON), &definition.Parameters); err != nil {
			_ = definitionRows.Close()
			return DatasetCatalogDetail{}, false, err
		}
		if err := json.Unmarshal([]byte(rowsJSON), &definition.Rows); err != nil {
			_ = definitionRows.Close()
			return DatasetCatalogDetail{}, false, err
		}
		detail.Definitions = append(detail.Definitions, definition)
	}
	_ = definitionRows.Close()
	usageRows, err := r.db.Query(`SELECT container_kind, container_key, container_summary FROM dataset_catalog_usage
		WHERE profile_id=? AND generation=? AND test_key=? ORDER BY container_kind, container_key`, profileID, generation, testKey)
	if err != nil {
		return DatasetCatalogDetail{}, false, err
	}
	for usageRows.Next() {
		usage := DatasetCatalogUsage{TestKey: testKey}
		if err := usageRows.Scan(&usage.ContainerKind, &usage.ContainerKey, &usage.ContainerSummary); err != nil {
			_ = usageRows.Close()
			return DatasetCatalogDetail{}, false, err
		}
		detail.Usages = append(detail.Usages, usage)
	}
	_ = usageRows.Close()
	linkRows, err := r.db.Query(`SELECT DISTINCT e.exec_key, e.plan_key FROM dataset_catalog_exec_plan e
		JOIN dataset_catalog_usage u ON u.profile_id=e.profile_id AND u.generation=e.generation AND u.container_kind='testexec' AND u.container_key=e.exec_key
		WHERE e.profile_id=? AND e.generation=? AND u.test_key=? ORDER BY e.exec_key, e.plan_key`, profileID, generation, testKey)
	if err != nil {
		return DatasetCatalogDetail{}, false, err
	}
	defer linkRows.Close()
	for linkRows.Next() {
		var link DatasetCatalogExecPlan
		if err := linkRows.Scan(&link.ExecKey, &link.PlanKey); err != nil {
			return DatasetCatalogDetail{}, false, err
		}
		detail.ExecPlans = append(detail.ExecPlans, link)
	}
	return detail, true, linkRows.Err()
}

func (r *Repository) datasetCatalogState(profileID string) (DatasetCatalogState, bool, error) {
	var state DatasetCatalogState
	err := r.db.QueryRow(`SELECT active_generation, last_synced_at, owned_test_count, external_test_count,
		relationship_count, test_group_count, definition_count FROM dataset_catalog_state WHERE profile_id=?`, profileID).
		Scan(&state.ActiveGeneration, &state.LastSyncedAt, &state.OwnedTestCount, &state.ExternalTestCount,
			&state.RelationshipCount, &state.TestGroupCount, &state.DefinitionCount)
	if errors.Is(err, sql.ErrNoRows) {
		return DatasetCatalogState{}, false, nil
	}
	if err != nil {
		return DatasetCatalogState{}, false, fmt.Errorf("read dataset catalog state: %w", err)
	}
	return state, true, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nonNilParameters(parameters []DatasetParameter) []DatasetParameter {
	if parameters == nil {
		return []DatasetParameter{}
	}
	return parameters
}

func nonNilRows(rows [][]string) [][]string {
	if rows == nil {
		return [][]string{}
	}
	return rows
}

func mapValues[K comparable, V any](values map[K]V) []V {
	out := make([]V, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
