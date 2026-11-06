package demo_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/pie/internal/demo"
	"agile-suite/pie/internal/query"
)

func TestIsDemo(t *testing.T) {
	for url, want := range map[string]bool{"demo": true, " DEMO ": true, "demo-x": false, "https://demo.example.com": false, "": false} {
		if got := demo.Is(url); got != want {
			t.Errorf("Is(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestAnyQueryListsTheSeedThroughRun(t *testing.T) {
	res, err := query.Run(context.Background(), demo.Client(), "category = ANYTHING")
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 63 || res.Total != len(res.Issues) || res.Truncated {
		t.Fatalf("total %d, fetched %d, truncated %v", res.Total, len(res.Issues), res.Truncated)
	}
	byKey := map[string]query.Issue{}
	for _, is := range res.Issues {
		byKey[is.Key] = is
	}
	got := byKey["ALPHA-101"]
	if got.Project != "ALPHA" || got.ProjectName != "Card Platform" || got.Status != "Closed" || got.Type != "Bug" || got.Summary == "" || got.Updated == "" {
		t.Errorf("ALPHA-101 = %+v", got)
	}
	if _, listed := byKey["ECHO-502"]; listed {
		t.Error("ECHO-502 is only reachable through a link, but a plain query listed it")
	}
}

func TestKeyInReturnsOnlyThoseKeys(t *testing.T) {
	res, err := query.Run(context.Background(), demo.Client(), "key in (ECHO-502, alpha-101, NOPE-1)")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, is := range res.Issues {
		keys = append(keys, is.Key)
	}
	if strings.Join(keys, ",") != "ALPHA-101,ECHO-502" || res.Total != 2 {
		t.Errorf("keys %v, total %d", keys, res.Total)
	}
}

func TestFilter99999IsRefusedWithJirasMessage(t *testing.T) {
	_, err := query.Run(context.Background(), demo.Client(), "category = X OR filter = 99999")
	if err == nil || err.Error() != "A value with ID '99999' does not exist for the field 'filter'." {
		t.Fatalf("err = %v", err)
	}
}

func TestPagesBySearchWindow(t *testing.T) {
	ctx := context.Background()
	all, err := demo.Client().SearchIssues(ctx, "x", nil, nil, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	tail, err := demo.Client().SearchIssues(ctx, "x", nil, nil, all.Total-3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail.Issues) != 3 || tail.Issues[2].Key != all.Issues[all.Total-1].Key {
		t.Errorf("tail holds %d issues", len(tail.Issues))
	}
}

// The link walk in Part 2 follows links from either end, so a link one issue
// shows and the other does not would split a linked set in two.
func TestEveryLinkAppearsOnBothIssues(t *testing.T) {
	issues := everything(t)
	type end struct{ from, to, typ, dir string }
	seen := map[end]bool{}
	for key, f := range issues {
		for _, l := range links(t, f) {
			e := end{key, l.other, l.typ, l.dir}
			seen[e] = true
			if _, ok := issues[l.other]; !ok {
				t.Errorf("%s links to %s, which the seed does not hold", key, l.other)
			}
		}
	}
	opposite := map[string]string{"outward": "inward", "inward": "outward"}
	for e := range seen {
		if !seen[end{e.to, e.from, e.typ, opposite[e.dir]}] {
			t.Errorf("%s %s %s %s has no reverse", e.from, e.dir, e.typ, e.to)
		}
	}
}

func TestSeedHoldsTheCasesTheViewsShow(t *testing.T) {
	issues := everything(t)
	category := func(key string) string { return field(t, issues[key], "status", "statusCategory", "key") }
	resolved := func(key string) bool {
		return string(issues[key]["resolutiondate"]) != "null" && len(issues[key]["resolutiondate"]) > 0
	}

	// Out of step: one clone finished, another not.
	if category("ALPHA-101") != "done" || category("BRAVO-201") != "new" {
		t.Error("ALPHA-101 and BRAVO-201 should be out of step")
	}
	if field(t, issues["BRAVO-202"], "status", "name") != "Corrected" || category("BRAVO-202") != "done" || resolved("BRAVO-202") {
		t.Error("BRAVO-202 should be Corrected, done, and have no resolution date")
	}
	if !resolved("ALPHA-101") || resolved("BRAVO-201") {
		t.Error("only finished issues other than Corrected carry a resolution date")
	}
	for _, k := range []string{"ALPHA-104", "BRAVO-203", "DELTA-402", "FOXTROT-601"} {
		if len(links(t, issues[k])) == 0 {
			t.Errorf("%s has no link, so the four-project set is broken", k)
		}
	}
	unlinked := 0
	for _, f := range issues {
		if len(links(t, f)) == 0 {
			unlinked++
		}
	}
	if unlinked < 20 {
		t.Errorf("only %d issues have no links", unlinked)
	}
}

func everything(t *testing.T) map[string]map[string]json.RawMessage {
	t.Helper()
	ctx := context.Background()
	page, err := demo.Client().SearchIssues(ctx, "x", nil, nil, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	// The linked-only issues come back through key in, as the walk will fetch them.
	hidden, err := demo.Client().SearchIssues(ctx, "key in (ECHO-502)", nil, nil, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]json.RawMessage{}
	for _, is := range append(page.Issues, hidden.Issues...) {
		out[is.Key] = is.Fields
	}
	return out
}

type link struct{ other, typ, dir string }

func links(t *testing.T, f map[string]json.RawMessage) []link {
	t.Helper()
	var raw []struct {
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
		Outward *corejira.RawIssue `json:"outwardIssue"`
		Inward  *corejira.RawIssue `json:"inwardIssue"`
	}
	if err := json.Unmarshal(f["issuelinks"], &raw); err != nil {
		t.Fatal(err)
	}
	var out []link
	for _, l := range raw {
		if l.Outward != nil {
			out = append(out, link{l.Outward.Key, l.Type.Name, "outward"})
		} else {
			out = append(out, link{l.Inward.Key, l.Type.Name, "inward"})
		}
	}
	return out
}

func field(t *testing.T, f map[string]json.RawMessage, path ...string) string {
	t.Helper()
	cur := f[path[0]]
	for _, p := range path[1:] {
		var obj map[string]json.RawMessage
		if json.Unmarshal(cur, &obj) != nil {
			return ""
		}
		cur = obj[p]
	}
	var s string
	_ = json.Unmarshal(cur, &s)
	return s
}

func TestExploreFindsTheSeedsSevenSets(t *testing.T) {
	res, err := query.Explore(context.Background(), demo.Client(), "anything", true, query.Fields{})
	if err != nil {
		t.Fatal(err)
	}
	out := 0
	for _, s := range res.Sets {
		if s.OutOfStep {
			out++
		}
	}
	if len(res.Sets) != 7 || out != 5 || res.LinkError != "" || res.LinksTruncated {
		t.Fatalf("%d sets, %d out of step, link error %q", len(res.Sets), out, res.LinkError)
	}
	for _, is := range res.Issues {
		if is.Matched == (is.Key == "ECHO-502") {
			t.Errorf("%s matched = %v", is.Key, is.Matched)
		}
	}
}

// Demo names an invented product base field, and an issue reached only
// through a link carries it too, because reach counts those issues.
func TestDemoHasAProductBaseField(t *testing.T) {
	ctx := context.Background()
	client := demo.Client()
	id, err := client.CustomFieldID(ctx, "product base")
	if err != nil {
		t.Fatal(err)
	}
	res, err := query.Explore(ctx, client, "anything", true, query.Fields{Base: id})
	if err != nil {
		t.Fatal(err)
	}
	bases := map[string]string{}
	for _, is := range res.Issues {
		if len(is.Bases) != 1 {
			t.Fatalf("%s bases %q", is.Key, is.Bases)
		}
		bases[is.Key] = is.Bases[0]
	}
	if bases["ALPHA-101"] != bases["BRAVO-201"] || bases["ALPHA-101"] == bases["CHARLIE-301"] || bases["ECHO-502"] == "" {
		t.Fatalf("bases %v", bases)
	}
}

func TestDemoHasClientFields(t *testing.T) {
	ctx := context.Background()
	client := demo.Client()
	var ids query.Fields
	var err error
	if ids.Visibility, err = client.CustomFieldID(ctx, "client visibility"); err != nil {
		t.Fatal(err)
	}
	if ids.Impact, err = client.CustomFieldID(ctx, "client impact"); err != nil {
		t.Fatal(err)
	}
	res, err := query.Explore(ctx, client, "anything", true, ids)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, is := range res.Issues {
		if is.ClientVisibility != "" {
			seen++
		}
		if is.Key == "ALPHA-101" && (is.ClientVisibility == "" || is.ClientImpact == "") {
			t.Errorf("ALPHA-101 = %+v", is)
		}
	}
	if seen == 0 || seen == len(res.Issues) {
		t.Fatalf("%d of %d issues have a client visibility; demo needs some with and some without", seen, len(res.Issues))
	}
}

func TestDemoHasJiraFilters(t *testing.T) {
	ctx := context.Background()
	starred, err := query.Starred(ctx, demo.Client())
	if err != nil || len(starred) < 2 {
		t.Fatalf("starred %+v, %v", starred, err)
	}
	if _, err := query.Filter(ctx, demo.Client(), starred[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := query.Filter(ctx, demo.Client(), "4300"); err != nil {
		t.Fatalf("the unstarred demo filter: %v", err)
	}
	if _, err := query.Filter(ctx, demo.Client(), "99999"); err == nil {
		t.Fatal("demo answered a filter it does not hold")
	}
}
