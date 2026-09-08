package demo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

func TestDemoBackendPagesTheWholeDataset(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	if !b.IsDemo() {
		t.Fatal("IsDemo = false")
	}
	u, err := b.TestConnection(ctx)
	if err != nil || u.Name != "demo" {
		t.Fatalf("connection = %+v, %v", u, err)
	}
	var got []backend.Issue
	total := -1
	for start := 0; total < 0 || start < total; {
		page, n, err := b.SearchIssuesPage(ctx, "PLAT", "", "", backend.AllTypes, start, 25)
		if err != nil {
			t.Fatalf("page at %d: %v", start, err)
		}
		total = n
		if len(page) == 0 {
			break
		}
		got = append(got, page...)
		start += len(page)
	}
	if total != 60 || len(got) != 60 {
		t.Errorf("total %d, fetched %d, want 60 and 60", total, len(got))
	}
}

func TestDemoBackendFiltersByTypeAndIgnoresScopeAndSince(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	page, total, err := b.SearchIssuesPage(ctx, "PLAT", "labels = nothing", "2030-01-01T00:00:00Z", []string{backend.TypeEpic}, 0, 100)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 4 || len(page) != 4 {
		t.Errorf("epics: total %d, rows %d, want 4 (scope and since are ignored)", total, len(page))
	}
	for _, iss := range page {
		if iss.Type != backend.TypeEpic {
			t.Errorf("non-epic in result: %+v", iss)
		}
	}
}

func TestDemoBackendDetailAndTypes(t *testing.T) {
	b := demobackend.New("DEMO")
	ctx := context.Background()
	d, err := b.GetIssueDetail(ctx, "DEMO-412")
	if err != nil || len(d.Links) != 3 {
		t.Errorf("detail = %+v, %v", d, err)
	}
	if _, err := b.GetIssueDetail(ctx, "DEMO-9999"); err == nil {
		t.Error("unknown key should fail")
	}
	types, err := b.IssueTypes(ctx, "DEMO")
	if err != nil || len(types) != 5 || types[4].Name != "Requirement" {
		t.Errorf("types = %+v, %v", types, err)
	}
}

