package jira_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
	jirabackend "agile-suite/tam/internal/backend/jira"
)

// fakeJira answers the four endpoints the backend touches and records the
// search requests it saw.
type fakeJira struct {
	fieldCalls int32
	searches   []string
	fields     string   // the /rest/api/2/field body
	writes     []string // "METHOD path body" for every PUT and POST
	createKey  string   // key the POST /issue answers with
	createFail bool     // POST /issue answers 400
	linkTypes  string   // the /rest/api/2/issueLinkType body
}

func (f *fakeJira) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/myself":
			_, _ = w.Write([]byte(`{"name":"jdoe","displayName":"J. Doe"}`))
		case r.URL.Path == "/rest/api/2/field":
			atomic.AddInt32(&f.fieldCalls, 1)
			_, _ = w.Write([]byte(f.fields))
		case r.URL.Path == "/rest/api/2/search":
			f.searches = append(f.searches, r.URL.Query().Get("jql")+" | fields="+r.URL.Query().Get("fields"))
			_, _ = w.Write([]byte(`{"total":2,"issues":[
				{"id":"1","key":"PLAT-412","fields":{"summary":"Promo","status":{"name":"In Progress"},"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[],"customfield_10020":[{"id":12,"name":"Sprint 12"}],"customfield_10016":5}},
				{"id":"2","key":"PLAT-388","fields":{"summary":"Single use","status":{"name":"Approved"},"issuetype":{"name":"Business Requirement"},"project":{"key":"PLAT"},"labels":["promo"]}}
			]}`))
		case r.URL.Path == "/rest/api/2/issueLinkType" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(f.linkTypes))
		case r.Method == http.MethodPut || r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			f.writes = append(f.writes, r.Method+" "+r.URL.Path+" "+string(body))
			if r.URL.Path == "/rest/api/2/issue" {
				if f.createFail {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"errorMessages":[],"errors":{"customfield_10050":"Severity is required."}}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"9001","key":"` + f.createKey + `"}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			f.searches = append(f.searches, "createmeta "+r.URL.RawQuery)
			if r.URL.Query().Get("issuetypeNames") == "Epic" {
				_, _ = w.Write([]byte(`{"projects":[{"key":"PLAT","issuetypes":[{"name":"Epic","fields":{
					"summary":{"required":true,"name":"Summary","schema":{"type":"string"}},
					"project":{"required":true,"name":"Project","schema":{"type":"project"}},
					"issuetype":{"required":true,"name":"Issue Type","schema":{"type":"issuetype"}},
					"customfield_10011":{"required":true,"name":"Epic Name","schema":{"type":"string"}}
				}}]}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"projects":[{"key":"PLAT","issuetypes":[{"name":"Bug","fields":{
				"summary":{"required":true,"name":"Summary","schema":{"type":"string"}},
				"project":{"required":true,"name":"Project","schema":{"type":"project"}},
				"issuetype":{"required":true,"name":"Issue Type","schema":{"type":"issuetype"}},
				"customfield_10016":{"required":true,"name":"Story Points","schema":{"type":"number"}},
				"customfield_10050":{"required":true,"name":"Severity","schema":{"type":"option"},"allowedValues":[{"id":"1","value":"Minor"},{"id":"3","value":"Critical"}]},
				"components":{"required":true,"name":"Component/s","schema":{"type":"array","items":"component"},"allowedValues":[{"id":"100","name":"Checkout"},{"id":"101","name":"Payments"}]},
				"customfield_10070":{"required":true,"name":"Release Note","schema":{"type":"option"}},
				"customfield_10071":{"required":true,"name":"Keywords","schema":{"type":"array","items":"string"}},
				"environment":{"required":false,"name":"Environment","schema":{"type":"string"}}
			}}]}]}`))
		case r.URL.Path == "/rest/api/2/issue/PLAT-412/transitions":
			f.searches = append(f.searches, "transitions "+r.URL.RawQuery)
			_, _ = w.Write([]byte(transitionsBody))
		case strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/PLAT-412"):
			_, _ = w.Write([]byte(`{"id":"1","key":"PLAT-412","fields":{"description":"As a shopper","issuelinks":[{"type":{"name":"Tested By"},"inwardIssue":{"key":"XT-1018","fields":{"summary":"Applies discount","issuetype":{"name":"Test"}}}}],"customfield_10016":5}}`))
		case r.URL.Path == "/rest/api/2/user/assignable/search":
			f.searches = append(f.searches, "users "+r.URL.RawQuery)
			_, _ = w.Write([]byte(`[
				{"name":"ranand","displayName":"R. Anand","active":true},
				{"name":"gone","displayName":"Left The Company","active":false},
				{"name":"nodisplay","displayName":"","active":true}
			]`))
		case r.URL.Path == "/rest/api/2/priority":
			_, _ = w.Write([]byte(`[{"name":"Highest"},{"name":"High"},{"name":""},{"name":"Low"}]`))
		case r.URL.Path == "/rest/api/2/project/TODOP":
			// A project whose task level is called "Todo", the shape seen on
			// a real instance.
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"10000","name":"Todo"},{"id":"18","name":"Story"},{"id":"19","name":"Technical task","subtask":true}]}`))
		case strings.HasPrefix(r.URL.Path, "/rest/agile/1.0/"):
			f.agile(w, r)
		case r.URL.Path == "/rest/api/2/project/PLAT":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"1","name":"Task"},{"id":"7","name":"Business Requirement"},{"id":"19","name":"Technical task","subtask":true}]}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// agile answers the Agile 1.0 paths the boards backend reads: one project
// with a scrum board, a kanban board, and a board type TAM does not draw.
func (f *fakeJira) agile(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/rest/agile/1.0/board":
		_, _ = w.Write([]byte(`{"isLast":true,"values":[
			{"id":1,"name":"PLAT Scrum","type":"scrum"},
			{"id":2,"name":"PLAT Kanban","type":"kanban"},
			{"id":3,"name":"PLAT Plans","type":"simple"}
		]}`))
	case "/rest/agile/1.0/board/1/configuration":
		_, _ = w.Write([]byte(`{"id":1,"columnConfig":{"columns":[
			{"name":"Backlog","statuses":[]},
			{"name":"To Do","statuses":[{"id":"1"}]},
			{"name":"In Progress","statuses":[{"id":"3"},{"id":"4"}]},
			{"name":"Done","statuses":[{"id":"5"}]}
		]}}`))
	case "/rest/agile/1.0/board/1/sprint":
		// originBoardId is board 7, not the board being read: the mapping
		// must take the board from the request, not from the payload.
		_, _ = w.Write([]byte(`{"isLast":true,"values":[
			{"id":11,"name":"Sprint 11","state":"closed","originBoardId":7,"startDate":"2026-08-04T09:00:00.000Z","endDate":"2026-08-18T09:00:00.000Z","goal":"Ship the promo code flow"},
			{"id":12,"name":"Sprint 12","state":"active","originBoardId":7,"startDate":"2026-08-18T09:00:00.000Z","endDate":"2026-09-01T09:00:00.000Z","goal":""}
		]}`))
	case "/rest/agile/1.0/board/2/sprint":
		// Jira's way of saying a kanban board has no sprints.
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["The board does not support sprints"]}`))
	case "/rest/agile/1.0/board/1/issue":
		_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":3,"issues":[
			{"key":"PLAT-412"},{"key":"PLAT-409"},{"key":"OPS-7"}
		]}`))
	case "/rest/agile/1.0/board/1/sprint/12/issue":
		_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":1,"issues":[{"key":"PLAT-412"}]}`))
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["no such board"]}`))
	}
}

func newBackend(t *testing.T, fields string) (*jirabackend.Backend, *fakeJira) {
	t.Helper()
	f := &fakeJira{fields: fields, linkTypes: `{"issueLinkTypes":[{"id":"10000","name":"Blocks","inward":"is blocked by","outward":"blocks"},{"id":"10003","name":"Relates","inward":"relates to","outward":"relates to"}]}`}
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	c := corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	return jirabackend.New(c, "Business Requirement"), f
}

// transitionsBody is what a Data Center workflow answers with: two
// transitions reaching Done, the lower id of them carrying the resolution
// screen almost every such workflow puts there, and one guarded by a custom
// field nobody can fill in from here.
const transitionsBody = `{"transitions":[
	{"id":"11","name":"Back to To Do","to":{"id":"1","name":"To Do"},"fields":{}},
	{"id":"31","name":"Done","to":{"id":"5","name":"Done"},"fields":{
		"resolution":{"name":"Resolution","required":true,"allowedValues":[{"id":"10000","name":"Done"},{"id":"10001","name":"Won't Do"}]}
	}},
	{"id":"21","name":"Start Progress","to":{"id":"3","name":"In Progress"},"fields":{
		"assignee":{"name":"Assignee","required":false}
	}},
	{"id":"41","name":"Close","to":{"id":"5","name":"Done"},"fields":{}},
	{"id":"51","name":"Sign off","to":{"id":"6","name":"Signed off"},"fields":{
		"customfield_11400":{"name":"Sign-off","required":true}
	}}
]}`

const twoFields = `[{"id":"customfield_10020","name":"Sprint","custom":true},{"id":"customfield_10016","name":"Story Points","custom":true}]`

const threeFields = `[{"id":"customfield_10020","name":"Sprint","custom":true},{"id":"customfield_10016","name":"Story Points","custom":true},{"id":"customfield_10014","name":"Epic Link","custom":true}]`

const fourFields = `[{"id":"customfield_10020","name":"Sprint","custom":true},{"id":"customfield_10016","name":"Story Points","custom":true},{"id":"customfield_10014","name":"Epic Link","custom":true},{"id":"customfield_10011","name":"Epic Name","custom":true}]`

func TestSearchBuildsTheScopeAndMapsDiscoveredFields(t *testing.T) {
	b, f := newBackend(t, twoFields)
	ctx := context.Background()
	if b.IsDemo() {
		t.Fatal("IsDemo = true")
	}
	u, err := b.TestConnection(ctx)
	if err != nil || u.DisplayName != "J. Doe" {
		t.Fatalf("connection = %+v, %v", u, err)
	}
	page, total, err := b.SearchIssuesPage(ctx, "PLAT", "labels = promo", "2026-09-05T10:42:00Z", backend.AllTypes, 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 2 || len(page) != 2 {
		t.Fatalf("total %d rows %d", total, len(page))
	}
	// The sub-task type is in the scope under the name the project gives it,
	// which is where "Technical task" comes from.
	wantJQL := `project = "PLAT" AND issuetype in ("Task", "Epic", "Story", "Bug", "Business Requirement", "Technical task") AND (labels = promo) AND updated >= "2026-09-05 09:42" ORDER BY key ASC`
	if len(f.searches) != 1 || !strings.HasPrefix(f.searches[0], wantJQL+" | fields=") {
		t.Errorf("search request = %q", f.searches)
	}
	if !strings.Contains(f.searches[0], "customfield_10020") || !strings.Contains(f.searches[0], "customfield_10016") {
		t.Errorf("discovered ids not requested: %q", f.searches[0])
	}
	if page[0].SprintID != "12" || page[0].StoryPoints == nil || *page[0].StoryPoints != 5 || page[0].Type != "story" {
		t.Errorf("row 0 = %+v", page[0])
	}
	if page[1].Type != "requirement" {
		t.Errorf("row 1 type = %q, want requirement (the profile's name)", page[1].Type)
	}
	if _, _, err := b.SearchIssuesPage(ctx, "PLAT", "", "", backend.AllTypes, 0, 50); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&f.fieldCalls); n != 1 {
		t.Errorf("/rest/api/2/field fetched %d times across two searches, want 1", n)
	}
}

func TestMissingCustomFieldsLeaveColumnsEmpty(t *testing.T) {
	b, _ := newBackend(t, `[{"id":"summary","name":"Summary","custom":false}]`)
	page, _, err := b.SearchIssuesPage(context.Background(), "PLAT", "", "", backend.AllTypes, 0, 50)
	if err != nil {
		t.Fatalf("search with no custom fields must not fail: %v", err)
	}
	if page[0].SprintID != "" || page[0].StoryPoints != nil || page[0].Rank != "" {
		t.Errorf("row = %+v, want empty custom columns", page[0])
	}
}

func TestGetIssueDetailAndIssueTypes(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	ctx := context.Background()
	d, err := b.GetIssueDetail(ctx, "PLAT-412")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.Key != "PLAT-412" || d.Description != "As a shopper" || len(d.Links) != 1 || d.Links[0].Key != "XT-1018" {
		t.Errorf("detail = %+v", d)
	}
	if d.Fields["customfield_10016"] != 5.0 {
		t.Errorf("custom fields = %+v", d.Fields)
	}
	types, err := b.IssueTypes(ctx, "PLAT")
	if err != nil || len(types) != 3 || types[1].Name != "Business Requirement" {
		t.Errorf("types = %+v, %v", types, err)
	}
}

func TestBoardsDropTheTypesTamCannotDraw(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	boards, err := b.Boards(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("boards: %v", err)
	}
	if len(boards) != 2 {
		t.Fatalf("boards = %+v, want the scrum and the kanban one only", boards)
	}
	if boards[0].ID != 1 || boards[0].Name != "PLAT Scrum" || boards[0].Type != backend.BoardTypeScrum {
		t.Errorf("board 0 = %+v", boards[0])
	}
	if boards[1].ID != 2 || boards[1].Type != backend.BoardTypeKanban {
		t.Errorf("board 1 = %+v", boards[1])
	}
}

func TestBoardColumnsFlattenTheStatusObjects(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	cols, err := b.BoardColumns(context.Background(), 1)
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	if len(cols) != 4 {
		t.Fatalf("columns = %+v, want 4", cols)
	}
	if cols[0].Name != "Backlog" || len(cols[0].StatusIDs) != 0 {
		t.Errorf("a column with no statuses keeps an empty list: %+v", cols[0])
	}
	if cols[2].Name != "In Progress" || len(cols[2].StatusIDs) != 2 || cols[2].StatusIDs[0] != "3" || cols[2].StatusIDs[1] != "4" {
		t.Errorf("column 2 = %+v", cols[2])
	}
}

func TestBoardSprintsTakeTheBoardFromTheRequestAndKanbanHasNone(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	ctx := context.Background()
	sprints, err := b.BoardSprints(ctx, 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	if len(sprints) != 2 {
		t.Fatalf("sprints = %+v, want 2", sprints)
	}
	for _, s := range sprints {
		if s.BoardID != 1 {
			t.Errorf("sprint %d filed under board %d, want the board that was read (originBoardId is 7)", s.ID, s.BoardID)
		}
	}
	if sprints[0].State != "closed" || sprints[1].State != "active" || sprints[1].Name != "Sprint 12" {
		t.Errorf("sprints = %+v", sprints)
	}
	if sprints[1].StartDate == "" || sprints[1].EndDate == "" {
		t.Errorf("sprint dates dropped: %+v", sprints[1])
	}
	none, err := b.BoardSprints(ctx, 2)
	if err != nil {
		t.Fatalf("a kanban board with no sprints must not fail: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("kanban sprints = %+v, want none", none)
	}
}

// TestBoardSprintsCarryTheGoalJiraSent used to be a couple of assertions
// bolted onto TestBoardSprintsTakeTheBoardFromTheRequestAndKanbanHasNone,
// describing a second behaviour under a name that only promised the first.
// It also covers the fixture's empty goal on sprint 12, which nothing used
// to check.
func TestBoardSprintsCarryTheGoalJiraSent(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	sprints, err := b.BoardSprints(context.Background(), 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	if len(sprints) != 2 {
		t.Fatalf("sprints = %+v, want 2", sprints)
	}
	if sprints[0].Goal != "Ship the promo code flow" {
		t.Errorf("sprint goal dropped: %+v", sprints[0])
	}
	if sprints[1].Goal != "" {
		t.Errorf("sprint goal = %q, want empty for Jira's own empty goal", sprints[1].Goal)
	}
}

func TestBoardIssueKeysPassThroughForTheBoardAndForOneSprint(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	ctx := context.Background()
	keys, err := b.BoardIssueKeys(ctx, 1, "", "PLAT")
	if err != nil {
		t.Fatalf("board keys: %v", err)
	}
	if len(keys) != 3 || keys[0] != "PLAT-412" || keys[2] != "OPS-7" {
		t.Errorf("board keys = %v, want the board's own order including the key outside the project", keys)
	}
	keys, err = b.BoardIssueKeys(ctx, 1, "12", "PLAT")
	if err != nil {
		t.Fatalf("sprint keys: %v", err)
	}
	if len(keys) != 1 || keys[0] != "PLAT-412" {
		t.Errorf("sprint keys = %v", keys)
	}
}

func TestTransitionTakesTheLowestIdThatReachesTheTargetAndFillsTheResolution(t *testing.T) {
	b, f := newBackend(t, twoFields)
	if err := b.Transition(context.Background(), "PLAT-412", []string{"5"}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	want := `POST /rest/api/2/issue/PLAT-412/transitions {"transition":{"id":"31"},"fields":{"resolution":{"id":"10000"}}}`
	if len(f.writes) != 1 || f.writes[0] != want {
		t.Errorf("wrote %v, want %s", f.writes, want)
	}
	if len(f.searches) == 0 || !strings.Contains(f.searches[0], "expand=transitions.fields") {
		t.Errorf("the transitions are read with their fields: %v", f.searches)
	}
}

func TestTransitionTakesTheProfilesResolutionWhenTheTransitionAllowsIt(t *testing.T) {
	b, f := newBackend(t, twoFields)
	b.SetTransitionResolution("won't do")
	if err := b.Transition(context.Background(), "PLAT-412", []string{"5"}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if len(f.writes) != 1 || !strings.Contains(f.writes[0], `"resolution":{"id":"10001"}`) {
		t.Errorf("wrote %v", f.writes)
	}

	// A setting the transition does not offer falls back to the first
	// allowed value rather than sending a value Jira will reject.
	b2, f2 := newBackend(t, twoFields)
	b2.SetTransitionResolution("Abandoned")
	if err := b2.Transition(context.Background(), "PLAT-412", []string{"5"}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if len(f2.writes) != 1 || !strings.Contains(f2.writes[0], `"resolution":{"id":"10000"}`) {
		t.Errorf("wrote %v", f2.writes)
	}
}

func TestATransitionThatNeedsNothingSendsNoFieldsAtAll(t *testing.T) {
	b, f := newBackend(t, twoFields)
	if err := b.Transition(context.Background(), "PLAT-412", []string{"3"}); err != nil {
		t.Fatalf("transition: %v", err)
	}
	want := `POST /rest/api/2/issue/PLAT-412/transitions {"transition":{"id":"21"}}`
	if len(f.writes) != 1 || f.writes[0] != want {
		t.Errorf("wrote %v, want %s", f.writes, want)
	}
}

func TestATransitionThatNeedsAnotherFieldIsRefusedByName(t *testing.T) {
	b, f := newBackend(t, twoFields)
	err := b.Transition(context.Background(), "PLAT-412", []string{"6"})
	if err == nil {
		t.Fatal("a transition asking for a custom field must be refused, not guessed at")
	}
	if !errors.Is(err, backend.ErrTransitionFields) {
		t.Errorf("error kind: %v", err)
	}
	for _, want := range []string{"PLAT-412", "status 6", "Sign-off (customfield_11400)", "in Jira"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if len(f.writes) != 0 {
		t.Errorf("nothing was pushed: %v", f.writes)
	}
}

func TestATransitionWithNoPathNamesTheIssueTheTargetAndWhatIsReachable(t *testing.T) {
	b, f := newBackend(t, twoFields)
	err := b.Transition(context.Background(), "PLAT-412", []string{"9"})
	if !errors.Is(err, backend.ErrNoTransition) {
		t.Fatalf("error kind: %v", err)
	}
	var noPath *backend.NoTransition
	if !errors.As(err, &noPath) {
		t.Fatalf("the error carries the reachable statuses: %v", err)
	}
	if strings.Join(noPath.Reachable, ",") != "To Do,Done,In Progress,Signed off" {
		t.Errorf("reachable: %v", noPath.Reachable)
	}
	for _, want := range []string{"PLAT-412", "status 9", "In Progress"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if len(f.writes) != 0 {
		t.Errorf("nothing was pushed: %v", f.writes)
	}
}

func TestCanTransitionAnswersWithoutWriting(t *testing.T) {
	b, f := newBackend(t, twoFields)
	ctx := context.Background()
	check, err := b.CanTransition(ctx, "PLAT-412", []string{"5"})
	if err != nil || !check.Allowed {
		t.Fatalf("check = %+v, %v", check, err)
	}
	check, err = b.CanTransition(ctx, "PLAT-412", []string{"9"})
	if err != nil || check.Allowed {
		t.Fatalf("check = %+v, %v", check, err)
	}
	if strings.Join(check.Reachable, ",") != "To Do,Done,In Progress,Signed off" {
		t.Errorf("reachable: %v", check.Reachable)
	}
	if len(f.writes) != 0 {
		t.Errorf("a check never writes: %v", f.writes)
	}
}

func TestTheBoardWritesPassThroughToTheAgileEndpoints(t *testing.T) {
	b, f := newBackend(t, twoFields)
	ctx := context.Background()
	if err := b.RankIssue(ctx, "PLAT-412", "PLAT-409", true); err != nil {
		t.Fatalf("rank: %v", err)
	}
	if err := b.MoveIssuesToSprint(ctx, "12", []string{"PLAT-412", "PLAT-409"}); err != nil {
		t.Fatalf("sprint: %v", err)
	}
	if err := b.MoveIssuesToSprint(ctx, "", []string{"PLAT-412"}); err != nil {
		t.Fatalf("backlog: %v", err)
	}
	if err := b.MoveIssuesToSprint(ctx, "13", nil); err != nil {
		t.Fatalf("empty batch: %v", err)
	}
	want := []string{
		`PUT /rest/agile/1.0/issue/rank {"issues":["PLAT-412"],"rankBeforeIssue":"PLAT-409"}`,
		`POST /rest/agile/1.0/sprint/12/issue {"issues":["PLAT-412","PLAT-409"]}`,
		`POST /rest/agile/1.0/backlog/issue {"issues":["PLAT-412"]}`,
	}
	if strings.Join(f.writes, " | ") != strings.Join(want, " | ") {
		t.Errorf("wrote %v", f.writes)
	}
}

func TestStartAndCompleteSprintPassThroughToTheAgileEndpoint(t *testing.T) {
	b, f := newBackend(t, twoFields)
	ctx := context.Background()
	draft := backend.SprintDraft{Name: "Sprint 12", Goal: "Ship the thing", StartDate: "2026-09-09T09:00:00.000+0000", EndDate: "2026-09-23T09:00:00.000+0000"}
	if err := b.StartSprint(ctx, 12, draft); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := b.CompleteSprint(ctx, 12); err != nil {
		t.Fatalf("complete: %v", err)
	}
	want := []string{
		`POST /rest/agile/1.0/sprint/12 {"endDate":"2026-09-23T09:00:00.000+0000","goal":"Ship the thing","name":"Sprint 12","startDate":"2026-09-09T09:00:00.000+0000","state":"active"}`,
		`POST /rest/agile/1.0/sprint/12 {"state":"closed"}`,
	}
	if strings.Join(f.writes, " | ") != strings.Join(want, " | ") {
		t.Errorf("wrote %v", f.writes)
	}
}
