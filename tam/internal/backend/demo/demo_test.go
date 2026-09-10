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

// TestDemoBackendFiltersByTypeAndIgnoresAScopeItCannotAnswer is the demo's
// half of the sync's query: the issue types are honoured, and a scope JQL
// this dataset has no engine for is ignored rather than guessed at. The one
// scope it does honour is the sprint query below.
func TestDemoBackendFiltersByTypeAndIgnoresAScopeItCannotAnswer(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	page, total, err := b.SearchIssuesPage(ctx, "PLAT", "labels = nothing", "2030-01-01T00:00:00Z", []string{backend.TypeEpic}, 0, 100)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 4 || len(page) != 4 {
		t.Errorf("epics: total %d, rows %d, want 4 (a scope it cannot answer, and since, are ignored)", total, len(page))
	}
	for _, iss := range page {
		if iss.Type != backend.TypeEpic {
			t.Errorf("non-epic in result: %+v", iss)
		}
	}
}

// TestDemoBackendNarrowsToTheSprintTheQueryNames is the read a sprint
// completion makes, against the backend the plan's own walk-through uses.
// The completion asks for "sprint = N" and moves everything that comes
// back, so a demo that answered with the whole project would empty the
// backlog into the destination and report it as a success.
func TestDemoBackendNarrowsToTheSprintTheQueryNames(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	whole, _, err := b.SearchIssuesPage(ctx, "PLAT", "", "", backend.AllTypes, 0, 500)
	if err != nil {
		t.Fatalf("whole project: %v", err)
	}

	page, total, err := b.SearchIssuesPage(ctx, "PLAT", "sprint = 12", "", backend.AllTypes, 0, 500)
	if err != nil {
		t.Fatalf("sprint 12: %v", err)
	}
	if total != len(page) {
		t.Errorf("total = %d over %d rows, want the narrowed count", total, len(page))
	}
	if len(page) == 0 || len(page) >= len(whole) {
		t.Fatalf("sprint 12 answered %d of the project's %d issues, want its own cards and no more", len(page), len(whole))
	}
	for _, iss := range page {
		if iss.SprintID != "12" {
			t.Errorf("%s reports sprint %q, want only sprint 12's cards", iss.Key, iss.SprintID)
		}
	}
	// Every one of them, not just some: a completion that reads half a
	// sprint leaves the other half behind in a closed one.
	want := 0
	for _, iss := range whole {
		if iss.SprintID == "12" {
			want++
		}
	}
	if len(page) != want {
		t.Errorf("sprint 12 answered %d cards, want the %d the dataset puts in it", len(page), want)
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

// TestDemoSprintsCarryTheDatasetsGoal is its own test rather than an extra
// assertion here, because an empty goal proves nothing in a walk-through:
// every demo sprint carries a real one so the field has something to show.
func TestDemoSprintsCarryTheDatasetsGoal(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	sprints, err := b.BoardSprints(ctx, 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	want := map[int]string{
		11: "Ship the promo code redemption flow end to end",
		12: "Clear the checkout defect backlog before the freeze",
		13: "Start the loyalty points redesign",
	}
	for _, s := range sprints {
		if s.Goal != want[s.ID] {
			t.Errorf("sprint %d goal = %q, want %q", s.ID, s.Goal, want[s.ID])
		}
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

func TestDemoStartSprintRefusesWhileAnotherIsActiveThenStartsTheFutureOne(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()

	// Sprint 12 is already active in the dataset, so starting the future
	// sprint 13 must be refused and must name the one that is active.
	err := b.StartSprint(ctx, 13, backend.SprintDraft{Name: "Sprint 13", StartDate: "2026-09-01T09:00:00.000+0000", EndDate: "2026-09-15T09:00:00.000+0000"})
	if err == nil || !strings.Contains(err.Error(), "Sprint 12") {
		t.Fatalf("err = %v, want a refusal naming the active sprint", err)
	}

	sprints, err := b.BoardSprints(ctx, 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	for _, s := range sprints {
		if s.ID == 13 && s.State != "future" {
			t.Errorf("sprint 13 state = %q, want future: a refused start must not change it", s.State)
		}
	}

	// Completing the active sprint frees the board, and the future one can
	// then start.
	if err := b.CompleteSprint(ctx, 12); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := b.StartSprint(ctx, 13, backend.SprintDraft{Name: "Sprint 13", StartDate: "2026-09-01T09:00:00.000+0000", EndDate: "2026-09-15T09:00:00.000+0000"}); err != nil {
		t.Fatalf("start after completing the active one: %v", err)
	}

	sprints, err = b.BoardSprints(ctx, 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	states := map[int]string{}
	for _, s := range sprints {
		states[s.ID] = s.State
	}
	if states[12] != "closed" || states[13] != "active" {
		t.Errorf("states = %v, want 12 closed and 13 active", states)
	}

	if err := b.StartSprint(ctx, 9999, backend.SprintDraft{}); err == nil {
		t.Error("starting a sprint the demo does not have is refused")
	}
	if err := b.CompleteSprint(ctx, 9999); err == nil {
		t.Error("completing a sprint the demo does not have is refused")
	}
}

// TestDemoEnforcesJirasFourSprintStateRules pins down the four state rules
// a real Jira enforces around a sprint's lifecycle, which the demo used to
// let three of slide: starting the sprint that is already active succeeded
// silently, starting an already-closed one reopened it, and completing a
// future or an already-closed sprint both succeeded. A real instance
// refuses all four, so the offline walk-through has to as well.
func TestDemoEnforcesJirasFourSprintStateRules(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()
	draft := backend.SprintDraft{Name: "Sprint 12", StartDate: "2026-08-18T09:00:00.000+0000", EndDate: "2026-09-01T09:00:00.000+0000"}

	// Starting the sprint that is already active must be refused, not a
	// silent no-op.
	if err := b.StartSprint(ctx, 12, draft); err == nil {
		t.Error("starting the already-active sprint is refused")
	}

	// Completing a sprint that has not started yet must be refused, not
	// treated as an early completion.
	if err := b.CompleteSprint(ctx, 13); err == nil {
		t.Error("completing a future sprint is refused")
	}

	// Closing the active sprint, then trying to start it again, must be
	// refused rather than reopening it.
	if err := b.CompleteSprint(ctx, 12); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := b.StartSprint(ctx, 12, draft); err == nil {
		t.Error("restarting an already-closed sprint is refused")
	}

	// And completing that same closed sprint again must be refused too.
	if err := b.CompleteSprint(ctx, 12); err == nil {
		t.Error("completing an already-closed sprint is refused")
	}

	sprints, err := b.BoardSprints(ctx, 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	for _, s := range sprints {
		if s.ID == 12 && s.State != "closed" {
			t.Errorf("sprint 12 state = %q, want closed: none of the refused calls above may have changed it", s.State)
		}
		if s.ID == 13 && s.State != "future" {
			t.Errorf("sprint 13 state = %q, want future: the refused completion may not have changed it", s.State)
		}
	}
}

// TestDemoStartSprintHoldsTheDraftBesideTheState is Fix 3: a demo start used
// to discard the whole SprintDraft, so a card looking at the started sprint
// afterward would still see the dataset's own unstarted name and dates
// instead of what the dialog just set.
func TestDemoStartSprintHoldsTheDraftBesideTheState(t *testing.T) {
	b := demobackend.New("PLAT")
	ctx := context.Background()

	if err := b.CompleteSprint(ctx, 12); err != nil {
		t.Fatalf("complete: %v", err)
	}
	draft := backend.SprintDraft{
		Name:      "Sprint 13: the launch",
		Goal:      "Ship the launch",
		StartDate: "2026-09-02T09:00:00.000+0000",
		EndDate:   "2026-09-16T09:00:00.000+0000",
	}
	if err := b.StartSprint(ctx, 13, draft); err != nil {
		t.Fatalf("start: %v", err)
	}

	sprints, err := b.BoardSprints(ctx, 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	var got backend.Sprint
	for _, s := range sprints {
		if s.ID == 13 {
			got = s
		}
	}
	if got.Name != draft.Name || got.StartDate != draft.StartDate || got.EndDate != draft.EndDate {
		t.Errorf("sprint 13 = %+v, want the name and dates from the start draft: %+v", got, draft)
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
