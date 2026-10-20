package testrepo

import (
	"fmt"
	"testing"
)

// seedStepTextTests stores two tests whose steps mention a Raven 1.0 endpoint
// and one that does not mention it at all.
func seedStepTextTests(t *testing.T, r *Repository) {
	t.Helper()
	if err := r.UpsertTests("p1", []TestCase{
		{Key: "QA-1", ID: "1", Summary: "Read a test", Status: "Open", Updated: "2026-01-01T00:00:00Z"},
		{Key: "QA-2", ID: "2", Summary: "Write a test", Status: "Open", Updated: "2026-01-01T00:00:00Z"},
		{Key: "QA-3", ID: "3", Summary: "Unrelated", Status: "Open", Updated: "2026-01-01T00:00:00Z"},
	}); err != nil {
		t.Fatalf("seed tests: %v", err)
	}
	if err := r.SetTestSteps("p1", "QA-1", []Step{
		{XrayID: "11", Index: 1, Action: "GET /rest/raven/1.0/api/test", Data: "key=QA-9", Expected: "1.0 payload"},
		{XrayID: "12", Index: 2, Action: "Check the body", Data: "", Expected: "Fields match"},
	}); err != nil {
		t.Fatalf("seed QA-1 steps: %v", err)
	}
	if err := r.SetTestSteps("p1", "QA-2", []Step{
		{XrayID: "21", Index: 1, Action: "PUT /rest/raven/1.0/api/test", Data: "/rest/raven/1.0/", Expected: "Accepted"},
	}); err != nil {
		t.Fatalf("seed QA-2 steps: %v", err)
	}
	if err := r.SetTestSteps("p1", "QA-3", []Step{
		{XrayID: "31", Index: 1, Action: "Open the dashboard", Data: "", Expected: "Charts render"},
	}); err != nil {
		t.Fatalf("seed QA-3 steps: %v", err)
	}
}

// noFetch is the fetcher for a scan that must not reach the backend.
func noFetch(t *testing.T) func(string) ([]Step, error) {
	t.Helper()
	return func(key string) ([]Step, error) {
		t.Errorf("fetched steps for %s, which has them cached", key)
		return nil, fmt.Errorf("unexpected fetch of %s", key)
	}
}

func matchKeys(matches []StepTextMatch) map[string]string {
	out := map[string]string{}
	for _, m := range matches {
		out[fmt.Sprintf("%s:%s:%s", m.TestKey, m.XrayID, m.Field)] = m.After
	}
	return out
}

// A scan reports the replacement text for every step field that matches, and
// names the tests that hold no match instead of silently dropping them.
func TestScanStepTextReportsMatchesAndSkips(t *testing.T) {
	r := newTestRepo(t)
	seedStepTextTests(t, r)

	scan, err := r.ScanStepText("p1", []string{"QA-1", "QA-2", "QA-3"}, StepTextQuery{
		Find:    "/rest/raven/1.0/",
		Replace: "/rest/raven/2.0/",
	}, noFetch(t))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	got := matchKeys(scan.Matches)
	want := map[string]string{
		"QA-1:11:action": "GET /rest/raven/2.0/api/test",
		"QA-2:21:action": "PUT /rest/raven/2.0/api/test",
		"QA-2:21:data":   "/rest/raven/2.0/",
	}
	if len(got) != len(want) {
		t.Errorf("matched %d step fields, want %d: %v", len(got), len(want), got)
	}
	for key, after := range want {
		if got[key] != after {
			t.Errorf("%s became %q, want %q", key, got[key], after)
		}
	}
	if len(scan.Skipped) != 1 || scan.Skipped[0] != "QA-3" {
		t.Errorf("skipped %v, want [QA-3]", scan.Skipped)
	}
	for _, m := range scan.Matches {
		if m.TestKey == "QA-1" && m.XrayID == "11" && m.Field == "action" {
			if m.Before != "GET /rest/raven/1.0/api/test" {
				t.Errorf("before text was %q", m.Before)
			}
			if m.Index != 1 {
				t.Errorf("step index was %d, want 1", m.Index)
			}
			if m.Hits != 1 {
				t.Errorf("hit count was %d, want 1", m.Hits)
			}
		}
	}
}

