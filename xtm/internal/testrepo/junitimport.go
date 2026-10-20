package testrepo

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"agile-suite/xtm/internal/backend"
)

type JUnitTargetKind string

const (
	JUnitTargetRun       JUnitTargetKind = "run"
	JUnitTargetIteration JUnitTargetKind = "iteration"
)

// JUnitProperty is one testcase-level property from a JUnit report.
type JUnitProperty struct {
	Name  string `xml:"name,attr" json:"name"`
	Value string `xml:"value,attr" json:"value"`
}

// JUnitMatch is one testcase matched to a single member test of the execution,
// with the run result it will set.
type JUnitMatch struct {
	Testcase          string                       `json:"testcase"`
	TestKey           string                       `json:"testKey"`
	Summary           string                       `json:"summary"`
	Result            string                       `json:"result"`     // "PASS" or "FAIL"
	CurrentRun        string                       `json:"currentRun"` // the test's current run status in this execution
	TargetKind        JUnitTargetKind              `json:"targetKind"`
	IterationID       string                       `json:"iterationId"`
	Parameters        []backend.IterationParameter `json:"parameters"`
	DatasetSourceKind backend.DatasetSourceKind    `json:"datasetSourceKind"`
	DatasetSourceKey  string                       `json:"datasetSourceKey"`
}

type JUnitIterationCandidate struct {
	IterationID string                       `json:"iterationId"`
	Parameters  []backend.IterationParameter `json:"parameters"`
	CurrentRun  string                       `json:"currentRun"`
}

type JUnitMapping struct {
	Testcase            string                    `json:"testcase"`
	TestKey             string                    `json:"testKey"`
	Summary             string                    `json:"summary"`
	Result              string                    `json:"result"`
	Included            bool                      `json:"included"`
	Reason              string                    `json:"reason"`
	DatasetSourceKind   backend.DatasetSourceKind `json:"datasetSourceKind"`
	DatasetSourceKey    string                    `json:"datasetSourceKey"`
	CandidateIterations []JUnitIterationCandidate `json:"candidateIterations"`
	Match               *JUnitMatch               `json:"match,omitempty"`
}

// JUnitSkip is a testcase that will not be applied, with the reason.
type JUnitSkip struct {
	Testcase string `json:"testcase"`
	Reason   string `json:"reason"`
}

// JUnitImportPreview is the result of analyzing a JUnit report against an
// execution.
type JUnitImportPreview struct {
	ExecKey  string         `json:"execKey"`
	Total    int            `json:"total"`
	Matched  []JUnitMatch   `json:"matched"`
	Skipped  []JUnitSkip    `json:"skipped"`
	Mappings []JUnitMapping `json:"mappings"`
}

// junitTestSuites is the root element when the XML wraps suites inside
// <testsuites>.
type junitTestSuites struct {
	XMLName    xml.Name         `xml:"testsuites"`
	TestSuites []junitTestSuite `xml:"testsuite"`
}

// junitTestSuite is a single <testsuite> element (also used as root for bare
// single-suite reports).
type junitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	TestCases []junitTestCase `xml:"testcase"`
}

// junitTestCase holds the fields we care about from a <testcase> element.
type junitTestCase struct {
	Name       string          `xml:"name,attr"`
	Properties []JUnitProperty `xml:"properties>property"`
	Failure    *junitPresence  `xml:"failure"`
	Error      *junitPresence  `xml:"error"`
	Skipped    *junitPresence  `xml:"skipped"`
}

// junitPresence is used for child elements whose mere presence is meaningful
// (failure, error, skipped) - the content is ignored.
type junitPresence struct{}

