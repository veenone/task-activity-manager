package backend

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatIDs is how an issue's components or fix versions read in the
// journal: catalog ids as a comma list, the shape labels already use.
func FormatIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ", ")
}

// ParseIDs reads FormatIDs back. Blanks and repeats are dropped and the
// order is kept; anything that is not a whole number is an error, because
// a name in this list would be sent to Jira as an id.
func ParseIDs(s string) ([]int, error) {
	out := []int{}
	seen := map[int]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("%q is not a release or component id", part)
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}
