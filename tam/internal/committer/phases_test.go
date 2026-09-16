package committer_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
	"agile-suite/tam/internal/issuerepo"
)

// CreateSprint is the one Agile write Commit's first phase makes.
func (f *fake) CreateSprint(_ context.Context, boardID int, d backend.SprintDraft) (backend.Sprint, error) {
	if f.sprintCreateErr != nil {
		return backend.Sprint{}, f.sprintCreateErr
	}
	id := 100 + f.nextSprint
	f.nextSprint++
	f.sprintsMade = append(f.sprintsMade, fmt.Sprintf("%d %s", boardID, d.Name))
	return backend.Sprint{ID: id, BoardID: boardID, Name: d.Name, State: "future", StartDate: d.StartDate, EndDate: d.EndDate, Goal: d.Goal}, nil
}

func draftSprint15() issuerepo.DraftSprint {
	return issuerepo.DraftSprint{
		BoardID: demoBoard, BoardName: "PLAT Scrum", Name: "Sprint 15",
		StartDate: "2026-09-16T09:00:00.000+0000", EndDate: "2026-09-30T09:00:00.000+0000",
	}
}

func summaries(ds []backend.IssueDraft) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Summary)
	}
	return out
}

// sort.Strings used to put TAM-NEW-10 before TAM-NEW-2.
func TestDraftsOfOneLevelAreCreatedInNumericOrder(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	var want []string
	for i := 1; i <= 10; i++ {
		summary := fmt.Sprintf("task %d", i)
		want = append(want, summary)
		if _, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: summary}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if got := summaries(h.jira.creates); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("create order = %v", got)
	}
	if last := res.Created[9]; last.TempKey != "TAM-NEW-10" || last.Key != "PLAT-510" {
		t.Errorf("TAM-NEW-10 is created tenth: %+v", last)
	}
}

// planOfFour drafts what the ticket drafted: a sprint, an epic, two stories
// under it (the first also in the sprint), and a technical task under the
// first story. The first story is drafted before the epic, so only the
// phases, not the numbers, put the epic first.
func planOfFour(t *testing.T, h harness) (sprintID int, storyA, epic, storyB, sub string) {
	t.Helper()
	ctx := context.Background()
	s, err := h.repo.CreateDraftSprint(ctx, "p1", draftSprint15())
	if err != nil {
		t.Fatal(err)
	}
	sid := strconv.Itoa(s.ID)
	must := func(key string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return key
	}
	storyA = must(h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Story A", SprintID: sid, SprintName: "Sprint 15"}))
	epic = must(h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeEpic, Summary: "Epic"}))
	if err := h.repo.EditField(ctx, "p1", storyA, "parentKey", epic); err != nil {
		t.Fatal(err)
	}
	storyB = must(h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Story B", ParentKey: epic}))
	sub = must(h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeSubtask, Summary: "Sub", ParentKey: storyA}))
	return s.ID, storyA, epic, storyB, sub
}

func TestASprintAnEpicItsStoriesAndATechnicalTaskCommitInOnePress(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	sprintID, storyA, _, _, _ := planOfFour(t, h)

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 0 || len(res.Held) != 0 || res.Remaining != 0 {
		t.Fatalf("result = %+v", res)
	}
	if len(h.jira.sprintsMade) != 1 || h.jira.sprintsMade[0] != "1 Sprint 15" {
		t.Errorf("sprints made = %v", h.jira.sprintsMade)
	}
	if len(res.CreatedSprints) != 1 || res.CreatedSprints[0] != (committer.CreatedSprint{DraftID: sprintID, ID: 100, Name: "Sprint 15"}) {
		t.Errorf("created sprints = %+v", res.CreatedSprints)
	}
	if got := strings.Join(summaries(h.jira.creates), ","); got != "Epic,Story A,Story B,Sub" {
		t.Fatalf("create order = %s, want the epic, then the stories in number order, then the sub-task", got)
	}
	parents := []string{h.jira.creates[1].ParentKey, h.jira.creates[2].ParentKey, h.jira.creates[3].ParentKey}
	if strings.Join(parents, ",") != "PLAT-501,PLAT-501,PLAT-502" {
		t.Errorf("parents sent = %v, want every one a real key", parents)
	}
	for _, d := range h.jira.creates {
		if strings.HasPrefix(d.ParentKey, issuerepo.DraftPrefix) || strings.HasPrefix(d.SprintID, "-") {
			t.Errorf("a placeholder reached Jira: %+v", d)
		}
	}
	if len(h.jira.pushed) != 1 || h.jira.pushed[0] != "sprint 100 PLAT-502" {
		t.Errorf("pushed = %v, want Story A moved into the real sprint", h.jira.pushed)
	}
	var storyAKey string
	for _, c := range res.Created {
		if c.TempKey == storyA {
			storyAKey = c.Key
		}
	}
	if iss, _ := h.repo.GetIssue(ctx, "p1", storyAKey); iss.SprintID != "100" {
		t.Errorf("Story A settled in the real sprint: %+v", iss)
	}
}

