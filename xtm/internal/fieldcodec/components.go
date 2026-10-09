// Package fieldcodec holds the storage encodings shared by the local store,
// the Jira client and the backends. Components are stored newline-bounded
// ("\nA\nB\n") so a LIKE '%\nName\n%' filter matches one whole name.
package fieldcodec

import "strings"

// componentSep separates component names in the stored, newline-bounded
// components string. A newline can't appear in a Jira component name, and
// bounding the whole value with separators lets a `components LIKE
// '%\nName\n%'` filter match one component exactly without a multi-word name
// like "User Management" colliding with "User".
const componentSep = "\n"

// EncodeComponents joins component names into the bounded storage form, or ""
// for none. Empty / whitespace-only names are dropped.
func EncodeComponents(names []string) string {
	clean := make([]string, 0, len(names))
	for _, n := range names {
		if s := strings.TrimSpace(n); s != "" {
			clean = append(clean, s)
		}
	}
	if len(clean) == 0 {
		return ""
	}
	return componentSep + strings.Join(clean, componentSep) + componentSep
}

// DecodeComponents parses the stored components string back into a slice.
func DecodeComponents(stored string) []string {
	out := []string{}
	for _, n := range strings.Split(stored, componentSep) {
		if s := strings.TrimSpace(n); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ComponentFilterPattern builds the LIKE pattern that matches a single
// component name within the bounded storage form.
func ComponentFilterPattern(name string) string {
	return "%" + componentSep + name + componentSep + "%"
}