func TestDemoBackendWritesInMemoryAndStagesOneConflict(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	conflictKey := b.ConflictKey()
	if conflictKey != "ACME-412" {
		t.Fatalf("conflict key = %s", conflictKey)
	}
	before, err := b.GetIssue(ctx, "ACME-409")
	if err != nil || before.Summary != "Rotate payment gateway API keys" {
		t.Fatalf("GetIssue: %+v %v", before, err)
	}
	if err := b.UpdateIssue(ctx, "ACME-409", map[string]string{"summary": "Rotate keys", "labels": "security, ops", "storyPoints": "5", "description": "Body"}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	after, _ := b.GetIssue(ctx, "ACME-409")
	if after.Summary != "Rotate keys" || len(after.Labels) != 2 || *after.StoryPoints != 5 || after.Updated <= before.Updated {
		t.Errorf("after update: %+v", after)
	}
	d, _ := b.GetIssueDetail(ctx, "ACME-409")
	if d.Description != "Body" {
		t.Errorf("description overlay: %+v", d)
	}
	page, _, _ := b.SearchIssuesPage(ctx, "ACME", "", "", nil, 0, 100)
	seen := false
	for _, iss := range page {
		if iss.Key == "ACME-409" && iss.Summary == "Rotate keys" {
			seen = true
		}
	}
	if !seen {
		t.Error("search must reflect the overlay")
	}
	if err := b.UpdateIssue(ctx, "ACME-999", map[string]string{"summary": "x"}); err == nil {
		t.Error("unknown key must fail")
	}

	key, err := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeTask, Summary: "New one", Labels: []string{"x"}, StoryPoints: pts(2)})
	if err != nil || key != "ACME-500" {
		t.Fatalf("CreateIssue: %q %v", key, err)
	}
	key2, _ := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeBug, Summary: "Second"})
	if key2 != "ACME-501" {
		t.Errorf("keys count up: %s", key2)
	}
	created, err := b.GetIssue(ctx, key)
	if err != nil || created.Type != backend.TypeTask || created.Status != "To Do" || created.Project != "ACME" || *created.StoryPoints != 2 {
		t.Errorf("created: %+v %v", created, err)
	}

	// The staged conflict fires once: the first GetIssue of the key reports
	// a version later than the search showed, and the next read agrees with
	// the first.
	var base backend.Issue
	for _, iss := range page {
		if iss.Key == conflictKey {
			base = iss
		}
	}
	first, _ := b.GetIssue(ctx, conflictKey)
	second, _ := b.GetIssue(ctx, conflictKey)
	if first.Updated <= base.Updated || second.Updated != first.Updated {
		t.Errorf("staged conflict: base=%s first=%s second=%s", base.Updated, first.Updated, second.Updated)
	}

	specs, err := b.CreateFields(ctx, "ACME", backend.TypeBug)
	if err != nil || len(specs) != 1 || specs[0].Type != "option" || len(specs[0].AllowedValues) != 3 {
		t.Errorf("bug create fields: %+v %v", specs, err)
	}
	if specs, _ := b.CreateFields(ctx, "ACME", backend.TypeTask); len(specs) != 0 {
		t.Errorf("tasks need nothing extra: %+v", specs)
	}
	req, _ := b.CreateFields(ctx, "ACME", backend.TypeRequirement)
	if len(req) != 1 || req[0].Name != "Source" || req[0].Type != "string" {
		t.Errorf("requirement create fields: %+v", req)
	}
	withParent, _ := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeStory, Summary: "Child", ParentKey: "ACME-350"})
	if got, _ := b.GetIssue(ctx, withParent); got.ParentKey != "ACME-350" {
		t.Errorf("parent stored: %+v", got)
	}
	epic, _ := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeEpic, Summary: "New epic", ParentKey: "ACME-320"})
	if got, _ := b.GetIssue(ctx, epic); got.ParentKey != "" {
		t.Errorf("an epic ignores a parent on create: %+v", got)
	}

	if err := b.UpdateIssue(ctx, "ACME-409", map[string]string{"parentKey": "ACME-320"}); err != nil {
		t.Fatalf("UpdateIssue parentKey: %v", err)
	}
	moved, _ := b.GetIssue(ctx, "ACME-409")
	if moved.ParentKey != "ACME-320" {
		t.Errorf("parentKey updated and read back: %+v", moved)
	}
}

func pts(v float64) *float64 { return &v }

func TestDemoBackendLinks(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	types, _ := b.LinkTypes(ctx)
	if len(types) != 3 || types[1].Inward != "is blocked by" {
		t.Errorf("link types: %+v", types)
	}
	foreign, err := b.GetIssue(ctx, "XT-1018")
	if err != nil || foreign.Summary != "Promo code applies discount" || foreign.Project != "XT" {
		t.Errorf("foreign lookup: %+v %v", foreign, err)
	}
	if err := b.CreateLink(ctx, "ACME-409", backend.LinkDraft{Type: "Blocks", Direction: "inward", ToKey: "XT-1018", ToSummary: "Promo code applies discount", ToType: "Test"}); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	d, _ := b.GetIssueDetail(ctx, "ACME-409")
	found := false
	for _, l := range d.Links {
		if l.Key == "XT-1018" && l.Type == "Blocks" && l.Direction == "inward" {
			found = true
		}
	}
	if !found {
		t.Errorf("created link missing from the detail: %+v", d.Links)
	}
	if err := b.CreateLink(ctx, "ACME-409", backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "NOPE-1"}); err == nil {
		t.Error("unknown target must fail")
	}
}

func TestDemoIssuesAllCarryAStatusID(t *testing.T) {
	b := demobackend.New("PLAT")
	page, total, err := b.SearchIssuesPage(context.Background(), "PLAT", "", "", backend.AllTypes, 0, 100)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 60 {
		t.Fatalf("total = %d, want the whole dataset", total)
	}
	for _, iss := range page {
		if iss.StatusID == "" {
			t.Errorf("%s (%s) has no status id; the board could not place it", iss.Key, iss.Status)
		}
		if iss.StatusID != demobackend.StatusID(iss.Status) {
			t.Errorf("%s status id = %q, want the helper's %q", iss.Key, iss.StatusID, demobackend.StatusID(iss.Status))
		}
	}
}

