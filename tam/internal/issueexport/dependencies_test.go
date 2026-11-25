package issueexport_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issueexport"
	"agile-suite/tam/internal/issuerepo"
)

// depGraph is A blocks B, B blocks C, A relates to C, a pending D blocks A,
// and E with no link at all. X is a far end that is not cached.
func depGraph() ([]backend.Issue, issuerepo.DependencyGraph) {
	iss := func(k string) backend.Issue {
		return backend.Issue{Key: k, Type: backend.TypeTask, Summary: "Summary of " + k, Status: "To Do", Labels: []string{}}
	}
	all := []backend.Issue{iss("A"), iss("B"), iss("C"), iss("D"), iss("E")}
	g := issuerepo.DependencyGraph{
		Issues:   all[:4],
		External: []issuerepo.ExternalIssue{{Key: "X", Summary: "Elsewhere", IssueType: "Bug"}},
		Links: []issuerepo.DependencyLink{
			{ID: "1", Type: "Blocks", Outward: "A", Inward: "B"},
			{ID: "2", Type: "Blocks", Outward: "B", Inward: "C"},
			{ID: "3", Type: "Relates", Outward: "A", Inward: "C"},
			{Type: "Blocks", Outward: "D", Inward: "A", Pending: true, PendingID: 7},
			{ID: "4", Type: "Blocks", Outward: "C", Inward: "X", Removal: true, Pending: true},
		},
	}
	return all, g
}

func keysOf(d issueexport.Dependencies) []string {
	out := []string{}
	for _, i := range d.Issues {
		out = append(out, i.Key)
	}
	for _, e := range d.External {
		out = append(out, e.Key)
	}
	return out
}

func TestTheWholeProfileKeepsIssuesWithNoLinkAndEveryLinkType(t *testing.T) {
	all, g := depGraph()
	d := issueexport.SelectDependencies(all, g, issueexport.DependencyScope{})
	if got := keysOf(d); len(got) != 6 || got[4] != "E" || got[5] != "X" {
		t.Errorf("issues = %v, want A..E and the far end X", got)
	}
	if len(d.Links) != 5 {
		t.Errorf("links = %d, want all 5 whatever their type", len(d.Links))
	}
}

func TestAChainKeepsOnlyItsIssuesAndTheChosenTypes(t *testing.T) {
	all, g := depGraph()
	d := issueexport.SelectDependencies(all, g, issueexport.DependencyScope{Focus: "B", Keys: []string{"A", "B", "C"}, Types: []string{"blocks"}})
	if got := keysOf(d); len(got) != 3 || got[0] != "A" || got[2] != "C" {
		t.Errorf("issues = %v, want A, B, C", got)
	}
	if len(d.Links) != 2 || d.Links[0].ID != "1" || d.Links[1].ID != "2" {
		t.Errorf("links = %+v, want the two Blocks links between them and not Relates", d.Links)
	}
	if d.Focus != "B" {
		t.Errorf("focus = %q", d.Focus)
	}
}

func TestAChainAlwaysHoldsItsFocus(t *testing.T) {
	all, g := depGraph()
	d := issueexport.SelectDependencies(all, g, issueexport.DependencyScope{Focus: "E"})
	if got := keysOf(d); len(got) != 1 || got[0] != "E" {
		t.Errorf("issues = %v, want just E", got)
	}
	if len(d.Links) != 0 {
		t.Errorf("links = %d, want none", len(d.Links))
	}
}

func cellRows(t *testing.T, data []byte, sheet string) [][]string {
	t.Helper()
	cells, err := read(t, data).GetRows(sheet)
	if err != nil {
		t.Fatalf("read %s: %v", sheet, err)
	}
	return cells
}

func rowFor(t *testing.T, cells [][]string, key string) []string {
	t.Helper()
	for _, r := range cells {
		if len(r) > 0 && r[0] == key {
			return r
		}
	}
	t.Fatalf("no row for %s in %v", key, cells)
	return nil
}

