package testrepo_test

import (
	"reflect"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

func reqLinksByType(t *testing.T, repo *testrepo.Repository, key string) map[string]map[string]string {
	t.Helper()
	links, err := repo.GetRequirementLinks("p1", key)
	if err != nil {
		t.Fatalf("get links of %s: %v", key, err)
	}
	out := map[string]map[string]string{}
	for _, l := range links {
		if out[l.LinkType] == nil {
			out[l.LinkType] = map[string]string{}
		}
		out[l.LinkType][l.ToKey] = l.LinkID
	}
	return out
}

func TestSyncKeepsPendingReqReqLinkEdit(t *testing.T) {
	repo := seedReqRepo(t)
	jira := []testrepo.ReqReqLink{
		{FromKey: "PRD-10", ToKey: "PRD-11", LinkType: "requires", LinkID: "200"},
		{FromKey: "PRD-10", ToKey: "PRD-12", LinkType: "relates", LinkID: "201"},
		{FromKey: "PRD-11", ToKey: "PRD-12", LinkType: "requires", LinkID: "202"},
	}
	if err := repo.ReplaceAllReqReqLinks("p1", jira); err != nil {
		t.Fatalf("seed links: %v", err)
	}
	// PRD-10 keeps PRD-11 and gains PRD-12 under "requires".
	if err := repo.SetRequirementLinks("p1", "PRD-10", "requires", []string{"PRD-11", "PRD-12"}); err != nil {
		t.Fatalf("set links: %v", err)
	}

	if err := repo.ReplaceAllReqReqLinks("p1", jira); err != nil {
		t.Fatalf("resync: %v", err)
	}

	want := map[string]map[string]string{
		"requires": {"PRD-11": "200", "PRD-12": ""},
		"relates":  {"PRD-12": "201"},
	}
	if got := reqLinksByType(t, repo, "PRD-10"); !reflect.DeepEqual(got, want) {
		t.Errorf("PRD-10 links = %v, want %v", got, want)
	}
	wantOther := map[string]map[string]string{"requires": {"PRD-12": "202"}}
	if got := reqLinksByType(t, repo, "PRD-11"); !reflect.DeepEqual(got, wantOther) {
		t.Errorf("PRD-11 links = %v, want the synced %v", got, wantOther)
	}
}

// The live backend returns no requirement-to-requirement links yet, so a sync
// hands the writer an empty list.
func TestSyncWithNoReqReqLinksKeepsStagedOnes(t *testing.T) {
	repo := seedReqRepo(t)
	if err := repo.SetRequirementLinks("p1", "PRD-10", "requires", []string{"PRD-11"}); err != nil {
		t.Fatalf("set links: %v", err)
	}

	if err := repo.ReplaceAllReqReqLinks("p1", nil); err != nil {
		t.Fatalf("resync: %v", err)
	}

	want := map[string]map[string]string{"requires": {"PRD-11": ""}}
	if got := reqLinksByType(t, repo, "PRD-10"); !reflect.DeepEqual(got, want) {
		t.Errorf("PRD-10 links = %v, want %v", got, want)
	}
}
