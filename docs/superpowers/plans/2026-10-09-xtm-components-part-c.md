# XTM components, part C: manage Jira project components — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Components view in XTM that lists the project's Jira components and creates, edits and deletes them with live Jira calls, keeping the local cache in step.

**Architecture:** New Jira client calls in `xtm/internal/jira/components.go`, reached through an optional `backend.ComponentManager` interface that only the Xray adapter implements. A small `internal/componentadmin` service does write, then re-fetch, then cache refresh, and refuses renames and deletes that would strand pending edits. `app.go` adapts it to Wails. The frontend adds a `ComponentsView` with a form modal and a delete modal, gated by a new `supportsComponentAdmin` capability.

**Tech Stack:** Go (net/http, SQLite through `internal/store`), Wails v2, React and TypeScript, TanStack Query, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-09-xtm-labels-components-design.md`, part C. The spec file lives on the part A branch (PR #160). Read it there: `git show origin/feat/xtm-labels-part-a:docs/superpowers/specs/2026-10-09-xtm-labels-components-design.md`.

**Issue:** #157.

## Deviations from the spec

These follow from the code and are decided here; the spec gets the matching edit once part A has merged.

1. **Demo state is package-level, not on the client.** `app.go` builds a new `jira.Client` on every call (`newBackend`, `backendFor`), so a map on the client would forget every demo write. A mutex-guarded package-level store in `internal/jira` keeps demo creates, renames and deletes for the life of the process.
2. **An optional interface, not new `Backend` methods.** The backend package already keeps optional features off `Backend` (`PreconditionStreamer`, `BugKeyReader`) and callers type-assert. `ComponentManager` follows that, so Kiwi and the test fakes need no stubs. The capability `SupportsComponentAdmin` is set by the Xray adapter and false for Kiwi.
3. **Every component write reports the HTTP code.** Core's `WriteError` has no numeric code and its DELETE error is untyped, so the component calls build their own `*jira.HTTPError` from `WriteJSONRaw` (POST, PUT) and from a hand-built DELETE. That is what makes a 403 detectable.
4. **The pending-edit guard looks at test `components` edits only**, as the spec says. Pending test creations (`test_create`) that name a component are not checked; renaming such a component makes that creation fail at commit with Jira's own error, which the pending list already shows.

## Global Constraints

- Run Go commands from `xtm/`; run npm commands from the repo root.
- Never hand-edit `xtm/frontend/wailsjs`; regenerate with `wails generate module` from `xtm/`, and commit only the files whose content changed (`git diff --ignore-cr-at-eol`).
- Backend logic lives in `internal/`; `app.go` only adapts it.
- No `console` calls; no em dash in a user-visible string; no hex colors or px font sizes in new CSS.
- Any file that sets a `modal` class imports `Modal` from `@agile-suite/core` (the `bespoke_modals` ratchet).
- Every class a component sets is defined in a stylesheet.
- New files stay under 400 lines.
- Test assertions check values and visible text (C6).
- The 403 text is exactly: `You need project admin rights in Jira to change components.`
- Commits reference #157 and carry no AI attribution.

## Review Focus

1. **Renaming a component to a name that differs only in case** (`core` to `Core`). Jira allows it. The local rewrite must replace the old name, not leave both. Task 4 pins this.
2. **Deleting a component while a test pending creation or an unrelated pending edit exists.** Only `components` edits on tests carrying that component block it. Task 4 pins the unrelated case.
3. **A Jira 404 on update or delete** (deleted elsewhere since the list loaded). The view must show Jira's error and reload the list, not leave the stale row. Task 7 pins the reload.
4. **A blank or whitespace-only name, or a duplicate name in the form.** The form refuses before calling the backend. Task 7 pins this.
5. **Two quick deletes in demo mode.** The package-level demo store must not race; it is mutex-guarded and Task 2 runs the demo test under `-race` where available.

---

### Task 1: Jira client component calls

**Files:**
- Create: `xtm/internal/jira/components.go`
- Create: `xtm/internal/jira/components_test.go`

**Interfaces:**
- Produces, in package `jira`:

```go
type Component struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	LeadName        string `json:"leadName"`
	LeadDisplayName string `json:"leadDisplayName"`
	AssigneeType    string `json:"assigneeType"`
}
type ComponentInput struct {
	Project      string
	Name         string
	Description  string
	LeadUserName string
	AssigneeType string
}
type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}
func (c *Client) ProjectComponentDetails(ctx context.Context, projectKey string) ([]Component, error)
func (c *Client) CreateComponent(ctx context.Context, in ComponentInput) (Component, error)
func (c *Client) UpdateComponent(ctx context.Context, id string, in ComponentInput) (Component, error)
func (c *Client) DeleteComponent(ctx context.Context, id, moveIssuesTo string) error
func (c *Client) ComponentIssueCount(ctx context.Context, id string) (int, error)
func (c *Client) SearchUsers(ctx context.Context, query string) ([]User, error)
```

All write errors are `*HTTPError` with `Code` set.

- [ ] **Step 1: Write the failing tests**

`components_test.go`:

```go
package jira

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestProjectComponentDetailsDecodesLead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/project/QA/components" {
			t.Errorf("path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":"10","name":"Core","description":"d",
			"lead":{"name":"alice","displayName":"Alice A"},"assigneeType":"COMPONENT_LEAD"}]`)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).ProjectComponentDetails(context.Background(), "QA")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []Component{{ID: "10", Name: "Core", Description: "d", LeadName: "alice",
		LeadDisplayName: "Alice A", AssigneeType: "COMPONENT_LEAD"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestCreateComponentSendsBody(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/component" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"11","name":"API","assigneeType":"PROJECT_DEFAULT"}`)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).CreateComponent(context.Background(), ComponentInput{
		Project: "QA", Name: "API", Description: "rest", LeadUserName: "bob", AssigneeType: "PROJECT_DEFAULT",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ID != "11" || got.Name != "API" {
		t.Fatalf("got %+v", got)
	}
	want := map[string]any{"project": "QA", "name": "API", "description": "rest",
		"leadUserName": "bob", "assigneeType": "PROJECT_DEFAULT"}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body %v", body)
	}
}

func TestUpdateComponentOmitsProjectAndClearsLead(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/2/component/11" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"id":"11","name":"Api"}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).UpdateComponent(context.Background(), "11",
		ComponentInput{Project: "QA", Name: "Api", AssigneeType: "UNASSIGNED"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := map[string]any{"name": "Api", "description": "", "leadUserName": "", "assigneeType": "UNASSIGNED"}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body %v", body)
	}
}

func TestDeleteComponentMovesIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/rest/api/2/component/11" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("moveIssuesTo"); got != "12" {
			t.Errorf("moveIssuesTo %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).DeleteComponent(context.Background(), "11", "12"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDeleteComponentWithoutMoveSendsNoQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("query %q", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).DeleteComponent(context.Background(), "11", ""); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestComponentWritesReport403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"errorMessages":["You do not have permission"],"errors":{}}`)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	ctx := context.Background()

	_, errCreate := c.CreateComponent(ctx, ComponentInput{Project: "QA", Name: "X"})
	_, errUpdate := c.UpdateComponent(ctx, "1", ComponentInput{Name: "X"})
	errDelete := c.DeleteComponent(ctx, "1", "")
	for name, err := range map[string]error{"create": errCreate, "update": errUpdate, "delete": errDelete} {
		var he *HTTPError
		if !errors.As(err, &he) || he.Code != http.StatusForbidden {
			t.Errorf("%s: want *HTTPError 403, got %v", name, err)
			continue
		}
		if he.Message != "You do not have permission" {
			t.Errorf("%s: message %q", name, he.Message)
		}
	}
}

func TestComponentIssueCountAndUserSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/component/11/relatedIssueCounts":
			_, _ = io.WriteString(w, `{"issueCount":7}`)
		case "/rest/api/2/user/search":
			if r.URL.Query().Get("username") != "al" {
				t.Errorf("username %q", r.URL.Query().Get("username"))
			}
			_, _ = io.WriteString(w, `[{"name":"alice","displayName":"Alice A"}]`)
		default:
			t.Errorf("path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := newTestClient(srv)

	n, err := c.ComponentIssueCount(context.Background(), "11")
	if err != nil || n != 7 {
		t.Fatalf("count %d err %v", n, err)
	}
	users, err := c.SearchUsers(context.Background(), "al")
	if err != nil || !reflect.DeepEqual(users, []User{{Name: "alice", DisplayName: "Alice A"}}) {
		t.Fatalf("users %+v err %v", users, err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/jira/ -run 'Component|UserSearch' -count=1`
Expected: build failure, `ProjectComponentDetails` and the rest undefined.

- [ ] **Step 3: Write components.go (live paths only; Task 2 adds demo branches)**

```go
package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Component is one Jira project component as the Components view shows it.
type Component struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	LeadName        string `json:"leadName"`
	LeadDisplayName string `json:"leadDisplayName"`
	AssigneeType    string `json:"assigneeType"`
}

// ComponentInput is what create and update send. Project is used on create
// only; Jira does not move a component between projects.
type ComponentInput struct {
	Project      string
	Name         string
	Description  string
	LeadUserName string
	AssigneeType string
}

// User is one result of a user search, for picking a component lead.
type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// componentWire is Jira's component JSON. The lead arrives nested.
type componentWire struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	AssigneeType string `json:"assigneeType"`
	Lead         *struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"lead"`
}

func (w componentWire) component() Component {
	out := Component{ID: w.ID, Name: w.Name, Description: w.Description, AssigneeType: w.AssigneeType}
	if w.Lead != nil {
		out.LeadName, out.LeadDisplayName = w.Lead.Name, w.Lead.DisplayName
	}
	return out
}

// ProjectComponentDetails lists a project's components with their ids, which
// the write calls need, and their description, lead and assignee type.
func (c *Client) ProjectComponentDetails(ctx context.Context, projectKey string) ([]Component, error) {
	var items []componentWire
	path := fmt.Sprintf("/rest/api/2/project/%s/components", url.PathEscape(projectKey))
	if err := c.get(ctx, path, &items); err != nil {
		return nil, fmt.Errorf("project components %s: %w", projectKey, err)
	}
	out := make([]Component, 0, len(items))
	for _, it := range items {
		out = append(out, it.component())
	}
	return out, nil
}

// CreateComponent creates a component in in.Project.
func (c *Client) CreateComponent(ctx context.Context, in ComponentInput) (Component, error) {
	body := map[string]string{
		"project": in.Project, "name": in.Name, "description": in.Description,
		"leadUserName": in.LeadUserName, "assigneeType": in.AssigneeType,
	}
	var out componentWire
	if err := c.componentWrite(ctx, http.MethodPost, "/rest/api/2/component", body, &out); err != nil {
		return Component{}, fmt.Errorf("create component %q: %w", in.Name, err)
	}
	return out.component(), nil
}

// UpdateComponent replaces a component's name, description, lead and assignee
// type. An empty LeadUserName clears the lead.
func (c *Client) UpdateComponent(ctx context.Context, id string, in ComponentInput) (Component, error) {
	body := map[string]string{
		"name": in.Name, "description": in.Description,
		"leadUserName": in.LeadUserName, "assigneeType": in.AssigneeType,
	}
	var out componentWire
	path := "/rest/api/2/component/" + url.PathEscape(id)
	if err := c.componentWrite(ctx, http.MethodPut, path, body, &out); err != nil {
		return Component{}, fmt.Errorf("update component %s: %w", id, err)
	}
	return out.component(), nil
}

// DeleteComponent deletes a component. With moveIssuesTo set, Jira first moves
// the component's issues to that component.
func (c *Client) DeleteComponent(ctx context.Context, id, moveIssuesTo string) error {
	path := "/rest/api/2/component/" + url.PathEscape(id)
	if moveIssuesTo != "" {
		path += "?moveIssuesTo=" + url.QueryEscape(moveIssuesTo)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("delete component %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("delete component %s: %w", id, componentHTTPError(http.MethodDelete, path, resp.StatusCode, resp.Status, b))
	}
	return nil
}

// ComponentIssueCount is how many issues (of any type) carry the component,
// shown before a delete.
func (c *Client) ComponentIssueCount(ctx context.Context, id string) (int, error) {
	var out struct {
		IssueCount int `json:"issueCount"`
	}
	path := "/rest/api/2/component/" + url.PathEscape(id) + "/relatedIssueCounts"
	if err := c.get(ctx, path, &out); err != nil {
		return 0, fmt.Errorf("component issue count %s: %w", id, err)
	}
	return out.IssueCount, nil
}

// SearchUsers finds users by username, name or email for the lead picker.
func (c *Client) SearchUsers(ctx context.Context, query string) ([]User, error) {
	var out []User
	path := "/rest/api/2/user/search?maxResults=10&username=" + url.QueryEscape(query)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	return out, nil
}

// componentWrite sends a JSON write and turns any non-2xx answer into an
// *HTTPError with the numeric code, which core's WriteError lacks.
func (c *Client) componentWrite(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.WriteJSONRaw(ctx, method, path, body)
	if err != nil {
		return err
	}
	if resp.Code >= 300 {
		return componentHTTPError(method, path, resp.Code, resp.Status, resp.Body)
	}
	if out == nil || len(strings.TrimSpace(string(resp.Body))) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}

// componentHTTPError builds an *HTTPError, reading Jira's errorMessages and
// errors fields into Message.
func componentHTTPError(method, path string, code int, status string, body []byte) *HTTPError {
	var parsed struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	_ = json.Unmarshal(body, &parsed)
	msgs := append([]string{}, parsed.ErrorMessages...)
	for field, m := range parsed.Errors {
		msgs = append(msgs, field+": "+m)
	}
	return &HTTPError{Method: method, Path: path, Code: code, Status: status, Message: strings.Join(msgs, "; ")}
}
```

Check `c.baseURL` and `c.token` are the field names on `xtm/internal/jira.Client` (`client.go`); the report on the client says it keeps copies under those names.

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./internal/jira/ -run 'Component|UserSearch' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add xtm/internal/jira/components.go xtm/internal/jira/components_test.go
git commit -m "feat(xtm): Jira client calls for project components (#157)"
```

---

### Task 2: Stateful demo components and one demo component list

**Files:**
- Create: `xtm/internal/jira/demo_components.go`
- Create: `xtm/internal/jira/demo_components_test.go`
- Modify: `xtm/internal/jira/components.go` (demo branches at the top of each method)
- Modify: `xtm/internal/jira/projectfields.go` (`demoComponentList`, `ProjectComponents` demo branch)
- Modify: `xtm/internal/jira/demo.go` (`demoComponentNames`)
- Modify: `xtm/internal/jira/customfields.go` (`demoComponents`)

**Interfaces:**
- Consumes: Task 1 types and methods.
- Produces: demo behaviour for all six methods and for `ProjectComponents`; `resetDemoComponents()` for tests.

- [ ] **Step 1: Write the failing demo test**

`demo_components_test.go`:

```go
package jira

import (
	"context"
	"sync"
	"testing"
)

func demoNames(t *testing.T, c *Client) []string {
	t.Helper()
	list, err := c.ProjectComponents(context.Background(), "DEMO")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return list
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestDemoComponentsCreateRenameDelete(t *testing.T) {
	resetDemoComponents()
	t.Cleanup(resetDemoComponents)
	ctx := context.Background()

	// A fresh client per call, as app.go does, must see earlier writes.
	created, err := NewClient("demo", "t").CreateComponent(ctx, ComponentInput{Project: "DEMO", Name: "Search"})
	if err != nil || created.ID == "" {
		t.Fatalf("create %+v %v", created, err)
	}
	if !contains(demoNames(t, NewClient("demo", "t")), "Search") {
		t.Fatal("created component not listed")
	}

	if _, err := NewClient("demo", "t").UpdateComponent(ctx, created.ID, ComponentInput{Name: "Find"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	names := demoNames(t, NewClient("demo", "t"))
	if contains(names, "Search") || !contains(names, "Find") {
		t.Fatalf("rename not applied: %v", names)
	}

	if err := NewClient("demo", "t").DeleteComponent(ctx, created.ID, ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if contains(demoNames(t, NewClient("demo", "t")), "Find") {
		t.Fatal("deleted component still listed")
	}
}

func TestDemoComponentListMatchesDemoTests(t *testing.T) {
	resetDemoComponents()
	t.Cleanup(resetDemoComponents)
	names := demoNames(t, NewClient("demo", "t"))
	for i := 0; i < 30; i++ {
		for _, n := range demoComponentsForIndex(i) {
			if !contains(names, n) {
				t.Fatalf("demo test component %q is not in the project list %v", n, names)
			}
		}
	}
}

func TestDemoComponentsConcurrentDeletes(t *testing.T) {
	resetDemoComponents()
	t.Cleanup(resetDemoComponents)
	ctx := context.Background()
	list, _ := NewClient("demo", "t").ProjectComponentDetails(ctx, "DEMO")
	var wg sync.WaitGroup
	for _, comp := range list[:2] {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_ = NewClient("demo", "t").DeleteComponent(ctx, id, "")
		}(comp.ID)
	}
	wg.Wait()
	after, _ := NewClient("demo", "t").ProjectComponentDetails(ctx, "DEMO")
	if len(after) != len(list)-2 {
		t.Fatalf("want %d components, got %d", len(list)-2, len(after))
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/jira/ -run DemoComponent -count=1`
Expected: build failure, `resetDemoComponents` undefined.

- [ ] **Step 3: Write the demo store**

`demo_components.go`:

```go
package jira

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// demoComponentStore holds demo-mode components for the life of the process.
// It is package-level because app.go builds a new Client for every call, so
// state on the Client would be lost between a create and the next list. It
// never touches a user database.
var demoComponentStore = struct {
	sync.Mutex
	seeded bool
	nextID int
	byID   map[string]Component
}{}

func demoComponentsLocked() map[string]Component {
	s := &demoComponentStore
	if !s.seeded {
		s.byID = map[string]Component{}
		for _, n := range demoComponentList {
			s.nextID++
			id := strconv.Itoa(10000 + s.nextID)
			s.byID[id] = Component{ID: id, Name: n, AssigneeType: "PROJECT_DEFAULT"}
		}
		s.seeded = true
	}
	return s.byID
}

// resetDemoComponents restores the seeded list. Tests call it.
func resetDemoComponents() {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	demoComponentStore.seeded = false
	demoComponentStore.nextID = 0
	demoComponentStore.byID = nil
}

func demoComponentDetails() []Component {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	out := make([]Component, 0, len(demoComponentsLocked()))
	for _, c := range demoComponentsLocked() {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func demoNameTaken(byID map[string]Component, name, exceptID string) bool {
	for id, c := range byID {
		if id != exceptID && strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

func demoCreateComponent(in ComponentInput) (Component, error) {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	byID := demoComponentsLocked()
	if demoNameTaken(byID, in.Name, "") {
		return Component{}, &HTTPError{Method: http.MethodPost, Code: 400, Status: "400 Bad Request",
			Message: fmt.Sprintf("name: A component with the name %s already exists in this project.", in.Name)}
	}
	demoComponentStore.nextID++
	id := strconv.Itoa(10000 + demoComponentStore.nextID)
	c := Component{ID: id, Name: in.Name, Description: in.Description, LeadName: in.LeadUserName,
		LeadDisplayName: in.LeadUserName, AssigneeType: in.AssigneeType}
	byID[id] = c
	return c, nil
}

func demoUpdateComponent(id string, in ComponentInput) (Component, error) {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	byID := demoComponentsLocked()
	if _, ok := byID[id]; !ok {
		return Component{}, &HTTPError{Method: http.MethodPut, Code: 404, Status: "404 Not Found",
			Message: "The component with id " + id + " does not exist."}
	}
	if demoNameTaken(byID, in.Name, id) {
		return Component{}, &HTTPError{Method: http.MethodPut, Code: 400, Status: "400 Bad Request",
			Message: fmt.Sprintf("name: A component with the name %s already exists in this project.", in.Name)}
	}
	c := Component{ID: id, Name: in.Name, Description: in.Description, LeadName: in.LeadUserName,
		LeadDisplayName: in.LeadUserName, AssigneeType: in.AssigneeType}
	byID[id] = c
	return c, nil
}

func demoDeleteComponent(id string) error {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	byID := demoComponentsLocked()
	if _, ok := byID[id]; !ok {
		return &HTTPError{Method: http.MethodDelete, Code: 404, Status: "404 Not Found",
			Message: "The component with id " + id + " does not exist."}
	}
	delete(byID, id)
	return nil
}

// demoUsers backs the lead picker in demo mode.
var demoUsers = []User{
	{Name: "alice", DisplayName: "Alice Andersen"},
	{Name: "bob", DisplayName: "Bob Brown"},
	{Name: "carol", DisplayName: "Carol Chen"},
}

func demoSearchUsers(q string) []User {
	q = strings.ToLower(q)
	out := []User{}
	for _, u := range demoUsers {
		if strings.Contains(strings.ToLower(u.Name), q) || strings.Contains(strings.ToLower(u.DisplayName), q) {
			out = append(out, u)
		}
	}
	return out
}
```

- [ ] **Step 4: Add the demo branches and unify the lists**

In `components.go`, add as the first statement of each method:

```go
// ProjectComponentDetails
	if isDemoURL(c.baseURL) {
		return demoComponentDetails(), nil
	}
// CreateComponent
	if isDemoURL(c.baseURL) {
		return demoCreateComponent(in)
	}
// UpdateComponent
	if isDemoURL(c.baseURL) {
		return demoUpdateComponent(id, in)
	}
// DeleteComponent
	if isDemoURL(c.baseURL) {
		return demoDeleteComponent(id)
	}
// ComponentIssueCount
	if isDemoURL(c.baseURL) {
		return 3, nil
	}
// SearchUsers
	if isDemoURL(c.baseURL) {
		return demoSearchUsers(query), nil
	}
```

In `projectfields.go`, make `demoComponentList` the one source and have `ProjectComponents` read the store:

```go
// demoComponentList seeds demo mode's project components. demo.go draws each
// demo test's components from it, so the list and the tests agree.
var demoComponentList = []string{
	"Frontend", "Backend", "API", "Database", "Authentication",
	"Payments", "Reporting", "User Management", "Infrastructure", "Mobile",
}
```

and in `ProjectComponents` replace the demo branch with:

```go
	if isDemoURL(c.baseURL) {
		details := demoComponentDetails()
		out := make([]string, 0, len(details))
		for _, d := range details {
			out = append(out, d.Name)
		}
		return out, nil
	}
```

In `demo.go`, replace the `demoComponentNames` literal with `var demoComponentNames = demoComponentList`. In `customfields.go`, replace the `demoComponents` literal with `var demoComponents = demoComponentList`.

The spec asks for one list; the old `demoComponentNames` had the same ten names, so demo tests and demo requirements keep their components. The "Component" custom field's demo values change from five names to these ten; check `go test ./internal/jira/` for any demo test pinning those values and update its expectation if one does.

- [ ] **Step 5: Run the jira package**

Run: `go test ./internal/jira/ -count=1` and, if a C compiler is available, `go test -race ./internal/jira/ -run DemoComponent -count=1`
Expected: PASS. If `-race` is unavailable on this Windows machine (`cgo` off), note it in the ledger; CI's Linux job runs the same test.

- [ ] **Step 6: Commit**

```bash
git add xtm/internal/jira/
git commit -m "feat(xtm): demo-mode component writes that persist for the session (#157)"
```

---

### Task 3: ComponentManager interface, capability, Xray adapter

**Files:**
- Modify: `xtm/internal/backend/backend.go` (types, `ComponentManager`, `SupportsComponentAdmin`)
- Modify: `xtm/internal/backend/xray/adapter.go` (methods, conversion, capability)
- Create: `xtm/internal/backend/xray/components_test.go`
- Modify: `xtm/internal/backend/kiwi/adapter_test.go` (`want` gains the field as false)

**Interfaces:**
- Consumes: Task 1/2 client methods.
- Produces, in package `backend`:

```go
type Component struct {
	ID, Name, Description, LeadName, LeadDisplayName, AssigneeType string
}
type ComponentInput struct {
	Name, Description, LeadUserName, AssigneeType string
}
type User struct{ Name, DisplayName string }
type ComponentManager interface {
	ProjectComponentDetails(ctx context.Context, projectKey string) ([]Component, error)
	CreateComponent(ctx context.Context, projectKey string, in ComponentInput) (Component, error)
	UpdateComponent(ctx context.Context, id string, in ComponentInput) (Component, error)
	DeleteComponent(ctx context.Context, id, moveIssuesTo string) error
	ComponentIssueCount(ctx context.Context, id string) (int, error)
	SearchUsers(ctx context.Context, query string) ([]User, error)
}
// Capabilities gains: SupportsComponentAdmin bool `json:"supportsComponentAdmin"`
```

Each struct field carries a camelCase json tag (`id`, `name`, `description`, `leadName`, `leadDisplayName`, `assigneeType`, `leadUserName`, `displayName`), so Wails emits matching TS models.

- [ ] **Step 1: Write the failing adapter test**

`xray/components_test.go`:

```go
package xray

import (
	"context"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

func TestAdapterManagesComponentsInDemo(t *testing.T) {
	a := New(jira.NewClient("demo", "t"))
	var b backend.Backend = a
	cm, ok := b.(backend.ComponentManager)
	if !ok {
		t.Fatal("xray adapter does not implement ComponentManager")
	}
	if !a.Capabilities().SupportsComponentAdmin {
		t.Fatal("SupportsComponentAdmin is false")
	}
	created, err := cm.CreateComponent(context.Background(), "DEMO",
		backend.ComponentInput{Name: "AdapterTest", AssigneeType: "PROJECT_DEFAULT"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = cm.DeleteComponent(context.Background(), created.ID, "") })
	list, err := cm.ProjectComponentDetails(context.Background(), "DEMO")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, c := range list {
		if c.ID == created.ID && c.Name == "AdapterTest" && c.AssigneeType == "PROJECT_DEFAULT" {
			found = true
		}
	}
	if !found {
		t.Fatalf("created component missing from %+v", list)
	}
}
```

In `kiwi/adapter_test.go` `TestCapabilitiesBaseValues`, add `SupportsComponentAdmin: false,` to `want`, and add a line asserting the Kiwi adapter does not implement `backend.ComponentManager`:

```go
	if _, ok := any(a).(backend.ComponentManager); ok {
		t.Error("kiwi adapter should not implement ComponentManager")
	}
```

(use whatever variable name that test gives the adapter).

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/backend/... -count=1`
Expected: build failure, `backend.ComponentManager` undefined.

- [ ] **Step 3: Add the types and interface**

In `backend.go`, next to the other optional interfaces:

```go
// Component is one project component, as the Components view shows it.
type Component struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	LeadName        string `json:"leadName"`
	LeadDisplayName string `json:"leadDisplayName"`
	AssigneeType    string `json:"assigneeType"`
}

// ComponentInput is what a create or update sends.
type ComponentInput struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	LeadUserName string `json:"leadUserName"`
	AssigneeType string `json:"assigneeType"`
}

// User is a user search result, for picking a component lead.
type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// ComponentManager creates, edits and deletes project components. It is kept
// off Backend because only Jira has it; callers type-assert and check
// Capabilities().SupportsComponentAdmin.
type ComponentManager interface {
	ProjectComponentDetails(ctx context.Context, projectKey string) ([]Component, error)
	CreateComponent(ctx context.Context, projectKey string, in ComponentInput) (Component, error)
	UpdateComponent(ctx context.Context, id string, in ComponentInput) (Component, error)
	DeleteComponent(ctx context.Context, id, moveIssuesTo string) error
	ComponentIssueCount(ctx context.Context, id string) (int, error)
	SearchUsers(ctx context.Context, query string) ([]User, error)
}
```

In `Capabilities`, after `SupportsTags`:

```go
	// SupportsComponentAdmin: the backend can create, edit and delete project
	// components (it implements ComponentManager).
	SupportsComponentAdmin bool `json:"supportsComponentAdmin"`
```

- [ ] **Step 4: Implement on the Xray adapter**

In `xray/adapter.go`, add `var _ backend.ComponentManager = (*Adapter)(nil)` beside the existing assertion, set `SupportsComponentAdmin: true` in `Capabilities()`, and add:

```go
func toBackendComponent(c jira.Component) backend.Component {
	return backend.Component{ID: c.ID, Name: c.Name, Description: c.Description,
		LeadName: c.LeadName, LeadDisplayName: c.LeadDisplayName, AssigneeType: c.AssigneeType}
}

func (a *Adapter) ProjectComponentDetails(ctx context.Context, projectKey string) ([]backend.Component, error) {
	list, err := a.c.ProjectComponentDetails(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	out := make([]backend.Component, 0, len(list))
	for _, c := range list {
		out = append(out, toBackendComponent(c))
	}
	return out, nil
}

func (a *Adapter) CreateComponent(ctx context.Context, projectKey string, in backend.ComponentInput) (backend.Component, error) {
	c, err := a.c.CreateComponent(ctx, jira.ComponentInput{Project: projectKey, Name: in.Name,
		Description: in.Description, LeadUserName: in.LeadUserName, AssigneeType: in.AssigneeType})
	return toBackendComponent(c), err
}

func (a *Adapter) UpdateComponent(ctx context.Context, id string, in backend.ComponentInput) (backend.Component, error) {
	c, err := a.c.UpdateComponent(ctx, id, jira.ComponentInput{Name: in.Name,
		Description: in.Description, LeadUserName: in.LeadUserName, AssigneeType: in.AssigneeType})
	return toBackendComponent(c), err
}

func (a *Adapter) DeleteComponent(ctx context.Context, id, moveIssuesTo string) error {
	return a.c.DeleteComponent(ctx, id, moveIssuesTo)
}

func (a *Adapter) ComponentIssueCount(ctx context.Context, id string) (int, error) {
	return a.c.ComponentIssueCount(ctx, id)
}

func (a *Adapter) SearchUsers(ctx context.Context, query string) ([]backend.User, error) {
	users, err := a.c.SearchUsers(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]backend.User, 0, len(users))
	for _, u := range users {
		out = append(out, backend.User{Name: u.Name, DisplayName: u.DisplayName})
	}
	return out, nil
}
```

- [ ] **Step 5: Run the backend packages**

Run: `go test ./internal/backend/... -count=1 && go build ./...`
Expected: PASS and a clean build.

- [ ] **Step 6: Commit**

```bash
git add xtm/internal/backend/
git commit -m "feat(xtm): component management behind an optional backend interface (#157)"
```

---

### Task 4: Local cache rewrite and the pending-edit guard

**Files:**
- Create: `xtm/internal/testrepo/componentadmin.go`
- Create: `xtm/internal/testrepo/componentadmin_test.go`

**Interfaces:**
- Produces:

```go
func (r *Repository) ComponentEditsPending(profileID, name string) (int, error)
func (r *Repository) RenameComponentOnTests(profileID, oldName, newName string) (int, error)
func (r *Repository) RemoveComponentFromTests(profileID, name string) (int, error)
```

`ComponentEditsPending` counts pending `components` field edits on tests (`entity_type = entityTestCase`) where the test's cached components or the edit's new value contain `name`. The rename and remove functions return how many tests changed.

- [ ] **Step 1: Write the failing tests**

`componentadmin_test.go`:

```go
package testrepo_test

import (
	"reflect"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

const caProfile = "p1"

func seedComponentAdmin(t *testing.T, repo *testrepo.Repository) {
	t.Helper()
	tests := []testrepo.TestCase{
		{Key: "QA-1", Summary: "a", Components: []string{"core", "API"}},
		{Key: "QA-2", Summary: "b", Components: []string{"core"}},
		{Key: "QA-3", Summary: "c", Components: []string{"Core Services"}},
		{Key: "QA-4", Summary: "d"},
	}
	if err := repo.UpsertTests(caProfile, tests); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func componentsOf(t *testing.T, repo *testrepo.Repository, key string) []string {
	t.Helper()
	tc, err := repo.GetTest(caProfile, key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	return tc.Components
}

func TestRenameComponentOnTestsCaseOnly(t *testing.T) {
	repo := newRepo(t)
	seedComponentAdmin(t, repo)

	n, err := repo.RenameComponentOnTests(caProfile, "core", "Core")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if n != 2 {
		t.Fatalf("changed %d tests, want 2", n)
	}
	if got := componentsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"Core", "API"}) {
		t.Fatalf("QA-1 %v", got)
	}
	if got := componentsOf(t, repo, "QA-3"); !reflect.DeepEqual(got, []string{"Core Services"}) {
		t.Fatalf("QA-3 must keep its longer name, got %v", got)
	}
}

func TestRenameComponentOnTestsMergesIntoExisting(t *testing.T) {
	repo := newRepo(t)
	seedComponentAdmin(t, repo)

	if _, err := repo.RenameComponentOnTests(caProfile, "API", "core"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if got := componentsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"core"}) {
		t.Fatalf("QA-1 should hold core once, got %v", got)
	}
}

func TestRemoveComponentFromTests(t *testing.T) {
	repo := newRepo(t)
	seedComponentAdmin(t, repo)

	n, err := repo.RemoveComponentFromTests(caProfile, "core")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if n != 2 {
		t.Fatalf("changed %d, want 2", n)
	}
	if got := componentsOf(t, repo, "QA-2"); len(got) != 0 {
		t.Fatalf("QA-2 %v", got)
	}
	if got := componentsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"API"}) {
		t.Fatalf("QA-1 %v", got)
	}
}

func TestComponentEditsPendingIgnoresOtherFields(t *testing.T) {
	repo := newRepo(t)
	seedComponentAdmin(t, repo)

	// An unrelated pending edit on a test that carries the component.
	if err := repo.EditTestField(caProfile, "QA-1", "summary", "changed"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	n, err := repo.ComponentEditsPending(caProfile, "core")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if n != 0 {
		t.Fatalf("summary edit counted as a component edit: %d", n)
	}
}
```

Part B adds `components` to `editableFields`, so no public call queues a `components` edit yet. The positive case inserts the row the way part B's `EditTestField` will, through the store (`store.Store.DB()` exists):

```go
func newRepoAndStore(t *testing.T) (*testrepo.Repository, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st), st
}

func TestComponentEditsPendingCountsMatchingEdits(t *testing.T) {
	repo, st := newRepoAndStore(t)
	seedComponentAdmin(t, repo)
	insert := func(key, after string) {
		t.Helper()
		_, err := st.DB().Exec(`INSERT INTO pending_change
			(profile_id, entity_type, entity_key, field, before_val, after_val, base_version, created_at)
			VALUES (?, 'test_case', ?, 'components', '', ?, '', '2026-10-09T00:00:00Z')`,
			caProfile, key, after)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	insert("QA-2", "\nAPI\n")   // the test carries core in its synced value
	insert("QA-4", "\ncore\n")  // the queued value adds core
	insert("QA-3", "\nOther\n") // unrelated

	n, err := repo.ComponentEditsPending(caProfile, "core")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2, got %d", n)
	}
}
```

Add `"path/filepath"` and `"agile-suite/xtm/internal/store"` to the imports. Check `entityTestCase`'s value (`grep -n "entityTestCase *=" xtm/internal/testrepo/testrepo.go`) matches the `'test_case'` literal, and the `pending_change` columns match `store.go`, before running.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/testrepo/ -run 'Component(Edits|OnTests|FromTests)' -count=1`
Expected: build failure, methods undefined.

- [ ] **Step 3: Write componentadmin.go**

```go
package testrepo

import "fmt"

// ComponentEditsPending counts pending components edits on tests that carry
// name, either in the synced value or in the queued one. A rename or delete
// of that component would leave those edits naming a component Jira no
// longer has, so the Components view refuses while any exist.
func (r *Repository) ComponentEditsPending(profileID, name string) (int, error) {
	pattern := componentFilterPattern(name)
	var n int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM pending_change pc
		LEFT JOIN test_case tc ON tc.profile_id = pc.profile_id AND tc.jira_key = pc.entity_key
		WHERE pc.profile_id = ? AND pc.entity_type = ? AND pc.field = 'components'
		  AND (tc.components LIKE ? OR pc.after_val LIKE ?)`,
		profileID, entityTestCase, pattern, pattern,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("pending component edits: %w", err)
	}
	return n, nil
}

// RenameComponentOnTests replaces oldName with newName in every cached test's
// components, after Jira has renamed it. A test that already carries newName
// keeps one copy.
func (r *Repository) RenameComponentOnTests(profileID, oldName, newName string) (int, error) {
	return r.rewriteComponent(profileID, oldName, func(names []string) []string {
		out := make([]string, 0, len(names))
		seen := map[string]bool{}
		for _, n := range names {
			if n == oldName {
				n = newName
			}
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
		return out
	})
}

// RemoveComponentFromTests drops name from every cached test's components,
// after Jira has deleted it.
func (r *Repository) RemoveComponentFromTests(profileID, name string) (int, error) {
	return r.rewriteComponent(profileID, name, func(names []string) []string {
		out := make([]string, 0, len(names))
		for _, n := range names {
			if n != name {
				out = append(out, n)
			}
		}
		return out
	})
}

// rewriteComponent applies fn to the components of every test carrying name,
// in one transaction.
func (r *Repository) rewriteComponent(profileID, name string, fn func([]string) []string) (int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.Query(
		`SELECT jira_key, components FROM test_case WHERE profile_id = ? AND components LIKE ?`,
		profileID, componentFilterPattern(name))
	if err != nil {
		return 0, fmt.Errorf("find tests with component %q: %w", name, err)
	}
	type change struct{ key, value string }
	var changes []change
	for rows.Next() {
		var key, stored string
		if err := rows.Scan(&key, &stored); err != nil {
			rows.Close()
			return 0, err
		}
		next := encodeComponents(fn(decodeComponents(stored)))
		if next != stored {
			changes = append(changes, change{key, next})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, c := range changes {
		if _, err := tx.Exec(
			`UPDATE test_case SET components = ? WHERE profile_id = ? AND jira_key = ?`,
			c.value, profileID, c.key); err != nil {
			return 0, fmt.Errorf("rewrite components on %s: %w", c.key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(changes), nil
}
```


Note on `LIKE`: SQLite's `LIKE` is case-insensitive for ASCII, so `componentFilterPattern("core")` also matches `\nCore\n`. That only widens the candidate rows; `fn` compares exactly, so a test carrying `Core` but not `core` comes back unchanged and is not counted. The same widening in `ComponentEditsPending` can over-count, which errs toward refusing; that is acceptable for a guard.

- [ ] **Step 4: Run the package**

Run: `go test ./internal/testrepo/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add xtm/internal/testrepo/componentadmin.go xtm/internal/testrepo/componentadmin_test.go
git commit -m "feat(xtm): rewrite cached test components after a rename or delete (#157)"
```

---

### Task 5: componentadmin service

**Files:**
- Create: `xtm/internal/componentadmin/service.go`
- Create: `xtm/internal/componentadmin/service_test.go`

**Interfaces:**
- Consumes: `backend.ComponentManager`, `backend.Backend.ProjectComponents`, `testrepo` methods from Task 4, `Repository.ReplaceProjectFieldOptions`.
- Produces:

```go
var ErrForbidden = errors.New("You need project admin rights in Jira to change components.")
type Repo interface {
	ReplaceProjectFieldOptions(profileID, projectKey, field string, values []string) error
	ComponentEditsPending(profileID, name string) (int, error)
	RenameComponentOnTests(profileID, oldName, newName string) (int, error)
	RemoveComponentFromTests(profileID, name string) (int, error)
}
type Backend interface {
	backend.ComponentManager
	ProjectComponents(ctx context.Context, projectKey string) ([]string, error)
}
type Service struct{ /* repo, backend, profileID, projectKey */ }
func New(repo Repo, b Backend, profileID, projectKey string) *Service
func (s *Service) List(ctx context.Context) ([]backend.Component, error)
func (s *Service) Create(ctx context.Context, in backend.ComponentInput) (backend.Component, error)
func (s *Service) Update(ctx context.Context, id string, in backend.ComponentInput) (backend.Component, error)
func (s *Service) Delete(ctx context.Context, id, moveIssuesTo string) error
```

- [ ] **Step 1: Write the failing tests**

`service_test.go`:

```go
package componentadmin

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

type fakeRepo struct {
	options  []string
	pending  map[string]int
	renamed  [][2]string
	removed  []string
}

func (f *fakeRepo) ReplaceProjectFieldOptions(_, _, field string, values []string) error {
	if field == "component" {
		f.options = values
	}
	return nil
}
func (f *fakeRepo) ComponentEditsPending(_, name string) (int, error) { return f.pending[name], nil }
func (f *fakeRepo) RenameComponentOnTests(_, o, n string) (int, error) {
	f.renamed = append(f.renamed, [2]string{o, n})
	return 1, nil
}
func (f *fakeRepo) RemoveComponentFromTests(_, name string) (int, error) {
	f.removed = append(f.removed, name)
	return 1, nil
}

type fakeBackend struct {
	backend.ComponentManager
	list    []backend.Component
	err     error
	deleted []string
}

func (f *fakeBackend) ProjectComponentDetails(context.Context, string) ([]backend.Component, error) {
	return f.list, nil
}
func (f *fakeBackend) ProjectComponents(context.Context, string) ([]string, error) {
	out := []string{}
	for _, c := range f.list {
		out = append(out, c.Name)
	}
	return out, nil
}
func (f *fakeBackend) CreateComponent(_ context.Context, _ string, in backend.ComponentInput) (backend.Component, error) {
	if f.err != nil {
		return backend.Component{}, f.err
	}
	c := backend.Component{ID: "new", Name: in.Name}
	f.list = append(f.list, c)
	return c, nil
}
func (f *fakeBackend) UpdateComponent(_ context.Context, id string, in backend.ComponentInput) (backend.Component, error) {
	for i := range f.list {
		if f.list[i].ID == id {
			f.list[i].Name = in.Name
			return f.list[i], nil
		}
	}
	return backend.Component{}, &jira.HTTPError{Code: 404, Status: "404 Not Found", Message: "gone"}
}
func (f *fakeBackend) DeleteComponent(_ context.Context, id, _ string) error {
	f.deleted = append(f.deleted, id)
	out := f.list[:0]
	for _, c := range f.list {
		if c.ID != id {
			out = append(out, c)
		}
	}
	f.list = out
	return nil
}

func newSvc(repo *fakeRepo, b *fakeBackend) *Service { return New(repo, b, "p1", "QA") }

func TestCreateRefreshesCache(t *testing.T) {
	repo := &fakeRepo{}
	b := &fakeBackend{list: []backend.Component{{ID: "1", Name: "Core"}}}
	if _, err := newSvc(repo, b).Create(context.Background(), backend.ComponentInput{Name: " API "}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !reflect.DeepEqual(repo.options, []string{"Core", "API"}) {
		t.Fatalf("cache %v", repo.options)
	}
}

func TestCreateRejectsBlankName(t *testing.T) {
	_, err := newSvc(&fakeRepo{}, &fakeBackend{}).Create(context.Background(), backend.ComponentInput{Name: "  "})
	if err == nil || err.Error() != "A component needs a name." {
		t.Fatalf("err %v", err)
	}
}

func TestForbiddenBecomesReadableError(t *testing.T) {
	b := &fakeBackend{err: &jira.HTTPError{Code: http.StatusForbidden, Status: "403 Forbidden"}}
	_, err := newSvc(&fakeRepo{}, b).Create(context.Background(), backend.ComponentInput{Name: "X"})
	if !errors.Is(err, ErrForbidden) || err.Error() != "You need project admin rights in Jira to change components." {
		t.Fatalf("err %v", err)
	}
}

func TestRenameRewritesTestsAndRefuses(t *testing.T) {
	repo := &fakeRepo{pending: map[string]int{}}
	b := &fakeBackend{list: []backend.Component{{ID: "1", Name: "core"}}}
	svc := newSvc(repo, b)
	if _, err := svc.Update(context.Background(), "1", backend.ComponentInput{Name: "Core"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !reflect.DeepEqual(repo.renamed, [][2]string{{"core", "Core"}}) {
		t.Fatalf("renamed %v", repo.renamed)
	}

	repo.pending["Core"] = 2
	_, err := svc.Update(context.Background(), "1", backend.ComponentInput{Name: "Kernel"})
	want := `Commit or discard the 2 pending component edits on tests that use "Core" first.`
	if err == nil || err.Error() != want {
		t.Fatalf("err %v", err)
	}
	if b.list[0].Name != "Core" {
		t.Fatal("Jira was called despite the guard")
	}
}

func TestUpdateWithoutRenameSkipsGuardAndRewrite(t *testing.T) {
	repo := &fakeRepo{pending: map[string]int{"Core": 3}}
	b := &fakeBackend{list: []backend.Component{{ID: "1", Name: "Core"}}}
	if _, err := newSvc(repo, b).Update(context.Background(), "1", backend.ComponentInput{Name: "Core", Description: "x"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(repo.renamed) != 0 {
		t.Fatalf("renamed %v", repo.renamed)
	}
}

func TestDeleteMovesOrRemoves(t *testing.T) {
	repo := &fakeRepo{pending: map[string]int{}}
	b := &fakeBackend{list: []backend.Component{{ID: "1", Name: "Old"}, {ID: "2", Name: "New"}}}
	svc := newSvc(repo, b)
	if err := svc.Delete(context.Background(), "1", "2"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !reflect.DeepEqual(repo.renamed, [][2]string{{"Old", "New"}}) {
		t.Fatalf("move should rename on tests, got %v", repo.renamed)
	}
	if err := svc.Delete(context.Background(), "2", ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !reflect.DeepEqual(repo.removed, []string{"New"}) {
		t.Fatalf("removed %v", repo.removed)
	}
	if !reflect.DeepEqual(repo.options, []string{}) {
		t.Fatalf("cache %v", repo.options)
	}
}

func TestUpdateOfMissingComponentRefreshesCache(t *testing.T) {
	repo := &fakeRepo{pending: map[string]int{}}
	b := &fakeBackend{list: []backend.Component{{ID: "2", Name: "Other"}}}
	_, err := newSvc(repo, b).Update(context.Background(), "1", backend.ComponentInput{Name: "X"})
	if err == nil {
		t.Fatal("want error")
	}
	if !reflect.DeepEqual(repo.options, []string{"Other"}) {
		t.Fatalf("cache not refreshed after 404: %v", repo.options)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/componentadmin/ -count=1`
Expected: build failure, package has no `New`.

- [ ] **Step 3: Write service.go**

```go
// Package componentadmin creates, edits and deletes Jira project components
// for the Components view. Writes go straight to Jira (a component belongs to
// the project, not to an issue, so the pending-change journal does not apply),
// then the cached component list and the cached test components are brought
// in line.
package componentadmin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

// ErrForbidden is what a 403 from Jira becomes. The text is shown as is.
var ErrForbidden = errors.New("You need project admin rights in Jira to change components.")

// Repo is the local store the service keeps in step with Jira.
type Repo interface {
	ReplaceProjectFieldOptions(profileID, projectKey, field string, values []string) error
	ComponentEditsPending(profileID, name string) (int, error)
	RenameComponentOnTests(profileID, oldName, newName string) (int, error)
	RemoveComponentFromTests(profileID, name string) (int, error)
}

// Backend is the slice of a backend the service needs.
type Backend interface {
	backend.ComponentManager
	ProjectComponents(ctx context.Context, projectKey string) ([]string, error)
}

// Service manages one profile's project components.
type Service struct {
	repo       Repo
	b          Backend
	profileID  string
	projectKey string
}

// New returns a Service for profileID's project.
func New(repo Repo, b Backend, profileID, projectKey string) *Service {
	return &Service{repo: repo, b: b, profileID: profileID, projectKey: projectKey}
}

// List returns the project's components from Jira.
func (s *Service) List(ctx context.Context) ([]backend.Component, error) {
	list, err := s.b.ProjectComponentDetails(ctx, s.projectKey)
	return list, translate(err)
}

// Create creates a component and refreshes the cached list.
func (s *Service) Create(ctx context.Context, in backend.ComponentInput) (backend.Component, error) {
	in, err := clean(in)
	if err != nil {
		return backend.Component{}, err
	}
	c, err := s.b.CreateComponent(ctx, s.projectKey, in)
	if err != nil {
		return backend.Component{}, translate(err)
	}
	return c, s.refresh(ctx)
}

// Update edits a component. A rename is refused while pending component edits
// name the old name, and rewrites the cached test components once Jira agrees.
func (s *Service) Update(ctx context.Context, id string, in backend.ComponentInput) (backend.Component, error) {
	in, err := clean(in)
	if err != nil {
		return backend.Component{}, err
	}
	old, err := s.find(ctx, id)
	if err != nil {
		return backend.Component{}, err
	}
	renaming := old.Name != "" && old.Name != in.Name
	if renaming {
		if err := s.guard(old.Name); err != nil {
			return backend.Component{}, err
		}
	}
	c, err := s.b.UpdateComponent(ctx, id, in)
	if err != nil {
		_ = s.refresh(ctx)
		return backend.Component{}, translate(err)
	}
	if renaming {
		if _, err := s.repo.RenameComponentOnTests(s.profileID, old.Name, c.Name); err != nil {
			return c, err
		}
	}
	return c, s.refresh(ctx)
}

// Delete deletes a component. With moveIssuesTo (a component id), Jira moves
// its issues there and the cached tests follow; otherwise the name is dropped
// from cached tests.
func (s *Service) Delete(ctx context.Context, id, moveIssuesTo string) error {
	old, err := s.find(ctx, id)
	if err != nil {
		return err
	}
	if old.Name != "" {
		if err := s.guard(old.Name); err != nil {
			return err
		}
	}
	target := backend.Component{}
	if moveIssuesTo != "" {
		if target, err = s.find(ctx, moveIssuesTo); err != nil {
			return err
		}
	}
	if err := s.b.DeleteComponent(ctx, id, moveIssuesTo); err != nil {
		_ = s.refresh(ctx)
		return translate(err)
	}
	if old.Name != "" {
		if target.Name != "" {
			_, err = s.repo.RenameComponentOnTests(s.profileID, old.Name, target.Name)
		} else {
			_, err = s.repo.RemoveComponentFromTests(s.profileID, old.Name)
		}
		if err != nil {
			return err
		}
	}
	return s.refresh(ctx)
}

// find returns the component with id from Jira's current list, or a zero
// Component when Jira no longer has it (the write then reports Jira's 404).
func (s *Service) find(ctx context.Context, id string) (backend.Component, error) {
	list, err := s.b.ProjectComponentDetails(ctx, s.projectKey)
	if err != nil {
		return backend.Component{}, translate(err)
	}
	for _, c := range list {
		if c.ID == id {
			return c, nil
		}
	}
	return backend.Component{}, nil
}

func (s *Service) guard(name string) error {
	n, err := s.repo.ComponentEditsPending(s.profileID, name)
	if err != nil {
		return err
	}
	if n > 0 {
		noun := "edit"
		if n > 1 {
			noun = "edits"
		}
		return fmt.Errorf("Commit or discard the %d pending component %s on tests that use %q first.", n, noun, name)
	}
	return nil
}

func (s *Service) refresh(ctx context.Context) error {
	names, err := s.b.ProjectComponents(ctx, s.projectKey)
	if err != nil {
		return translate(err)
	}
	if names == nil {
		names = []string{}
	}
	return s.repo.ReplaceProjectFieldOptions(s.profileID, s.projectKey, "component", names)
}

func clean(in backend.ComponentInput) (backend.ComponentInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if in.Name == "" {
		return in, errors.New("A component needs a name.")
	}
	if in.AssigneeType == "" {
		in.AssigneeType = "PROJECT_DEFAULT"
	}
	return in, nil
}

func translate(err error) error {
	var he *jira.HTTPError
	if errors.As(err, &he) && he.Code == http.StatusForbidden {
		return ErrForbidden
	}
	return err
}
```

`TestRenameRewritesTestsAndRefuses` expects the 2-edit message with the plural "edits"; the singular form reads "1 pending component edit".

`go vet` does not flag capitalized error strings; `staticcheck` would (ST1005). These errors are user-facing text shown verbatim, which is the reason; add `//lint:ignore ST1005 user-facing text` above `ErrForbidden` only if the repo runs staticcheck (`grep -rn staticcheck .github Makefile`).

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./internal/componentadmin/ -count=1 && go vet ./internal/componentadmin/`
Expected: PASS, vet clean.

- [ ] **Step 5: Commit**

```bash
git add xtm/internal/componentadmin/
git commit -m "feat(xtm): component admin service with cache refresh and pending guard (#157)"
```

---

### Task 6: Bindings, capability on the frontend, query hooks

**Files:**
- Modify: `xtm/app.go`
- Create: `xtm/app_components_test.go`
- Regenerate: `xtm/frontend/wailsjs/go/main/App.{js,d.ts}`, `xtm/frontend/wailsjs/go/models.ts` (the new `backend.Component`, `backend.ComponentInput`, `backend.User` models)
- Modify: `xtm/frontend/src/api.ts` (exports, `Capabilities.supportsComponentAdmin`, type re-exports)
- Modify: `xtm/frontend/src/features.ts` (`defaultCapabilities.supportsComponentAdmin: true`)
- Modify: `xtm/frontend/src/components/ContainersView.test.tsx` (caps literal)
- Create: `xtm/frontend/src/queries/components.ts`, `xtm/frontend/src/queries/components.test.tsx`
- Modify: `xtm/frontend/src/queries/keys.ts`

**Interfaces:**
- Produces Go bindings:

```go
func (a *App) ListProjectComponentDetails(profileID string) ([]backend.Component, error)
func (a *App) CreateComponent(profileID string, in backend.ComponentInput) (backend.Component, error)
func (a *App) UpdateComponent(profileID, id string, in backend.ComponentInput) (backend.Component, error)
func (a *App) DeleteComponent(profileID, id, moveIssuesTo string) error
func (a *App) ComponentIssueCount(profileID, id string) (int, error)
func (a *App) SearchUsers(profileID, query string) ([]backend.User, error)
```

- Produces TS: `keys.projectComponents(profileId)` = `[profileId, "components", "project"]`; hooks `useProjectComponents(profileId)`, `useComponentCounts(profileId)` (returns `Map<string, number>`), `useUserSearch(profileId, query)`.

- [ ] **Step 1: Write the failing Go binding test**

`app_components_test.go`. `newTestApp(t)` and `a.CreateProfile(...)` are the helpers `newTestAppWithKiwiProfile` in `app_bugconnection_test.go` uses; a demo Jira profile has URL `demo` and backend `xray`:

```go
package main

import (
	"errors"
	"testing"

	"agile-suite/xtm/internal/backend"
)

func TestComponentBindingsInDemo(t *testing.T) {
	a := newTestApp(t)
	p, err := a.CreateProfile("Demo", "demo", "DEMO", "", "", "", "", "tok", "", false, "xray")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	created, err := a.CreateComponent(p.ID, backend.ComponentInput{Name: "BindingTest"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = a.DeleteComponent(p.ID, created.ID, "") })
	opts, err := a.ListProjectComponents(p.ID, "DEMO")
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	found := false
	for _, o := range opts {
		if o == "BindingTest" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cached options %v lack the new component", opts)
	}
}

func TestComponentBindingsUnsupportedOnKiwi(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)
	_, err := a.CreateComponent(profileID, backend.ComponentInput{Name: "X"})
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}
```

Check `CreateProfile`'s parameter order against its definition in `app.go` before running.

- [ ] **Step 2: Write the failing query test**

`queries/components.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useComponentCounts, useProjectComponents } from "./components";
import * as api from "../api";

vi.mock("../api", () => ({
  ListProjectComponentDetails: vi.fn(),
  ListComponents: vi.fn(),
  SearchUsers: vi.fn(),
  errMsg: (e: unknown) => String(e),
}));

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

beforeEach(() => vi.clearAllMocks());

describe("component queries", () => {
  it("useProjectComponents returns Jira's components", async () => {
    (api.ListProjectComponentDetails as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: "1", name: "Core", description: "", leadName: "", leadDisplayName: "", assigneeType: "PROJECT_DEFAULT" },
    ]);
    const { result } = renderHook(() => useProjectComponents("p1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.ListProjectComponentDetails).toHaveBeenCalledWith("p1");
    expect(result.current.data?.map((c) => c.name)).toEqual(["Core"]);
  });

  it("useComponentCounts maps names to test counts", async () => {
    (api.ListComponents as ReturnType<typeof vi.fn>).mockResolvedValue([
      { label: "Core", count: 4 },
    ]);
    const { result } = renderHook(() => useComponentCounts("p1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.get("Core")).toBe(4);
  });
});
```

- [ ] **Step 3: Run both to see them fail**

Run from `xtm/`: `go test . -run ComponentBindings -count=1` (build failure)
Run from `xtm/frontend`: `npx vitest run src/queries/components.test.tsx` (module not found)

- [ ] **Step 4: Add the bindings**

In `app.go`, near `ListProjectComponents`:

```go
// componentAdminBackend joins a backend's component writes with its
// component-name list, which is what componentadmin.Service needs.
type componentAdminBackend struct {
	backend.ComponentManager
	names backend.Backend
}

func (c componentAdminBackend) ProjectComponents(ctx context.Context, key string) ([]string, error) {
	return c.names.ProjectComponents(ctx, key)
}

// componentAdmin returns the component service for profileID, or
// backend.ErrUnsupported when the profile's backend cannot manage components.
func (a *App) componentAdmin(profileID string) (*componentadmin.Service, backend.ComponentManager, error) {
	if err := a.requireStore(); err != nil {
		return nil, nil, err
	}
	b, err := a.backendFor(profileID)
	if err != nil {
		return nil, nil, err
	}
	cm, ok := b.(backend.ComponentManager)
	if !ok || !b.Capabilities().SupportsComponentAdmin {
		return nil, nil, fmt.Errorf("components: %w", backend.ErrUnsupported)
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return nil, nil, err
	}
	return componentadmin.New(a.repo, componentAdminBackend{cm, b}, profileID, p.ProjectKey), cm, nil
}
```

Then:

```go
// ListProjectComponentDetails lists the project's components live from Jira.
func (a *App) ListProjectComponentDetails(profileID string) (out []backend.Component, err error) {
	defer recoverToError("ListProjectComponentDetails", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return nil, err
	}
	return svc.List(a.ctx)
}

// CreateComponent creates a project component in Jira.
func (a *App) CreateComponent(profileID string, in backend.ComponentInput) (out backend.Component, err error) {
	defer recoverToError("CreateComponent", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return backend.Component{}, err
	}
	return svc.Create(a.ctx, in)
}

// UpdateComponent edits a project component in Jira.
func (a *App) UpdateComponent(profileID, id string, in backend.ComponentInput) (out backend.Component, err error) {
	defer recoverToError("UpdateComponent", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return backend.Component{}, err
	}
	return svc.Update(a.ctx, id, in)
}

// DeleteComponent deletes a project component in Jira, optionally moving its
// issues to another component first.
func (a *App) DeleteComponent(profileID, id, moveIssuesTo string) (err error) {
	defer recoverToError("DeleteComponent", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return err
	}
	return svc.Delete(a.ctx, id, moveIssuesTo)
}

// ComponentIssueCount is how many issues carry a component, for the delete
// dialog.
func (a *App) ComponentIssueCount(profileID, id string) (n int, err error) {
	defer recoverToError("ComponentIssueCount", &err)
	_, cm, err := a.componentAdmin(profileID)
	if err != nil {
		return 0, err
	}
	return cm.ComponentIssueCount(a.ctx, id)
}

// SearchUsers finds Jira users for the component lead picker.
func (a *App) SearchUsers(profileID, query string) (out []backend.User, err error) {
	defer recoverToError("SearchUsers", &err)
	_, cm, err := a.componentAdmin(profileID)
	if err != nil {
		return nil, err
	}
	return cm.SearchUsers(a.ctx, query)
}
```

Add the `componentadmin` import. Run `go build ./... && go test . -run ComponentBindings -count=1`.

- [ ] **Step 5: Regenerate and wire the frontend**

Run from `xtm/`: `wails generate module`. Keep only content changes (`git diff --ignore-cr-at-eol --stat frontend/wailsjs`): `App.js`, `App.d.ts`, and `models.ts` gaining the `backend.Component`, `backend.ComponentInput`, `backend.User` classes.

`api.ts`: add `ListProjectComponentDetails, CreateComponent, UpdateComponent, DeleteComponent, ComponentIssueCount, SearchUsers,` to the re-export list beside `ListProjectComponents`; add `supportsComponentAdmin: boolean;` to `interface Capabilities` after the last field; export the model types the same way the file exports other `backend` models (look for an existing `export type … = backend.…` line and copy its form) as `ProjectComponent`, `ComponentInput`, `JiraUser`.

`features.ts`: add `supportsComponentAdmin: true,` to `defaultCapabilities`.

`ContainersView.test.tsx`: add `supportsComponentAdmin: true,` to the caps literal.

`queries/keys.ts`, after `components`:

```ts
  // Live list from Jira. Under the "components" prefix so
  // invalidateProfileData refreshes it with the local counts.
  projectComponents: (profileId: string) =>
    [profileId, "components", "project"] as const,
  userSearch: (profileId: string, q: string) =>
    [profileId, "userSearch", q] as const,
```

`queries/components.ts`:

```ts
import { useQuery } from "@tanstack/react-query";
import { ListComponents, ListProjectComponentDetails, SearchUsers } from "../api";
import { call } from "../lib/apiCall";
import { keys } from "./keys";

// useProjectComponents loads the project's components live from Jira for the
// Components view.
export function useProjectComponents(profileId: string) {
  return useQuery({
    queryKey: keys.projectComponents(profileId),
    queryFn: () => call(() => ListProjectComponentDetails(profileId)),
    enabled: !!profileId,
  });
}

// useComponentCounts maps each component name to how many synced tests carry
// it. It shares ListComponents' key with the group-by sidebar.
export function useComponentCounts(profileId: string) {
  return useQuery({
    queryKey: keys.components(profileId),
    queryFn: () => call(() => ListComponents(profileId)),
    enabled: !!profileId,
    select: (buckets) => new Map(buckets.map((b) => [b.label, b.count])),
  });
}

// useUserSearch looks users up for the lead picker once two characters are
// typed.
export function useUserSearch(profileId: string, query: string) {
  const q = query.trim();
  return useQuery({
    queryKey: keys.userSearch(profileId, q),
    queryFn: () => call(() => SearchUsers(profileId, q)),
    enabled: !!profileId && q.length >= 2,
    staleTime: 60_000,
  });
}
```

`useComponentCounts` shares its key with `useComponents`, whose query function returns the same `Bucket[]`; the `select` runs per observer, so the sidebar still sees buckets.

- [ ] **Step 6: Run the checks**

Run from `xtm/`: `go test . -run 'ComponentBindings|Capabilities' -count=1 && go vet .`
Run from `xtm/frontend`: `npx vitest run src/queries src/components/ContainersView.test.tsx && npx tsc --noEmit -p .`
Expected: PASS, clean.

- [ ] **Step 7: Commit**

```bash
git add xtm/app.go xtm/app_components_test.go xtm/frontend/wailsjs/go/main/App.js xtm/frontend/wailsjs/go/main/App.d.ts xtm/frontend/wailsjs/go/models.ts xtm/frontend/src/api.ts xtm/frontend/src/features.ts xtm/frontend/src/components/ContainersView.test.tsx xtm/frontend/src/queries
git commit -m "feat(xtm): expose component management to the frontend (#157)"
```

`models.ts` may carry line-ending churn from before this branch; stage it only if `git diff --ignore-cr-at-eol xtm/frontend/wailsjs/go/models.ts` shows the new classes, and check the staged diff (`git diff --cached --ignore-cr-at-eol`) contains nothing else.

---

### Task 7: Components view and its dialogs

**Files:**
- Create: `xtm/frontend/src/components/components-admin/ComponentsView.tsx`
- Create: `xtm/frontend/src/components/components-admin/ComponentFormModal.tsx`
- Create: `xtm/frontend/src/components/components-admin/DeleteComponentModal.tsx`
- Create: `xtm/frontend/src/components/components-admin/ComponentsView.test.tsx`
- Create: `xtm/frontend/src/components/components-admin/ComponentFormModal.test.tsx`
- Modify: `xtm/frontend/src/App.css` (a `/* Components view */` block)

New modules go in a domain folder (C5), hence `components-admin/`.

**Interfaces:**
- Consumes: Task 6 bindings and hooks; `Modal`, `useConfirm` from `@agile-suite/core`.
- Produces: `ComponentsView({ onChanged }: { onChanged: () => void })`.

```ts
// ComponentFormModal
interface ComponentFormModalProps {
  initial?: ProjectComponent;     // absent for New
  takenNames: string[];           // other components' names
  onSubmit: (in: ComponentInput) => Promise<void>;
  onCancel: () => void;
}
// DeleteComponentModal
interface DeleteComponentModalProps {
  component: ProjectComponent;
  others: ProjectComponent[];
  issueCount: number;
  onConfirm: (moveIssuesTo: string) => Promise<void>;
  onCancel: () => void;
}
```

- [ ] **Step 1: Write the failing form test**

`ComponentFormModal.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ComponentFormModal } from "./ComponentFormModal";

vi.mock("../../api", () => ({
  SearchUsers: vi.fn(async () => [{ name: "alice", displayName: "Alice Andersen" }]),
  errMsg: (e: unknown) => String(e),
}));
vi.mock("../../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

function renderForm(props: Partial<Parameters<typeof ComponentFormModal>[0]> = {}) {
  const onSubmit = props.onSubmit ?? vi.fn(async () => {});
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <ComponentFormModal takenNames={["Core"]} onSubmit={onSubmit} onCancel={() => {}} {...props} />
    </QueryClientProvider>,
  );
  return onSubmit;
}

describe("ComponentFormModal", () => {
  it("submits name, description, picked lead and assignee type", async () => {
    const onSubmit = renderForm();
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "API");
    await userEvent.type(screen.getByRole("textbox", { name: "Description" }), "REST layer");
    await userEvent.type(screen.getByRole("searchbox", { name: "Lead" }), "al");
    await userEvent.click(await screen.findByRole("button", { name: "Alice Andersen (alice)" }));
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Default assignee" }), "COMPONENT_LEAD");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(onSubmit).toHaveBeenCalledWith({
      name: "API",
      description: "REST layer",
      leadUserName: "alice",
      assigneeType: "COMPONENT_LEAD",
    });
  });

  it("refuses a blank or taken name without submitting", async () => {
    const onSubmit = renderForm();
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "   ");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(screen.getByText("A component needs a name.")).toBeTruthy();
    await userEvent.clear(screen.getByRole("textbox", { name: "Name" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "core");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(screen.getByText('A component named "core" already exists.')).toBeTruthy();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("edits an existing component and can clear its lead", async () => {
    const onSubmit = renderForm({
      initial: { id: "1", name: "Core", description: "d", leadName: "bob", leadDisplayName: "Bob Brown", assigneeType: "PROJECT_DEFAULT" },
      takenNames: ["API"],
    });
    expect((screen.getByRole("textbox", { name: "Name" }) as HTMLInputElement).value).toBe("Core");
    await userEvent.click(screen.getByRole("button", { name: "Clear lead" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).toHaveBeenCalledWith({
      name: "Core", description: "d", leadUserName: "", assigneeType: "PROJECT_DEFAULT",
    });
  });
});
```

- [ ] **Step 2: Write the failing view test**

`ComponentsView.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ComponentsView } from "./ComponentsView";

const api = vi.hoisted(() => ({
  ListProjectComponentDetails: vi.fn(),
  ListComponents: vi.fn(),
  CreateComponent: vi.fn(),
  UpdateComponent: vi.fn(),
  DeleteComponent: vi.fn(),
  ComponentIssueCount: vi.fn(),
  SearchUsers: vi.fn(async () => []),
}));
vi.mock("../../api", () => ({ ...api, errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)) }));
vi.mock("../../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1", activeProfile: { projectKey: "QA" } }),
}));

const core = { id: "1", name: "Core", description: "Engine", leadName: "bob", leadDisplayName: "Bob Brown", assigneeType: "PROJECT_DEFAULT" };
const apiComp = { id: "2", name: "API", description: "", leadName: "", leadDisplayName: "", assigneeType: "PROJECT_DEFAULT" };

function renderView(onChanged = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <ComponentsView onChanged={onChanged} />
    </QueryClientProvider>,
  );
  return onChanged;
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.ListProjectComponentDetails.mockResolvedValue([core, apiComp]);
  api.ListComponents.mockResolvedValue([{ label: "Core", count: 4 }]);
  api.SearchUsers.mockResolvedValue([]);
});

describe("ComponentsView", () => {
  it("lists components with lead and local test count", async () => {
    renderView();
    const row = (await screen.findByText("Core")).closest("tr")!;
    expect(within(row).getByText("Engine")).toBeTruthy();
    expect(within(row).getByText("Bob Brown")).toBeTruthy();
    expect(within(row).getByText("4")).toBeTruthy();
    const apiRow = screen.getByText("API").closest("tr")!;
    expect(within(apiRow).getByText("0")).toBeTruthy();
  });

  it("creates a component and reloads the list", async () => {
    api.CreateComponent.mockResolvedValue({ ...apiComp, id: "3", name: "Mobile" });
    const onChanged = renderView();
    await screen.findByText("Core");
    await userEvent.click(screen.getByRole("button", { name: "New component" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "Mobile");
    api.ListProjectComponentDetails.mockResolvedValue([core, apiComp, { ...apiComp, id: "3", name: "Mobile" }]);
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(api.CreateComponent).toHaveBeenCalledWith("p1", expect.objectContaining({ name: "Mobile" }));
    expect(await screen.findByText("Mobile")).toBeTruthy();
    expect(onChanged).toHaveBeenCalled();
  });

  it("deletes with a move target after showing the issue count", async () => {
    api.ComponentIssueCount.mockResolvedValue(5);
    api.DeleteComponent.mockResolvedValue(undefined);
    renderView();
    const row = (await screen.findByText("Core")).closest("tr")!;
    await userEvent.click(within(row).getByRole("button", { name: "Delete Core" }));
    expect(await screen.findByText(/5 issues use Core/)).toBeTruthy();
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Move its issues to" }), "2");
    await userEvent.click(screen.getByRole("button", { name: "Delete component" }));
    expect(api.DeleteComponent).toHaveBeenCalledWith("p1", "1", "2");
  });

  it("shows the admin-rights message and reloads after a failed write", async () => {
    api.UpdateComponent.mockRejectedValue(
      new Error("You need project admin rights in Jira to change components."),
    );
    renderView();
    const row = (await screen.findByText("Core")).closest("tr")!;
    await userEvent.click(within(row).getByRole("button", { name: "Edit Core" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(
      await screen.findByText("You need project admin rights in Jira to change components."),
    ).toBeTruthy();
    expect(api.ListProjectComponentDetails.mock.calls.length).toBeGreaterThanOrEqual(2);
  });
});
```

- [ ] **Step 3: Run them to see them fail**

Run from `xtm/frontend`: `npx vitest run src/components/components-admin`
Expected: FAIL, modules not found.

- [ ] **Step 4: Write ComponentFormModal.tsx**

```tsx
import { useId, useState } from "react";
import { Modal } from "@agile-suite/core";
import { useProfile } from "../../contexts/ProfileContext";
import { useUserSearch } from "../../queries/components";
import type { ComponentInput, ProjectComponent } from "../../api";
import { errMsg } from "../../api";

const ASSIGNEE_TYPES = [
  { value: "PROJECT_DEFAULT", label: "Project default" },
  { value: "COMPONENT_LEAD", label: "Component lead" },
  { value: "PROJECT_LEAD", label: "Project lead" },
  { value: "UNASSIGNED", label: "Unassigned" },
];

interface Props {
  initial?: ProjectComponent;
  takenNames: string[];
  onSubmit: (input: ComponentInput) => Promise<void>;
  onCancel: () => void;
}

// ComponentFormModal creates or edits one component. Name checks run here so
// a blank or duplicate name never reaches Jira; Jira checks again.
export function ComponentFormModal({ initial, takenNames, onSubmit, onCancel }: Props) {
  const { activeId: profileId } = useProfile();
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [lead, setLead] = useState({ name: initial?.leadName ?? "", display: initial?.leadDisplayName ?? "" });
  const [leadQuery, setLeadQuery] = useState("");
  const [assigneeType, setAssigneeType] = useState(initial?.assigneeType || "PROJECT_DEFAULT");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const users = useUserSearch(profileId, leadQuery);
  const titleId = useId();

  async function submit() {
    const trimmed = name.trim();
    if (!trimmed) {
      setError("A component needs a name.");
      return;
    }
    if (takenNames.some((n) => n.toLowerCase() === trimmed.toLowerCase())) {
      setError(`A component named "${trimmed}" already exists.`);
      return;
    }
    setSaving(true);
    setError("");
    try {
      await onSubmit({ name: trimmed, description: description.trim(), leadUserName: lead.name, assigneeType });
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal onClose={onCancel} className="modal bulk-modal" labelledBy={titleId}>
      <div className="pending-head">
        <h2 id={titleId}>{initial ? `Edit ${initial.name}` : "New component"}</h2>
        <button className="btn btn-ghost" onClick={onCancel} title="Close">
          ✕
        </button>
      </div>
      <div className="bulk-body">
        <label className="bulk-row">
          <span>Name</span>
          <input value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="bulk-row">
          <span>Description</span>
          <textarea rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <div className="bulk-row">
          <span>Lead</span>
          <div className="component-lead">
            {lead.name && (
              <p className="component-lead-current">
                {lead.display || lead.name}
                <button className="btn btn-ghost" onClick={() => setLead({ name: "", display: "" })}>
                  Clear lead
                </button>
              </p>
            )}
            <input
              type="search"
              aria-label="Lead"
              placeholder="Search users"
              value={leadQuery}
              onChange={(e) => setLeadQuery(e.target.value)}
            />
            {(users.data ?? []).length > 0 && (
              <ul className="component-lead-results">
                {(users.data ?? []).map((u) => (
                  <li key={u.name}>
                    <button
                      className="btn btn-ghost"
                      onClick={() => {
                        setLead({ name: u.name, display: u.displayName });
                        setLeadQuery("");
                      }}
                    >
                      {`${u.displayName} (${u.name})`}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
        <label className="bulk-row">
          <span>Default assignee</span>
          <select value={assigneeType} onChange={(e) => setAssigneeType(e.target.value)}>
            {ASSIGNEE_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </select>
        </label>
        {error && <div className="error-text">{error}</div>}
      </div>
      <div className="pending-actions">
        <button className="btn" onClick={onCancel} disabled={saving}>
          Cancel
        </button>
        <button className="btn btn-primary" onClick={submit} disabled={saving}>
          {initial ? "Save" : "Create"}
        </button>
      </div>
    </Modal>
  );
}
```

The "Lead" search input sits inside a `div`, not a `label`, so it carries its own `aria-label`; its `<span>Lead</span>` is a visual heading.

- [ ] **Step 5: Write DeleteComponentModal.tsx**

```tsx
import { useId, useState } from "react";
import { Modal } from "@agile-suite/core";
import type { ProjectComponent } from "../../api";
import { errMsg } from "../../api";

interface Props {
  component: ProjectComponent;
  others: ProjectComponent[];
  issueCount: number;
  onConfirm: (moveIssuesTo: string) => Promise<void>;
  onCancel: () => void;
}

// DeleteComponentModal confirms a delete. When issues use the component it
// offers to move them to another one first, which Jira does as part of the
// delete.
export function DeleteComponentModal({ component, others, issueCount, onConfirm, onCancel }: Props) {
  const [moveTo, setMoveTo] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const titleId = useId();

  async function confirm() {
    setBusy(true);
    setError("");
    try {
      await onConfirm(moveTo);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal onClose={onCancel} className="modal bulk-modal" role="alertdialog" labelledBy={titleId}>
      <div className="pending-head">
        <h2 id={titleId}>Delete {component.name}</h2>
      </div>
      <div className="bulk-body">
        <p>
          {issueCount === 0
            ? `No issues use ${component.name}.`
            : `${issueCount} ${issueCount === 1 ? "issue uses" : "issues use"} ${component.name} in Jira.`}
        </p>
        {issueCount > 0 && others.length > 0 && (
          <label className="bulk-row">
            <span>Move its issues to</span>
            <select value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
              <option value="">Nowhere (just remove the component)</option>
              {others.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </label>
        )}
        {error && <div className="error-text">{error}</div>}
      </div>
      <div className="pending-actions">
        <button className="btn" onClick={onCancel} disabled={busy}>
          Cancel
        </button>
        <button className="btn btn-danger" onClick={confirm} disabled={busy}>
          Delete component
        </button>
      </div>
    </Modal>
  );
}
```

Check `.btn-danger` exists in `App.css` (`grep -n "\.btn-danger" xtm/frontend/src/App.css`); if not, use the class `useConfirm`'s danger button uses in core and confirm it is defined in a stylesheet XTM loads.

The view test expects the text `5 issues use Core`; the sentence above renders "5 issues use Core in Jira.", which the test's regex matches.

- [ ] **Step 6: Write ComponentsView.tsx**

```tsx
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useProfile } from "../../contexts/ProfileContext";
import { useComponentCounts, useProjectComponents } from "../../queries/components";
import { keys } from "../../queries/keys";
import {
  ComponentIssueCount,
  CreateComponent,
  DeleteComponent,
  UpdateComponent,
  errMsg,
} from "../../api";
import type { ComponentInput, ProjectComponent } from "../../api";
import { ComponentFormModal } from "./ComponentFormModal";
import { DeleteComponentModal } from "./DeleteComponentModal";

type Dialog =
  | { kind: "none" }
  | { kind: "new" }
  | { kind: "edit"; component: ProjectComponent }
  | { kind: "delete"; component: ProjectComponent; issueCount: number };

// ComponentsView lists the project's Jira components and creates, edits and
// deletes them. Writes go straight to Jira; afterwards the list, the local
// test counts and the rest of the profile's data reload.
export function ComponentsView({ onChanged }: { onChanged: () => void }) {
  const { activeId: profileId } = useProfile();
  const qc = useQueryClient();
  const list = useProjectComponents(profileId);
  const counts = useComponentCounts(profileId);
  const [dialog, setDialog] = useState<Dialog>({ kind: "none" });
  const [error, setError] = useState("");

  const components = list.data ?? [];

  async function reload() {
    await qc.invalidateQueries({ queryKey: keys.components(profileId) });
    onChanged();
  }

  async function write(run: () => Promise<unknown>) {
    try {
      await run();
      setDialog({ kind: "none" });
      setError("");
    } finally {
      await reload();
    }
  }

  async function openDelete(c: ProjectComponent) {
    setError("");
    try {
      const issueCount = await ComponentIssueCount(profileId, c.id);
      setDialog({ kind: "delete", component: c, issueCount });
    } catch (e) {
      setError(errMsg(e));
    }
  }

  const others = (c: ProjectComponent) => components.filter((o) => o.id !== c.id);

  return (
    <div className="components-view">
      <div className="components-head">
        <h2>Components</h2>
        <button className="btn btn-primary" onClick={() => setDialog({ kind: "new" })}>
          New component
        </button>
      </div>
      {(error || list.error) && <div className="error-text">{error || errMsg(list.error)}</div>}
      {list.isLoading ? (
        <p className="muted">Loading components…</p>
      ) : components.length === 0 ? (
        <p className="muted">This project has no components yet.</p>
      ) : (
        <table className="components-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Description</th>
              <th>Lead</th>
              <th>Tests</th>
              <th>
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {components.map((c) => (
              <tr key={c.id}>
                <td>{c.name}</td>
                <td>{c.description}</td>
                <td>{c.leadDisplayName || c.leadName}</td>
                <td>{counts.data?.get(c.name) ?? 0}</td>
                <td className="components-actions">
                  <button
                    className="btn btn-ghost"
                    aria-label={`Edit ${c.name}`}
                    onClick={() => setDialog({ kind: "edit", component: c })}
                  >
                    Edit
                  </button>
                  <button className="btn btn-ghost" aria-label={`Delete ${c.name}`} onClick={() => openDelete(c)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {dialog.kind === "new" && (
        <ComponentFormModal
          takenNames={components.map((c) => c.name)}
          onSubmit={(input: ComponentInput) => write(() => CreateComponent(profileId, input))}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
      {dialog.kind === "edit" && (
        <ComponentFormModal
          initial={dialog.component}
          takenNames={others(dialog.component).map((c) => c.name)}
          onSubmit={(input: ComponentInput) =>
            write(() => UpdateComponent(profileId, dialog.component.id, input))
          }
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
      {dialog.kind === "delete" && (
        <DeleteComponentModal
          component={dialog.component}
          others={others(dialog.component)}
          issueCount={dialog.issueCount}
          onConfirm={(moveTo) => write(() => DeleteComponent(profileId, dialog.component.id, moveTo))}
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
    </div>
  );
}
```

A failed write rejects inside `write`, after `finally` has reloaded; the modal's own `catch` shows the error under its fields (that is where the 403 text appears in the test), and the list reload covers Review Focus item 3.

Check `.sr-only` is defined (`grep -rn "\.sr-only" xtm/frontend/src/*.css`). If not, add it in Step 7.

- [ ] **Step 7: Add the styles**

Append to `App.css`, using existing tokens only:

```css
/* Components view */
.components-view {
  padding: 12px 16px;
  overflow: auto;
}
.components-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.components-table {
  width: 100%;
  border-collapse: collapse;
}
.components-table th,
.components-table td {
  text-align: left;
  padding: 6px 8px;
  border-bottom: 1px solid var(--border);
}
.components-actions {
  white-space: nowrap;
  text-align: right;
}
.component-lead {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}
.component-lead-current {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
}
.component-lead-results {
  list-style: none;
  margin: 0;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 5px;
}
```

plus `.sr-only` if Step 6's check found none:

```css
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
```

- [ ] **Step 8: Run tests, lint, ratchet**

Run from `xtm/frontend`: `npx vitest run src/components/components-admin && npx tsc --noEmit -p .`
Run from the repo root: `npm run lint && bash scripts/ratchet.sh`
Expected: PASS; no counter rises (`bespoke_modals`, `eslint_a11y`, `ui_em_dashes`, `hardcoded_px_font`).

- [ ] **Step 9: Commit**

```bash
git add xtm/frontend/src/components/components-admin xtm/frontend/src/App.css
git commit -m "feat(xtm): Components view with create, edit and delete dialogs (#157)"
```

---

### Task 8: Register the view

**Files:**
- Modify: `xtm/frontend/src/contexts/NavContext.tsx` (`View` gains `"components"`)
- Modify: `xtm/frontend/src/App.tsx` (tab after Containers, gated by `caps.supportsComponentAdmin`; view branch; `"menu:view-components"` handler)
- Modify: `xtm/main.go` (View menu item)
- Modify: `xtm/frontend/src/contexts/NavContext.test.tsx` (switch to the new view)

- [ ] **Step 1: Write the failing NavContext test**

Add to `NavContext.test.tsx`, copying the existing `setView("coverage")` case:

```tsx
  it("switches to the Components view", () => {
    // render the provider the way the coverage case does, then:
    act(() => result.current.setView("components"));
    expect(result.current.view).toBe("components");
  });
```

It fails type-checking (`"components"` is not a `View`); run `npx tsc --noEmit -p .` from `xtm/frontend` to see the red, since Vitest strips types.

- [ ] **Step 2: Add the view**

`NavContext.tsx`: add `| "components"` to `View`.

`App.tsx`:
- Import `ComponentsView` from `./components/components-admin/ComponentsView`.
- After the Containers tab button:

```tsx
            {caps.supportsComponentAdmin && (
              <button
                data-tour="tab-components"
                className={`view-tab${view === "components" ? " view-tab-active" : ""}`}
                onClick={() => setView("components")}
              >
                Components
              </button>
            )}
```

- In the view switch, before `view === "coverage"`:

```tsx
      ) : view === "components" && caps.supportsComponentAdmin ? (
        <main className="content content-components">
          <ComponentsView
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
```

- In `menuActions.current`, add `"menu:view-components": () => setView("components"),`.

Check `content-components` against the stylesheet; if `.content-containers` has a rule, add a matching `.content-components` rule beside it with the same body, otherwise drop the extra class and use `className="content"`.

`main.go`: after the Containers item, `view.AddText("Components", nil, emit("menu:view-components"))`. On a Kiwi profile the menu item switches to a view the switch does not render (gated by caps), which falls through to the default branch; check what the final `: (` branch renders and, if it is Browse, that is acceptable.

- [ ] **Step 3: Run the checks**

Run from `xtm/frontend`: `npx tsc --noEmit -p . && npx vitest run`
Run from `xtm/`: `go build ./...`
Expected: clean and PASS.

- [ ] **Step 4: Commit**

```bash
git add xtm/frontend/src/contexts xtm/frontend/src/App.tsx xtm/main.go xtm/frontend/src/App.css
git commit -m "feat(xtm): add the Components view to the tabs and View menu (#157)"
```

---

### Task 9: User guide, full gates, PR

**Files:**
- Modify: `docs/user-guide/USER_GUIDE.md`

- [ ] **Step 1: Write the guide section**

Add a "Components" section after the Containers section (`grep -n "^## " docs/user-guide/USER_GUIDE.md`), and a line in the overview list near "Containers" pointing to it. Cover: where the view is (tab and View menu, Jira profiles only); the table columns and that Tests counts come from synced tests; New and Edit (name, description, lead search, default assignee); that changes go to Jira at once rather than through pending changes; Delete showing Jira's issue count and the move option; the refusal while pending component edits exist; the admin-rights message. Write it with the humanizer skill in file mode (AGENTS.project.md prose rule).

- [ ] **Step 2: Run the full gates**

Run from the repo root: `make gates`
Expected: PASS, apart from the known locale-dependent `tam/frontend` `App.test.tsx` case on this machine; name it in the PR if it still fails and this branch has not touched `tam`.

- [ ] **Step 3: Commit and open the PR**

```bash
git add docs/user-guide/USER_GUIDE.md docs/superpowers/plans/2026-10-09-xtm-components-part-c.md
git commit -m "docs(xtm): describe the Components view (#157)"
git push -u origin feat/xtm-components-admin
```

Open a PR against `main` whose body quotes #157's acceptance lines under `## Acceptance`, links the spec (on the part A branch / PR #160) under `## Spec` with the four deviations listed above, and lists the gates under `## Checks run`. No AI attribution.