func TestDemoBoardsAreDerivedFromTheProjectKey(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	boards, err := b.Boards(ctx, "ACME")
	if err != nil {
		t.Fatalf("boards: %v", err)
	}
	if len(boards) != 2 {
		t.Fatalf("boards = %+v, want 2", boards)
	}
	if boards[0].ID != 1 || boards[0].Name != "ACME Scrum" || boards[0].Type != backend.BoardTypeScrum {
		t.Errorf("board 0 = %+v", boards[0])
	}
	if boards[1].ID != 2 || boards[1].Name != "ACME Kanban" || boards[1].Type != backend.BoardTypeKanban {
		t.Errorf("board 1 = %+v", boards[1])
	}
}

func TestDemoBoardColumnsUseTheStatusIDHelper(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	for _, boardID := range []int{1, 2} {
		cols, err := b.BoardColumns(ctx, boardID)
		if err != nil {
			t.Fatalf("columns of board %d: %v", boardID, err)
		}
		want := []struct{ name, id string }{
			{"To Do", demobackend.StatusID("To Do")},
			{"In Progress", demobackend.StatusID("In Progress")},
			{"Done", demobackend.StatusID("Done")},
		}
		if len(cols) != len(want) {
			t.Fatalf("board %d columns = %+v", boardID, cols)
		}
		for i, w := range want {
			if cols[i].Name != w.name || len(cols[i].StatusIDs) != 1 || cols[i].StatusIDs[0] != w.id {
				t.Errorf("board %d column %d = %+v, want %s/%s", boardID, i, cols[i], w.name, w.id)
			}
		}
	}
	if _, err := b.BoardColumns(ctx, 9); err == nil {
		t.Error("columns of an unknown board must fail")
	}
}