// parseJUnitXML decodes a JUnit XML byte slice, supporting both a bare
// <testsuite> root and a <testsuites> wrapper.
func parseJUnitXML(data []byte) ([]junitTestCase, error) {
	// Try <testsuites> wrapper first.
	var suites junitTestSuites
	if err := xml.Unmarshal(data, &suites); err == nil && len(suites.TestSuites) > 0 {
		var all []junitTestCase
		for _, s := range suites.TestSuites {
			all = append(all, s.TestCases...)
		}
		return normalizeJUnitProperties(all), nil
	}

	// Fall back to bare <testsuite>.
	var suite junitTestSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("parse JUnit XML: %w", err)
	}
	return normalizeJUnitProperties(suite.TestCases), nil
}

func normalizeJUnitProperties(cases []junitTestCase) []junitTestCase {
	for caseIndex := range cases {
		for propertyIndex := range cases[caseIndex].Properties {
			property := &cases[caseIndex].Properties[propertyIndex]
			property.Name = strings.TrimSpace(property.Name)
			property.Value = strings.TrimSpace(property.Value)
		}
		if cases[caseIndex].Properties == nil {
			cases[caseIndex].Properties = []JUnitProperty{}
		}
	}
	return cases
}

// matchIteration resolves a testcase only when its properties contain one
// complete dataset signature and that signature identifies exactly one row.
func matchIteration(properties []JUnitProperty, run backend.ParameterizedRun) (string, []backend.IterationParameter, string) {
	if len(run.Iterations) == 0 {
		return "", nil, "no dataset iterations are available"
	}
	required := make(map[string]struct{}, len(run.Iterations[0].Parameters))
	for _, parameter := range run.Iterations[0].Parameters {
		required[parameter.Name] = struct{}{}
	}
	values := make(map[string]string, len(required))
	for _, property := range properties {
		if _, relevant := required[property.Name]; !relevant {
			continue
		}
		if _, duplicate := values[property.Name]; duplicate {
			return "", nil, fmt.Sprintf("duplicate property %q", property.Name)
		}
		values[property.Name] = property.Value
	}
	for _, parameter := range run.Iterations[0].Parameters {
		if _, present := values[parameter.Name]; !present {
			return "", nil, fmt.Sprintf("missing dataset property %q", parameter.Name)
		}
	}

	matches := make([]backend.RunIteration, 0, 1)
	for _, iteration := range run.Iterations {
		if len(iteration.Parameters) != len(required) {
			continue
		}
		exact := true
		for _, parameter := range iteration.Parameters {
			if values[parameter.Name] != parameter.Value {
				exact = false
				break
			}
		}
		if exact {
			matches = append(matches, iteration)
		}
	}
	if len(matches) == 0 {
		return "", nil, "no iteration matches the complete dataset signature"
	}
	if len(matches) > 1 {
		return "", nil, "multiple iterations share the dataset signature"
	}
	signature := append([]backend.IterationParameter(nil), matches[0].Parameters...)
	return matches[0].ID, signature, ""
}

// execMember holds the summary and current run status for one execution member.
type execMember struct {
	testKey   string
	summary   string
	runStatus string
}

