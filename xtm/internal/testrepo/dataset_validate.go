// Dataset limits, checked before anything is journalled (#277 W1).
//
// Xray enforces these itself and reports them at
// GET /rest/raven/1.0/dataset/info. They are checked here instead of on the
// way out because a table refused at commit time has already been edited,
// queued and forgotten about: the user would be told their work is invalid
// long after they could fix it, by a message from a server they never saw.

package testrepo

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// maxDatasetParameters and maxDatasetValueLength are Xray's own
	// maxParametersSize and maxParametersValueSize, as the instance the #277
	// W0 spike ran against reported them. No row limit is published, so none
	// is enforced: refusing a row count no server asked for would be an
	// invention.
	maxDatasetParameters  = 20
	maxDatasetValueLength = 100
	// maxDatasetParameterNameLength is the longest name Xray's parameter
	// dialog accepts.
	maxDatasetParameterNameLength = 64
	datasetParameterText          = "TEXT"
	datasetParameterList          = "LIST"
	datasetListScopeAdHoc         = "PARAMETER_AD_HOC"
)

// datasetParameterName is the shape Xray requires: a letter or underscore,
// then letters, digits, underscores and hyphens, with single spaces between
// words. A ${placeholder} in a step refers to the name, which is why the
// grammar is this tight.
var datasetParameterName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*( [A-Za-z0-9_-]+)*$`)

// ValidateDataset reports the first thing about a table that Xray would
// refuse, phrased for the person who typed it.
func ValidateDataset(ds Dataset) error {
	if len(ds.Parameters) > maxDatasetParameters {
		return fmt.Errorf("a dataset holds at most %d parameters, and this one has %d",
			maxDatasetParameters, len(ds.Parameters))
	}

	seen := make(map[string]bool, len(ds.Parameters))
	for i, p := range ds.Parameters {
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("parameter %d needs a name", i+1)
		}
		if len(p.Name) > maxDatasetParameterNameLength {
			return fmt.Errorf("the name %q is longer than %d characters",
				p.Name, maxDatasetParameterNameLength)
		}
		if !datasetParameterName.MatchString(p.Name) {
			return fmt.Errorf("%q cannot be a parameter name: start with a letter or underscore, then letters, digits, underscores, hyphens and single spaces", p.Name)
		}
		if seen[p.Name] {
			return fmt.Errorf("the parameter %q appears more than once", p.Name)
		}
		seen[p.Name] = true

		parameterType := p.Type
		if parameterType == "" {
			parameterType = datasetParameterText
		}
		switch parameterType {
		case datasetParameterText:
		case datasetParameterList:
			if len(p.Options) == 0 {
				return fmt.Errorf("the list parameter %q needs at least one option", p.Name)
			}
			options := make(map[string]bool, len(p.Options))
			for _, option := range p.Options {
				if strings.TrimSpace(option) == "" {
					return fmt.Errorf("the list parameter %q has an empty option", p.Name)
				}
				if options[option] {
					return fmt.Errorf("the list parameter %q option %q appears more than once", p.Name, option)
				}
				options[option] = true
			}
			if p.ID <= 0 && p.ListScope != datasetListScopeAdHoc {
				return fmt.Errorf("the new list parameter %q must use an ad-hoc list", p.Name)
			}
			if p.ID <= 0 && p.ListID > 0 {
				return fmt.Errorf("the new list parameter %q cannot use an existing list id", p.Name)
			}
		default:
			return fmt.Errorf("the parameter %q has unsupported type %q", p.Name, p.Type)
		}
	}

	rows := make(map[string]int, len(ds.Rows))
	for i, row := range ds.Rows {
		if len(row) > len(ds.Parameters) {
			return fmt.Errorf("row %d has more cells (%d) than there are parameters (%d)",
				i+1, len(row), len(ds.Parameters))
		}
		for j, cell := range row {
			if len(cell) > maxDatasetValueLength {
				return fmt.Errorf("row %d, %s is longer than %d characters",
					i+1, ds.Parameters[j].Name, maxDatasetValueLength)
			}
			parameter := ds.Parameters[j]
			if parameter.Type == datasetParameterList && !containsDatasetOption(parameter.Options, cell) {
				return fmt.Errorf("row %d, %s uses %q, which is not a list option", i+1, parameter.Name, cell)
			}
		}
		// Xray runs one iteration per row, so two identical rows run the same
		// iteration twice and the second tells nobody anything.
		key := strings.Join(row, "\x00")
		if first, dup := rows[key]; dup {
			return fmt.Errorf("rows %d and %d are the same", first+1, i+1)
		}
		rows[key] = i
	}
	return nil
}

func containsDatasetOption(options []string, value string) bool {
	for _, option := range options {
		if option == value {
			return true
		}
	}
	return false
}