// A test whose steps have never been loaded has an empty cache, so the scan
// has to ask for them. A test that already holds steps must not be refetched:
// a force-refreshing fetcher would overwrite uncommitted local step edits.
func TestScanStepTextFetchesOnlyUncachedSteps(t *testing.T) {
	r := newTestRepo(t)
	seedStepTextTests(t, r)
	if err := r.UpsertTests("p1", []TestCase{
		{Key: "QA-4", ID: "4", Summary: "Never opened", Status: "Open", Updated: "2026-01-01T00:00:00Z"},
	}); err != nil {
		t.Fatalf("seed QA-4: %v", err)
	}

	var fetched []string
	scan, err := r.ScanStepText("p1", []string{"QA-1", "QA-4"}, StepTextQuery{
		Find:    "raven/1.0",
		Replace: "raven/2.0",
	}, func(key string) ([]Step, error) {
		fetched = append(fetched, key)
		return []Step{{XrayID: "41", Index: 1, Action: "DELETE /rest/raven/1.0/api/test"}}, nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(fetched) != 1 || fetched[0] != "QA-4" {
		t.Errorf("fetched %v, want only [QA-4]", fetched)
	}
	if after := matchKeys(scan.Matches)["QA-4:41:action"]; after != "DELETE /rest/raven/2.0/api/test" {
		t.Errorf("QA-4 step 1 action became %q", after)
	}
}

// A local step edit is in the cache and not yet in Jira. The scan has to see
// the edited text, because that is the text the user is looking at.
func TestScanStepTextSeesUncommittedStepEdits(t *testing.T) {
	r := newTestRepo(t)
	seedStepTextTests(t, r)
	if err := r.EditTestStepField("p1", "QA-1", "12", "expected", "1.0 payload returned"); err != nil {
		t.Fatalf("edit step: %v", err)
	}

	scan, err := r.ScanStepText("p1", []string{"QA-1"}, StepTextQuery{
		Find:    "1.0",
		Replace: "2.0",
	}, noFetch(t))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if after := matchKeys(scan.Matches)["QA-1:12:expected"]; after != "2.0 payload returned" {
		t.Errorf("edited step text scanned as %q, want %q", after, "2.0 payload returned")
	}
}

// Fields narrows which of the three a scan reads.
func TestScanStepTextHonoursFieldSelection(t *testing.T) {
	r := newTestRepo(t)
	seedStepTextTests(t, r)

	scan, err := r.ScanStepText("p1", []string{"QA-2"}, StepTextQuery{
		Find:    "/rest/raven/1.0/",
		Replace: "/rest/raven/2.0/",
		Fields:  []string{"data"},
	}, noFetch(t))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	got := matchKeys(scan.Matches)
	if len(got) != 1 {
		t.Fatalf("matched %v, want the data field alone", got)
	}
	if got["QA-2:21:data"] != "/rest/raven/2.0/" {
		t.Errorf("data became %q", got["QA-2:21:data"])
	}
}

// Case sensitivity is off by default and on when asked for.
func TestScanStepTextMatchCase(t *testing.T) {
	r := newTestRepo(t)
	if err := r.UpsertTests("p1", []TestCase{
		{Key: "QA-1", ID: "1", Summary: "Case", Status: "Open", Updated: "2026-01-01T00:00:00Z"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := r.SetTestSteps("p1", "QA-1", []Step{
		{XrayID: "11", Index: 1, Action: "Click SAVE then save"},
	}); err != nil {
		t.Fatalf("seed steps: %v", err)
	}

	insensitive, err := r.ScanStepText("p1", []string{"QA-1"},
		StepTextQuery{Find: "save", Replace: "Submit"}, noFetch(t))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if after := matchKeys(insensitive.Matches)["QA-1:11:action"]; after != "Click Submit then Submit" {
		t.Errorf("case-insensitive replace gave %q", after)
	}

	sensitive, err := r.ScanStepText("p1", []string{"QA-1"},
		StepTextQuery{Find: "save", Replace: "Submit", MatchCase: true}, noFetch(t))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if after := matchKeys(sensitive.Matches)["QA-1:11:action"]; after != "Click SAVE then Submit" {
		t.Errorf("case-sensitive replace gave %q", after)
	}
}

// Whole-word matching leaves a longer word that merely contains the term.
func TestScanStepTextWholeWord(t *testing.T) {
	r := newTestRepo(t)
	if err := r.UpsertTests("p1", []TestCase{
		{Key: "QA-1", ID: "1", Summary: "Words", Status: "Open", Updated: "2026-01-01T00:00:00Z"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := r.SetTestSteps("p1", "QA-1", []Step{
		{XrayID: "11", Index: 1, Action: "The user and the username differ"},
	}); err != nil {
		t.Fatalf("seed steps: %v", err)
	}

	scan, err := r.ScanStepText("p1", []string{"QA-1"},
		StepTextQuery{Find: "user", Replace: "account", WholeWord: true}, noFetch(t))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if after := matchKeys(scan.Matches)["QA-1:11:action"]; after != "The account and the username differ" {
		t.Errorf("whole-word replace gave %q", after)
	}
}

// An empty search term is a mistake the user can make in the dialog, so it is
// refused rather than matching everything.
func TestScanStepTextRefusesEmptyFind(t *testing.T) {
	r := newTestRepo(t)
	seedStepTextTests(t, r)

	if _, err := r.ScanStepText("p1", []string{"QA-1"},
		StepTextQuery{Replace: "x"}, noFetch(t)); err == nil {
		t.Error("an empty find was accepted")
	}
}

// Replacing writes one pending change per step field and reports the tests it
// touched, in the shape the other bulk operations return.
func TestReplaceStepTextQueuesOnePendingChangePerField(t *testing.T) {
	r := newTestRepo(t)
	seedStepTextTests(t, r)
	q := StepTextQuery{Find: "/rest/raven/1.0/", Replace: "/rest/raven/2.0/"}

	result, err := r.ReplaceStepText("p1", []StepTextTarget{
		{TestKey: "QA-1", XrayID: "11", Field: "action"},
		{TestKey: "QA-2", XrayID: "21", Field: "action"},
		{TestKey: "QA-2", XrayID: "21", Field: "data"},
	}, q)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Errorf("failed: %v", result.Failed)
	}
	if len(result.Succeeded) != 2 {
		t.Errorf("succeeded %v, want QA-1 and QA-2", result.Succeeded)
	}

	queued := pendingFields(t, r)
	for key, want := range map[string]string{
		"test_step:QA-1:11:action": "GET /rest/raven/2.0/api/test",
		"test_step:QA-2:21:action": "PUT /rest/raven/2.0/api/test",
		"test_step:QA-2:21:data":   "/rest/raven/2.0/",
	} {
		if queued[key] != want {
			t.Errorf("%s queued %q, want %q", key, queued[key], want)
		}
	}
	if _, ok := queued["test_step:QA-1:12:expected"]; ok {
		t.Error("a step field nobody targeted was rewritten")
	}
}

// The new text comes from the store, not from the caller, so a step edited
// after the preview was taken is rewritten from its current text.
func TestReplaceStepTextComputesFromStoredText(t *testing.T) {
	r := newTestRepo(t)
	seedStepTextTests(t, r)
	if err := r.EditTestStepField("p1", "QA-1", "11", "action", "GET /rest/raven/1.0/api/test/42"); err != nil {
		t.Fatalf("edit step: %v", err)
	}

	if _, err := r.ReplaceStepText("p1", []StepTextTarget{
		{TestKey: "QA-1", XrayID: "11", Field: "action"},
	}, StepTextQuery{Find: "/rest/raven/1.0/", Replace: "/rest/raven/2.0/"}); err != nil {
		t.Fatalf("replace: %v", err)
	}

	if got := pendingFields(t, r)["test_step:QA-1:11:action"]; got != "GET /rest/raven/2.0/api/test/42" {
		t.Errorf("queued %q, want the rewrite of the stored text", got)
	}
}

// A field name the dialog could never send is refused for that test instead of
// being written somewhere unexpected.
func TestReplaceStepTextRejectsUnknownField(t *testing.T) {
	r := newTestRepo(t)
	seedStepTextTests(t, r)

	result, err := r.ReplaceStepText("p1", []StepTextTarget{
		{TestKey: "QA-1", XrayID: "11", Field: "summary"},
	}, StepTextQuery{Find: "GET", Replace: "HEAD"})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(result.Succeeded) != 0 {
		t.Errorf("succeeded %v, want none", result.Succeeded)
	}
	if len(result.Failed) != 1 || result.Failed[0].TestKey != "QA-1" {
		t.Fatalf("failed %v, want QA-1", result.Failed)
	}
	if len(pendingFields(t, r)) != 0 {
		t.Error("an unknown field still queued a change")
	}
}

// A target whose step is gone is reported for its test rather than aborting
// the whole run: the other tests still get their replacement.
func TestReplaceStepTextReportsMissingStepAndContinues(t *testing.T) {
	r := newTestRepo(t)
	seedStepTextTests(t, r)

	result, err := r.ReplaceStepText("p1", []StepTextTarget{
		{TestKey: "QA-1", XrayID: "999", Field: "action"},
		{TestKey: "QA-2", XrayID: "21", Field: "action"},
	}, StepTextQuery{Find: "/rest/raven/1.0/", Replace: "/rest/raven/2.0/"})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0].TestKey != "QA-1" {
		t.Errorf("failed %v, want QA-1", result.Failed)
	}
	if len(result.Succeeded) != 1 || result.Succeeded[0] != "QA-2" {
		t.Errorf("succeeded %v, want QA-2", result.Succeeded)
	}
	if got := pendingFields(t, r)["test_step:QA-2:21:action"]; got != "PUT /rest/raven/2.0/api/test" {
		t.Errorf("QA-2 queued %q", got)
	}
}