func TestDemoSprintsAreOnTheScrumBoardOnly(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	sprints, err := b.BoardSprints(ctx, 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	if len(sprints) != 3 {
		t.Fatalf("sprints = %+v, want 3", sprints)
	}
	states := map[int]string{}
	for _, s := range sprints {
		if s.BoardID != 1 {
			t.Errorf("sprint %d is on board %d", s.ID, s.BoardID)
		}
		states[s.ID] = s.State
	}
	if states[11] != "closed" || states[12] != "active" || states[13] != "future" {
		t.Errorf("states = %v, want lowercase closed, active, future", states)
	}
	kanban, err := b.BoardSprints(ctx, 2)
	if err != nil || len(kanban) != 0 {
		t.Errorf("kanban sprints = %+v, %v; want none", kanban, err)
	}
}

func TestDemoBoardIssueKeys(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	inSprint, err := b.BoardIssueKeys(ctx, 1, "12", "PLAT")
	if err != nil {
		t.Fatalf("sprint keys: %v", err)
	}
	if len(inSprint) == 0 {
		t.Fatal("sprint 12 has no keys")
	}
	all, err := b.BoardIssueKeys(ctx, 1, "", "PLAT")
	if err != nil {
		t.Fatalf("board keys: %v", err)
	}
	kanban, err := b.BoardIssueKeys(ctx, 2, "", "PLAT")
	if err != nil {
		t.Fatalf("kanban keys: %v", err)
	}
	if len(all) != len(kanban) {
		t.Errorf("board 1 has %d keys and board 2 has %d, want the same list", len(all), len(kanban))
	}
	// Every whole-board key is a non-requirement issue, and every sprint 12
	// key is one of them.
	page, _, err := b.SearchIssuesPage(ctx, "PLAT", "", "", backend.AllTypes, 0, 100)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	byKey := map[string]backend.Issue{}
	wantAll := 0
	for _, iss := range page {
		byKey[iss.Key] = iss
		if iss.Type != backend.TypeRequirement {
			wantAll++
		}
	}
	if len(all) != wantAll {
		t.Errorf("whole board = %d keys, want the %d non-requirement issues", len(all), wantAll)
	}
	for _, k := range all {
		if byKey[k].Type == backend.TypeRequirement {
			t.Errorf("%s is a requirement and is not on a board", k)
		}
	}
	returned := map[string]bool{}
	for _, k := range inSprint {
		returned[k] = true
		if byKey[k].SprintID != "12" {
			t.Errorf("%s is in sprint %q, not 12", k, byKey[k].SprintID)
		}
	}
	// The other direction: a sprint 12 issue the board left out would be a
	// card missing from the sprint, which no assertion above would catch.
	for _, iss := range page {
		if iss.Type == backend.TypeRequirement || iss.SprintID != "12" {
			continue
		}
		if !returned[iss.Key] {
			t.Errorf("%s is in sprint 12 and the board's key list left it out", iss.Key)
		}
	}
	if _, err := b.BoardIssueKeys(ctx, 9, "", "PLAT"); err == nil {
		t.Error("keys of an unknown board must fail")
	}
}

func TestDemoTransitionMovesTheCardAndRefusesTheCuratedStory(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	// A card the demo lets through.
	if err := b.Transition(ctx, "ACME-409", []string{demobackend.StatusID("Done")}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	iss, err := b.GetIssue(ctx, "ACME-409")
	if err != nil {
		t.Fatal(err)
	}
	if iss.Status != "Done" || iss.StatusID != demobackend.StatusID("Done") {
		t.Errorf("the card moved: %+v", iss)
	}

	// The curated story cannot reach Done, so the offline walk-through can
	// see a failure without a real Jira.
	err = b.Transition(ctx, b.ConflictKey(), []string{demobackend.StatusID("Done")})
	if !errors.Is(err, backend.ErrNoTransition) {
		t.Fatalf("the curated story is refused: %v", err)
	}
	var noPath *backend.NoTransition
	if !errors.As(err, &noPath) || strings.Join(noPath.Reachable, ",") != "To Do,In Progress" {
		t.Errorf("the refusal names what is reachable: %v", err)
	}
	if err := b.Transition(ctx, b.ConflictKey(), []string{demobackend.StatusID("To Do")}); err != nil {
		t.Errorf("only the one target is refused: %v", err)
	}
	check, err := b.CanTransition(ctx, b.ConflictKey(), []string{demobackend.StatusID("Done")})
	if err != nil || check.Allowed || len(check.Reachable) != 2 {
		t.Errorf("check = %+v, %v", check, err)
	}
}

func TestDemoSprintMovesAndRanksApplyToTheDataset(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	if err := b.MoveIssuesToSprint(ctx, "13", []string{"ACME-412", "ACME-409"}); err != nil {
		t.Fatalf("sprint move: %v", err)
	}
	for _, key := range []string{"ACME-412", "ACME-409"} {
		iss, err := b.GetIssue(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if iss.SprintID != "13" || iss.SprintName != "Sprint 13" {
			t.Errorf("%s = %+v", key, iss)
		}
	}
	if err := b.MoveIssuesToSprint(ctx, "", []string{"ACME-412"}); err != nil {
		t.Fatalf("backlog move: %v", err)
	}
	iss, _ := b.GetIssue(ctx, "ACME-412")
	if iss.SprintID != "" || iss.SprintName != "" {
		t.Errorf("the backlog is a destination: %+v", iss)
	}
	if err := b.MoveIssuesToSprint(ctx, "99", []string{"ACME-409"}); err == nil {
		t.Error("a sprint the demo does not have is refused")
	}
	if err := b.RankIssue(ctx, "ACME-412", "ACME-409", true); err != nil {
		t.Errorf("rank: %v", err)
	}
	if err := b.RankIssue(ctx, "ACME-412", "ACME-9999", true); err == nil {
		t.Error("a rank against a card the demo does not hold is refused")
	}
}

// TestARefusedSprintBatchMovesNothing is Jira's own rule: a sprint move
// takes the whole batch or none of it. The demo used to write each card as
// it walked the list and return on the first key it did not hold, leaving
// the earlier ones moved.
func TestARefusedSprintBatchMovesNothing(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	if err := b.MoveIssuesToSprint(ctx, "13", []string{"ACME-412", "ACME-9999"}); err == nil {
		t.Fatal("a batch naming a card the demo does not hold is refused")
	}
	iss, err := b.GetIssue(ctx, "ACME-412")
	if err != nil {
		t.Fatal(err)
	}
	if iss.SprintID == "13" {
		t.Errorf("the first half of a refused batch was applied: %+v", iss)
	}
}