func TestAnEpicJiraRefusesHoldsExactlyItsDescendantsAndKeepsTheSprint(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, storyA, epic, storyB, sub := planOfFour(t, h)
	storyC, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Story C"})
	if err != nil {
		t.Fatal(err)
	}
	h.jira.createErrFor["Epic"] = errors.New("POST failed: 400 Epic Name is required")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.sprintsMade) != 1 {
		t.Errorf("the sprint is created whatever happens to the epic: %v", h.jira.sprintsMade)
	}
	if got := strings.Join(summaries(h.jira.creates), ","); got != "Story C" {
		t.Errorf("creates = %s, want only the story that does not wait for the epic", got)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != epic || !res.Failures[0].Retryable {
		t.Errorf("failures = %+v", res.Failures)
	}
	reasons := map[string]string{}
	for _, held := range res.Held {
		reasons[held.Key] = held.Reason
	}
	want := map[string]string{
		storyA: "waits for " + epic + ", which Jira refused",
		storyB: "waits for " + epic + ", which Jira refused",
		sub:    "waits for " + storyA + ", which is waiting for " + epic,
	}
	if len(reasons) != len(want) {
		t.Fatalf("held = %+v", res.Held)
	}
	for key, reason := range want {
		if reasons[key] != reason {
			t.Errorf("%s held because %q, want %q", key, reasons[key], reason)
		}
	}
	if _, held := reasons[storyC]; held {
		t.Error("a story with no epic is not held")
	}
	if res.Remaining != 4 {
		t.Errorf("remaining = %d, want the epic and its three descendants", res.Remaining)
	}
	pend, _ := h.repo.PendingForKey(ctx, "p1", storyA)
	if len(pend) != 1 || !strings.Contains(pend[0].AfterVal, `"sprintId":"100"`) {
		t.Errorf("Story A keeps the real sprint id for the next Commit: %+v", pend)
	}

	delete(h.jira.createErrFor, "Epic")
	res, err = h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 4 || len(res.Held) != 0 || res.Remaining != 0 || len(h.jira.sprintsMade) != 1 {
		t.Errorf("the next Commit finishes the plan without a second sprint: %+v, sprints %v", res, h.jira.sprintsMade)
	}
}

