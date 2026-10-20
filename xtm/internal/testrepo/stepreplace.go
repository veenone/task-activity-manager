// Find and replace over step text across several tests (#311).
//
// Step editing is otherwise one test at a time, so a renamed field or a moved
// endpoint that appears in the steps of two hundred tests has to be fixed two
// hundred times by hand. The scan here collects every match first and the
// replace writes only the matches the user kept, each one through the ordinary
// EditTestStepField path so it journals, coalesces, conflicts and discards like
// a step edited in the detail panel.

package testrepo

import (
	"errors"
	"fmt"
	"regexp"
)

// stepTextFields are the step fields a search reads, in the order a match list
// shows them. They are the editable set stepFields whitelists, named again here
// because a map has no order and the preview has to be stable.
var stepTextFields = []string{"action", "data", "expected"}

// StepTextQuery is the text to look for in step text and what to put in its
// place. An empty Fields searches all three.
type StepTextQuery struct {
	Find      string   `json:"find"`
	Replace   string   `json:"replace"`
	Fields    []string `json:"fields"`
	MatchCase bool     `json:"matchCase"`
	WholeWord bool     `json:"wholeWord"`
}

// StepTextMatch is one step field the query matches, with the text it becomes.
type StepTextMatch struct {
	TestKey string `json:"testKey"`
	XrayID  string `json:"xrayId"`
	Index   int    `json:"index"`
	Field   string `json:"field"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Hits    int    `json:"hits"`
}

// StepTextScan reports what a scan found: the matches, the tests whose steps
// hold no match, and the tests whose steps could not be read at all.
type StepTextScan struct {
	Matches []StepTextMatch `json:"matches"`
	Skipped []string        `json:"skipped"`
	Failed  []BulkFailure   `json:"failed"`
}

// StepTextTarget names one step field to rewrite.
type StepTextTarget struct {
	TestKey string `json:"testKey"`
	XrayID  string `json:"xrayId"`
	Field   string `json:"field"`
}

// ScanStepText collects every match of q in the steps of testKeys.
//
// Steps load lazily, so a test nobody has opened has an empty cache and its
// steps have to come from the backend; fetchSteps is how. The cache is read
// first and fetchSteps is called only when it holds nothing for that test,
// which is what keeps a force-refreshing fetcher away from a test carrying
// uncommitted local step edits: refreshing one would overwrite the edited text
// while its pending_change row survived.
func (r *Repository) ScanStepText(
	profileID string,
	testKeys []string,
	q StepTextQuery,
	fetchSteps func(testKey string) ([]Step, error),
) (StepTextScan, error) {
	re, err := q.compile()
	if err != nil {
		return StepTextScan{}, err
	}
	fields, err := q.searchFields()
	if err != nil {
		return StepTextScan{}, err
	}

	var scan StepTextScan
	for _, key := range testKeys {
		steps, err := r.ListTestSteps(profileID, key)
		if err != nil {
			scan.Failed = append(scan.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		if len(steps) == 0 && fetchSteps != nil {
			if steps, err = fetchSteps(key); err != nil {
				scan.Failed = append(scan.Failed, BulkFailure{TestKey: key, Error: err.Error()})
				continue
			}
		}

		found := len(scan.Matches)
		for _, s := range steps {
			for _, field := range fields {
				value := stepTextValue(s, field)
				hits := len(re.FindAllStringIndex(value, -1))
				if hits == 0 {
					continue
				}
				scan.Matches = append(scan.Matches, StepTextMatch{
					TestKey: key,
					XrayID:  s.XrayID,
					Index:   s.Index,
					Field:   field,
					Before:  value,
					After:   re.ReplaceAllLiteralString(value, q.Replace),
					Hits:    hits,
				})
			}
		}
		if len(scan.Matches) == found {
			scan.Skipped = append(scan.Skipped, key)
		}
	}
	return scan, nil
}

// ReplaceStepText rewrites the named step fields, queuing one pending change
// each. The new text is computed from what the store holds now rather than
// from a value the caller sends back, so a stale preview cannot overwrite a
// step with text nobody is looking at any more.
//
// One test's failure does not stop the others: its remaining targets are left
// alone and it is reported, the way every other bulk operation reports.
func (r *Repository) ReplaceStepText(
	profileID string,
	targets []StepTextTarget,
	q StepTextQuery,
) (BulkEditResult, error) {
	re, err := q.compile()
	if err != nil {
		return BulkEditResult{}, err
	}

	var order []string
	seen := map[string]bool{}
	failures := map[string]string{}
	steps := map[string][]Step{}

	for _, t := range targets {
		if !seen[t.TestKey] {
			seen[t.TestKey] = true
			order = append(order, t.TestKey)
		}
		if _, stop := failures[t.TestKey]; stop {
			continue
		}
		if _, ok := stepFields[t.Field]; !ok {
			failures[t.TestKey] = fmt.Sprintf("step field %q is not editable", t.Field)
			continue
		}
		if _, read := steps[t.TestKey]; !read {
			cached, err := r.ListTestSteps(profileID, t.TestKey)
			if err != nil {
				failures[t.TestKey] = err.Error()
				continue
			}
			steps[t.TestKey] = cached
		}
		step, ok := findStepByID(steps[t.TestKey], t.XrayID)
		if !ok {
			failures[t.TestKey] = fmt.Sprintf("step %s is no longer in %s", t.XrayID, t.TestKey)
			continue
		}
		value := stepTextValue(step, t.Field)
		rewritten := re.ReplaceAllLiteralString(value, q.Replace)
		if rewritten == value {
			continue
		}
		if err := r.EditTestStepField(profileID, t.TestKey, t.XrayID, t.Field, rewritten); err != nil {
			failures[t.TestKey] = err.Error()
		}
	}

	var result BulkEditResult
	for _, key := range order {
		if msg, bad := failures[key]; bad {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: msg})
			continue
		}
		result.Succeeded = append(result.Succeeded, key)
	}
	return result, nil
}

// compile turns the query into the pattern that finds it. The term is matched
// literally: a user searching for "1.0" means those three characters, not any
// character between 1 and 0.
func (q StepTextQuery) compile() (*regexp.Regexp, error) {
	if q.Find == "" {
		return nil, errors.New("type the text to find")
	}
	pattern := regexp.QuoteMeta(q.Find)
	if q.WholeWord {
		// A word boundary only means something next to a word character. Asking
		// for whole words around "/rest/" would otherwise match nothing and read
		// as a broken search rather than an inapplicable option.
		runes := []rune(q.Find)
		if isWordRune(runes[0]) {
			pattern = `\b` + pattern
		}
		if isWordRune(runes[len(runes)-1]) {
			pattern += `\b`
		}
	}
	if !q.MatchCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("build the search for %q: %w", q.Find, err)
	}
	return re, nil
}

// searchFields resolves which step fields to read, keeping the display order
// and refusing a field that is not one of them.
func (q StepTextQuery) searchFields() ([]string, error) {
	if len(q.Fields) == 0 {
		return stepTextFields, nil
	}
	wanted := make(map[string]bool, len(q.Fields))
	for _, f := range q.Fields {
		if _, ok := stepFields[f]; !ok {
			return nil, fmt.Errorf("step field %q cannot be searched", f)
		}
		wanted[f] = true
	}
	fields := make([]string, 0, len(wanted))
	for _, f := range stepTextFields {
		if wanted[f] {
			fields = append(fields, f)
		}
	}
	return fields, nil
}

// isWordRune reports whether a word boundary beside r would mean anything. Go's
// regexp reads \b over ASCII word characters only, so this matches that.
func isWordRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		return true
	}
	return false
}

// stepTextValue reads one searchable field off a step.
func stepTextValue(s Step, field string) string {
	switch field {
	case "action":
		return s.Action
	case "data":
		return s.Data
	case "expected":
		return s.Expected
	}
	return ""
}

// findStepByID looks a step up by the id the preview carried.
func findStepByID(steps []Step, xrayID string) (Step, bool) {
	for _, s := range steps {
		if s.XrayID == xrayID {
			return s, true
		}
	}
	return Step{}, false
}
