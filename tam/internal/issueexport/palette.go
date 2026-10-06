package issueexport

import "strings"

// The app's palette, and the two rules a row's colours are chosen by.
//
// The values live in frontend/core/styles/tokens.css, which is the one
// place the suite's colours are defined, and nothing in Go can read a
// stylesheet at render time. So they are written once here and
// palette_test.go parses the token file and fails when the two disagree,
// which is what keeps a copy from drifting the first time somebody
// retunes a chip.
//
// The light values are the ones an export takes, whatever theme the app
// is in. It is the rule lib/chartImage already follows for the pictures a
// report carries: a reader in dark mode must not publish a dark table
// onto a white page.

// paletteTokens is every colour the workbook paints, under the token name
// the app defines it with. The test reads this map; the constants below
// are what the renderer uses, so a value appears once.
var paletteTokens = map[string]string{
	"--chrome-bg":        chromeBG,
	"--chrome-text":      chromeText,
	"--border":           borderLine,
	"--border-subtle":    rowLine,
	"--surface-3":        neutralBG,
	"--text":             neutralText,
	"--text-muted":       mutedText,
	"--chip-blue-bg":     blueBG,
	"--chip-blue-text":   blueText,
	"--chip-green-bg":    greenBG,
	"--chip-green-text":  greenText,
	"--chip-red-bg":      redBG,
	"--chip-red-text":    redText,
	"--chip-purple-bg":   purpleBG,
	"--chip-purple-text": purpleText,
	"--chip-sky-bg":      skyBG,
	"--chip-sky-text":    skyText,
	"--chip-teal-bg":     tealBG,
	"--chip-teal-text":   tealText,
	"--chip-pink-bg":     pinkBG,
	"--chip-pink-text":   pinkText,
	"--chip-orange-bg":   orangeBG,
	"--chip-orange-text": orangeText,
	"--warning-bg":       warningBG,
	"--warning-text":     warningText,
}

const (
	chromeBG    = "#1b2638"
	chromeText  = "#e8eaed"
	borderLine  = "#dfe1e6"
	rowLine     = "#eceef1"
	neutralBG   = "#fafbfc"
	neutralText = "#1f2933"
	mutedText   = "#6b7280"

	blueBG     = "#dbeafe"
	blueText   = "#1e40af"
	greenBG    = "#dcfce7"
	greenText  = "#166534"
	redBG      = "#fee2e2"
	redText    = "#991b1b"
	purpleBG   = "#ede9fe"
	purpleText = "#5b21b6"
	skyBG      = "#e0f2fe"
	skyText    = "#075985"
	tealBG     = "#ccfbf1"
	tealText   = "#115e59"
	pinkBG     = "#fce7f3"
	pinkText   = "#9d174d"
	orangeBG   = "#ffedd5"
	orangeText = "#9a3412"

	warningBG   = "#fef3c7"
	warningText = "#92400e"
)

// chip is one chip's pair, the way a .chip-* rule sets them.
type chip struct {
	fill, text string
}

// chips are the palette entries by the class suffix typeChipClass and
// statusClass answer with, so the lookup here is the same lookup the
// stylesheet does.
var chips = map[string]chip{
	// Type chips, in the order ISSUE_TYPES lists them.
	"task":        {blueBG, blueText},
	"epic":        {purpleBG, purpleText},
	"story":       {greenBG, greenText},
	"bug":         {redBG, redText},
	"requirement": {warningBG, warningText},
	"subtask":     {skyBG, skyText},
	"none":        {neutralBG, mutedText},
	// The three a type TAM does not model draws from (#79).
	"alt-teal":   {tealBG, tealText},
	"alt-pink":   {pinkBG, pinkText},
	"alt-orange": {orangeBG, orangeText},
	// Status chips. "todo" is the neutral surface with ordinary text
	// rather than the muted text a typeless row gets.
	"todo":   {neutralBG, neutralText},
	"active": {blueBG, blueText},
	"done":   {greenBG, greenText},
}

// modelled are the six logical types, in the order the app lists them.
var modelled = []string{"task", "epic", "story", "bug", "requirement", "subtask"}

// altChips are the entries a type TAM does not model draws from, in the
// order the frontend's TYPE_CHIP_ALT_CLASSES lists them. The order is the
// contract: it is what the hash below indexes.
var altChips = []string{"alt-teal", "alt-pink", "alt-orange"}

// typeChipClass is lib/typeChip.ts's rule, ported. A modelled type has
// its own colour; a row with no type at all keeps the neutral chip, since
// there is nothing to tell apart; anything else is spread over the three
// alternates by name, so Improvement is the same colour in the file as it
// is in the grid.
func typeChipClass(issueType string) string {
	for _, t := range modelled {
		if issueType == t {
			return t
		}
	}
	if strings.TrimSpace(issueType) == "" {
		return "none"
	}
	return altChips[djb2(issueType)%uint32(len(altChips))]
}

// djb2 is the frontend's hash, to the letter, including the unsigned
// 32-bit wrap: a different answer here would colour the same type two
// ways in two places. It is not a security hash and is not trying to be;
// it only has to spread a few dozen names over three buckets the same way
// every time, in both languages.
func djb2(s string) uint32 {
	h := uint32(5381)
	for _, c := range s {
		h = (h * 33) ^ uint32(c)
	}
	return h
}

// statusClass is lib/statusClass.ts's rule, ported: Jira's own category
// first, since it is the same three keys on every instance and in every
// language, and the English name guess behind it for a row that carries
// no category. The guess matches the backend's IsDone, so the file and
// the grid never disagree about an English-named issue.
func statusClass(status, category string) string {
	switch category {
	case "done":
		return "done"
	case "indeterminate":
		return "active"
	case "new":
		return "todo"
	}
	s := strings.ToLower(status)
	if s == "done" || s == "closed" || s == "resolved" {
		return "done"
	}
	if strings.Contains(s, "progress") || s == "in review" {
		return "active"
	}
	return "todo"
}

// equalHex compares a token's value with the exporter's, ignoring case
// and the spacing a stylesheet may carry around it.
func equalHex(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
