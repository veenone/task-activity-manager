package testrepo

import "strings"

// headerColumn returns the index of the column whose header is name, compared
// case-insensitively with surrounding space ignored, or -1 when name is empty
// or no header matches. Every spreadsheet import maps its columns through it.
func headerColumn(header []string, name string) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return -1
	}
	for i, h := range header {
		if strings.EqualFold(strings.TrimSpace(h), name) {
			return i
		}
	}
	return -1
}
