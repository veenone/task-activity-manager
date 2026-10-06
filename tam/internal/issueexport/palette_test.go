package issueexport

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// tokensPath is the stylesheet the app's palette lives in. The exporter
// cannot read CSS at render time, so it carries the values and this test
// is what keeps the copy honest.
const tokensPath = "../../../frontend/core/styles/tokens.css"

// lightTokens reads the :root block, which is the light theme. Dark
// values re-point the same names further down the file and are not what
// an export uses: a dark workbook on a white page is nobody's idea of an
// export, the rule lib/chartImage already follows on the frontend.
func lightTokens(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(tokensPath))
	if err != nil {
		t.Fatalf("read the tokens: %v", err)
	}
	css := string(raw)
	root := regexp.MustCompile(`(?s):root\s*\{(.*?)\n\}`).FindStringSubmatch(css)
	if root == nil {
		t.Fatal("no :root block in the token file")
	}
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`(--[a-z0-9-]+):\s*([^;]+);`).FindAllStringSubmatch(root[1], -1) {
		out[m[1]] = m[2]
	}
	if len(out) == 0 {
		t.Fatal("read no tokens out of the :root block")
	}
	return out
}

// Every colour the workbook paints is one the app defines, with the value
// the app defines. A palette copied by hand drifts the first time
// somebody retunes a chip; this is what says so.
func TestEveryExportedColourMatchesTheAppsToken(t *testing.T) {
	tokens := lightTokens(t)
	for name, hex := range paletteTokens {
		got, ok := tokens[name]
		if !ok {
			t.Errorf("%s is not a token the app defines", name)
			continue
		}
		if !equalHex(got, hex) {
			t.Errorf("%s = %s in the app, %s in the export", name, got, hex)
		}
	}
	// M2: the check is worth nothing if the table is empty.
	if len(paletteTokens) < 10 {
		t.Errorf("the palette has %d entries, which is too few to be the chip set", len(paletteTokens))
	}
}

// The type colours are the grid's own rule, ported. These pairs are
// pinned on both sides: lib/typeChip.test.ts holds the same names against
// the same answers, so either implementation drifting fails its own test.
func TestTypeChipClassAgreesWithTheGrid(t *testing.T) {
	cases := map[string]string{
		"task": "task", "epic": "epic", "story": "story",
		"bug": "bug", "requirement": "requirement", "subtask": "subtask",
		"": "none",
		// The three alternates, from the same djb2 the frontend uses.
		"Improvement":    "alt-pink",
		"Change Request": "alt-orange",
		"Technical task": "alt-teal",
		"Spike":          "alt-pink",
		"Incident":       "alt-teal",
	}
	for in, want := range cases {
		if got := typeChipClass(in); got != want {
			t.Errorf("typeChipClass(%q) = %q, want %q", in, got, want)
		}
	}
}

// The status colours are the three buckets the chip uses, by Jira's own
// category, with the name guess behind it for a row that carries none.
func TestStatusClassBucketsTheWayTheChipDoes(t *testing.T) {
	cases := []struct {
		status, category, want string
	}{
		{"Erledigt", "done", "done"},
		{"In Arbeit", "indeterminate", "active"},
		{"Offen", "new", "todo"},
		// No category: the name guess, which matches the backend's IsDone.
		{"Done", "", "done"},
		{"In Progress", "", "active"},
		{"Backlog", "", "todo"},
	}
	for _, c := range cases {
		if got := statusClass(c.status, c.category); got != c.want {
			t.Errorf("statusClass(%q, %q) = %q, want %q", c.status, c.category, got, c.want)
		}
	}
}