// loadExecMembers returns every member test of the given execution, resolved
// with summary from test_case or external_test and run status from test_run or
// test_container_test.
func (r *Repository) loadExecMembers(profileID, execKey string) ([]execMember, error) {
	rows, err := r.db.Query(`
		SELECT l.test_key,
		       COALESCE(t.summary, x.summary, '') AS summary,
		       COALESCE(tr.run_status, l.run_status, '') AS run_status
		FROM test_container_test l
		LEFT JOIN test_case     t  ON t.profile_id  = l.profile_id AND t.jira_key = l.test_key
		LEFT JOIN external_test x  ON x.profile_id  = l.profile_id AND x.jira_key = l.test_key
		LEFT JOIN test_run      tr ON tr.profile_id = l.profile_id
		                          AND tr.exec_key   = l.container_key
		                          AND tr.test_key   = l.test_key
		WHERE l.profile_id = ? AND l.container_key = ?
		ORDER BY l.test_key`,
		profileID, execKey)
	if err != nil {
		return nil, fmt.Errorf("load exec members: %w", err)
	}
	defer rows.Close()

	var out []execMember
	for rows.Next() {
		var m execMember
		if err := rows.Scan(&m.testKey, &m.summary, &m.runStatus); err != nil {
			return nil, fmt.Errorf("scan exec member: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// junitResult returns "PASS", "FAIL", or "" (skipped) for a parsed testcase.
// A testcase with a <skipped/> element is left unset (caller decides).
func junitResult(tc junitTestCase) (result string, skipped bool) {
	if tc.Skipped != nil {
		return "", true
	}
	if tc.Failure != nil || tc.Error != nil {
		return "FAIL", false
	}
	return "PASS", false
}

// isExecMember checks whether testKey is a member of execKey for the given
// profile, using the db directly (used for re-validation in ApplyJUnitImport).
func isExecMember(db *sql.DB, profileID, execKey, testKey string) (bool, error) {
	var one int
	err := db.QueryRow(
		`SELECT 1 FROM test_container_test
		 WHERE profile_id = ? AND container_key = ? AND test_key = ?`,
		profileID, execKey, testKey,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check membership: %w", err)
	}
	return true, nil
}

// AnalyzeJUnitImport decodes a base64-encoded JUnit XML report and matches its
// testcases to member tests of the given execution by summary, returning a
// preview of what would be applied. Each testcase is either matched (one member
// shares its name as summary) or skipped (no match, ambiguous match, or skipped
// in the report). The caller can then pass Matched to ApplyJUnitImport.
func (r *Repository) AnalyzeJUnitImport(profileID, execKey, xmlBase64 string) (JUnitImportPreview, error) {
	preview := JUnitImportPreview{
		ExecKey:  execKey,
		Matched:  []JUnitMatch{},
		Skipped:  []JUnitSkip{},
		Mappings: []JUnitMapping{},
	}

	xmlBytes, err := base64.StdEncoding.DecodeString(xmlBase64)
	if err != nil {
		return preview, fmt.Errorf("decode XML: %w", err)
	}

	testcases, err := parseJUnitXML(xmlBytes)
	if err != nil {
		return preview, err
	}
	if len(testcases) == 0 {
		return JUnitImportPreview{}, fmt.Errorf("JUnit XML contained no testcase elements")
	}
	preview.Total = len(testcases)

	members, err := r.loadExecMembers(profileID, execKey)
	if err != nil {
		return preview, err
	}

	// Build summary -> []member map (normalized: trimmed and lowercased for case-insensitive matching).
	byName := make(map[string][]execMember, len(members))
	for _, m := range members {
		key := strings.ToLower(strings.TrimSpace(m.summary))
		byName[key] = append(byName[key], m)
	}

	for _, tc := range testcases {
		result, skipped := junitResult(tc)

		if skipped {
			skip := JUnitSkip{
				Testcase: tc.Name,
				Reason:   "skipped in the report; run left unchanged",
			}
			preview.Skipped = append(preview.Skipped, skip)
			preview.Mappings = append(preview.Mappings, JUnitMapping{Testcase: tc.Name, Included: false, Reason: skip.Reason, CandidateIterations: []JUnitIterationCandidate{}})
			continue
		}

		hits := byName[strings.ToLower(strings.TrimSpace(tc.Name))]
		switch len(hits) {
		case 0:
			skip := JUnitSkip{
				Testcase: tc.Name,
				Reason:   "no matching test in this execution",
			}
			preview.Skipped = append(preview.Skipped, skip)
			preview.Mappings = append(preview.Mappings, JUnitMapping{Testcase: tc.Name, Included: false, Reason: skip.Reason, CandidateIterations: []JUnitIterationCandidate{}})
		case 1:
			match := JUnitMatch{
				Testcase:   tc.Name,
				TestKey:    hits[0].testKey,
				Summary:    hits[0].summary,
				Result:     result,
				CurrentRun: hits[0].runStatus,
				TargetKind: JUnitTargetRun,
				Parameters: []backend.IterationParameter{},
			}
			if run, found, readErr := r.ParameterizedRun(profileID, execKey, hits[0].testKey); readErr != nil {
				return preview, readErr
			} else if found {
				candidates := junitIterationCandidates(run)
				iterationID, signature, reason := matchIteration(tc.Properties, run)
				if reason != "" {
					preview.Mappings = append(preview.Mappings, JUnitMapping{
						Testcase: tc.Name, TestKey: hits[0].testKey, Summary: hits[0].summary, Result: result,
						Included: true, Reason: reason, CandidateIterations: candidates,
						DatasetSourceKind: run.DatasetSourceKind, DatasetSourceKey: run.DatasetSourceKey,
					})
					continue
				}
				match.TargetKind = JUnitTargetIteration
				match.IterationID = iterationID
				match.Parameters = signature
				match.DatasetSourceKind = run.DatasetSourceKind
				match.DatasetSourceKey = run.DatasetSourceKey
				for _, candidate := range candidates {
					if candidate.IterationID == iterationID {
						match.CurrentRun = candidate.CurrentRun
						break
					}
				}
				preview.Matched = append(preview.Matched, match)
				preview.Mappings = append(preview.Mappings, JUnitMapping{
					Testcase: tc.Name, TestKey: match.TestKey, Summary: match.Summary, Result: match.Result,
					Included: true, CandidateIterations: candidates, Match: &match,
					DatasetSourceKind: run.DatasetSourceKind, DatasetSourceKey: run.DatasetSourceKey,
				})
				continue
			}
			preview.Matched = append(preview.Matched, match)
			preview.Mappings = append(preview.Mappings, JUnitMapping{
				Testcase: tc.Name, TestKey: match.TestKey, Summary: match.Summary, Result: match.Result,
				Included: true, CandidateIterations: []JUnitIterationCandidate{}, Match: &match,
			})
		default:
			skip := JUnitSkip{
				Testcase: tc.Name,
				Reason:   fmt.Sprintf("ambiguous: %d tests share this summary", len(hits)),
			}
			preview.Skipped = append(preview.Skipped, skip)
			preview.Mappings = append(preview.Mappings, JUnitMapping{Testcase: tc.Name, Included: false, Reason: skip.Reason, CandidateIterations: []JUnitIterationCandidate{}})
		}
	}

	return preview, nil
}

func junitIterationCandidates(run backend.ParameterizedRun) []JUnitIterationCandidate {
	candidates := make([]JUnitIterationCandidate, len(run.Iterations))
	for index, iteration := range run.Iterations {
		candidates[index] = JUnitIterationCandidate{
			IterationID: iteration.ID,
			Parameters:  append([]backend.IterationParameter(nil), iteration.Parameters...),
			CurrentRun:  iteration.Status,
		}
	}
	return candidates
}

// JUnitNewExecRow is one testcase mapped to a test that will be placed in the
// new execution. TestKey is the key of an existing test (when Create is false)
// or empty for a test that will be created on commit (when Create is true).
// Result is "PASS", "FAIL", or "" (skipped in the report; the test is still
// allocated but its run result is left unset).
type JUnitNewExecRow struct {
	Testcase            string                       `json:"testcase"`
	TestKey             string                       `json:"testKey"`
	Summary             string                       `json:"summary"`
	Result              string                       `json:"result"`
	Create              bool                         `json:"create"`
	TargetKind          JUnitTargetKind              `json:"targetKind"`
	Parameters          []backend.IterationParameter `json:"parameters"`
	DatasetSourceKind   backend.DatasetSourceKind    `json:"datasetSourceKind"`
	DatasetSourceKey    string                       `json:"datasetSourceKey"`
	Reason              string                       `json:"reason"`
	CandidateIterations []JUnitIterationCandidate    `json:"candidateIterations"`
}

type DeferredIterationResult struct {
	TestKey           string                       `json:"testKey"`
	Parameters        []backend.IterationParameter `json:"parameters"`
	Result            string                       `json:"result"`
	DatasetSourceKind backend.DatasetSourceKind    `json:"datasetSourceKind"`
	DatasetSourceKey  string                       `json:"datasetSourceKey"`
}

// JUnitNewExecPreview is the analysis of a JUnit report for creating a new
// Test Execution. Rows contains existing tests to allocate and tests to be
// created; Skipped contains testcases that will not be included (ambiguous
// summary match, or unmatched when createMissing is false).
type JUnitNewExecPreview struct {
	Total   int               `json:"total"`
	Rows    []JUnitNewExecRow `json:"rows"`
	Skipped []JUnitSkip       `json:"skipped"`
}

// JUnitNewExecResult summarizes what ApplyJUnitImportNewExec queued as pending
// changes. ExecKey is the temporary key for the newly created execution (it
// will be replaced with the real Jira key on commit). Created is the number of
// brand-new tests queued. Allocated is the total number of tests linked to the
// execution. ResultsSet counts the run-result pending changes queued (one per
// row with a non-empty Result). Failed lists any row-level errors that did not
// prevent the overall operation from completing.
type JUnitNewExecResult struct {
	ExecKey    string   `json:"execKey"`
	Created    int      `json:"created"`
	Allocated  int      `json:"allocated"`
	ResultsSet int      `json:"resultsSet"`
	Failed     []string `json:"failed"`
}

// AnalyzeJUnitImportNewExec decodes a base64-encoded JUnit XML report and
// classifies each testcase against ALL tests in the profile's local cache.
// Matching is case-insensitive and trims surrounding whitespace. Each testcase
// is placed in one of three buckets:
//   - exactly one summary match: added to Rows with Create=false and the
//     resolved TestKey and Result ("PASS", "FAIL", or "" for skipped).
//   - zero matches and createMissing=true: added to Rows with Create=true, an
//     empty TestKey, and the computed Result.
//   - zero matches and createMissing=false, or more than one match: added to
//     Skipped with an explanatory reason.
//
// The caller passes Rows to ApplyJUnitImportNewExec to queue all pending
// changes without further analysis.
func (r *Repository) AnalyzeJUnitImportNewExec(profileID, xmlBase64 string, createMissing bool) (JUnitNewExecPreview, error) {
	preview := JUnitNewExecPreview{
		Rows:    []JUnitNewExecRow{},
		Skipped: []JUnitSkip{},
	}

	xmlBytes, err := base64.StdEncoding.DecodeString(xmlBase64)
	if err != nil {
		return preview, fmt.Errorf("decode XML: %w", err)
	}

	testcases, err := parseJUnitXML(xmlBytes)
	if err != nil {
		return preview, err
	}
	if len(testcases) == 0 {
		return preview, fmt.Errorf("JUnit XML contained no testcase elements")
	}
	preview.Total = len(testcases)

	// Build normalized summary -> []testKey across all tests for this profile.
	rows, err := r.db.Query(
		`SELECT jira_key, summary FROM test_case WHERE profile_id = ?`,
		profileID,
	)
	if err != nil {
		return preview, fmt.Errorf("load tests for matching: %w", err)
	}
	defer rows.Close()

	byName := make(map[string][]string) // normalized summary -> []testKey
	for rows.Next() {
		var key, summary string
		if err := rows.Scan(&key, &summary); err != nil {
			return preview, fmt.Errorf("scan test row: %w", err)
		}
		norm := strings.ToLower(strings.TrimSpace(summary))
		byName[norm] = append(byName[norm], key)
	}
	if err := rows.Err(); err != nil {
		return preview, fmt.Errorf("iterate tests: %w", err)
	}

	for _, tc := range testcases {
		result, skipped := junitResult(tc)
		norm := strings.ToLower(strings.TrimSpace(tc.Name))
		hits := byName[norm]

		switch {
		case len(hits) == 1:
			// Exactly one match: allocate existing test.
			row := JUnitNewExecRow{
				Testcase:            tc.Name,
				TestKey:             hits[0],
				Summary:             tc.Name,
				Result:              result,
				Create:              false,
				TargetKind:          JUnitTargetRun,
				Parameters:          []backend.IterationParameter{},
				CandidateIterations: []JUnitIterationCandidate{},
			}
			if dataset, found, datasetErr := r.GetTestDataset(profileID, hits[0], ""); datasetErr != nil {
				return preview, datasetErr
			} else if !skipped && found && len(dataset.Parameters) > 0 {
				parameterized, expandErr := expandStoredDataset(dataset)
				if expandErr != nil {
					return preview, expandErr
				}
				row.TargetKind = JUnitTargetIteration
				row.DatasetSourceKind = backend.DatasetSourceTest
				row.DatasetSourceKey = hits[0]
				row.CandidateIterations = junitIterationCandidates(parameterized)
				_, signature, reason := matchIteration(tc.Properties, parameterized)
				row.Parameters = signature
				row.Reason = reason
			}
			// Skipped in JUnit: allocated but result left unset.
			if skipped {
				row.Result = ""
			}
			preview.Rows = append(preview.Rows, row)

		case len(hits) == 0:
			if createMissing {
				row := JUnitNewExecRow{
					Testcase:            tc.Name,
					TestKey:             "",
					Summary:             tc.Name,
					Create:              true,
					TargetKind:          JUnitTargetRun,
					Parameters:          []backend.IterationParameter{},
					CandidateIterations: []JUnitIterationCandidate{},
				}
				if !skipped {
					row.Result = result
				}
				preview.Rows = append(preview.Rows, row)
			} else {
				preview.Skipped = append(preview.Skipped, JUnitSkip{
					Testcase: tc.Name,
					Reason:   "no matching test; create-missing disabled",
				})
			}

		default:
			// More than one match: ambiguous.
			preview.Skipped = append(preview.Skipped, JUnitSkip{
				Testcase: tc.Name,
				Reason:   fmt.Sprintf("ambiguous: %d tests share this summary", len(hits)),
			})
		}
	}

	return preview, nil
}

func expandStoredDataset(dataset Dataset) (backend.ParameterizedRun, error) {
	parameters := make([]backend.DatasetParameter, len(dataset.Parameters))
	for index, parameter := range dataset.Parameters {
		parameters[index] = backend.DatasetParameter{Name: parameter.Name, Combinatorial: parameter.Combinatorial}
	}
	iterations, err := backend.ExpandDataset(backend.Dataset{Parameters: parameters, Rows: dataset.Rows}, parameterizedRunLimit)
	if err != nil {
		return backend.ParameterizedRun{}, err
	}
	for index := range iterations {
		iterations[index].ID = iterationSignatureHash(iterations[index].Parameters)
	}
	return backend.ParameterizedRun{Iterations: iterations}, nil
}

func iterationSignatureHash(parameters []backend.IterationParameter) string {
	encoded, _ := json.Marshal(parameters)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:8])
}

// ApplyJUnitImportNewExec queues all pending changes needed to materialise a
// new Test Execution from a JUnit analysis preview:
//  1. For each row with Create=true, CreateTest is called and the returned temp
//     key is used as the working test key.
//  2. All working test keys (existing + created) are passed to
//     CreateContainerAllocation to create the execution and allocate them.
//  3. For each row with a non-empty Result, SetTestRunStatus is called with the
//     new execution's temp key so the result is queued for commit.
//
// summary is the name for the new Test Execution; projectKey is the Jira
// project key used when creating the container (resolved by the App binding
// from the active profile). An error is returned if summary is blank.
func (r *Repository) ApplyJUnitImportNewExec(profileID, projectKey, summary string, rows []JUnitNewExecRow) (JUnitNewExecResult, error) {
	var res JUnitNewExecResult
	if strings.TrimSpace(summary) == "" {
		return res, fmt.Errorf("execution summary must not be blank")
	}
	seenTargets := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.Result != "" && row.Result != "PASS" && row.Result != "FAIL" {
			return res, fmt.Errorf("invalid result %q for %q", row.Result, row.Testcase)
		}
		kind := row.TargetKind
		if kind == "" {
			kind = JUnitTargetRun
		}
		if kind != JUnitTargetRun && kind != JUnitTargetIteration {
			return res, fmt.Errorf("invalid target kind %q for %q", kind, row.Testcase)
		}
		if kind == JUnitTargetIteration {
			if row.Create || row.TestKey == "" || len(row.Parameters) == 0 || row.DatasetSourceKind == "" || row.DatasetSourceKey == "" {
				return res, fmt.Errorf("dataset iteration for %q is unresolved", row.Testcase)
			}
			target := row.TestKey + ":" + iterationSignatureHash(row.Parameters)
			if _, duplicate := seenTargets[target]; duplicate {
				return res, fmt.Errorf("duplicate dataset iteration target for %q", row.Testcase)
			}
			seenTargets[target] = struct{}{}
		}
	}

	// Step 1: create brand-new tests for rows marked Create=true.
	workingKeys := make([]string, len(rows)) // parallel to rows
	for i, row := range rows {
		if !row.Create {
			workingKeys[i] = row.TestKey
			continue
		}
		tempKey, err := r.CreateTest(profileID, TestDraft{
			Summary: row.Summary,
		})
		if err != nil {
			res.Failed = append(res.Failed, fmt.Sprintf("create test %q: %v", row.Summary, err))
			continue
		}
		workingKeys[i] = tempKey
		res.Created++
	}

	// Collect all non-empty working keys in row order.
	allKeys := make([]string, 0, len(workingKeys))
	for _, k := range workingKeys {
		if k != "" {
			allKeys = append(allKeys, k)
		}
	}

	// Step 2: create the execution and allocate all tests.
	containerRes, err := r.CreateContainerAllocation(profileID, projectKey, "testexec", summary, allKeys)
	if err != nil {
		return res, fmt.Errorf("create execution: %w", err)
	}
	res.ExecKey = containerRes.TempKey
	res.Allocated = containerRes.Added

	// Step 3: set run results for rows with a non-empty Result.
	for i, row := range rows {
		wk := workingKeys[i]
		if wk == "" || row.Result == "" {
			continue
		}
		kind := row.TargetKind
		if kind == "" {
			kind = JUnitTargetRun
		}
		if kind == JUnitTargetIteration {
			if len(row.Parameters) == 0 || row.DatasetSourceKind == "" || row.DatasetSourceKey == "" {
				res.Failed = append(res.Failed, fmt.Sprintf("set result for %q: dataset iteration is unresolved", wk))
				continue
			}
			if err := r.queueDeferredIterationResult(profileID, res.ExecKey, wk, DeferredIterationResult{
				TestKey: wk, Parameters: row.Parameters, Result: row.Result,
				DatasetSourceKind: row.DatasetSourceKind, DatasetSourceKey: row.DatasetSourceKey,
			}); err != nil {
				res.Failed = append(res.Failed, fmt.Sprintf("set result for %q: %v", wk, err))
				continue
			}
			res.ResultsSet++
			continue
		}
		if err := r.SetTestRunStatus(profileID, res.ExecKey, wk, row.Result); err != nil {
			res.Failed = append(res.Failed, fmt.Sprintf("set result for %q: %v", wk, err))
			continue
		}
		res.ResultsSet++
	}

	return res, nil
}

func (r *Repository) queueDeferredIterationResult(profileID, execKey, testKey string, result DeferredIterationResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode deferred iteration result: %w", err)
	}
	base, err := json.Marshal(result.Parameters)
	if err != nil {
		return fmt.Errorf("encode deferred iteration signature: %w", err)
	}
	entityKey := strings.Join([]string{execKey, testKey, iterationSignatureHash(result.Parameters)}, ":")
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin deferred iteration result: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := upsertPendingChange(tx, profileID, entityTestRunIterationImport, entityKey, "status", string(base), string(payload), ""); err != nil {
		return err
	}
	if err := writeAudit(tx, profileID, entityTestRunIterationImport, entityKey, "import-result-local", "status", "", result.Result, ""); err != nil {
		return err
	}
	return tx.Commit()
}

