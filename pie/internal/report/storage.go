package report

import (
	"fmt"
	"html"
	"slices"
	"strings"

	"agile-suite/pie/internal/cases"
	"agile-suite/pie/internal/cycles"
)

// Storage is the cycle as a Confluence storage-format body: what changed
// since the last published cycle, the cases by project grid, then each case's
// verdicts with their reasons. Every value is escaped, because reasons are
// free text.
func Storage(d cycles.Detail) string {
	var b strings.Builder
	ch := d.Changes
	if ch.Since != "" {
		para(&b, fmt.Sprintf("Compared with %s, the last cycle published before this one.", ch.Since))
	} else {
		para(&b, "No cycle was published before this one, so every case counts as new.")
	}
	b.WriteString("<h2>What changed</h2><ul>")
	b.WriteString("<li>" + counted(len(ch.New), "new cases", ch.New) + "</li>")
	b.WriteString("<li>" + counted(len(ch.Gone), "cases gone", ch.Gone) + "</li>")
	b.WriteString("<li>" + counted(len(ch.Verdicts), "verdicts changed", nil) + "</li></ul>")
	if len(ch.Verdicts) > 0 {
		b.WriteString("<table><tbody>")
		row(&b, "th", "Case", "Project", "Was", "Now")
		for _, v := range ch.Verdicts {
			row(&b, "td", v.Case, v.Project, v.From, v.To)
		}
		b.WriteString("</tbody></table>")
	}

	b.WriteString("<h2>Cases by project</h2>")
	projects := projectKeys(d.Cases)
	b.WriteString("<table><tbody>")
	row(&b, "th", append([]string{"Case"}, projects...)...)
	for _, c := range d.Cases {
		verdicts := map[string]string{}
		for _, p := range c.Projects {
			verdicts[p.Key] = p.Verdict
		}
		cells := []string{c.Key}
		for _, p := range projects {
			cells = append(cells, verdicts[p])
		}
		row(&b, "td", cells...)
	}
	b.WriteString("</tbody></table>")

	b.WriteString("<h2>Verdicts and reasons</h2>")
	for _, c := range d.Cases {
		b.WriteString("<h3>" + esc(c.Key) + "</h3>")
		if len(c.Projects) == 0 {
			para(&b, "No projects on this case.")
			continue
		}
		ps := slices.Clone(c.Projects)
		slices.SortFunc(ps, func(x, y cases.Project) int { return strings.Compare(x.Key, y.Key) })
		b.WriteString("<table><tbody>")
		row(&b, "th", "Project", "Verdict", "Reason", "Assessed")
		for _, p := range ps {
			row(&b, "td", p.Key, p.Verdict, p.Reason, day(p.AssessedAt))
		}
		b.WriteString("</tbody></table>")
	}
	return b.String()
}

func para(b *strings.Builder, text string) { b.WriteString("<p>" + esc(text) + "</p>") }

func row(b *strings.Builder, cell string, values ...string) {
	b.WriteString("<tr>")
	for _, v := range values {
		b.WriteString("<" + cell + ">" + esc(v) + "</" + cell + ">")
	}
	b.WriteString("</tr>")
}

func counted(n int, what string, keys []string) string {
	s := fmt.Sprintf("%d %s", n, what)
	if len(keys) > 0 {
		s += ": " + strings.Join(keys, ", ")
	}
	return esc(s)
}

func projectKeys(cs []cases.Detail) []string {
	var out []string
	for _, c := range cs {
		for _, p := range c.Projects {
			if !slices.Contains(out, p.Key) {
				out = append(out, p.Key)
			}
		}
	}
	slices.Sort(out)
	return out
}

func day(stamp string) string {
	if len(stamp) >= 10 {
		return stamp[:10]
	}
	return stamp
}

// esc escapes text for the storage format and drops the control characters
// XML 1.0 cannot hold, which reasons pasted from elsewhere sometimes carry.
// Confluence refuses a body containing one.
func esc(s string) string {
	return html.EscapeString(strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, s))
}