func TestASprintJiraRefusesHoldsTheDraftsAndTheMovesIntoIt(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	s, err := h.repo.CreateDraftSprint(ctx, "p1", draftSprint15())
	if err != nil {
		t.Fatal(err)
	}
	sid := strconv.Itoa(s.ID)
	if err := h.repo.MoveToSprint(ctx, "p1", "PLAT-1", sid, "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	temp, err := h.repo.CreateDraft(ctx, "p1", "PLAT", draftInSprint(sid, "Sprint 15"))
	if err != nil {
		t.Fatal(err)
	}
	h.jira.sprintCreateErr = errors.New("POST failed: 403 you cannot manage sprints")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != "Sprint 15" || res.Failures[0].EntityType != issuerepo.EntitySprintCreate {
		t.Errorf("failures = %+v", res.Failures)
	}
	if len(h.jira.creates) != 0 || len(h.jira.pushed) != 0 {
		t.Errorf("nothing waiting on the sprint reached Jira: creates %v, pushed %v", h.jira.creates, h.jira.pushed)
	}
	want := `waits for sprint "Sprint 15", which Jira refused`
	got := map[string]string{}
	for _, held := range res.Held {
		got[held.Key] = held.Reason
	}
	if got[temp] != want || got["PLAT-1"] != want || len(got) != 2 {
		t.Errorf("held = %+v", res.Held)
	}
	if res.Remaining != 3 {
		t.Errorf("remaining = %d, want the sprint, the draft and the move", res.Remaining)
	}
}

// A sprint Jira created on an earlier Commit whose local rename failed has no
// sprint_create row left, so the cards moved into it still name its negative
// id and nothing will ever make that id real. They fail, with what to do,
// rather than wait forever or reach Jira.
func TestMovesIntoASprintCreatedButNotRenamedFailWithWhatToDo(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	s, err := h.repo.CreateDraftSprint(ctx, "p1", draftSprint15())
	if err != nil {
		t.Fatal(err)
	}
	sid := strconv.Itoa(s.ID)
	if err := h.repo.MoveToSprint(ctx, "p1", "PLAT-1", sid, "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	temp, err := h.repo.CreateDraft(ctx, "p1", "PLAT", draftInSprint(sid, "Sprint 15"))
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.MarkSprintCreatedWithoutRekey(ctx, "p1", s.ID, 88); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.sprintsMade) != 0 || len(h.jira.creates) != 0 || len(h.jira.pushed) != 0 {
		t.Errorf("nothing reached Jira: sprints %v, creates %v, pushed %v", h.jira.sprintsMade, h.jira.creates, h.jira.pushed)
	}
	const want = "its sprint was created in Jira but not renamed in TAM; move it to the sprint again"
	got := map[string]committer.Failure{}
	for _, f := range res.Failures {
		got[f.Key] = f
	}
	if len(got) != 2 || len(res.Held) != 0 {
		t.Fatalf("failures = %+v, held = %+v", res.Failures, res.Held)
	}
	for _, key := range []string{temp, "PLAT-1"} {
		if f := got[key]; f.Error != want || f.Retryable {
			t.Errorf("%s: %+v", key, f)
		}
	}
	if got["PLAT-1"].EntityType != issuerepo.EntitySprintMove || got["PLAT-1"].RowID == 0 {
		t.Errorf("the move's failure names its row: %+v", got["PLAT-1"])
	}
	if res.Remaining != 2 {
		t.Errorf("remaining = %d, want the draft and the move", res.Remaining)
	}
}

// A link from an issue Jira holds to a draft names the draft's real key by
// the time the link phase sends it, in the same Commit that creates it.
func TestALinkToADraftIsSentWithTheKeyItsCreateGot(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	temp, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Target"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.AddLink(ctx, "p1", "PLAT-1", backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: temp}); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 0 || len(res.Held) != 0 || res.Remaining != 0 {
		t.Fatalf("result = %+v", res)
	}
	if len(h.jira.links) != 1 || h.jira.links[0] != "PLAT-1 outward Relates PLAT-501" {
		t.Errorf("links = %v, want the draft's real key", h.jira.links)
	}
}