func TestTheIssuesSheetSaysHowEachIssueIsLinked(t *testing.T) {
	all, g := depGraph()
	data, err := issueexport.DependencyWorkbook(issueexport.SelectDependencies(all, g, issueexport.DependencyScope{}))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	cells := cellRows(t, data, "Issues")
	head := cells[0]
	want := []string{"Key", "Type", "Summary", "Status", "Assignee", "Sprint", "Blocked by", "Blocks", "Other links", "Linked issues"}
	for i, h := range want {
		if i >= len(head) || head[i] != h {
			t.Fatalf("header = %v, want %v", head, want)
		}
	}
	a := rowFor(t, cells, "A")
	if a[6] != "1" || a[7] != "1" || a[8] != "1" {
		t.Errorf("A counts = %v, want blocked by 1 (D), blocks 1 (B), 1 other", a[6:9])
	}
	if a[9] != "Blocks → B; Relates → C; Blocks ← D (pending)" {
		t.Errorf("A linked = %q", a[9])
	}
	if e := rowFor(t, cells, "E"); e[9] != "No dependencies" || e[6] != "0" {
		t.Errorf("E = %v, want no dependencies and zero counts", e)
	}
	if x := rowFor(t, cells, "X"); x[1] != "Bug" || x[3] != "Not in this profile" {
		t.Errorf("X = %v, want its type and that it is not cached", x)
	}
}

func TestTheLinksSheetHasOneRowPerLinkWithItsState(t *testing.T) {
	all, g := depGraph()
	data, err := issueexport.DependencyWorkbook(issueexport.SelectDependencies(all, g, issueexport.DependencyScope{}))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	cells := cellRows(t, data, "Links")
	if len(cells) != 6 {
		t.Fatalf("rows = %d, want a header and 5 links", len(cells))
	}
	if h := cells[0]; h[0] != "From" || h[2] != "Type" || h[3] != "To" || h[5] != "State" {
		t.Errorf("header = %v", h)
	}
	states := map[string]string{}
	for _, r := range cells[1:] {
		states[r[0]+">"+r[3]] = r[5]
		if r[0] == "A" && r[3] == "B" && r[1] != "Summary of A" {
			t.Errorf("from summary = %q", r[1])
		}
	}
	if states["A>B"] != "Committed" || states["D>A"] != "Pending" || states["C>X"] != "Removal pending" {
		t.Errorf("states = %v", states)
	}
}

func TestAChainMarksEachIssueByWhereItSitsAroundTheFocus(t *testing.T) {
	all, g := depGraph()
	data, err := issueexport.DependencyWorkbook(issueexport.SelectDependencies(all, g, issueexport.DependencyScope{Focus: "B", Keys: []string{"A", "B", "C", "D"}, Types: []string{"Blocks"}}))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	cells := cellRows(t, data, "Issues")
	if cells[0][10] != "In the chain" {
		t.Fatalf("header = %v, want an In the chain column", cells[0])
	}
	for key, want := range map[string]string{"B": "Selected", "A": "Upstream", "D": "Upstream", "C": "Downstream"} {
		if r := rowFor(t, cells, key); r[10] != want {
			t.Errorf("%s = %q, want %q", key, r[10], want)
		}
	}
	whole, _ := issueexport.DependencyWorkbook(issueexport.SelectDependencies(all, g, issueexport.DependencyScope{}))
	if h := cellRows(t, whole, "Issues")[0]; len(h) != 10 {
		t.Errorf("whole-profile header = %v, want no chain column", h)
	}
}

func pngDataURL(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestTheGraphSheetCarriesThePictureWhenOneIsGiven(t *testing.T) {
	all, g := depGraph()
	d := issueexport.SelectDependencies(all, g, issueexport.DependencyScope{})
	without, err := issueexport.DependencyWorkbook(d)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if idx, _ := read(t, without).GetSheetIndex("Graph"); idx != -1 {
		t.Errorf("a workbook with no picture has a Graph sheet")
	}
	if d.Image, err = issueexport.DecodePNG(pngDataURL(t)); err != nil {
		t.Fatalf("decode: %v", err)
	}
	with, err := issueexport.DependencyWorkbook(d)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	pics, err := read(t, with).GetPictures("Graph", "A1")
	if err != nil || len(pics) != 1 {
		t.Errorf("pictures = %d, %v, want the graph at A1", len(pics), err)
	}
}

func TestDecodePNGRefusesWhatIsNotAPNG(t *testing.T) {
	for _, in := range []string{"data:image/jpeg;base64,AAAA", "data:image/png;base64,!!!", "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not a png"))} {
		if _, err := issueexport.DecodePNG(in); err == nil {
			t.Errorf("DecodePNG(%.30q) = nil error", in)
		}
	}
	if b, err := issueexport.DecodePNG(""); err != nil || b != nil {
		t.Errorf("an empty picture = %v, %v, want none and no error", b, err)
	}
}
