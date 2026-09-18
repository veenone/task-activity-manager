package committer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"agile-suite/tam/internal/issuerepo"
)

// assertNoPlaceholders is the last check before a write leaves the machine.
// Every phase rewrites placeholders before a later phase reads them, and
// holds what it could not make real; if a bug in that rewriting ever lets a
// TAM-NEW key or a draft sprint's negative id through, it ends here as an
// internal error on that one write, and never as a 400 from Jira.
//
// It checks only the reference values each call site hands it (parentKey,
// sprintId, boardId, the issue key, a link's from and to keys, a rank
// neighbour), never free text such as a summary or a label, which may
// legitimately read TAM-NEW-9. A new phase must pass its references the
// same way, not its whole payload.
func assertNoPlaceholders(payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("internal error: the payload could not be checked for placeholders, so it was not sent to Jira: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return fmt.Errorf("internal error: the payload could not be checked for placeholders, so it was not sent to Jira: %w", err)
	}
	if where, found := findPlaceholder(tree, "the payload", false); found {
		return fmt.Errorf("internal error: %s still names a local placeholder, so it was not sent to Jira; this is a TAM bug, discard the change and make it again", where)
	}
	return nil
}

// findPlaceholder walks a decoded payload. negRef says the value sits under
// a key naming a sprint or a board, where a negative whole number is a
// draft sprint's or draft board's id.
func findPlaceholder(v any, path string, negRef bool) (string, bool) {
	switch t := v.(type) {
	case string:
		if strings.HasPrefix(t, issuerepo.DraftPrefix) || (negRef && negativeWhole(t)) {
			return fmt.Sprintf("%s (%s)", path, t), true
		}
	case json.Number:
		if negRef && negativeWhole(t.String()) {
			return fmt.Sprintf("%s (%s)", path, t), true
		}
	case []any:
		for i, e := range t {
			if where, found := findPlaceholder(e, fmt.Sprintf("%s[%d]", path, i), negRef); found {
				return where, true
			}
		}
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lower := strings.ToLower(k)
			if where, found := findPlaceholder(t[k], path+"."+k, strings.Contains(lower, "sprint") || strings.Contains(lower, "board")); found {
				return where, true
			}
		}
	}
	return "", false
}

func negativeWhole(s string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil && n < 0
}