// sprintMovedInto drafts Sprint 15 and journals PLAT-1 into it, the smallest
// plan with something waiting on a draft sprint, and answers its draft id.
func sprintMovedInto(t *testing.T, h harness) int {
	t.Helper()
	ctx := context.Background()
	s, err := h.repo.CreateDraftSprint(ctx, "p1", draftSprint15())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.MoveToSprint(ctx, "p1", "PLAT-1", strconv.Itoa(s.ID), "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	return s.ID
}

func heldReasons(res committer.Result) map[string]string {
	out := map[string]string{}
	for _, held := range res.Held {
		out[held.Key] = held.Reason
	}
	return out
}

// A sprint_create row that will not decode has no name, so what waits on it
// names the draft id.
func TestAMoveIntoADraftSprintThatCannotBeReadIsHeldByItsID(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	id := sprintMovedInto(t, h)
	if _, err := h.db.Exec(`UPDATE pending_change SET after_val = '{not json' WHERE profile_id = 'p1' AND entity_type = ?`, issuerepo.EntitySprintCreate); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	label := fmt.Sprintf("draft sprint %d", id)
	if len(res.Failures) != 1 || res.Failures[0].Key != label || res.Failures[0].Retryable {
		t.Errorf("failures = %+v", res.Failures)
	}
	if got := heldReasons(res)["PLAT-1"]; got != "waits for "+label+", which could not be read" {
		t.Errorf("held = %+v", res.Held)
	}
	if len(h.jira.sprintsMade) != 0 || len(h.jira.pushed) != 0 {
		t.Errorf("nothing reached Jira: sprints %v, pushed %v", h.jira.sprintsMade, h.jira.pushed)
	}
}

// idlessSprints is a Jira that creates a sprint and answers with no id.
type idlessSprints struct {
	*fake
	calls *int
}

func (j idlessSprints) CreateSprint(context.Context, int, backend.SprintDraft) (backend.Sprint, error) {
	*j.calls++
	return backend.Sprint{}, nil
}

// A sprint Jira created that TAM could not rename clears its draft, so the
// next Commit does not create it a second time.
func TestASprintCreatedWithNoIDIsNotCreatedAgainNextCommit(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	sprintMovedInto(t, h)
	calls := 0
	eng := committer.New(idlessSprints{fake: h.jira, calls: &calls}, h.repo, h.order)

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Error, "answered with no id") || res.Failures[0].Retryable {
		t.Errorf("failures = %+v", res.Failures)
	}
	want := `waits for sprint "Sprint 15", which Jira created but TAM could not rename; refresh the board, then move its cards again`
	if got := heldReasons(res)["PLAT-1"]; got != want {
		t.Errorf("held = %+v", res.Held)
	}

	if _, err := eng.Commit(ctx, "p1", "PLAT"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("CreateSprint called %d times, want once across both Commits", calls)
	}
}

// noSprintCreate is a backend that answers nothing beyond IssueBackend.
type noSprintCreate struct{ backend.IssueBackend }

func TestADraftSprintOnAConnectionThatCannotCreateOneIsHeld(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	sprintMovedInto(t, h)
	eng := committer.New(noSprintCreate{h.jira}, h.repo, h.order)

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Error != "this connection cannot create sprints" || res.Failures[0].Retryable {
		t.Errorf("failures = %+v", res.Failures)
	}
	if got := heldReasons(res)["PLAT-1"]; got != `waits for sprint "Sprint 15", which this connection cannot create` {
		t.Errorf("held = %+v", res.Held)
	}
	if len(h.jira.sprintsMade) != 0 {
		t.Errorf("sprints made = %v", h.jira.sprintsMade)
	}
}

// journalLostOnLink is a Jira whose link lands and whose journal then cannot
// be read: the moment after the last phase that used to turn a Commit that
// had written to Jira into a Go error, beside which Wails drops the Result.
type journalLostOnLink struct {
	*fake
	db *sql.DB
}

func (j journalLostOnLink) CreateLink(ctx context.Context, fromKey string, d backend.LinkDraft) error {
	if err := j.fake.CreateLink(ctx, fromKey, d); err != nil {
		return err
	}
	_, err := j.db.Exec(`ALTER TABLE pending_change RENAME TO pending_change_gone`)
	return err
}

func TestAJournalLostAfterTheLastPhaseStillReturnsTheResult(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Made"}); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.AddLink(ctx, "p1", "PLAT-1", backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "XT-9"}); err != nil {
		t.Fatal(err)
	}
	eng := committer.New(journalLostOnLink{fake: h.jira, db: h.db}, h.repo, h.order)

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("a Commit that wrote to Jira returned an error: %v", err)
	}
	if len(res.Created) != 1 || len(h.jira.links) != 1 {
		t.Errorf("created %+v, links %v", res.Created, h.jira.links)
	}
	lost := false
	for _, f := range res.Failures {
		if strings.Contains(f.Error, "did not run") {
			t.Errorf("no phase follows the last one: %+v", f)
		}
		lost = lost || strings.Contains(f.Error, "linked in Jira but the journal could not be cleared")
	}
	if !lost {
		t.Errorf("the journal was not lost, so this proves nothing: %+v", res.Failures)
	}
	if res.Remaining != 1 {
		t.Errorf("remaining = %d, want the link row the last successful read counted", res.Remaining)
	}
}