// ApplyJUnitImport sets the run result for each matched testcase in the given
// execution, re-validating membership before each update. Results are collected
// as a BulkEditResult. A match whose test is no longer a member of the
// execution is reported as failed.
func (r *Repository) ApplyJUnitImport(profileID, execKey string, matches []JUnitMatch) (BulkEditResult, error) {
	result := BulkEditResult{Succeeded: []string{}, Failed: []BulkFailure{}}
	seenTargets := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		kind := match.TargetKind
		if kind == "" {
			kind = JUnitTargetRun
		}
		target := string(kind) + ":" + match.TestKey
		if kind == JUnitTargetIteration {
			target += ":" + match.IterationID
		}
		if _, duplicate := seenTargets[target]; duplicate {
			return result, fmt.Errorf("duplicate JUnit target %s", target)
		}
		seenTargets[target] = struct{}{}
	}

	for _, m := range matches {
		switch m.Result {
		case "PASS", "FAIL":
			// valid
		default:
			result.Failed = append(result.Failed, BulkFailure{
				TestKey: m.TestKey,
				Error:   fmt.Sprintf("invalid result %q: must be PASS or FAIL", m.Result),
			})
			continue
		}

		ok, err := isExecMember(r.db, profileID, execKey, m.TestKey)
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: m.TestKey, Error: err.Error()})
			continue
		}
		if !ok {
			result.Failed = append(result.Failed, BulkFailure{
				TestKey: m.TestKey,
				Error:   fmt.Sprintf("%s is not a member of execution %s", m.TestKey, execKey),
			})
			continue
		}

		kind := m.TargetKind
		if kind == "" {
			kind = JUnitTargetRun
		}
		if kind == JUnitTargetIteration {
			if err := r.applyJUnitIteration(profileID, execKey, m); err != nil {
				result.Failed = append(result.Failed, BulkFailure{TestKey: m.TestKey, Error: err.Error()})
				continue
			}
			result.Succeeded = append(result.Succeeded, m.TestKey)
			continue
		}
		if kind != JUnitTargetRun {
			result.Failed = append(result.Failed, BulkFailure{TestKey: m.TestKey, Error: fmt.Sprintf("invalid JUnit target kind %q", kind)})
			continue
		}
		if err := r.SetTestRunStatus(profileID, execKey, m.TestKey, m.Result); err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: m.TestKey, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, m.TestKey)
	}

	return result, nil
}

func (r *Repository) applyJUnitIteration(profileID, execKey string, match JUnitMatch) error {
	run, found, err := r.ParameterizedRun(profileID, execKey, match.TestKey)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("parameterized run is no longer available")
	}
	if run.DatasetSourceKind != match.DatasetSourceKind || run.DatasetSourceKey != match.DatasetSourceKey {
		return fmt.Errorf("dataset source changed after preview")
	}
	for _, iteration := range run.Iterations {
		if iteration.ID != match.IterationID {
			continue
		}
		if !sameIterationParameters(iteration.Parameters, match.Parameters) {
			return fmt.Errorf("dataset signature changed after preview")
		}
		return r.SetIterationStatus(profileID, execKey, match.TestKey, match.IterationID, match.Result)
	}
	return fmt.Errorf("dataset iteration is no longer available")
}

func sameIterationParameters(left, right []backend.IterationParameter) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Name != right[index].Name || left[index].Value != right[index].Value {
			return false
		}
	}
	return true
}
