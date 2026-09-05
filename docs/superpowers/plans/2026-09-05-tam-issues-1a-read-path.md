# Task Activity Manager issues, plan 1a: the read path

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** TAM syncs a project's issues from Jira DC (or the demo dataset) into its own database and shows them in the Backlog grid with a read-only detail panel, on a Jira transport shared with XTM.

**Architecture:** `core/jira` is the transport lifted out of XTM's `jira` package (auth, TLS, paced requests, JSON helpers, generic issue reads); XTM's `jira.Client` embeds it and keeps every Xray method. TAM adds four internal packages: `backend` (a narrow `IssueBackend` interface with Jira and demo implementations), `demo` (a fixed Acme Platform dataset), `issuerepo` (the store layer over `tam.db` schema version 2), and `syncer` (the paging engine). The bound `App` grows eight methods. The frontend adds a query layer, a sync provider built on XTM's `syncMachine` reducer (which moves to `frontend/core`), the Backlog view with its virtualised table, and the detail panel.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, `golang.org/x/time/rate`, React 19, TanStack Query 5, `@tanstack/react-virtual` 3, Vite 8, Vitest 4, npm workspaces.

**Spec:** [`../specs/2026-09-05-tam-issues-design.md`](../specs/2026-09-05-tam-issues-design.md). **Mockup:** [`../specs/assets/2026-09-05-tam-backlog-read-path.svg`](../specs/assets/2026-09-05-tam-backlog-read-path.svg).

## Global Constraints

- Go modules stay `agile-suite/core`, `agile-suite/xtm`, `agile-suite/tam`; each app module carries `replace agile-suite/core => ../core`. Run Go commands from inside the module directory.
- XTM is never edited for TAM's sake beyond what this plan names: the `jira.Client` embedding in Task 1 (every Xray method body unchanged), the eight test-file constructor swaps in Task 1, and the `syncMachine` re-export shim in Task 8. Nothing else under `xtm/` changes.
- Task 1 lands as its own PR before Task 2 starts. Its gate is XTM's full Go suite (`go test ./internal/...` inside `xtm/`) and XTM's Vitest suite (191 tests) staying green with `go vet ./...` clean.
- Every later task leaves XTM's Go suite, every Vitest workspace, and `npm run typecheck --workspaces --if-present` green.
- `core` and `frontend/core` hold only what a task in this plan needs. `core/backend`, `core/journal`, and `core/demo` are not created. XTM's `SyncProvider`, `SelectionContext`, `NavContext`, `viewState`, and query files stay in XTM.
- Sync scope is `project = KEY AND issuetype in (<the five type names>)`, narrowed by `AND (<scope JQL>)` when the profile has one, plus `updated >= "<since minus one hour>"` on an incremental sync, `ORDER BY key ASC`.
- Logical issue types are exactly `task`, `epic`, `story`, `bug`, `requirement`. Jira names for the first four are `Task`, `Epic`, `Story`, `Bug`; the requirement name is the per-profile setting `requirement_issue_type`, default `Requirement`. Comparison is case-insensitive.
- TAM's database is `<user config dir>/task-activity-manager/tam.db`, schema version 2. Only `profiles.db` is shared. The Windows Credential Manager prefix and keyring service name stay as they are.
- The PAT is read from the credential store at backend construction and never written to SQLite or a log line.
- The sync progress event name is `tam:sync-progress`; its payload has the same shape as XTM's `syncer.Progress` (`phase`, `fetched`, `total`, `done`, `stage`).
- Detail fetches are cached in `issue.detail_json` with `detail_fetched_at`; a cached detail younger than ten minutes is served without a network call.
- Bound method signatures (Task 7) are exactly: `SyncIssues(profileID string, full bool) (syncer.Summary, error)`, `GetSyncState(profileID string) (issuerepo.SyncState, error)`, `ListIssues(profileID string, q issuerepo.IssueQuery) (issuerepo.IssuePage, error)`, `GetIssueDetail(profileID, key string) (backend.IssueDetail, error)`, `ListLinkedTests(profileID, key string) ([]issuerepo.LinkedTest, error)`, `ListSprints(profileID string) ([]issuerepo.SprintRef, error)`, `GetProfileSetting(profileID, key string) (string, error)`, `SetProfileSetting(profileID, key, value string) error`.
- UI text uses no em dashes. No AI attribution or mentions in any commit message, PR, file, or comment. Run the humanizer pass over prose, including code comments.
- Commit messages use the repo's conventional prefixes (`feat:`, `refactor:`, `test:`, `docs:`, `chore:`) with a scope in parentheses where one applies, and no trailers.
- The working tree holds untracked local tooling files (`agentdb.rvf`, `.claude/`, `reference/` and similar). Never add, commit, or delete them. Check state with `git status --short --untracked-files=no`. `wails build` rewrites line endings under `xtm/frontend/wailsjs` and `xtm/go.mod`; revert those with `git checkout -- xtm/` before committing.

## Decisions

1. **XTM's client keeps `baseURL` and `token` copies.** Twenty-odd Xray methods read `c.baseURL` for the demo check and one reads `c.token` for a raw multipart request. Copying the two strings onto XTM's struct at construction keeps every method body untouched, which is the whole point of embedding rather than rewriting.
2. **Unexported delegators in XTM.** XTM's methods call `c.get`, `c.post`, `c.do` and friends. Task 1 keeps those names as one-line wrappers over the embedded client's exported `Get`, `Post`, `Do`, so no call site changes.
3. **One `/rest/api/2/field` fetch per client.** `CustomFieldID` loads every custom field into the client's cache on first use, so discovering Sprint, Story Points, Epic Link, and Rank costs one request.
4. **DTOs live in `tam/internal/backend`.** `issuerepo` stores `backend.Issue` rows and caches `backend.IssueDetail` as JSON, so there is one issue shape from Jira to the grid.
5. **Full sync clears inside the first page's transaction.** `UpsertPage` takes a `clearFirst` flag; the engine passes it only for page one of a full sync, so a failure on that page leaves the previous data in place.
6. **`since` is the sync's start time.** Recording the start rather than the end as `last_synced` means an issue updated during a long sync is picked up next time. XTM's one-hour backdating is kept.
7. **The sync provider is TAM-local.** It uses the shared reducer and owns the orchestration (call, progress events, invalidation, error notice) because TAM has no commit path yet. XTM's provider stays where it is.
8. **Rank orders the grid.** `ORDER BY rank` with empty ranks last, then key, matches the Jira backlog order when the Rank field exists and degrades to key order when it does not.

## File structure

**Created**

- `core/jira/client.go`, `core/jira/errors.go`, `core/jira/issues.go`, `core/jira/client_test.go`, `core/jira/issues_test.go` (Task 1).
- `tam/internal/backend/backend.go` (Task 4).
- `tam/internal/demo/demo.go`, `tam/internal/demo/demo_test.go` (Task 4).
- `tam/internal/backend/demo/demo.go`, `tam/internal/backend/demo/demo_test.go` (Task 4).
- `tam/internal/backend/jira/jira.go`, `tam/internal/backend/jira/fields.go`, `tam/internal/backend/jira/jira_test.go`, `tam/internal/backend/jira/fields_test.go` (Task 5).
- `tam/internal/issuerepo/issuerepo.go`, `tam/internal/issuerepo/issues.go`, `tam/internal/issuerepo/detail.go`, `tam/internal/issuerepo/state.go`, and their tests (Tasks 2, 3).
- `tam/internal/syncer/syncer.go`, `tam/internal/syncer/syncer_test.go` (Task 6).
- `tam/app_issues.go` (Task 7).
- `frontend/core/src/contexts/syncMachine.ts`, `frontend/core/src/contexts/syncMachine.test.ts` (moved, Task 8).
- `tam/frontend/src/queries/keys.ts`, `queries/issues.ts`, `queries/invalidate.ts`, `queries/issues.test.tsx` (Task 9).
- `tam/frontend/src/contexts/SyncContext.tsx`, `contexts/SyncContext.test.tsx` (Task 9).
- `tam/frontend/src/components/TypeChip.tsx`, `IssueTable.tsx`, `BacklogView.tsx`, `BacklogView.test.tsx` (Task 10).
- `tam/frontend/src/components/IssueDetailPanel.tsx`, `IssueDetailPanel.test.tsx` (Task 11).

**Modified**

- `core/go.mod`, `core/go.sum` (Task 1).
- `xtm/internal/jira/client.go` and eight `xtm/internal/jira/*_test.go` files (Task 1).
- `tam/internal/tamstore/tamstore.go`, `tamstore_test.go` (Task 2).
- `tam/app.go` (Task 7), `tam/frontend/wailsjs/**` (regenerated, Task 7).
- `xtm/frontend/src/contexts/syncMachine.ts` (becomes a shim), `frontend/core/src/index.ts` (Task 8).
- `tam/frontend/package.json`, `tam/frontend/src/api.ts`, `App.tsx`, `App.css`, `App.test.tsx`, `main.tsx` (Tasks 9 to 12).
- `tam/frontend/src/components/ProfilesModal.tsx`, `ProfilesModal.test.tsx` (Task 12).
- `tam/CLAUDE.md`, `README.md` (Task 12).

---

### Task 1: `core/jira`, the shared transport, embedded by XTM

Lift the transport out of `xtm/internal/jira/client.go` into a new `core/jira` package and make XTM's `Client` embed it. Every Xray method keeps its body. This task is its own PR.

**Files:**
- Create: `core/jira/client.go`, `core/jira/errors.go`, `core/jira/issues.go`, `core/jira/client_test.go`, `core/jira/issues_test.go`
- Modify: `core/go.mod` (adds `golang.org/x/time`), `xtm/internal/jira/client.go`, and the eight test files listed in Step 6
- Test: `go test ./...` inside `core/`; `go test ./internal/...` and `go vet ./...` inside `xtm/`

**Interfaces:**
- Produces (package `agile-suite/core/jira`): `NewClient(baseURL, token string, opts ...Option) *Client`, `NewClientWithHTTP(baseURL, token string, h *http.Client) *Client`, `WithCACert(pem string) Option`, `WithInsecureTLS(b bool) Option`, `(*Client).BaseURL() string`, `Do`, `Get(ctx, path string, out any) error`, `GetBytes`, `GetBytesStatus`, `Put`, `Post`, `Delete`, `WriteJSON`, `WriteJSONReturning`, `type User`, `type HTTPError`, `type RawIssue`, `type SearchPage`, `type IssueType`, `SearchIssues(ctx, jql string, fields []string, startAt, maxResults int) (SearchPage, error)`, `GetIssue(ctx, key string, fields []string) (RawIssue, error)`, `IssueTypes(ctx, projectKey string) ([]IssueType, error)`, `CustomFieldID(ctx, name string) (string, error)`, `ErrFieldNotFound`, `Myself(ctx) (User, error)`.

- [ ] **Step 1: Write the failing transport tests**

Create `core/jira/client_test.go`:

```go
package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSendsBearerAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want Bearer tok", got)
		}
		if r.URL.Path != "/rest/api/2/myself" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"jdoe","displayName":"J. Doe","emailAddress":"j@x"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL+"/", "tok", srv.Client())
	if c.BaseURL() != srv.URL {
		t.Fatalf("BaseURL = %q, want trailing slash trimmed", c.BaseURL())
	}
	u, err := c.Myself(context.Background())
	if err != nil {
		t.Fatalf("Myself: %v", err)
	}
	if u.Name != "jdoe" || u.DisplayName != "J. Doe" {
		t.Errorf("user = %+v", u)
	}
}

func TestGetTurnsNon200IntoHTTPErrorWithJiraMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["Field 'foo' does not exist"],"errors":{"jql":"bad"}}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	var out map[string]any
	err := c.Get(context.Background(), "/rest/api/2/search", &out)
	var he *HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("want *HTTPError, got %T: %v", err, err)
	}
	if he.Code != 400 || he.Method != "GET" || he.Path != "/rest/api/2/search" {
		t.Errorf("HTTPError = %+v", he)
	}
	if he.Message != "Field 'foo' does not exist; jql: bad" {
		t.Errorf("Message = %q", he.Message)
	}
	if he.Error() != "jira: 400 Bad Request: Field 'foo' does not exist; jql: bad" {
		t.Errorf("Error() = %q", he.Error())
	}
}

func TestHTTPErrorFallsBackToMethodAndPath(t *testing.T) {
	e := &HTTPError{Method: "GET", Path: "/x", Code: 500, Status: "500 Internal Server Error"}
	if got := e.Error(); got != "jira: GET /x -> 500 Internal Server Error" {
		t.Errorf("Error() = %q", got)
	}
}

func TestWriteJSONReturningDecodesAndReportsFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("method %s content-type %s", r.Method, r.Header.Get("Content-Type"))
		}
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key":"PLAT-1"}`))
			return
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`nope`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	var out struct {
		Key string `json:"key"`
	}
	if err := c.WriteJSONReturning(context.Background(), http.MethodPost, "/ok", map[string]string{"a": "b"}, &out); err != nil {
		t.Fatalf("post ok: %v", err)
	}
	if out.Key != "PLAT-1" {
		t.Errorf("decoded key = %q", out.Key)
	}
	err := c.Post(context.Background(), "/bad", map[string]string{})
	if err == nil || err.Error() != "jira: POST /bad -> 409 Conflict: nope" {
		t.Errorf("post bad err = %v", err)
	}
}

func TestDeleteAcceptsAny2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.Delete(context.Background(), "/rest/api/2/issue/PLAT-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestGetBytesStatusReturnsBodyAndStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/raw" {
			_, _ = w.Write([]byte(`[1,2]`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`not a test`))
	}))
	defer srv.Close()
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	body, code, err := c.GetBytesStatus(context.Background(), "/raw")
	if err != nil || code != 200 || string(body) != "[1,2]" {
		t.Fatalf("raw: %q %d %v", body, code, err)
	}
	_, code, err = c.GetBytesStatus(context.Background(), "/bad")
	if code != 400 || err == nil || err.Error() != "jira: GET /bad -> 400 Bad Request: not a test" {
		t.Errorf("bad: %d %v", code, err)
	}
}

func TestNilHTTPClientUsesDefaultTimeout(t *testing.T) {
	c := NewClientWithHTTP("https://jira.example", "t", nil)
	if c.http == nil || c.http.Timeout == 0 {
		t.Fatalf("expected a default http client with a timeout, got %+v", c.http)
	}
}
```

Create `core/jira/issues_test.go`:

```go
package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSearchIssuesPassesPagingAndFieldsAndDecodesRawFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("jql") != `project = PLAT ORDER BY key ASC` {
			t.Errorf("jql = %q", q.Get("jql"))
		}
		if q.Get("startAt") != "50" || q.Get("maxResults") != "25" || q.Get("fields") != "summary,status" {
			t.Errorf("paging/fields = %v", q)
		}
		_, _ = w.Write([]byte(`{"total":77,"issues":[{"id":"10001","key":"PLAT-1","fields":{"summary":"One","status":{"name":"To Do"}}}]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	page, err := c.SearchIssues(context.Background(), "project = PLAT ORDER BY key ASC", []string{"summary", "status"}, 50, 25)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if page.Total != 77 || len(page.Issues) != 1 {
		t.Fatalf("page = %+v", page)
	}
	iss := page.Issues[0]
	if iss.ID != "10001" || iss.Key != "PLAT-1" {
		t.Errorf("issue = %+v", iss)
	}
	if string(iss.Fields["summary"]) != `"One"` {
		t.Errorf("summary raw = %s", iss.Fields["summary"])
	}
}

func TestGetIssueRequestsFieldsAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/PLAT-9" || r.URL.Query().Get("fields") != "description,issuelinks" {
			t.Errorf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"id":"9","key":"PLAT-9","fields":{"description":"d"}}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	iss, err := c.GetIssue(context.Background(), "PLAT-9", []string{"description", "issuelinks"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.Key != "PLAT-9" || string(iss.Fields["description"]) != `"d"` {
		t.Errorf("issue = %+v", iss)
	}
}

func TestIssueTypesReadsTheProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/project/PLAT" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"key":"PLAT","issueTypes":[{"id":"1","name":"Task","subtask":false},{"id":"5","name":"Sub-task","subtask":true}]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	types, err := c.IssueTypes(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("types: %v", err)
	}
	if len(types) != 2 || types[0].Name != "Task" || !types[1].Subtask {
		t.Errorf("types = %+v", types)
	}
}

func TestCustomFieldIDLoadsOnceAndReportsAbsence(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.URL.Path != "/rest/api/2/field" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"id":"summary","name":"Summary","custom":false},{"id":"customfield_10020","name":"Sprint","custom":true},{"id":"customfield_10016","name":"Story Points","custom":true}]`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	ctx := context.Background()
	id, err := c.CustomFieldID(ctx, "sprint")
	if err != nil || id != "customfield_10020" {
		t.Fatalf("Sprint = %q, %v", id, err)
	}
	id, err = c.CustomFieldID(ctx, "Story Points")
	if err != nil || id != "customfield_10016" {
		t.Fatalf("Story Points = %q, %v", id, err)
	}
	if _, err := c.CustomFieldID(ctx, "Epic Link"); !errors.Is(err, ErrFieldNotFound) {
		t.Fatalf("Epic Link err = %v, want ErrFieldNotFound", err)
	}
	if _, err := c.CustomFieldID(ctx, "Summary"); !errors.Is(err, ErrFieldNotFound) {
		t.Fatalf("a system field is not a custom field: err = %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("/rest/api/2/field fetched %d times, want 1", n)
	}
}

func TestCustomFieldIDRejectsEmptyName(t *testing.T) {
	c := NewClientWithHTTP("https://jira.example", "t", nil)
	if _, err := c.CustomFieldID(context.Background(), "  "); err == nil {
		t.Fatal("want an error for an empty field name")
	}
}
```

- [ ] **Step 2: Run them to see the compile failure**

Run (inside `core/`): `go test ./jira/`
Expected: FAIL, `no Go files in` or `undefined: NewClientWithHTTP` (the package does not exist yet).

- [ ] **Step 3: Add the dependency and write the transport**

Inside `core/`, run `go get golang.org/x/time@v0.15.0` (the version XTM pins). Then create `core/jira/client.go`:

```go
// Package jira is the transport the suite's apps share for talking to Jira
// Data Center: personal access token auth, TLS options, paced requests, the
// JSON helpers, and the handful of generic issue reads both apps need. It
// targets Jira DC 8.14+ and /rest/api/2/. Everything Xray-specific lives in
// XTM, which embeds this client.
package jira

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Request pacing: a token-bucket limiter governs every outbound call (see
// Client.Do), so concurrent per-item fetches stay within a safe rate.
const (
	reqPerSec = 20 // sustained requests per second across all calls on one client
	burst     = 10 // allowed burst above the sustained rate
)

// Client talks to a single Jira Data Center instance.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	// limiter paces every outbound request. Shared across goroutines so
	// concurrent fetches respect one global rate.
	limiter *rate.Limiter

	// fieldMu guards fieldIDs, the per-instance cache of custom field ids by
	// lower-cased name, filled from one /rest/api/2/field fetch.
	fieldMu       sync.Mutex
	fieldIDs      map[string]string
	fieldsLoaded  bool
}

// User is the subset of /rest/api/2/myself the apps need to confirm a
// connection.
type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}

// HTTPError carries the HTTP status of a failed request so callers can treat
// specific statuses as soft failures.
type HTTPError struct {
	Method  string
	Path    string
	Code    int
	Status  string
	Message string // readable Jira error, parsed from the response body
}

func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("jira: %s: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("jira: %s %s -> %s", e.Method, e.Path, e.Status)
}

// clientConfig holds optional TLS settings.
type clientConfig struct {
	caCertPEM string
	insecure  bool
}

// Option is a functional option for NewClient.
type Option func(*clientConfig)

// WithCACert adds a PEM-encoded CA certificate to the TLS trust pool. The
// system pool is the base, so existing roots are preserved. An empty or
// unparseable PEM adds nothing.
func WithCACert(pem string) Option {
	return func(c *clientConfig) { c.caCertPEM = pem }
}

// WithInsecureTLS disables TLS certificate verification, hostname checks
// included. Only for trusted internal servers with no CA certificate.
func WithInsecureTLS(b bool) Option {
	return func(c *clientConfig) { c.insecure = b }
}

// buildHTTPClient returns the plain 30 second client when no TLS option is
// set, otherwise a clone of the default transport with a custom TLS config so
// pooling and keep-alives are preserved.
func buildHTTPClient(cfg clientConfig) *http.Client {
	if cfg.caCertPEM == "" && !cfg.insecure {
		return &http.Client{Timeout: 30 * time.Second}
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if cfg.caCertPEM != "" {
		pool.AppendCertsFromPEM([]byte(cfg.caCertPEM))
	}

	base := http.DefaultTransport.(*http.Transport).Clone()
	base.TLSClientConfig = &tls.Config{
		RootCAs:            pool,
		InsecureSkipVerify: cfg.insecure, //nolint:gosec // user-controlled escape hatch
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: base}
}

// NewClient builds a client for baseURL (the instance root, for example
// https://jira.example.com) authenticated with a personal access token.
func NewClient(baseURL, token string, opts ...Option) *Client {
	var cfg clientConfig
	for _, o := range opts {
		o(&cfg)
	}
	return NewClientWithHTTP(baseURL, token, buildHTTPClient(cfg))
}

// NewClientWithHTTP builds a client on a caller-supplied http.Client, which
// tests use to point at an httptest server. A nil h uses the default client.
func NewClientWithHTTP(baseURL, token string, h *http.Client) *Client {
	if h == nil {
		h = buildHTTPClient(clientConfig{})
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    h,
		limiter: rate.NewLimiter(reqPerSec, burst),
	}
}

// BaseURL is the instance root with any trailing slash removed.
func (c *Client) BaseURL() string { return c.baseURL }

// Do sends a request after waiting for a slot from the rate limiter. It is
// the single throttle point every helper goes through. A nil limiter is a
// no-op so hand-built clients still work.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.limiter != nil {
		if err := c.limiter.Wait(req.Context()); err != nil {
			return nil, err
		}
	}
	return c.http.Do(req)
}

// Myself fetches the authenticated user, which is the connection test.
func (c *Client) Myself(ctx context.Context) (User, error) {
	var u User
	if err := c.Get(ctx, "/rest/api/2/myself", &u); err != nil {
		return User{}, err
	}
	return u, nil
}

// Get performs an authenticated GET and decodes a JSON response into out. Any
// status other than 200 becomes an *HTTPError carrying Jira's message.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := jiraErrorMessage(body)
		return &HTTPError{Method: http.MethodGet, Path: path, Code: resp.StatusCode, Status: resp.Status, Message: msg}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// GetBytes performs an authenticated GET and returns the raw body, for
// responses whose shape has to be sniffed before decoding.
func (c *Client) GetBytes(ctx context.Context, path string) ([]byte, error) {
	body, _, err := c.GetBytesStatus(ctx, path)
	return body, err
}

// GetBytesStatus is GetBytes plus the HTTP status code, for callers that treat
// a particular status as data rather than failure. The error text stays what
// GetBytes has always produced.
func (c *Client) GetBytesStatus(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("jira: GET %s -> %s: %s", path, resp.Status, snippet(body, 1024))
	}
	return body, resp.StatusCode, nil
}

// Put performs an authenticated JSON PUT.
func (c *Client) Put(ctx context.Context, path string, body any) error {
	return c.WriteJSON(ctx, http.MethodPut, path, body)
}

// Post performs an authenticated JSON POST.
func (c *Client) Post(ctx context.Context, path string, body any) error {
	return c.WriteJSON(ctx, http.MethodPost, path, body)
}

// Delete performs an authenticated DELETE with no body. Any 2xx is success.
func (c *Client) Delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf(
			"jira: DELETE %s -> %s: %s",
			path, resp.Status, strings.TrimSpace(string(respBody)),
		)
	}
	return nil
}

// WriteJSON marshals body and sends it with method, discarding the response.
func (c *Client) WriteJSON(ctx context.Context, method, path string, body any) error {
	return c.WriteJSONReturning(ctx, method, path, body, nil)
}

// WriteJSONReturning marshals body, sends it with method, and decodes a 2xx
// response into out when out is non-nil and a body is present. A non-2xx
// status returns an error carrying a short slice of the response body.
func (c *Client) WriteJSONReturning(ctx context.Context, method, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx, method, c.baseURL+path, bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf(
			"jira: %s %s -> %s: %s",
			method, path, resp.Status, strings.TrimSpace(string(respBody)),
		)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
```

Create `core/jira/errors.go`:

```go
package jira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// jiraErrorMessage pulls the readable parts out of a Jira error body:
// errorMessages, then the errors map in key order, then error and message.
func jiraErrorMessage(body []byte) string {
	var e struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
		Error         string            `json:"error"`
		Message       string            `json:"message"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return ""
	}
	parts := append([]string{}, e.ErrorMessages...)
	keys := make([]string, 0, len(e.Errors))
	for k := range e.Errors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, e.Errors[k]))
	}
	if e.Error != "" {
		parts = append(parts, e.Error)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, "; ")
}

// snippet trims a response body for an error message.
func snippet(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}
```

Create `core/jira/issues.go`:

```go
package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// RawIssue is one issue as Jira returns it: key, id, and the fields object
// with each value left as raw JSON for the caller to decode.
type RawIssue struct {
	ID     string                     `json:"id"`
	Key    string                     `json:"key"`
	Fields map[string]json.RawMessage `json:"fields"`
}

// SearchPage is one page of a JQL search plus the total match count.
type SearchPage struct {
	Issues []RawIssue `json:"issues"`
	Total  int        `json:"total"`
}

// IssueType is one entry of a project's issue type list.
type IssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// ErrFieldNotFound is returned by CustomFieldID when the instance has no
// custom field with the requested name.
var ErrFieldNotFound = errors.New("jira: custom field not found")

// SearchIssues runs one page of /rest/api/2/search. fields names the fields
// to return; an empty list asks Jira for its default set.
func (c *Client) SearchIssues(ctx context.Context, jql string, fields []string, startAt, maxResults int) (SearchPage, error) {
	q := url.Values{}
	q.Set("jql", jql)
	q.Set("startAt", strconv.Itoa(startAt))
	q.Set("maxResults", strconv.Itoa(maxResults))
	if len(fields) > 0 {
		q.Set("fields", strings.Join(fields, ","))
	}
	var page SearchPage
	if err := c.Get(ctx, "/rest/api/2/search?"+q.Encode(), &page); err != nil {
		return SearchPage{}, err
	}
	return page, nil
}

// GetIssue fetches one issue by key with the named fields.
func (c *Client) GetIssue(ctx context.Context, key string, fields []string) (RawIssue, error) {
	path := "/rest/api/2/issue/" + url.PathEscape(key)
	if len(fields) > 0 {
		path += "?fields=" + url.QueryEscape(strings.Join(fields, ","))
	}
	var iss RawIssue
	if err := c.Get(ctx, path, &iss); err != nil {
		return RawIssue{}, err
	}
	return iss, nil
}

// IssueTypes lists the issue types available in a project.
func (c *Client) IssueTypes(ctx context.Context, projectKey string) ([]IssueType, error) {
	var project struct {
		IssueTypes []IssueType `json:"issueTypes"`
	}
	if err := c.Get(ctx, "/rest/api/2/project/"+url.PathEscape(projectKey), &project); err != nil {
		return nil, err
	}
	return project.IssueTypes, nil
}

// CustomFieldID returns the customfield_NNNNN id for a custom field name,
// compared case-insensitively. The instance's field list is fetched once per
// client and cached, so resolving several names costs one request.
func (c *Client) CustomFieldID(ctx context.Context, name string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		return "", fmt.Errorf("jira: custom field name is empty")
	}
	c.fieldMu.Lock()
	loaded := c.fieldsLoaded
	c.fieldMu.Unlock()
	if !loaded {
		var fields []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Custom bool   `json:"custom"`
		}
		if err := c.Get(ctx, "/rest/api/2/field", &fields); err != nil {
			return "", err
		}
		ids := make(map[string]string, len(fields))
		for _, f := range fields {
			if f.Custom {
				ids[strings.ToLower(strings.TrimSpace(f.Name))] = f.ID
			}
		}
		c.fieldMu.Lock()
		c.fieldIDs = ids
		c.fieldsLoaded = true
		c.fieldMu.Unlock()
	}
	c.fieldMu.Lock()
	id, ok := c.fieldIDs[want]
	c.fieldMu.Unlock()
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrFieldNotFound, name)
	}
	return id, nil
}
```

- [ ] **Step 4: Run the core tests**

Run (inside `core/`): `go vet ./jira/ && go test ./jira/ -v`
Expected: PASS for all twelve tests.

- [ ] **Step 5: Point XTM's client at the shared transport**

Replace `xtm/internal/jira/client.go` in full with:

```go
// Package jira is the REST client for Jira Data Center and Xray Server / DC.
//
// The transport (auth, TLS, request pacing, JSON helpers) is the suite's
// shared core/jira client, embedded below. This package adds the Xray
// endpoints (/rest/raven/2.0/) and the per-instance caches XTM's sync and
// commit passes rely on. It targets Xray Server / DC 8.4.0.
package jira

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	corejira "agile-suite/core/jira"
)

// Client talks to a single Jira Data Center instance with Xray.
type Client struct {
	*corejira.Client
	// baseURL and token are copies of what the embedded client holds. The
	// Xray methods read c.baseURL for the demo checks and c.token for the one
	// raw multipart request, so keeping the two here leaves their bodies as
	// they were.
	baseURL string
	token   string

	// precondTypeOnce lazily resolves and caches the Precondition issue type
	// for this instance (its name varies / may be localised), so the JQL search
	// and the create call both target the right type.
	precondTypeOnce sync.Once
	precondTypeID   string
	precondTypeName string
	precondTypeErr  error

	// testTypeOnce lazily resolves and caches the plain "Test" issue type id for
	// this instance, used when creating new Tests (FR-1).
	testTypeOnce sync.Once
	testTypeID   string
	testTypeName string
	testTypeErr  error

	// subTaskTEOnce lazily resolves and caches the issue type name(s) used for
	// sub-task Test Executions on this instance. Their name varies (default "Sub
	// Test Execution", but instances may rename / localise it), so they are
	// discovered from the instance issue type list rather than hardcoded.
	subTaskTEOnce  sync.Once
	subTaskTENames []string

	// customFieldMu guards customFieldIDs, the per-instance cache of resolved
	// custom field ids keyed by field name (see resolveCustomFieldID), so a sync
	// or commit resolves a given field (e.g. "Test Type") from /rest/api/2/field
	// at most once.
	customFieldMu  sync.Mutex
	customFieldIDs map[string]string
	// customFieldTypes caches the coarse schema type of every custom field on
	// the instance, keyed by field id (see customFieldType), filled from one
	// /rest/api/2/field fetch so a commit pushing several custom field edits
	// resolves each field's type without re-fetching.
	customFieldTypes map[string]string
	// customFieldTypesLoaded records that the one-shot /rest/api/2/field type
	// fetch has run, so an unknown id (absent from customFieldTypes) does not
	// trigger a redundant re-fetch.
	customFieldTypesLoaded bool

	// bugLinkTypeOnce lazily resolves and caches the issue-link type CreateBugLink
	// uses (a defect-oriented type if the instance defines one, else "Relates"),
	// so linking many bugs in one commit resolves the type just once.
	bugLinkTypeOnce sync.Once
	bugLinkTypeName string
	bugLinkTypeErr  error

	// reqLinkTypeOnce lazily resolves and caches the Requirement->Requirement
	// issue-link type used by UpdateRequirementLinks. Preferred candidates:
	// "requires", "Requires", "depends on", "Depends".
	reqLinkTypeOnce sync.Once
	reqLinkTypeName string
	reqLinkTypeErr  error

	// requirementLinkType is the configured issue-link type for Test->Requirement
	// coverage links (e.g. "Tested By"). When non-empty it overrides
	// resolveRequirementLinkType; set at construction from the persisted setting.
	requirementLinkType string

	// covLinkTypeOnce lazily resolves and caches the issue-link type for
	// Test->Requirement coverage when no explicit type is configured.
	// Preferred candidates: "tested by", "tests", "relates".
	covLinkTypeOnce sync.Once
	covLinkTypeName string
	covLinkTypeErr  error

	// currentUserOnce lazily resolves and caches the authenticated user's
	// username (PAT owner) via GET /rest/api/2/myself, so CreateBug can set the
	// reporter field without an extra round-trip on every call.
	currentUserOnce sync.Once
	currentUserName string
	currentUserErr  error
}

// User is the subset of /rest/api/2/myself the app needs to confirm a connection.
type User = corejira.User

// HTTPError carries the HTTP status of a failed Jira request so callers can
// treat specific statuses as soft failures.
type HTTPError = corejira.HTTPError

// Option is a functional option for NewClient.
type Option = corejira.Option

// WithCACert adds a PEM-encoded CA certificate to the TLS trust pool.
func WithCACert(pem string) Option { return corejira.WithCACert(pem) }

// WithInsecureTLS disables TLS certificate verification.
func WithInsecureTLS(b bool) Option { return corejira.WithInsecureTLS(b) }

// NewClient builds a client for the given Jira base URL authenticated with a
// Personal Access Token. baseURL is the instance root, e.g.
// https://jira.example.com. Pass WithCACert or WithInsecureTLS to override the
// default system TLS trust (FR-8.4 / RND_P_4TFINT_05-243).
func NewClient(baseURL, token string, opts ...Option) *Client {
	return wrap(corejira.NewClient(baseURL, token, opts...), token)
}

// newClientWith builds a client on a caller-supplied http.Client; tests use
// it to point at an httptest server. A nil h uses the default client.
func newClientWith(baseURL, token string, h *http.Client) *Client {
	return wrap(corejira.NewClientWithHTTP(baseURL, token, h), token)
}

func wrap(core *corejira.Client, token string) *Client {
	return &Client{Client: core, baseURL: core.BaseURL(), token: token}
}

// The helpers below keep every Xray method's body unchanged: they are the
// names those methods have always called, delegating to the shared transport.

func (c *Client) do(req *http.Request) (*http.Response, error) { return c.Client.Do(req) }

func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.Client.Get(ctx, path, out)
}

func (c *Client) getBytes(ctx context.Context, path string) ([]byte, error) {
	return c.Client.GetBytes(ctx, path)
}

func (c *Client) getBytesStatus(ctx context.Context, path string) ([]byte, int, error) {
	return c.Client.GetBytesStatus(ctx, path)
}

func (c *Client) put(ctx context.Context, path string, body any) error {
	return c.Client.Put(ctx, path, body)
}

func (c *Client) post(ctx context.Context, path string, body any) error {
	return c.Client.Post(ctx, path, body)
}

func (c *Client) delete(ctx context.Context, path string) error {
	return c.Client.Delete(ctx, path)
}

func (c *Client) writeJSON(ctx context.Context, method, path string, body any) error {
	return c.Client.WriteJSON(ctx, method, path, body)
}

func (c *Client) writeJSONReturning(ctx context.Context, method, path string, body, out any) error {
	return c.Client.WriteJSONReturning(ctx, method, path, body, out)
}

// IsDemo reports whether this client is in demo mode (no Jira network calls).
func (c *Client) IsDemo() bool { return isDemoURL(c.baseURL) }

// SetRequirementLinkType configures the issue-link type name used when linking
// a Test to a Requirement (FR-13 / #275). When non-empty it takes precedence
// over the auto-resolved type from resolveRequirementLinkType. Call this once
// at construction; the field is not guarded by a mutex.
func (c *Client) SetRequirementLinkType(name string) {
	c.requirementLinkType = name
}

// TestConnection verifies the base URL and token by fetching the current user
// (FR-8.4). It returns the authenticated user on success. Demo URLs
// short-circuit to a fake user so the UI can be exercised without Jira.
func (c *Client) TestConnection(ctx context.Context) (*User, error) {
	if isDemoURL(c.baseURL) {
		return &User{Name: "demo", DisplayName: "Demo User", Email: "demo@local"}, nil
	}
	var u User
	if err := c.get(ctx, "/rest/api/2/myself", &u); err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}
	return &u, nil
}

// currentUser resolves and caches the authenticated user's username (PAT
// owner) via GET /rest/api/2/myself. The result is computed at most once per
// client (sync.Once). Demo mode returns the synthetic username "demo.user".
//
// NOTE(xtm): Jira DC REST v2 identifies users by their login name ("name").
// Some newer Jira instances (Server/DC migrated from Cloud) may use "accountId"
// instead; verify against the live Xray Server/DC 8.4.0 instance and adjust
// the reporter field key in CreateBug if needed.
func (c *Client) currentUser(ctx context.Context) (string, error) {
	c.currentUserOnce.Do(func() {
		if isDemoURL(c.baseURL) {
			c.currentUserName = "demo.user"
			return
		}
		var u User
		if e := c.get(ctx, "/rest/api/2/myself", &u); e != nil {
			c.currentUserErr = e
			return
		}
		c.currentUserName = u.Name
	})
	return c.currentUserName, c.currentUserErr
}
```

XTM's `jiraErrorMessage` and `snippet` stay in `xtm/internal/jira/steps.go`; `containers.go`, `folders.go`, and `steps.go` still call them.

- [ ] **Step 6: Swap the eight test constructors**

Each of these test files builds the client as a struct literal, which no longer compiles. Replace each literal with `newClientWith`, changing nothing else in the line:

| File and line | Old | New |
|---|---|---|
| `xtm/internal/jira/comments_realpath_test.go:65` | `c := &Client{baseURL: "demo", token: "t", http: srv.Client()}` | `c := newClientWith("demo", "t", srv.Client())` |
| `xtm/internal/jira/createrequirement_test.go:139` | `demo := &Client{baseURL: "demo", token: "t", http: http.DefaultClient}` | `demo := newClientWith("demo", "t", http.DefaultClient)` |
| `xtm/internal/jira/preconditions_test.go:17` | `return &Client{baseURL: srv.URL, token: "t", http: srv.Client()}` | `return newClientWith(srv.URL, "t", srv.Client())` |
| `xtm/internal/jira/requirements_realpath_test.go:196` | `demo := &Client{baseURL: "demo", token: "t", http: srv.Client()}` | `demo := newClientWith("demo", "t", srv.Client())` |
| `xtm/internal/jira/requirements_realpath_test.go:237` | same | same |
| `xtm/internal/jira/testpreconditions_test.go:163` | `c := &Client{baseURL: "demo", token: "t", http: http.DefaultClient}` | `c := newClientWith("demo", "t", http.DefaultClient)` |
| `xtm/internal/jira/testpreconditions_test.go:201` | same | same |
| `xtm/internal/jira/testtypes_fieldvalue_test.go:9` | `c := &Client{baseURL: "demo"} // demo short-circuits resolveCustomFieldID to ""` | `c := newClientWith("demo", "", nil) // demo short-circuits resolveCustomFieldID to ""` |

Run `grep -rn "&Client{" xtm/internal/jira/` afterwards; expected: no matches. If a test file no longer uses the `net/http` import after the swap, remove that import.

- [ ] **Step 7: Prove XTM did not move**

Run, inside `xtm/`:

```bash
go build ./... && go vet ./... && go test ./internal/... -count=1
```

Expected: every package `ok`, including `internal/jira` (the httperror tests now exercise the embedded transport through the `HTTPError` alias). Then, from the repo root: `npm test --workspace xtm/frontend` still reports 191 passing (no frontend file changed; this confirms the tree is intact).

- [ ] **Step 8: Commit and open the extraction PR**

```bash
git add core/go.mod core/go.sum core/jira xtm/internal/jira
git commit -m "refactor(core): lift the Jira transport into core/jira and embed it in XTM's client"
```

Push the branch and open a PR titled "Lift the Jira transport into core/jira" whose description says what moved, that every Xray method body is unchanged, and that the eight test constructors were swapped. The remaining tasks continue on a branch created from this one; they do not wait for the merge, but this PR merges first.

---

### Task 2: Issue shapes, schema version 2, and the issue store

Define the one issue shape the rest of the plan uses, bump TAM's database to version 2 with the four new tables, and write the store methods the grid and the sync need.

**Files:**
- Create: `tam/internal/backend/backend.go`, `tam/internal/issuerepo/issuerepo.go`, `tam/internal/issuerepo/issues.go`, `tam/internal/issuerepo/issues_test.go`
- Modify: `tam/internal/tamstore/tamstore.go`, `tam/internal/tamstore/tamstore_test.go`
- Test: `go test ./internal/...` inside `tam/`

**Interfaces:**
- Produces (package `backend`): `Issue`, `Link`, `IssueDetail`, `IssueType`, `User`, the `IssueBackend` interface, the type constants `TypeTask`, `TypeEpic`, `TypeStory`, `TypeBug`, `TypeRequirement`, and `AllTypes []string`.
- Produces (package `issuerepo`): `New(db *sql.DB) *Repository`, `ErrNotFound`, `IssueQuery`, `IssuePage`, `SprintRef`, `(*Repository).UpsertPage(ctx, profileID string, page []backend.Issue, syncedAt time.Time, clearFirst bool) error`, `ListIssues(ctx, profileID string, q IssueQuery) (IssuePage, error)`, `GetIssue(ctx, profileID, key string) (backend.Issue, error)`, `CountIssues(ctx, profileID string) (int, error)`, `ListSprints(ctx, profileID string) ([]SprintRef, error)`.

- [ ] **Step 1: The shapes**

Create `tam/internal/backend/backend.go`:

```go
// Package backend is the seam between Task Activity Manager and the system
// that holds its issues. IssueBackend is deliberately small: what the read
// path needs, and nothing the write path will add later. The Jira
// implementation lives in backend/jira, the offline one in backend/demo.
package backend

import "context"

// Logical issue types. Every issue TAM manages is one of these five; the
// Jira names they map to are the backend's business.
const (
	TypeTask        = "task"
	TypeEpic        = "epic"
	TypeStory       = "story"
	TypeBug         = "bug"
	TypeRequirement = "requirement"
)

// AllTypes is the five logical types in display order.
var AllTypes = []string{TypeTask, TypeEpic, TypeStory, TypeBug, TypeRequirement}

// Issue is one row of the Backlog: the columns the grid shows plus what sync
// needs to keep it current. StoryPoints is nil when the issue has none.
type Issue struct {
	Key         string   `json:"key"`
	ID          string   `json:"id"`
	Project     string   `json:"project"`
	Type        string   `json:"type"`
	Summary     string   `json:"summary"`
	Status      string   `json:"status"`
	Assignee    string   `json:"assignee"`
	Reporter    string   `json:"reporter"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels"`
	SprintID    string   `json:"sprintId"`
	SprintName  string   `json:"sprintName"`
	ParentKey   string   `json:"parentKey"`
	StoryPoints *float64 `json:"storyPoints"`
	Rank        string   `json:"rank"`
	Created     string   `json:"created"`
	Updated     string   `json:"updated"`
}

// Link is one issue link seen from the issue that owns it.
type Link struct {
	Direction string `json:"direction"` // "outward" or "inward"
	Type      string `json:"type"`      // the Jira link type name, e.g. "Tested By"
	Key       string `json:"key"`       // the other issue
	Summary   string `json:"summary"`
	IssueType string `json:"issueType"` // the other issue's Jira type name
}

// IssueDetail is what the detail panel shows beyond the grid columns.
// Fields holds the decoded custom fields keyed by field id, so a later
// phase's edit form can build on the same shape.
type IssueDetail struct {
	Key         string         `json:"key"`
	Description string         `json:"description"`
	Links       []Link         `json:"links"`
	Fields      map[string]any `json:"fields"`
}

// IssueType is one issue type a project offers.
type IssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// User is the authenticated user, from the connection test.
type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// IssueBackend is what the read path needs from the issue system.
type IssueBackend interface {
	TestConnection(ctx context.Context) (User, error)
	IsDemo() bool
	// SearchIssuesPage returns one page of issues in projectKey whose logical
	// type is in types, narrowed by scopeJQL when non-empty and by
	// updated >= since (RFC3339) when non-empty, plus the total match count.
	SearchIssuesPage(ctx context.Context, projectKey, scopeJQL, since string, types []string, startAt, maxResults int) ([]Issue, int, error)
	// GetIssueDetail fetches what the grid does not carry: description,
	// links, and the custom fields.
	GetIssueDetail(ctx context.Context, key string) (IssueDetail, error)
	IssueTypes(ctx context.Context, projectKey string) ([]IssueType, error)
}
```

- [ ] **Step 2: Failing store tests**

In `tam/internal/tamstore/tamstore_test.go`, wherever the existing test compares the value from `store.ReadSchemaVersion` against the literal `1`, change that literal to `2`. Then append this test to the same file:

```go
func TestSchemaVersionTwoHasTheIssueTables(t *testing.T) {
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"issue", "issue_link", "sync_state", "profile_setting"} {
		var name string
		err := db.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %s: %v", table, err)
		}
	}
}
```

Create `tam/internal/issuerepo/issues_test.go`:

```go
package issuerepo_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

func newRepo(t *testing.T) *issuerepo.Repository {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return issuerepo.New(db.DB())
}

func pts(v float64) *float64 { return &v }

func sample() []backend.Issue {
	return []backend.Issue{
		{Key: "PLAT-412", ID: "1", Project: "PLAT", Type: "story", Summary: "Checkout: apply promo code", Status: "In Progress", Assignee: "R. Anand", Labels: []string{"checkout", "promo"}, SprintID: "12", SprintName: "Sprint 12", StoryPoints: pts(5), Rank: "0|i0002:", Updated: "2026-09-05T09:58:00Z"},
		{Key: "PLAT-409", ID: "2", Project: "PLAT", Type: "task", Summary: "Rotate payment gateway API keys", Status: "To Do", SprintID: "12", SprintName: "Sprint 12", StoryPoints: pts(2), Rank: "0|i0001:", Updated: "2026-09-04T10:00:00Z"},
		{Key: "PLAT-350", ID: "3", Project: "PLAT", Type: "epic", Summary: "Promotions and discounts", Status: "In Progress", Assignee: "PO", Labels: []string{"promo"}, StoryPoints: pts(21), Rank: "", Updated: "2026-09-01T10:00:00Z"},
		{Key: "PLAT-347", ID: "4", Project: "PLAT", Type: "task", Summary: "Write retro notes template", Status: "To Do", Assignee: "S. Kim", SprintID: "13", SprintName: "Sprint 13", Rank: "0|i0003:", Updated: "2026-09-03T10:00:00Z"},
	}
}

func TestUpsertPageInsertsThenUpdatesWithoutDuplicates(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	if err := r.UpsertPage(ctx, "p1", sample(), now, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	changed := sample()[:1]
	changed[0].Summary = "Checkout: apply promo code at payment step"
	if err := r.UpsertPage(ctx, "p1", changed, now.Add(time.Minute), false); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	n, err := r.CountIssues(ctx, "p1")
	if err != nil || n != 4 {
		t.Fatalf("count = %d, %v; want 4", n, err)
	}
	got, err := r.GetIssue(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Summary != "Checkout: apply promo code at payment step" || got.Labels[1] != "promo" || *got.StoryPoints != 5 {
		t.Errorf("row = %+v", got)
	}
	if _, err := r.GetIssue(ctx, "p1", "PLAT-1"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("missing key err = %v", err)
	}
	if _, err := r.GetIssue(ctx, "other", "PLAT-412"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("rows are scoped by profile: err = %v", err)
	}
}

func TestListIssuesOrdersByRankThenKeyAndPages(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	page, err := r.ListIssues(ctx, "p1", issuerepo.IssueQuery{Limit: 3})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 4 || len(page.Issues) != 3 {
		t.Fatalf("page = total %d, %d rows", page.Total, len(page.Issues))
	}
	want := []string{"PLAT-409", "PLAT-412", "PLAT-347"}
	for i, k := range want {
		if page.Issues[i].Key != k {
			t.Errorf("row %d = %s, want %s", i, page.Issues[i].Key, k)
		}
	}
	page, err = r.ListIssues(ctx, "p1", issuerepo.IssueQuery{Offset: 3, Limit: 3})
	if err != nil || len(page.Issues) != 1 || page.Issues[0].Key != "PLAT-350" {
		t.Errorf("last page = %+v, %v (the empty rank sorts last)", page.Issues, err)
	}
}

func TestListIssuesFilters(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	cases := []struct {
		name string
		q    issuerepo.IssueQuery
		want []string
	}{
		{"by type", issuerepo.IssueQuery{Types: []string{"task"}}, []string{"PLAT-409", "PLAT-347"}},
		{"by two types", issuerepo.IssueQuery{Types: []string{"epic", "story"}}, []string{"PLAT-412", "PLAT-350"}},
		{"by sprint", issuerepo.IssueQuery{SprintID: "12"}, []string{"PLAT-409", "PLAT-412"}},
		{"by key text", issuerepo.IssueQuery{Text: "plat-35"}, []string{"PLAT-350"}},
		{"by summary text", issuerepo.IssueQuery{Text: "retro"}, []string{"PLAT-347"}},
		{"by label text", issuerepo.IssueQuery{Text: "promo"}, []string{"PLAT-412", "PLAT-350"}},
		{"type and sprint", issuerepo.IssueQuery{Types: []string{"task"}, SprintID: "13"}, []string{"PLAT-347"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page, err := r.ListIssues(ctx, "p1", c.q)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if page.Total != len(c.want) {
				t.Fatalf("total = %d, want %d", page.Total, len(c.want))
			}
			for i, k := range c.want {
				if page.Issues[i].Key != k {
					t.Errorf("row %d = %s, want %s", i, page.Issues[i].Key, k)
				}
			}
		})
	}
}

func TestClearFirstReplacesTheProfileOnly(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed p1: %v", err)
	}
	if err := r.UpsertPage(ctx, "p2", sample()[:1], time.Now(), false); err != nil {
		t.Fatalf("seed p2: %v", err)
	}
	if err := r.UpsertPage(ctx, "p1", sample()[3:], time.Now(), true); err != nil {
		t.Fatalf("replace: %v", err)
	}
	n, _ := r.CountIssues(ctx, "p1")
	if n != 1 {
		t.Errorf("p1 count after clear = %d, want 1", n)
	}
	n, _ = r.CountIssues(ctx, "p2")
	if n != 1 {
		t.Errorf("p2 count = %d, want 1 (untouched)", n)
	}
}

func TestListSprintsIsDistinctAndSorted(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	sprints, err := r.ListSprints(ctx, "p1")
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	if len(sprints) != 2 || sprints[0].ID != "12" || sprints[0].Name != "Sprint 12" || sprints[1].ID != "13" {
		t.Errorf("sprints = %+v", sprints)
	}
}
```

- [ ] **Step 3: Run them to see the failure**

Run (inside `tam/`): `go test ./internal/...`
Expected: FAIL. `tamstore` reports version 1 and missing tables; `issuerepo` does not compile.

- [ ] **Step 4: Schema version 2**

Replace `tam/internal/tamstore/tamstore.go`'s `Schema` declaration (keep `Open`, `DefaultDir`, and `DefaultPath` as they are) with:

```go
// Schema is TAM's database layout. Version 2 adds the issue cache, issue
// links, per-profile sync state, and per-profile settings. Every table is
// created with IF NOT EXISTS from Base, so a version 1 file (which had no app
// tables) gains them on open; a migration entry is only needed when an
// existing table changes shape.
var Schema = store.Schema{
	Version: 2,
	Base:    baseDDL,
	Indexes: indexDDL,
}

const baseDDL = `
CREATE TABLE IF NOT EXISTS issue (
	profile_id        TEXT NOT NULL,
	key               TEXT NOT NULL,
	id                TEXT NOT NULL DEFAULT '',
	project           TEXT NOT NULL DEFAULT '',
	type              TEXT NOT NULL DEFAULT '',
	summary           TEXT NOT NULL DEFAULT '',
	status            TEXT NOT NULL DEFAULT '',
	assignee          TEXT NOT NULL DEFAULT '',
	reporter          TEXT NOT NULL DEFAULT '',
	priority          TEXT NOT NULL DEFAULT '',
	labels            TEXT NOT NULL DEFAULT '[]',
	sprint_id         TEXT NOT NULL DEFAULT '',
	sprint_name       TEXT NOT NULL DEFAULT '',
	parent_key        TEXT NOT NULL DEFAULT '',
	story_points      REAL,
	rank              TEXT NOT NULL DEFAULT '',
	created           TEXT NOT NULL DEFAULT '',
	updated           TEXT NOT NULL DEFAULT '',
	synced_at         TEXT NOT NULL DEFAULT '',
	detail_json       TEXT,
	detail_fetched_at TEXT,
	PRIMARY KEY (profile_id, key)
);
CREATE TABLE IF NOT EXISTS issue_link (
	profile_id TEXT NOT NULL,
	from_key   TEXT NOT NULL,
	to_key     TEXT NOT NULL,
	link_type  TEXT NOT NULL,
	direction  TEXT NOT NULL,
	to_summary TEXT NOT NULL DEFAULT '',
	to_type    TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, from_key, to_key, link_type, direction)
);
CREATE TABLE IF NOT EXISTS sync_state (
	profile_id  TEXT PRIMARY KEY,
	last_synced TEXT NOT NULL DEFAULT '',
	last_full   TEXT NOT NULL DEFAULT '',
	last_error  TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS profile_setting (
	profile_id TEXT NOT NULL,
	key        TEXT NOT NULL,
	value      TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, key)
);`

const indexDDL = `
CREATE INDEX IF NOT EXISTS issue_profile_type   ON issue (profile_id, type);
CREATE INDEX IF NOT EXISTS issue_profile_sprint ON issue (profile_id, sprint_id);`
```

Change the last sentence of the package comment to "Version 2 adds the issue tables." so it stops promising a migration that is not needed.

- [ ] **Step 5: The store**

Create `tam/internal/issuerepo/issuerepo.go`:

```go
// Package issuerepo is the store layer over tam.db for issues: the cached
// rows the Backlog reads, the per-issue detail cache, issue links, sync
// state, and per-profile settings. Every method takes the profile id
// because every table is scoped by it.
package issuerepo

import (
	"database/sql"
	"errors"

	"agile-suite/tam/internal/backend"
)

// ErrNotFound is returned when a key is not cached for the profile.
var ErrNotFound = errors.New("issuerepo: not found")

// Repository runs the queries. It holds no state beyond the handle.
type Repository struct {
	db *sql.DB
}

// New wraps an open tam.db handle.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

// IssueQuery is the Backlog's filter and page. Text matches key, summary,
// and labels, case-insensitively. An empty Types list means every type.
// Limit defaults to 25 and is capped at 500.
type IssueQuery struct {
	Text     string   `json:"text"`
	Types    []string `json:"types"`
	SprintID string   `json:"sprintId"`
	Offset   int      `json:"offset"`
	Limit    int      `json:"limit"`
}

// IssuePage is one page of rows plus the total the filter matches.
type IssuePage struct {
	Issues []backend.Issue `json:"issues"`
	Total  int             `json:"total"`
}

// SprintRef is a sprint seen in the cached issues, for the filter dropdown.
type SprintRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
```

Create `tam/internal/issuerepo/issues.go`:

```go
package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
)

const (
	defaultPageSize = 25
	maxPageSize     = 500
)

// issueColumns is the SELECT list every row read uses, in scan order.
const issueColumns = `key, id, project, type, summary, status, assignee, reporter, priority, labels,
	sprint_id, sprint_name, parent_key, story_points, rank, created, updated`

// UpsertPage writes one page of issues for the profile inside one
// transaction. With clearFirst the profile's issues and links are deleted
// first, in the same transaction, which is how a full sync starts.
func (r *Repository) UpsertPage(ctx context.Context, profileID string, page []backend.Issue, syncedAt time.Time, clearFirst bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if clearFirst {
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ?`, profileID); err != nil {
			return fmt.Errorf("clear links: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue WHERE profile_id = ?`, profileID); err != nil {
			return fmt.Errorf("clear issues: %w", err)
		}
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO issue (profile_id, key, id, project, type, summary, status, assignee, reporter, priority, labels,
			sprint_id, sprint_name, parent_key, story_points, rank, created, updated, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, key) DO UPDATE SET
			id = excluded.id, project = excluded.project, type = excluded.type, summary = excluded.summary,
			status = excluded.status, assignee = excluded.assignee, reporter = excluded.reporter,
			priority = excluded.priority, labels = excluded.labels, sprint_id = excluded.sprint_id,
			sprint_name = excluded.sprint_name, parent_key = excluded.parent_key,
			story_points = excluded.story_points, rank = excluded.rank, created = excluded.created,
			updated = excluded.updated, synced_at = excluded.synced_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	synced := syncedAt.UTC().Format(time.RFC3339)
	for _, iss := range page {
		labels, err := json.Marshal(nonNil(iss.Labels))
		if err != nil {
			return fmt.Errorf("labels for %s: %w", iss.Key, err)
		}
		var points sql.NullFloat64
		if iss.StoryPoints != nil {
			points = sql.NullFloat64{Float64: *iss.StoryPoints, Valid: true}
		}
		if _, err := stmt.ExecContext(ctx, profileID, iss.Key, iss.ID, iss.Project, iss.Type, iss.Summary, iss.Status,
			iss.Assignee, iss.Reporter, iss.Priority, string(labels), iss.SprintID, iss.SprintName, iss.ParentKey,
			points, iss.Rank, iss.Created, iss.Updated, synced); err != nil {
			return fmt.Errorf("upsert %s: %w", iss.Key, err)
		}
	}
	return tx.Commit()
}

// ListIssues returns one page matching q, ordered by rank with unranked rows
// last, then by key.
func (r *Repository) ListIssues(ctx context.Context, profileID string, q IssueQuery) (IssuePage, error) {
	where, args := issueFilter(profileID, q)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue WHERE `+where, args...).Scan(&total); err != nil {
		return IssuePage{}, fmt.Errorf("count issues: %w", err)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+issueColumns+` FROM issue WHERE `+where+
			` ORDER BY CASE WHEN rank = '' THEN 1 ELSE 0 END, rank, key LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return IssuePage{}, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()
	issues := make([]backend.Issue, 0, limit)
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return IssuePage{}, err
		}
		issues = append(issues, iss)
	}
	if err := rows.Err(); err != nil {
		return IssuePage{}, err
	}
	return IssuePage{Issues: issues, Total: total}, nil
}

// GetIssue returns one cached row or ErrNotFound.
func (r *Repository) GetIssue(ctx context.Context, profileID, key string) (backend.Issue, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issue WHERE profile_id = ? AND key = ?`, profileID, key)
	iss, err := scanIssue(row)
	if errors.Is(err, sql.ErrNoRows) {
		return backend.Issue{}, ErrNotFound
	}
	return iss, err
}

// CountIssues is the profile's cached row count, for the status bar.
func (r *Repository) CountIssues(ctx context.Context, profileID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue WHERE profile_id = ?`, profileID).Scan(&n)
	return n, err
}

// ListSprints returns the distinct sprints in the cache, sorted by numeric
// id so "Sprint 12" precedes "Sprint 13".
func (r *Repository) ListSprints(ctx context.Context, profileID string) ([]SprintRef, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT sprint_id, sprint_name FROM issue WHERE profile_id = ? AND sprint_id <> '' ORDER BY CAST(sprint_id AS INTEGER), sprint_id`,
		profileID)
	if err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}
	defer rows.Close()
	var out []SprintRef
	for rows.Next() {
		var s SprintRef
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// issueFilter builds the WHERE clause for q. The profile is always the first
// condition, so an empty query still scopes to one profile.
func issueFilter(profileID string, q IssueQuery) (string, []any) {
	where := []string{"profile_id = ?"}
	args := []any{profileID}
	if len(q.Types) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(q.Types)), ", ")
		where = append(where, "type IN ("+marks+")")
		for _, t := range q.Types {
			args = append(args, t)
		}
	}
	if q.SprintID != "" {
		where = append(where, "sprint_id = ?")
		args = append(args, q.SprintID)
	}
	if text := strings.TrimSpace(q.Text); text != "" {
		like := "%" + escapeLike(text) + "%"
		where = append(where, "(key LIKE ? ESCAPE '\\' OR summary LIKE ? ESCAPE '\\' OR labels LIKE ? ESCAPE '\\')")
		args = append(args, like, like, like)
	}
	return strings.Join(where, " AND "), args
}

// escapeLike makes the user's text literal inside a LIKE pattern.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanIssue(s scanner) (backend.Issue, error) {
	var (
		iss    backend.Issue
		labels string
		points sql.NullFloat64
	)
	if err := s.Scan(&iss.Key, &iss.ID, &iss.Project, &iss.Type, &iss.Summary, &iss.Status, &iss.Assignee,
		&iss.Reporter, &iss.Priority, &labels, &iss.SprintID, &iss.SprintName, &iss.ParentKey, &points,
		&iss.Rank, &iss.Created, &iss.Updated); err != nil {
		return backend.Issue{}, err
	}
	if err := json.Unmarshal([]byte(labels), &iss.Labels); err != nil {
		return backend.Issue{}, fmt.Errorf("labels for %s: %w", iss.Key, err)
	}
	iss.Labels = nonNil(iss.Labels)
	if points.Valid {
		v := points.Float64
		iss.StoryPoints = &v
	}
	return iss, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
```

- [ ] **Step 6: Run the tests**

Run (inside `tam/`): `go vet ./... && go test ./internal/... -v -count=1`
Expected: PASS for `tamstore` (version 2, the four tables, the reopen test) and all five `issuerepo` tests including the seven filter subtests.

- [ ] **Step 7: Commit**

```bash
git add tam/internal/backend/backend.go tam/internal/tamstore tam/internal/issuerepo
git commit -m "feat(tam): issue shapes, schema version 2, and the issue store"
```

---

### Task 3: Detail cache, links, sync state, and profile settings

The rest of the store: the per-issue detail cache with its fetch timestamp, the links written alongside it, the linked-tests view over those links, per-profile sync state, and per-profile settings.

**Files:**
- Create: `tam/internal/issuerepo/detail.go`, `tam/internal/issuerepo/detail_test.go`, `tam/internal/issuerepo/state.go`, `tam/internal/issuerepo/state_test.go`
- Test: `go test ./internal/issuerepo/` inside `tam/`

**Interfaces:**
- Consumes: `Repository`, `ErrNotFound`, and the `newRepo`, `sample` test helpers from Task 2.
- Produces: `LinkedTest`, `SyncState`, `(*Repository).ReadDetail(ctx, profileID, key string) (backend.IssueDetail, time.Time, bool, error)`, `WriteDetail(ctx, profileID, key string, d backend.IssueDetail, fetchedAt time.Time) error`, `ListLinks(ctx, profileID, key string) ([]backend.Link, error)`, `ListLinkedTests(ctx, profileID, key, linkType string) ([]LinkedTest, error)`, `SyncState(ctx, profileID string) (SyncState, error)`, `SetSyncState(ctx, profileID string, s SyncState) error`, `ProfileSetting(ctx, profileID, key string) (string, error)`, `SetProfileSetting(ctx, profileID, key, value string) error`.

- [ ] **Step 1: Failing tests**

Create `tam/internal/issuerepo/detail_test.go`:

```go
package issuerepo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func TestDetailIsCachedWithItsFetchTime(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, _, ok, err := r.ReadDetail(ctx, "p1", "PLAT-412"); err != nil || ok {
		t.Fatalf("before write: ok=%v err=%v; want no cached detail", ok, err)
	}
	at := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	d := backend.IssueDetail{
		Key:         "PLAT-412",
		Description: "As a shopper I can enter a promo code.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "Promo code applies discount", IssueType: "Test"},
			{Direction: "outward", Type: "Relates", Key: "PLAT-350", Summary: "Promotions and discounts", IssueType: "Epic"},
		},
		Fields: map[string]any{"customfield_10016": 5.0},
	}
	if err := r.WriteDetail(ctx, "p1", "PLAT-412", d, at); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, fetchedAt, ok, err := r.ReadDetail(ctx, "p1", "PLAT-412")
	if err != nil || !ok {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}
	if !fetchedAt.Equal(at) {
		t.Errorf("fetchedAt = %v, want %v", fetchedAt, at)
	}
	if got.Description != d.Description || len(got.Links) != 2 || got.Fields["customfield_10016"] != 5.0 {
		t.Errorf("detail = %+v", got)
	}
	if err := r.WriteDetail(ctx, "p1", "PLAT-1", d, at); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("writing detail for an uncached key: err = %v, want ErrNotFound", err)
	}
}

func TestWriteDetailReplacesLinks(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	first := backend.IssueDetail{Links: []backend.Link{
		{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "old", IssueType: "Test"},
		{Direction: "inward", Type: "Tested By", Key: "XT-1019", Summary: "Expired promo code rejected", IssueType: "Test"},
	}}
	if err := r.WriteDetail(ctx, "p1", "PLAT-412", first, time.Now()); err != nil {
		t.Fatalf("first write: %v", err)
	}
	second := backend.IssueDetail{Links: []backend.Link{
		{Direction: "inward", Type: "Tested By", Key: "XT-1019", Summary: "Expired promo code rejected", IssueType: "Test"},
	}}
	if err := r.WriteDetail(ctx, "p1", "PLAT-412", second, time.Now()); err != nil {
		t.Fatalf("second write: %v", err)
	}
	links, err := r.ListLinks(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("links: %v", err)
	}
	if len(links) != 1 || links[0].Key != "XT-1019" {
		t.Errorf("links = %+v, want only XT-1019", links)
	}
}

func TestListLinkedTestsFiltersByLinkTypeCaseInsensitively(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	d := backend.IssueDetail{Links: []backend.Link{
		{Direction: "inward", Type: "Tested By", Key: "XT-1019", Summary: "Expired promo code rejected", IssueType: "Test"},
		{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "Promo code applies discount", IssueType: "Test"},
		{Direction: "outward", Type: "Relates", Key: "PLAT-350", Summary: "Promotions and discounts", IssueType: "Epic"},
	}}
	if err := r.WriteDetail(ctx, "p1", "PLAT-412", d, time.Now()); err != nil {
		t.Fatalf("write: %v", err)
	}
	tests, err := r.ListLinkedTests(ctx, "p1", "PLAT-412", "tested by")
	if err != nil {
		t.Fatalf("linked tests: %v", err)
	}
	if len(tests) != 2 || tests[0].Key != "XT-1018" || tests[1].Key != "XT-1019" || tests[0].LinkType != "Tested By" {
		t.Errorf("tests = %+v", tests)
	}
	tests, err = r.ListLinkedTests(ctx, "p1", "PLAT-412", "")
	if err != nil || len(tests) != 2 {
		t.Errorf("empty link type falls back to Tested By: %+v, %v", tests, err)
	}
	tests, err = r.ListLinkedTests(ctx, "p1", "PLAT-412", "Verifies")
	if err != nil || len(tests) != 0 {
		t.Errorf("unrelated link type: %+v, %v", tests, err)
	}
}
```

Create `tam/internal/issuerepo/state_test.go`:

```go
package issuerepo_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/issuerepo"
)

func TestSyncStateRoundTripsAndCountsIssues(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	s, err := r.SyncState(ctx, "p1")
	if err != nil || s.LastSynced != "" || s.IssueCount != 0 {
		t.Fatalf("empty state = %+v, %v", s, err)
	}
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	want := issuerepo.SyncState{LastSynced: "2026-09-05T10:42:00Z", LastFull: "2026-09-01T08:00:00Z", LastError: ""}
	if err := r.SetSyncState(ctx, "p1", want); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := r.SetSyncState(ctx, "p1", issuerepo.SyncState{LastSynced: want.LastSynced, LastFull: want.LastFull, LastError: "page 3: timeout"}); err != nil {
		t.Fatalf("set again: %v", err)
	}
	s, err = r.SyncState(ctx, "p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.LastSynced != want.LastSynced || s.LastFull != want.LastFull || s.LastError != "page 3: timeout" || s.IssueCount != 4 {
		t.Errorf("state = %+v", s)
	}
	other, _ := r.SyncState(ctx, "p2")
	if other.LastSynced != "" {
		t.Errorf("state leaked across profiles: %+v", other)
	}
}

func TestProfileSettingsAreScopedAndOverwritten(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if v, err := r.ProfileSetting(ctx, "p1", "requirement_issue_type"); err != nil || v != "" {
		t.Fatalf("unset = %q, %v", v, err)
	}
	if err := r.SetProfileSetting(ctx, "p1", "requirement_issue_type", "Requirement"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := r.SetProfileSetting(ctx, "p1", "requirement_issue_type", "Business Requirement"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if v, _ := r.ProfileSetting(ctx, "p1", "requirement_issue_type"); v != "Business Requirement" {
		t.Errorf("p1 = %q", v)
	}
	if v, _ := r.ProfileSetting(ctx, "p2", "requirement_issue_type"); v != "" {
		t.Errorf("p2 = %q, want unset", v)
	}
}
```

- [ ] **Step 2: Run them to see the compile failure**

Run (inside `tam/`): `go test ./internal/issuerepo/`
Expected: FAIL, `r.ReadDetail undefined`, `r.SyncState undefined`, and friends.

- [ ] **Step 3: The detail cache and links**

Create `tam/internal/issuerepo/detail.go`:

```go
package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
)

// defaultTestLinkType is the link type TAM assumes between a requirement or
// story and its tests when the shared setting is empty; it is XTM's first
// candidate too.
const defaultTestLinkType = "Tested By"

// LinkedTest is a test reached from an issue through the requirement link
// type, for the detail panel's Tests tab.
type LinkedTest struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	LinkType string `json:"linkType"`
}

// ReadDetail returns the cached detail for key, when it was fetched, and
// whether a cached detail exists. A cached row with no detail yet reports
// ok=false and no error.
func (r *Repository) ReadDetail(ctx context.Context, profileID, key string) (backend.IssueDetail, time.Time, bool, error) {
	var (
		raw       sql.NullString
		fetchedAt sql.NullString
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT detail_json, detail_fetched_at FROM issue WHERE profile_id = ? AND key = ?`, profileID, key,
	).Scan(&raw, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return backend.IssueDetail{}, time.Time{}, false, ErrNotFound
	}
	if err != nil {
		return backend.IssueDetail{}, time.Time{}, false, err
	}
	if !raw.Valid || raw.String == "" {
		return backend.IssueDetail{}, time.Time{}, false, nil
	}
	var d backend.IssueDetail
	if err := json.Unmarshal([]byte(raw.String), &d); err != nil {
		return backend.IssueDetail{}, time.Time{}, false, fmt.Errorf("decode detail for %s: %w", key, err)
	}
	at, err := time.Parse(time.RFC3339, fetchedAt.String)
	if err != nil {
		return backend.IssueDetail{}, time.Time{}, false, fmt.Errorf("detail_fetched_at for %s: %w", key, err)
	}
	links, err := r.ListLinks(ctx, profileID, key)
	if err != nil {
		return backend.IssueDetail{}, time.Time{}, false, err
	}
	d.Key = key
	d.Links = links
	return d, at, true, nil
}

// WriteDetail caches d for key and replaces the issue's links, in one
// transaction. The links are stored in issue_link rather than inside the
// JSON so the Tests tab and later phases can query them.
func (r *Repository) WriteDetail(ctx context.Context, profileID, key string, d backend.IssueDetail, fetchedAt time.Time) error {
	stored := d
	stored.Key = key
	stored.Links = nil
	raw, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encode detail for %s: %w", key, err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE issue SET detail_json = ?, detail_fetched_at = ? WHERE profile_id = ? AND key = ?`,
		string(raw), fetchedAt.UTC().Format(time.RFC3339), profileID, key)
	if err != nil {
		return fmt.Errorf("write detail for %s: %w", key, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key = ?`, profileID, key); err != nil {
		return fmt.Errorf("clear links for %s: %w", key, err)
	}
	for _, l := range d.Links {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO issue_link (profile_id, from_key, to_key, link_type, direction, to_summary, to_type) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			profileID, key, l.Key, l.Type, l.Direction, l.Summary, l.IssueType); err != nil {
			return fmt.Errorf("write link %s -> %s: %w", key, l.Key, err)
		}
	}
	return tx.Commit()
}

// ListLinks returns the cached links of key, ordered by type, then direction,
// then the other key.
func (r *Repository) ListLinks(ctx context.Context, profileID, key string) ([]backend.Link, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT direction, link_type, to_key, to_summary, to_type FROM issue_link
		 WHERE profile_id = ? AND from_key = ? ORDER BY link_type, direction, to_key`, profileID, key)
	if err != nil {
		return nil, fmt.Errorf("list links for %s: %w", key, err)
	}
	defer rows.Close()
	links := []backend.Link{}
	for rows.Next() {
		var l backend.Link
		if err := rows.Scan(&l.Direction, &l.Type, &l.Key, &l.Summary, &l.IssueType); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// ListLinkedTests returns the tests linked to key through linkType, compared
// case-insensitively, in either direction. An empty linkType means the
// default "Tested By".
func (r *Repository) ListLinkedTests(ctx context.Context, profileID, key, linkType string) ([]LinkedTest, error) {
	want := strings.TrimSpace(linkType)
	if want == "" {
		want = defaultTestLinkType
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT to_key, to_summary, link_type FROM issue_link
		 WHERE profile_id = ? AND from_key = ? AND lower(link_type) = lower(?) ORDER BY to_key`, profileID, key, want)
	if err != nil {
		return nil, fmt.Errorf("linked tests for %s: %w", key, err)
	}
	defer rows.Close()
	out := []LinkedTest{}
	for rows.Next() {
		var lt LinkedTest
		if err := rows.Scan(&lt.Key, &lt.Summary, &lt.LinkType); err != nil {
			return nil, err
		}
		out = append(out, lt)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Sync state and profile settings**

Create `tam/internal/issuerepo/state.go`:

```go
package issuerepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SyncState is what the status bar and the sync engine need to know about a
// profile: when it last synced (RFC3339, empty when never), when it last did
// a full sync, the last error, and how many issues are cached.
type SyncState struct {
	LastSynced string `json:"lastSynced"`
	LastFull   string `json:"lastFull"`
	LastError  string `json:"lastError"`
	IssueCount int    `json:"issueCount"`
}

// SyncState reads the profile's state; a profile that never synced returns
// the zero value with the issue count filled in.
func (r *Repository) SyncState(ctx context.Context, profileID string) (SyncState, error) {
	var s SyncState
	err := r.db.QueryRowContext(ctx,
		`SELECT last_synced, last_full, last_error FROM sync_state WHERE profile_id = ?`, profileID,
	).Scan(&s.LastSynced, &s.LastFull, &s.LastError)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SyncState{}, fmt.Errorf("sync state: %w", err)
	}
	n, err := r.CountIssues(ctx, profileID)
	if err != nil {
		return SyncState{}, err
	}
	s.IssueCount = n
	return s, nil
}

// SetSyncState writes the three timestamps and the error; IssueCount is
// derived and ignored on write.
func (r *Repository) SetSyncState(ctx context.Context, profileID string, s SyncState) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sync_state (profile_id, last_synced, last_full, last_error) VALUES (?, ?, ?, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET last_synced = excluded.last_synced, last_full = excluded.last_full, last_error = excluded.last_error`,
		profileID, s.LastSynced, s.LastFull, s.LastError)
	if err != nil {
		return fmt.Errorf("set sync state: %w", err)
	}
	return nil
}

// ProfileSetting returns the value stored for key under the profile, or ""
// when unset.
func (r *Repository) ProfileSetting(ctx context.Context, profileID, key string) (string, error) {
	var v string
	err := r.db.QueryRowContext(ctx,
		`SELECT value FROM profile_setting WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("profile setting %s: %w", key, err)
	}
	return v, nil
}

// SetProfileSetting stores value for key under the profile.
func (r *Repository) SetProfileSetting(ctx context.Context, profileID, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO profile_setting (profile_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(profile_id, key) DO UPDATE SET value = excluded.value`, profileID, key, value)
	if err != nil {
		return fmt.Errorf("set profile setting %s: %w", key, err)
	}
	return nil
}
```

- [ ] **Step 5: Run the package tests**

Run (inside `tam/`): `go vet ./internal/issuerepo/ && go test ./internal/issuerepo/ -v -count=1`
Expected: PASS for all ten tests in the package.

- [ ] **Step 6: Commit**

```bash
git add tam/internal/issuerepo
git commit -m "feat(tam): cache issue details and links, and record sync state per profile"
```

---

### Task 4: The demo dataset and the demo backend

A fixed Acme Platform (PLAT) dataset that matches the mockup, and the `IssueBackend` that serves it. Everything downstream (sync, grid, panel) can be exercised offline against this.

**Files:**
- Create: `tam/internal/demo/demo.go`, `tam/internal/demo/demo_test.go`, `tam/internal/backend/demo/demo.go`, `tam/internal/backend/demo/demo_test.go`
- Test: `go test ./internal/demo/ ./internal/backend/...` inside `tam/`

**Interfaces:**
- Consumes: `backend.Issue`, `backend.IssueDetail`, `backend.Link`, `backend.IssueType`, `backend.User`, `backend.AllTypes`, the type constants (Task 2).
- Produces (package `agile-suite/tam/internal/demo`): `const ProjectKey = "PLAT"`, `Issues(projectKey string) []backend.Issue`, `Detail(projectKey, key string) (backend.IssueDetail, bool)`.
- Produces (package `agile-suite/tam/internal/backend/demo`): `New(projectKey string) *Backend` implementing `backend.IssueBackend`.

- [ ] **Step 1: Failing dataset tests**

Create `tam/internal/demo/demo_test.go`:

```go
package demo_test

import (
	"reflect"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

func TestIssuesAreDeterministicAndWellFormed(t *testing.T) {
	a := demo.Issues(demo.ProjectKey)
	b := demo.Issues(demo.ProjectKey)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two calls returned different datasets")
	}
	if len(a) != 60 {
		t.Fatalf("len = %d, want 60", len(a))
	}
	seen := map[string]bool{}
	types := map[string]int{}
	for _, iss := range a {
		if seen[iss.Key] {
			t.Errorf("duplicate key %s", iss.Key)
		}
		seen[iss.Key] = true
		types[iss.Type]++
		if iss.Project != "PLAT" || iss.Summary == "" || iss.Status == "" || iss.Rank == "" || iss.Updated == "" || iss.ID == "" {
			t.Errorf("incomplete issue %+v", iss)
		}
		if iss.Labels == nil {
			t.Errorf("%s has nil labels; want an empty slice", iss.Key)
		}
	}
	for _, typ := range backend.AllTypes {
		if types[typ] == 0 {
			t.Errorf("no issues of type %s", typ)
		}
	}
}

func TestCuratedIssuesMatchTheMockup(t *testing.T) {
	by := map[string]backend.Issue{}
	for _, iss := range demo.Issues(demo.ProjectKey) {
		by[iss.Key] = iss
	}
	s := by["PLAT-412"]
	if s.Type != "story" || s.Summary != "Checkout: apply promo code at payment step" || s.Status != "In Progress" ||
		s.Assignee != "R. Anand" || s.SprintID != "12" || s.SprintName != "Sprint 12" || s.StoryPoints == nil || *s.StoryPoints != 5 ||
		s.ParentKey != "PLAT-350" || len(s.Labels) != 2 {
		t.Errorf("PLAT-412 = %+v", s)
	}
	if r := by["PLAT-388"]; r.Type != "requirement" || r.Status != "Approved" || r.SprintID != "" || r.StoryPoints != nil {
		t.Errorf("PLAT-388 = %+v", r)
	}
	if e := by["PLAT-350"]; e.Type != "epic" || e.StoryPoints == nil || *e.StoryPoints != 21 {
		t.Errorf("PLAT-350 = %+v", e)
	}
	if b := by["PLAT-401"]; b.Type != "bug" || b.Assignee != "M. Ortiz" {
		t.Errorf("PLAT-401 = %+v", b)
	}
}

func TestProjectKeyIsSubstituted(t *testing.T) {
	issues := demo.Issues("DEMO")
	if issues[0].Key != "DEMO-412" || issues[0].Project != "DEMO" || issues[0].ParentKey != "DEMO-350" {
		t.Errorf("first issue = %+v", issues[0])
	}
	d, ok := demo.Detail("DEMO", "DEMO-412")
	if !ok || d.Key != "DEMO-412" {
		t.Errorf("detail = %+v, ok=%v", d, ok)
	}
}

func TestDetailCarriesLinksForTheCuratedStory(t *testing.T) {
	d, ok := demo.Detail(demo.ProjectKey, "PLAT-412")
	if !ok {
		t.Fatal("no detail for PLAT-412")
	}
	if d.Description == "" {
		t.Error("empty description")
	}
	tested := 0
	for _, l := range d.Links {
		if l.Type == "Tested By" && l.IssueType == "Test" {
			tested++
		}
	}
	if tested != 2 {
		t.Errorf("PLAT-412 has %d Tested By links, want 2: %+v", tested, d.Links)
	}
	if _, ok := demo.Detail(demo.ProjectKey, "PLAT-260"); !ok {
		t.Error("filler issues have a detail too")
	}
	if _, ok := demo.Detail(demo.ProjectKey, "PLAT-9999"); ok {
		t.Error("unknown key should not have a detail")
	}
}
```

- [ ] **Step 2: Run to see the failure**

Run (inside `tam/`): `go test ./internal/demo/`
Expected: FAIL, package does not exist.

- [ ] **Step 3: The dataset**

Create `tam/internal/demo/demo.go`:

```go
// Package demo is the deterministic Acme Platform dataset behind the demo
// profile: a dozen hand-written issues that match the design mockup plus
// generated filler, so the grid has pages to turn and the filters have
// something to hide. Keys, ids, and timestamps are fixed so tests can assert
// on them.
package demo

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
)

// ProjectKey is the project the curated issues are written for. Issues and
// Detail substitute the profile's real key so a demo profile with any key
// works.
const ProjectKey = "PLAT"

const seed = 20260905

var base = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

func pts(v float64) *float64 { return &v }

// curated are the issues the mockup shows, in backlog order. Keys use the
// PLAT prefix; Issues rewrites them for the profile's project key.
var curated = []backend.Issue{
	{Key: "PLAT-412", Type: backend.TypeStory, Summary: "Checkout: apply promo code at payment step", Status: "In Progress", Assignee: "R. Anand", Reporter: "PO", Priority: "High", Labels: []string{"checkout", "promo"}, SprintID: "12", SprintName: "Sprint 12", ParentKey: "PLAT-350", StoryPoints: pts(5)},
	{Key: "PLAT-409", Type: backend.TypeTask, Summary: "Rotate payment gateway API keys", Status: "To Do", Reporter: "M. Ortiz", Priority: "Medium", Labels: []string{"security"}, SprintID: "12", SprintName: "Sprint 12", ParentKey: "PLAT-310", StoryPoints: pts(2)},
	{Key: "PLAT-401", Type: backend.TypeBug, Summary: "Order total wrong when VAT changes mid-session", Status: "In Progress", Assignee: "M. Ortiz", Reporter: "S. Kim", Priority: "Highest", Labels: []string{"checkout"}, SprintID: "12", SprintName: "Sprint 12", ParentKey: "PLAT-320", StoryPoints: pts(3)},
	{Key: "PLAT-388", Type: backend.TypeRequirement, Summary: "Promo codes must be single-use per customer", Status: "Approved", Assignee: "PO", Reporter: "PO", Priority: "High", Labels: []string{"promo"}, ParentKey: "PLAT-350"},
	{Key: "PLAT-350", Type: backend.TypeEpic, Summary: "Promotions and discounts", Status: "In Progress", Assignee: "PO", Reporter: "PO", Priority: "High", Labels: []string{"promo"}, StoryPoints: pts(21)},
	{Key: "PLAT-347", Type: backend.TypeTask, Summary: "Write retro notes template", Status: "To Do", Assignee: "S. Kim", Reporter: "S. Kim", Priority: "Low", Labels: []string{"process"}, SprintID: "13", SprintName: "Sprint 13", ParentKey: "PLAT-305", StoryPoints: pts(1)},
	{Key: "PLAT-331", Type: backend.TypeStory, Summary: "Guest checkout without an account", Status: "Done", Assignee: "R. Anand", Reporter: "PO", Priority: "Medium", Labels: []string{"checkout"}, SprintID: "11", SprintName: "Sprint 11", ParentKey: "PLAT-320", StoryPoints: pts(8)},
	{Key: "PLAT-398", Type: backend.TypeStory, Summary: "Show the discount breakdown on the order summary", Status: "To Do", Reporter: "PO", Priority: "Medium", Labels: []string{"promo", "checkout"}, SprintID: "13", SprintName: "Sprint 13", ParentKey: "PLAT-350", StoryPoints: pts(3)},
	{Key: "PLAT-395", Type: backend.TypeRequirement, Summary: "Every payment request must carry an idempotency key", Status: "Approved", Assignee: "PO", Reporter: "M. Ortiz", Priority: "Highest", Labels: []string{"payments"}, ParentKey: "PLAT-310"},
	{Key: "PLAT-390", Type: backend.TypeBug, Summary: "Promo code field accepts whitespace-only input", Status: "To Do", Reporter: "J. Park", Priority: "Low", Labels: []string{"promo"}, SprintID: "13", SprintName: "Sprint 13", ParentKey: "PLAT-350", StoryPoints: pts(1)},
	{Key: "PLAT-385", Type: backend.TypeTask, Summary: "Add the promo code analytics event", Status: "Done", Assignee: "M. Ortiz", Reporter: "PO", Priority: "Medium", Labels: []string{"promo", "analytics"}, SprintID: "11", SprintName: "Sprint 11", ParentKey: "PLAT-350", StoryPoints: pts(2)},
	{Key: "PLAT-320", Type: backend.TypeEpic, Summary: "Checkout experience", Status: "In Progress", Assignee: "PO", Reporter: "PO", Priority: "High", Labels: []string{"checkout"}, StoryPoints: pts(34)},
	{Key: "PLAT-310", Type: backend.TypeEpic, Summary: "Payments platform hardening", Status: "To Do", Assignee: "PO", Reporter: "M. Ortiz", Priority: "Medium", Labels: []string{"payments", "security"}, StoryPoints: pts(13)},
	{Key: "PLAT-305", Type: backend.TypeEpic, Summary: "Team rituals and process", Status: "Done", Assignee: "S. Kim", Reporter: "S. Kim", Priority: "Low", Labels: []string{"process"}, StoryPoints: pts(5)},
}

// descriptions and links for the curated issues. Test keys use the XT
// prefix, the project XTM's own demo dataset uses, so the seam reads the
// way it will against a real suite.
var curatedDetails = map[string]backend.IssueDetail{
	"PLAT-412": {
		Description: "As a shopper I can enter a promo code on the payment step and see the discount before I pay.\n\nAcceptance: the code is validated against the promotions service; an invalid or expired code shows an inline message and leaves the total unchanged.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "Promo code applies discount", IssueType: "Test"},
			{Direction: "inward", Type: "Tested By", Key: "XT-1019", Summary: "Expired promo code rejected", IssueType: "Test"},
			{Direction: "outward", Type: "Relates", Key: "PLAT-388", Summary: "Promo codes must be single-use per customer", IssueType: "Requirement"},
		},
	},
	"PLAT-388": {
		Description: "A promo code may be redeemed once per customer account. A second attempt is rejected with a message that names the earlier order.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-1020", Summary: "Promo code single-use enforced", IssueType: "Test"},
			{Direction: "inward", Type: "Relates", Key: "PLAT-412", Summary: "Checkout: apply promo code at payment step", IssueType: "Story"},
		},
	},
	"PLAT-331": {
		Description: "As a shopper without an account I can complete checkout with an email address only.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-990", Summary: "Guest checkout completes", IssueType: "Test"},
		},
	},
	"PLAT-395": {
		Description: "Every request to the payment gateway carries an idempotency key derived from the order id, so a retried request never charges twice.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-1031", Summary: "Retried payment is not charged twice", IssueType: "Test"},
		},
	},
	"PLAT-401": {
		Description: "Steps: add an item, change the shipping country to one with a different VAT rate, return to the summary. The total still shows the old VAT amount.",
		Links: []backend.Link{
			{Direction: "outward", Type: "Relates", Key: "PLAT-331", Summary: "Guest checkout without an account", IssueType: "Story"},
		},
	},
}

var (
	fillerTypes     = []string{backend.TypeTask, backend.TypeStory, backend.TypeBug, backend.TypeRequirement, backend.TypeTask, backend.TypeStory}
	fillerAreas     = []string{"cart", "search", "account", "shipping", "returns", "catalogue", "notifications", "invoicing"}
	fillerAssignees = []string{"R. Anand", "M. Ortiz", "S. Kim", "J. Park", ""}
	fillerStatuses  = []string{"To Do", "To Do", "In Progress", "Done"}
	fillerPriority  = []string{"Low", "Medium", "Medium", "High"}
	fillerSprints   = []string{"", "11", "12", "13"}
	fillerPoints    = []float64{1, 2, 3, 5, 8}
	fillerEpics     = []string{"PLAT-320", "PLAT-310", "PLAT-350", ""}
)

func fillerSummary(typ, area string, n int) string {
	switch typ {
	case backend.TypeStory:
		return fmt.Sprintf("As a shopper I can use the %s page on a phone (%d)", area, n)
	case backend.TypeBug:
		return fmt.Sprintf("The %s page loses its state after a refresh (%d)", area, n)
	case backend.TypeRequirement:
		return fmt.Sprintf("The %s service must answer within two seconds (%d)", area, n)
	default:
		return fmt.Sprintf("Update the %s docs and dashboards (%d)", area, n)
	}
}

// Issues returns the whole dataset, curated issues first in backlog order,
// then the filler, all rewritten to projectKey.
func Issues(projectKey string) []backend.Issue {
	out := make([]backend.Issue, 0, 60)
	for i, c := range curated {
		iss := c
		iss.Key = rekey(iss.Key, projectKey)
		iss.ParentKey = rekey(iss.ParentKey, projectKey)
		iss.Labels = nonNil(iss.Labels)
		iss.Project = projectKey
		iss.ID = fmt.Sprintf("%d", 10000+i)
		iss.Rank = fmt.Sprintf("0|i%04d:", i)
		iss.Created = base.Add(-time.Duration(30+i) * 24 * time.Hour).Format(time.RFC3339)
		iss.Updated = base.Add(time.Duration(4*24-i) * time.Hour).Format(time.RFC3339)
		out = append(out, iss)
	}
	rng := rand.New(rand.NewSource(seed))
	for n := 0; len(out) < 60; n++ {
		typ := fillerTypes[n%len(fillerTypes)]
		area := fillerAreas[rng.Intn(len(fillerAreas))]
		iss := backend.Issue{
			Key:      fmt.Sprintf("%s-%d", projectKey, 250+n),
			ID:       fmt.Sprintf("%d", 20000+n),
			Project:  projectKey,
			Type:     typ,
			Summary:  fillerSummary(typ, area, n),
			Status:   fillerStatuses[rng.Intn(len(fillerStatuses))],
			Assignee: fillerAssignees[rng.Intn(len(fillerAssignees))],
			Reporter: "PO",
			Priority: fillerPriority[rng.Intn(len(fillerPriority))],
			Labels:   []string{area},
			Rank:     fmt.Sprintf("0|i%04d:", len(curated)+n),
			Created:  base.Add(-time.Duration(60+n) * 24 * time.Hour).Format(time.RFC3339),
			Updated:  base.Add(-time.Duration(n) * 6 * time.Hour).Format(time.RFC3339),
		}
		if typ == backend.TypeRequirement {
			iss.Status = []string{"Draft", "Approved"}[rng.Intn(2)]
		} else {
			if s := fillerSprints[rng.Intn(len(fillerSprints))]; s != "" {
				iss.SprintID = s
				iss.SprintName = "Sprint " + s
			}
			if rng.Intn(4) != 0 {
				iss.StoryPoints = pts(fillerPoints[rng.Intn(len(fillerPoints))])
			}
		}
		if e := fillerEpics[rng.Intn(len(fillerEpics))]; e != "" {
			iss.ParentKey = rekey(e, projectKey)
		}
		out = append(out, iss)
	}
	return out
}

// Detail returns the description and links for key. Curated issues have
// hand-written details; filler issues get a short generated one.
func Detail(projectKey, key string) (backend.IssueDetail, bool) {
	// Curated details are keyed by their PLAT keys; map the profile's key back.
	canonical := key
	if strings.HasPrefix(key, projectKey+"-") {
		canonical = ProjectKey + strings.TrimPrefix(key, projectKey)
	}
	if d, ok := curatedDetails[canonical]; ok {
		out := backend.IssueDetail{Key: key, Description: d.Description, Fields: map[string]any{}}
		for _, l := range d.Links {
			l.Key = rekey(l.Key, projectKey)
			out.Links = append(out.Links, l)
		}
		return out, true
	}
	for _, iss := range Issues(projectKey) {
		if iss.Key == key {
			return backend.IssueDetail{
				Key:         key,
				Description: "Generated demo issue. " + iss.Summary + ".",
				Links:       []backend.Link{},
				Fields:      map[string]any{},
			}, true
		}
	}
	return backend.IssueDetail{}, false
}

// rekey swaps the PLAT prefix for projectKey. Keys with another prefix (the
// XT test keys) are returned unchanged.
func rekey(key, projectKey string) string {
	if strings.HasPrefix(key, ProjectKey+"-") {
		return projectKey + strings.TrimPrefix(key, ProjectKey)
	}
	return key
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return append([]string{}, s...)
}
```

- [ ] **Step 4: Run the dataset tests**

Run (inside `tam/`): `go test ./internal/demo/ -v`
Expected: PASS, four tests.

- [ ] **Step 5: Failing backend tests**

Create `tam/internal/backend/demo/demo_test.go`:

```go
package demo_test

import (
	"context"
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
```

- [ ] **Step 6: The demo backend**

Create `tam/internal/backend/demo/demo.go`:

```go
// Package demo serves the offline dataset through the IssueBackend seam, so
// a profile whose Jira URL is "demo" exercises the same sync, cache, and
// views as a live one.
package demo

import (
	"context"
	"fmt"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

// Backend is the demo IssueBackend for one project key.
type Backend struct {
	project string
}

// New returns the demo backend for projectKey.
func New(projectKey string) *Backend {
	if projectKey == "" {
		projectKey = demo.ProjectKey
	}
	return &Backend{project: projectKey}
}

// TestConnection always succeeds with the demo user.
func (b *Backend) TestConnection(context.Context) (backend.User, error) {
	return backend.User{Name: "demo", DisplayName: "Demo User"}, nil
}

// IsDemo is true.
func (b *Backend) IsDemo() bool { return true }

// SearchIssuesPage pages the dataset filtered by type. The scope JQL and
// since are ignored, as in XTM's demo mode, so an incremental sync still
// returns everything.
func (b *Backend) SearchIssuesPage(_ context.Context, _, _, _ string, types []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	want := map[string]bool{}
	for _, t := range types {
		want[t] = true
	}
	var all []backend.Issue
	for _, iss := range demo.Issues(b.project) {
		if len(want) == 0 || want[iss.Type] {
			all = append(all, iss)
		}
	}
	total := len(all)
	if startAt >= total || maxResults <= 0 {
		return []backend.Issue{}, total, nil
	}
	end := startAt + maxResults
	if end > total {
		end = total
	}
	return all[startAt:end], total, nil
}

// GetIssueDetail returns the curated or generated detail for key.
func (b *Backend) GetIssueDetail(_ context.Context, key string) (backend.IssueDetail, error) {
	d, ok := demo.Detail(b.project, key)
	if !ok {
		return backend.IssueDetail{}, fmt.Errorf("demo: no issue %s", key)
	}
	return d, nil
}

// IssueTypes lists the five types the dataset uses.
func (b *Backend) IssueTypes(context.Context, string) ([]backend.IssueType, error) {
	return []backend.IssueType{
		{ID: "1", Name: "Task"},
		{ID: "2", Name: "Epic"},
		{ID: "3", Name: "Story"},
		{ID: "4", Name: "Bug"},
		{ID: "5", Name: "Requirement"},
	}, nil
}
```

- [ ] **Step 7: Run both packages and commit**

Run (inside `tam/`): `go vet ./internal/... && go test ./internal/demo/ ./internal/backend/... -v -count=1`
Expected: PASS, seven tests across the two packages.

```bash
git add tam/internal/demo tam/internal/backend/demo
git commit -m "feat(tam): the Acme Platform demo dataset and the demo issue backend"
```

---

### Task 5: The Jira backend

The `IssueBackend` for a live Jira DC on top of `core/jira`: the JQL the decision table fixes, custom-field discovery that tolerates absence, the field mapping with both Sprint shapes, and the detail fetch with links.

**Files:**
- Create: `tam/internal/backend/jira/fields.go`, `tam/internal/backend/jira/fields_test.go`, `tam/internal/backend/jira/jira.go`, `tam/internal/backend/jira/jira_test.go`
- Test: `go test ./internal/backend/jira/` inside `tam/`

**Interfaces:**
- Consumes: `corejira.Client`, `corejira.RawIssue`, `corejira.SearchIssues`, `GetIssue`, `IssueTypes`, `CustomFieldID`, `ErrFieldNotFound`, `Myself` (Task 1); the `backend` shapes (Task 2).
- Produces (package `agile-suite/tam/internal/backend/jira`): `New(c *corejira.Client, requirementType string) *Backend` implementing `backend.IssueBackend`; `DefaultRequirementType = "Requirement"`.

- [ ] **Step 1: Failing mapping tests**

Create `tam/internal/backend/jira/fields_test.go`:

```go
package jira

import (
	"encoding/json"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

func TestParseSprintHandlesBothShapes(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantID   string
		wantName string
	}{
		{"objects, last is current", `[{"id":11,"name":"Sprint 11","state":"CLOSED"},{"id":12,"name":"Sprint 12","state":"ACTIVE"}]`, "12", "Sprint 12"},
		{"legacy strings", `["com.atlassian.greenhopper.service.sprint.Sprint@1a2b[id=11,rapidViewId=3,state=CLOSED,name=Sprint 11,startDate=2026-08-01T00:00:00.000Z,goal=]","com.atlassian.greenhopper.service.sprint.Sprint@3c4d[id=12,rapidViewId=3,state=ACTIVE,name=Sprint 12 - Checkout polish,goal=]"]`, "12", "Sprint 12 - Checkout polish"},
		{"single object", `{"id":13,"name":"Sprint 13","state":"FUTURE"}`, "13", "Sprint 13"},
		{"null", `null`, "", ""},
		{"absent", ``, "", ""},
		{"empty list", `[]`, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, name := parseSprint(json.RawMessage(c.raw))
			if id != c.wantID || name != c.wantName {
				t.Errorf("parseSprint = %q, %q; want %q, %q", id, name, c.wantID, c.wantName)
			}
		})
	}
}

func TestLogicalTypeIsCaseInsensitiveAndUsesTheRequirementName(t *testing.T) {
	cases := map[string]string{
		"Task": "task", "task": "task", "EPIC": "epic", "Story": "story", "Bug": "bug",
		"Business Requirement": "requirement", "Requirement": "", "Sub-task": "",
	}
	for name, want := range cases {
		if got := logicalType(name, "Business Requirement"); got != want {
			t.Errorf("logicalType(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestBuildJQL(t *testing.T) {
	names := jiraTypeNames(backend.AllTypes, "Requirement")
	got := buildJQL("PLAT", " labels = promo ", "2026-09-05T10:42:00Z", names)
	want := `project = PLAT AND issuetype in ("Task", "Epic", "Story", "Bug", "Requirement") AND (labels = promo) AND updated >= "2026-09-05 09:42" ORDER BY key ASC`
	if got != want {
		t.Errorf("jql =\n%s\nwant\n%s", got, want)
	}
	got = buildJQL("PLAT", "", "", jiraTypeNames([]string{backend.TypeBug}, "Requirement"))
	want = `project = PLAT AND issuetype in ("Bug") ORDER BY key ASC`
	if got != want {
		t.Errorf("minimal jql = %s", got)
	}
	if got := buildJQL("PLAT", "", "not a time", names); got != `project = PLAT AND issuetype in ("Task", "Epic", "Story", "Bug", "Requirement") ORDER BY key ASC` {
		t.Errorf("an unparseable since is dropped: %s", got)
	}
}

func TestParseIssueMapsEveryColumn(t *testing.T) {
	raw := corejira.RawIssue{ID: "10412", Key: "PLAT-412", Fields: map[string]json.RawMessage{
		"summary":           json.RawMessage(`"Checkout: apply promo code at payment step"`),
		"status":            json.RawMessage(`{"name":"In Progress"}`),
		"assignee":          json.RawMessage(`{"name":"ranand","displayName":"R. Anand"}`),
		"reporter":          json.RawMessage(`{"name":"po","displayName":"Product Owner"}`),
		"priority":          json.RawMessage(`{"name":"High"}`),
		"labels":            json.RawMessage(`["checkout","promo"]`),
		"issuetype":         json.RawMessage(`{"name":"Story"}`),
		"project":           json.RawMessage(`{"key":"PLAT"}`),
		"created":           json.RawMessage(`"2026-08-01T09:00:00.000+0000"`),
		"updated":           json.RawMessage(`"2026-09-05T09:58:00.000+0000"`),
		"customfield_10020": json.RawMessage(`[{"id":12,"name":"Sprint 12","state":"ACTIVE"}]`),
		"customfield_10016": json.RawMessage(`5`),
		"customfield_10014": json.RawMessage(`"PLAT-350"`),
		"customfield_10019": json.RawMessage(`"0|i0002:"`),
	}}
	ids := fieldIDs{Sprint: "customfield_10020", Points: "customfield_10016", EpicLink: "customfield_10014", Rank: "customfield_10019"}
	iss := parseIssue(raw, ids, "Requirement")
	if iss.Key != "PLAT-412" || iss.ID != "10412" || iss.Project != "PLAT" || iss.Type != "story" ||
		iss.Summary != "Checkout: apply promo code at payment step" || iss.Status != "In Progress" ||
		iss.Assignee != "R. Anand" || iss.Reporter != "Product Owner" || iss.Priority != "High" ||
		len(iss.Labels) != 2 || iss.SprintID != "12" || iss.SprintName != "Sprint 12" ||
		iss.ParentKey != "PLAT-350" || iss.StoryPoints == nil || *iss.StoryPoints != 5 ||
		iss.Rank != "0|i0002:" || iss.Created != "2026-08-01T09:00:00.000+0000" {
		t.Errorf("issue = %+v", iss)
	}
}

func TestParseIssuePrefersParentOverEpicLinkAndToleratesMissingFields(t *testing.T) {
	raw := corejira.RawIssue{ID: "1", Key: "PLAT-1", Fields: map[string]json.RawMessage{
		"summary":           json.RawMessage(`"x"`),
		"issuetype":         json.RawMessage(`{"name":"Sub-task"}`),
		"parent":            json.RawMessage(`{"key":"PLAT-412"}`),
		"customfield_10014": json.RawMessage(`"PLAT-350"`),
		"assignee":          json.RawMessage(`null`),
		"labels":            json.RawMessage(`null`),
	}}
	iss := parseIssue(raw, fieldIDs{EpicLink: "customfield_10014"}, "Requirement")
	if iss.ParentKey != "PLAT-412" {
		t.Errorf("parent = %q, want the parent field to win", iss.ParentKey)
	}
	if iss.Type != "" {
		t.Errorf("type = %q, want empty for an unknown Jira type", iss.Type)
	}
	if iss.Assignee != "" || iss.Labels == nil || len(iss.Labels) != 0 || iss.StoryPoints != nil {
		t.Errorf("issue = %+v", iss)
	}
}

func TestParseLinksSeesBothDirections(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":{"name":"Tested By","inward":"is tested by","outward":"tests"},"inwardIssue":{"key":"XT-1018","fields":{"summary":"Promo code applies discount","issuetype":{"name":"Test"}}}},
		{"type":{"name":"Relates"},"outwardIssue":{"key":"PLAT-388","fields":{"summary":"Promo codes must be single-use","issuetype":{"name":"Requirement"}}}}
	]`)
	links := parseLinks(raw)
	if len(links) != 2 {
		t.Fatalf("links = %+v", links)
	}
	if links[0] != (backend.Link{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "Promo code applies discount", IssueType: "Test"}) {
		t.Errorf("first = %+v", links[0])
	}
	if links[1] != (backend.Link{Direction: "outward", Type: "Relates", Key: "PLAT-388", Summary: "Promo codes must be single-use", IssueType: "Requirement"}) {
		t.Errorf("second = %+v", links[1])
	}
	if got := parseLinks(nil); got == nil || len(got) != 0 {
		t.Errorf("nil raw should give an empty slice, got %#v", got)
	}
}
```

- [ ] **Step 2: Run to see the compile failure**

Run (inside `tam/`): `go test ./internal/backend/jira/`
Expected: FAIL, `undefined: parseSprint` and friends.

- [ ] **Step 3: The field mapping**

Create `tam/internal/backend/jira/fields.go`:

```go
package jira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// DefaultRequirementType is the Jira issue type name TAM assumes for
// requirements when the profile has not set one.
const DefaultRequirementType = "Requirement"

// baseFields are the search fields the grid needs, before the custom ones.
var baseFields = []string{"summary", "status", "assignee", "reporter", "priority", "labels", "issuetype", "project", "parent", "created", "updated"}

// fieldIDs are the discovered custom field ids. Any may be empty when the
// instance lacks the field.
type fieldIDs struct {
	Sprint   string
	Points   string
	EpicLink string
	Rank     string
}

func (f fieldIDs) list() []string {
	var out []string
	for _, id := range []string{f.Sprint, f.Points, f.EpicLink, f.Rank} {
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

// logicalType maps a Jira issue type name to one of the five TAM types, or
// "" when it is none of them.
func logicalType(name, requirementType string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "task":
		return backend.TypeTask
	case "epic":
		return backend.TypeEpic
	case "story":
		return backend.TypeStory
	case "bug":
		return backend.TypeBug
	}
	if n != "" && n == strings.ToLower(strings.TrimSpace(requirementType)) {
		return backend.TypeRequirement
	}
	return ""
}

// jiraTypeNames turns logical types into the Jira names the JQL quotes.
func jiraTypeNames(types []string, requirementType string) []string {
	names := make([]string, 0, len(types))
	for _, t := range types {
		switch t {
		case backend.TypeTask:
			names = append(names, "Task")
		case backend.TypeEpic:
			names = append(names, "Epic")
		case backend.TypeStory:
			names = append(names, "Story")
		case backend.TypeBug:
			names = append(names, "Bug")
		case backend.TypeRequirement:
			names = append(names, requirementType)
		}
	}
	return names
}

// buildJQL is the sync scope: the project, the type list, the profile's
// scope JQL in parentheses when set, the incremental clause when since
// parses, and a stable order by key so paging never skips an issue.
func buildJQL(projectKey, scopeJQL, since string, typeNames []string) string {
	quoted := make([]string, len(typeNames))
	for i, n := range typeNames {
		quoted[i] = strconv.Quote(n)
	}
	jql := fmt.Sprintf("project = %s AND issuetype in (%s)", projectKey, strings.Join(quoted, ", "))
	if s := strings.TrimSpace(scopeJQL); s != "" {
		jql += " AND (" + s + ")"
	}
	if extra := sinceClause(since); extra != "" {
		jql += " AND " + extra
	}
	return jql + " ORDER BY key ASC"
}

// sinceClause backdates the cut-off by an hour, as XTM does, so clock skew
// between the app and Jira cannot hide an update. An unparseable value
// yields no clause, which makes the sync a full pull rather than a wrong one.
func sinceClause(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`updated >= "%s"`, t.UTC().Add(-time.Hour).Format("2006-01-02 15:04"))
}

type named struct {
	Name string `json:"name"`
}

type userRef struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type keyed struct {
	Key string `json:"key"`
}

// parseIssue maps one raw search hit onto the grid row.
func parseIssue(raw corejira.RawIssue, ids fieldIDs, requirementType string) backend.Issue {
	iss := backend.Issue{Key: raw.Key, ID: raw.ID, Labels: []string{}}
	f := raw.Fields
	_ = json.Unmarshal(f["summary"], &iss.Summary)
	_ = json.Unmarshal(f["created"], &iss.Created)
	_ = json.Unmarshal(f["updated"], &iss.Updated)
	var labels []string
	if err := json.Unmarshal(f["labels"], &labels); err == nil && labels != nil {
		iss.Labels = labels
	}
	var status, priority, issueType named
	if err := json.Unmarshal(f["status"], &status); err == nil {
		iss.Status = status.Name
	}
	if err := json.Unmarshal(f["priority"], &priority); err == nil {
		iss.Priority = priority.Name
	}
	if err := json.Unmarshal(f["issuetype"], &issueType); err == nil {
		iss.Type = logicalType(issueType.Name, requirementType)
	}
	var assignee, reporter *userRef
	if err := json.Unmarshal(f["assignee"], &assignee); err == nil && assignee != nil {
		iss.Assignee = displayName(*assignee)
	}
	if err := json.Unmarshal(f["reporter"], &reporter); err == nil && reporter != nil {
		iss.Reporter = displayName(*reporter)
	}
	var project, parent *keyed
	if err := json.Unmarshal(f["project"], &project); err == nil && project != nil {
		iss.Project = project.Key
	}
	if err := json.Unmarshal(f["parent"], &parent); err == nil && parent != nil && parent.Key != "" {
		iss.ParentKey = parent.Key
	} else if ids.EpicLink != "" {
		var epic string
		if err := json.Unmarshal(f[ids.EpicLink], &epic); err == nil {
			iss.ParentKey = epic
		}
	}
	if ids.Sprint != "" {
		iss.SprintID, iss.SprintName = parseSprint(f[ids.Sprint])
	}
	if ids.Points != "" {
		var pts *float64
		if err := json.Unmarshal(f[ids.Points], &pts); err == nil && pts != nil {
			iss.StoryPoints = pts
		}
	}
	if ids.Rank != "" {
		_ = json.Unmarshal(f[ids.Rank], &iss.Rank)
	}
	return iss
}

func displayName(u userRef) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Name
}

// Legacy Sprint values are toString dumps of the GreenHopper sprint object.
var (
	legacySprintID   = regexp.MustCompile(`\bid=(\d+)`)
	legacySprintName = regexp.MustCompile(`\bname=([^,\]]*)`)
)

// parseSprint reads the Sprint custom field in either shape Jira DC uses
// (an array of objects on Jira Software 8.x and later, an array of
// toString dumps before that; a single object also occurs) and returns the
// last entry, which is the sprint the issue is in now.
//
// NOTE(tam): both shapes come from documentation and XTM's field notes, not
// from a live instance. Verify against a real Jira DC before Phase 3 builds
// sprint entities on top of this.
func parseSprint(raw json.RawMessage) (id, name string) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", ""
	}
	type sprintObj struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
	}
	var objs []sprintObj
	if err := json.Unmarshal(raw, &objs); err == nil {
		if len(objs) == 0 {
			return "", ""
		}
		last := objs[len(objs)-1]
		return last.ID.String(), last.Name
	}
	var one sprintObj
	if err := json.Unmarshal(raw, &one); err == nil && (one.ID != "" || one.Name != "") {
		return one.ID.String(), one.Name
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil && len(strs) > 0 {
		last := strs[len(strs)-1]
		if m := legacySprintID.FindStringSubmatch(last); m != nil {
			id = m[1]
		}
		if m := legacySprintName.FindStringSubmatch(last); m != nil {
			name = strings.TrimSpace(m[1])
		}
		return id, name
	}
	return "", ""
}

// parseLinks flattens Jira's issuelinks array: each entry names the link
// type and carries either an inwardIssue or an outwardIssue.
func parseLinks(raw json.RawMessage) []backend.Link {
	out := []backend.Link{}
	if len(raw) == 0 {
		return out
	}
	type linked struct {
		Key    string `json:"key"`
		Fields struct {
			Summary   string `json:"summary"`
			IssueType named  `json:"issuetype"`
		} `json:"fields"`
	}
	var entries []struct {
		Type    named   `json:"type"`
		Inward  *linked `json:"inwardIssue"`
		Outward *linked `json:"outwardIssue"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return out
	}
	for _, e := range entries {
		if e.Inward != nil {
			out = append(out, backend.Link{Direction: "inward", Type: e.Type.Name, Key: e.Inward.Key, Summary: e.Inward.Fields.Summary, IssueType: e.Inward.Fields.IssueType.Name})
		}
		if e.Outward != nil {
			out = append(out, backend.Link{Direction: "outward", Type: e.Type.Name, Key: e.Outward.Key, Summary: e.Outward.Fields.Summary, IssueType: e.Outward.Fields.IssueType.Name})
		}
	}
	return out
}
```

- [ ] **Step 4: Run the mapping tests**

Run (inside `tam/`): `go test ./internal/backend/jira/ -run 'TestParse|TestLogical|TestBuild' -v`
Expected: PASS for the six tests.

- [ ] **Step 5: Failing backend tests**

Create `tam/internal/backend/jira/jira_test.go`:

```go
package jira_test

import (
	"context"
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
	fields     string // the /rest/api/2/field body
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
		case strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/PLAT-412"):
			_, _ = w.Write([]byte(`{"id":"1","key":"PLAT-412","fields":{"description":"As a shopper","issuelinks":[{"type":{"name":"Tested By"},"inwardIssue":{"key":"XT-1018","fields":{"summary":"Applies discount","issuetype":{"name":"Test"}}}}],"customfield_10016":5}}`))
		case r.URL.Path == "/rest/api/2/project/PLAT":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"1","name":"Task"},{"id":"7","name":"Business Requirement"}]}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func newBackend(t *testing.T, fields string) (*jirabackend.Backend, *fakeJira) {
	t.Helper()
	f := &fakeJira{fields: fields}
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	c := corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	return jirabackend.New(c, "Business Requirement"), f
}

const twoFields = `[{"id":"customfield_10020","name":"Sprint","custom":true},{"id":"customfield_10016","name":"Story Points","custom":true}]`

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
	wantJQL := `project = PLAT AND issuetype in ("Task", "Epic", "Story", "Bug", "Business Requirement") AND (labels = promo) AND updated >= "2026-09-05 09:42" ORDER BY key ASC`
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
	if err != nil || len(types) != 2 || types[1].Name != "Business Requirement" {
		t.Errorf("types = %+v, %v", types, err)
	}
}
```

- [ ] **Step 6: The backend**

Create `tam/internal/backend/jira/jira.go`:

```go
// Package jira is the IssueBackend for a live Jira Data Center, built on the
// suite's shared transport. It owns the JQL, the custom-field discovery, and
// the mapping from Jira's field shapes to TAM's issue.
package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// Backend talks to one Jira instance for one profile.
type Backend struct {
	c               *corejira.Client
	requirementType string

	mu         sync.Mutex
	ids        fieldIDs
	discovered bool
}

// New builds the backend. requirementType is the profile's Jira name for
// requirements; empty means DefaultRequirementType.
func New(c *corejira.Client, requirementType string) *Backend {
	if strings.TrimSpace(requirementType) == "" {
		requirementType = DefaultRequirementType
	}
	return &Backend{c: c, requirementType: strings.TrimSpace(requirementType)}
}

// TestConnection fetches the authenticated user.
func (b *Backend) TestConnection(ctx context.Context) (backend.User, error) {
	u, err := b.c.Myself(ctx)
	if err != nil {
		return backend.User{}, fmt.Errorf("connection test failed: %w", err)
	}
	return backend.User{Name: u.Name, DisplayName: u.DisplayName}, nil
}

// IsDemo is false: this backend always talks to a server.
func (b *Backend) IsDemo() bool { return false }

// discover resolves the custom field ids once. A field the instance does not
// have is logged once and left empty; a transport failure is logged and
// retried on the next call, so a flaky first request does not blank the
// columns for the rest of the session.
func (b *Backend) discover(ctx context.Context) fieldIDs {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.discovered {
		return b.ids
	}
	var ids fieldIDs
	ok := true
	for _, want := range []struct {
		name string
		dst  *string
	}{
		{"Sprint", &ids.Sprint},
		{"Story Points", &ids.Points},
		{"Epic Link", &ids.EpicLink},
		{"Rank", &ids.Rank},
	} {
		id, err := b.c.CustomFieldID(ctx, want.name)
		switch {
		case err == nil:
			*want.dst = id
		case errors.Is(err, corejira.ErrFieldNotFound):
			log.Printf("tam: %s has no %q custom field; that column stays empty", b.c.BaseURL(), want.name)
		default:
			log.Printf("tam: custom field discovery failed, syncing without custom columns this time: %v", err)
			ok = false
		}
	}
	if ok {
		b.ids = ids
		b.discovered = true
	}
	return ids
}

// SearchIssuesPage runs one page of the scope JQL and maps the rows.
func (b *Backend) SearchIssuesPage(ctx context.Context, projectKey, scopeJQL, since string, types []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	ids := b.discover(ctx)
	jql := buildJQL(projectKey, scopeJQL, since, jiraTypeNames(types, b.requirementType))
	fields := append(append([]string{}, baseFields...), ids.list()...)
	page, err := b.c.SearchIssues(ctx, jql, fields, startAt, maxResults)
	if err != nil {
		return nil, 0, err
	}
	issues := make([]backend.Issue, 0, len(page.Issues))
	for _, raw := range page.Issues {
		issues = append(issues, parseIssue(raw, ids, b.requirementType))
	}
	return issues, page.Total, nil
}

// GetIssueDetail fetches the description, the links, and the discovered
// custom fields for one issue.
func (b *Backend) GetIssueDetail(ctx context.Context, key string) (backend.IssueDetail, error) {
	ids := b.discover(ctx)
	fields := append([]string{"description", "issuelinks"}, ids.list()...)
	raw, err := b.c.GetIssue(ctx, key, fields)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	d := backend.IssueDetail{Key: raw.Key, Links: parseLinks(raw.Fields["issuelinks"]), Fields: map[string]any{}}
	_ = json.Unmarshal(raw.Fields["description"], &d.Description)
	for _, id := range ids.list() {
		if v, ok := raw.Fields[id]; ok && len(v) > 0 && string(v) != "null" {
			var decoded any
			if err := json.Unmarshal(v, &decoded); err == nil {
				d.Fields[id] = decoded
			}
		}
	}
	return d, nil
}

// IssueTypes lists the project's issue types.
func (b *Backend) IssueTypes(ctx context.Context, projectKey string) ([]backend.IssueType, error) {
	types, err := b.c.IssueTypes(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	out := make([]backend.IssueType, 0, len(types))
	for _, t := range types {
		out = append(out, backend.IssueType{ID: t.ID, Name: t.Name})
	}
	return out, nil
}
```

- [ ] **Step 7: Run the package and commit**

Run (inside `tam/`): `go vet ./internal/backend/... && go test ./internal/backend/jira/ -v -count=1`
Expected: PASS for all nine tests. The "no Epic Link" and "no Rank" log lines from the first test are expected output.

```bash
git add tam/internal/backend/jira
git commit -m "feat(tam): the Jira issue backend on the shared transport"
```

---

### Task 6: The sync engine

Pages issues from the backend into the store, reports progress, records sync state, and turns a mid-sync failure into a partial-sync error that keeps what landed.

**Files:**
- Create: `tam/internal/syncer/syncer.go`, `tam/internal/syncer/syncer_test.go`
- Test: `go test ./internal/syncer/` inside `tam/`

**Interfaces:**
- Consumes: `backend.IssueBackend`, `backend.AllTypes` (Task 2); `issuerepo.Repository.UpsertPage`, `SyncState`, `SetSyncState`, `CountIssues` (Tasks 2, 3); `demo.New` (Task 4) in tests.
- Produces (package `agile-suite/tam/internal/syncer`): `Progress`, `Summary`, `PartialSyncError`, `New(b backend.IssueBackend, repo *issuerepo.Repository) *Engine`, `(*Engine).Sync(ctx, profileID, projectKey, scopeJQL string, full bool, onProgress func(Progress)) (Summary, error)`, and the tunable fields `Engine.PageSize` and `Engine.Now`.

- [ ] **Step 1: Failing tests**

Create `tam/internal/syncer/syncer_test.go`:

```go
package syncer_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/syncer"
	"agile-suite/tam/internal/tamstore"
)

func newRepo(t *testing.T) *issuerepo.Repository {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return issuerepo.New(db.DB())
}

// fake is a scripted IssueBackend: fixed pages, an optional page that
// fails, and a record of the since values it was asked for.
type fake struct {
	pages     [][]backend.Issue
	failPage  int // 1-based page index that returns failErr; 0 for none
	failErr   error
	connErr   error
	sinceSeen []string
}

func (f *fake) TestConnection(context.Context) (backend.User, error) {
	return backend.User{Name: "fake"}, f.connErr
}
func (f *fake) IsDemo() bool { return false }
func (f *fake) SearchIssuesPage(_ context.Context, _, _, since string, _ []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	f.sinceSeen = append(f.sinceSeen, since)
	total := 0
	for _, p := range f.pages {
		total += len(p)
	}
	idx := startAt / maxResults
	if f.failPage > 0 && idx+1 == f.failPage {
		return nil, 0, f.failErr
	}
	if idx >= len(f.pages) {
		return []backend.Issue{}, total, nil
	}
	return f.pages[idx], total, nil
}
func (f *fake) GetIssueDetail(context.Context, string) (backend.IssueDetail, error) {
	return backend.IssueDetail{}, errors.New("not used")
}
func (f *fake) IssueTypes(context.Context, string) ([]backend.IssueType, error) { return nil, nil }

func issue(key, typ string) backend.Issue {
	return backend.Issue{Key: key, ID: key, Project: "PLAT", Type: typ, Summary: key, Status: "To Do", Rank: key, Updated: "2026-09-01T00:00:00Z"}
}

func fixedClock(ts ...time.Time) func() time.Time {
	i := 0
	return func() time.Time {
		t := ts[i]
		if i < len(ts)-1 {
			i++
		}
		return t
	}
}

func TestSyncPagesEverythingAndRecordsState(t *testing.T) {
	repo := newRepo(t)
	fb := &fake{pages: [][]backend.Issue{
		{issue("PLAT-1", "task"), issue("PLAT-2", "story")},
		{issue("PLAT-3", "bug"), issue("PLAT-4", "")},
	}}
	e := syncer.New(fb, repo)
	e.PageSize = 2
	start := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	e.Now = fixedClock(start, start.Add(3*time.Second))

	var events []syncer.Progress
	sum, err := e.Sync(context.Background(), "p1", "PLAT", "", false, func(p syncer.Progress) { events = append(events, p) })
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if sum.Fetched != 4 || sum.Upserted != 3 || sum.Skipped != 1 || sum.Full || sum.Elapsed != "3s" {
		t.Errorf("summary = %+v", sum)
	}
	n, _ := repo.CountIssues(context.Background(), "p1")
	if n != 3 {
		t.Errorf("cached = %d, want 3 (the untyped issue is skipped)", n)
	}
	st, _ := repo.SyncState(context.Background(), "p1")
	if st.LastSynced != "2026-09-05T10:42:00Z" || st.LastFull != "" || st.LastError != "" {
		t.Errorf("state = %+v", st)
	}
	if len(fb.sinceSeen) == 0 || fb.sinceSeen[0] != "" {
		t.Errorf("first sync must not send a since: %v", fb.sinceSeen)
	}
	last := events[len(events)-1]
	if !last.Done || last.Fetched != 4 || last.Total != 4 {
		t.Errorf("last event = %+v", last)
	}
	if events[0].Stage == "" {
		t.Error("first event should name a stage")
	}
}

func TestIncrementalSendsLastSyncedAndFullClearsAndSendsNothing(t *testing.T) {
	repo := newRepo(t)
	fb := &fake{pages: [][]backend.Issue{{issue("PLAT-1", "task")}}}
	e := syncer.New(fb, repo)
	e.PageSize = 10
	first := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	e.Now = fixedClock(first)
	if _, err := e.Sync(context.Background(), "p1", "PLAT", "", false, nil); err != nil {
		t.Fatal(err)
	}
	second := first.Add(time.Hour)
	e.Now = fixedClock(second)
	fb.pages = [][]backend.Issue{{issue("PLAT-2", "task")}}
	if _, err := e.Sync(context.Background(), "p1", "PLAT", "", false, nil); err != nil {
		t.Fatal(err)
	}
	if got := fb.sinceSeen[len(fb.sinceSeen)-1]; got != "2026-09-05T10:00:00Z" {
		t.Errorf("incremental since = %q", got)
	}
	n, _ := repo.CountIssues(context.Background(), "p1")
	if n != 2 {
		t.Errorf("after incremental: %d rows, want 2 (PLAT-1 kept)", n)
	}
	e.Now = fixedClock(second.Add(time.Hour))
	sum, err := e.Sync(context.Background(), "p1", "PLAT", "", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := fb.sinceSeen[len(fb.sinceSeen)-1]; got != "" {
		t.Errorf("full sync since = %q, want empty", got)
	}
	if !sum.Full {
		t.Error("summary should say full")
	}
	n, _ = repo.CountIssues(context.Background(), "p1")
	if n != 1 {
		t.Errorf("after full: %d rows, want 1 (PLAT-1 cleared)", n)
	}
	st, _ := repo.SyncState(context.Background(), "p1")
	if st.LastFull != "2026-09-05T12:00:00Z" || st.LastSynced != st.LastFull {
		t.Errorf("state after full = %+v", st)
	}
}

func TestPageFailureKeepsWhatLandedAndDoesNotAdvanceState(t *testing.T) {
	repo := newRepo(t)
	fb := &fake{
		pages:    [][]backend.Issue{{issue("PLAT-1", "task")}, {issue("PLAT-2", "task")}},
		failPage: 2,
		failErr:  errors.New("jira: 502 Bad Gateway"),
	}
	e := syncer.New(fb, repo)
	e.PageSize = 1
	var done bool
	_, err := e.Sync(context.Background(), "p1", "PLAT", "", false, func(p syncer.Progress) { done = done || p.Done })
	var pse *syncer.PartialSyncError
	if !errors.As(err, &pse) || pse.Pages != 1 || !errors.Is(err, fb.failErr) {
		t.Fatalf("err = %v, want a PartialSyncError after 1 page wrapping the cause", err)
	}
	if !done {
		t.Error("a failed sync still sends the terminal progress event")
	}
	n, _ := repo.CountIssues(context.Background(), "p1")
	if n != 1 {
		t.Errorf("rows = %d, want the first page kept", n)
	}
	st, _ := repo.SyncState(context.Background(), "p1")
	if st.LastSynced != "" || st.LastError != "jira: 502 Bad Gateway" {
		t.Errorf("state = %+v: last_synced must stay empty, last_error must be set", st)
	}
}

func TestConnectionFailureStopsBeforeAnyPage(t *testing.T) {
	repo := newRepo(t)
	fb := &fake{pages: [][]backend.Issue{{issue("PLAT-1", "task")}}, connErr: errors.New("401 Unauthorized")}
	e := syncer.New(fb, repo)
	_, err := e.Sync(context.Background(), "p1", "PLAT", "", false, nil)
	if err == nil || errors.As(err, new(*syncer.PartialSyncError)) {
		t.Fatalf("err = %v, want a plain error, not a partial sync", err)
	}
	if len(fb.sinceSeen) != 0 {
		t.Error("no page must be requested after a failed connection test")
	}
	st, _ := repo.SyncState(context.Background(), "p1")
	if st.LastError != "401 Unauthorized" {
		t.Errorf("last error = %q", st.LastError)
	}
}

func TestSyncAgainstTheDemoBackend(t *testing.T) {
	repo := newRepo(t)
	e := syncer.New(demobackend.New("DEMO"), repo)
	sum, err := e.Sync(context.Background(), "p1", "DEMO", "", false, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if sum.Fetched != 60 || sum.Upserted != 60 {
		t.Errorf("summary = %+v", sum)
	}
	page, err := repo.ListIssues(context.Background(), "p1", issuerepo.IssueQuery{Types: []string{"epic"}})
	if err != nil || page.Total != 4 {
		t.Errorf("epics after sync = %d, %v", page.Total, err)
	}
}
```

- [ ] **Step 2: Run to see the compile failure**

Run (inside `tam/`): `go test ./internal/syncer/`
Expected: FAIL, package does not exist.

- [ ] **Step 3: The engine**

Create `tam/internal/syncer/syncer.go`:

```go
// Package syncer pulls a profile's issues from its backend into the store,
// one page per transaction, and keeps the per-profile sync state. It has
// no Wails dependency: the app forwards Progress as events.
package syncer

import (
	"context"
	"fmt"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Progress is one frame of a running sync. Its JSON shape matches XTM's so
// the shared sync reducer understands it: Done marks the terminal frame.
type Progress struct {
	Phase   string `json:"phase"`
	Fetched int    `json:"fetched"`
	Total   int    `json:"total"`
	Done    bool   `json:"done"`
	Stage   string `json:"stage"`
}

// Summary is what a finished sync reports.
type Summary struct {
	Fetched  int    `json:"fetched"`
	Upserted int    `json:"upserted"`
	Skipped  int    `json:"skipped"`
	Full     bool   `json:"full"`
	Elapsed  string `json:"elapsed"`
}

// PartialSyncError says a sync stopped after some pages had already been
// written. The rows stay; the sync state is not advanced.
type PartialSyncError struct {
	Pages int
	Err   error
}

func (e *PartialSyncError) Error() string {
	return fmt.Sprintf("sync stopped after %d page(s): %v", e.Pages, e.Err)
}

func (e *PartialSyncError) Unwrap() error { return e.Err }

// Engine syncs one backend into one store.
type Engine struct {
	b    backend.IssueBackend
	repo *issuerepo.Repository
	// PageSize is how many issues one search asks for. Fifty keeps a page
	// well under Jira DC's response limits with the fields the grid needs.
	PageSize int
	// Now is the clock, replaceable in tests.
	Now func() time.Time
}

// New builds an engine with the default page size and clock.
func New(b backend.IssueBackend, repo *issuerepo.Repository) *Engine {
	return &Engine{b: b, repo: repo, PageSize: 50, Now: time.Now}
}

// Sync pulls the profile's issues. An incremental sync (full=false) asks
// the backend for issues updated since the last sync; a full sync clears
// the profile's rows inside the first page's transaction and pulls
// everything. The sync's start time becomes the new last_synced so an
// issue updated during the sync is picked up next time.
func (e *Engine) Sync(ctx context.Context, profileID, projectKey, scopeJQL string, full bool, onProgress func(Progress)) (Summary, error) {
	emit := func(p Progress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	start := e.Now().UTC()
	sum := Summary{Full: full}

	emit(Progress{Phase: "issues", Stage: "Checking the connection"})
	state, err := e.repo.SyncState(ctx, profileID)
	if err != nil {
		return sum, err
	}
	if _, err := e.b.TestConnection(ctx); err != nil {
		return e.fail(ctx, profileID, state, 0, sum, err, emit)
	}

	since := ""
	if !full {
		since = state.LastSynced
	}
	pages, startAt, total := 0, 0, -1
	for total < 0 || startAt < total {
		page, n, err := e.b.SearchIssuesPage(ctx, projectKey, scopeJQL, since, backend.AllTypes, startAt, e.PageSize)
		if err != nil {
			return e.fail(ctx, profileID, state, pages, sum, err, emit)
		}
		total = n
		keep := make([]backend.Issue, 0, len(page))
		for _, iss := range page {
			if iss.Type != "" {
				keep = append(keep, iss)
			}
		}
		if err := e.repo.UpsertPage(ctx, profileID, keep, start, full && pages == 0); err != nil {
			return e.fail(ctx, profileID, state, pages, sum, err, emit)
		}
		pages++
		sum.Fetched += len(page)
		sum.Upserted += len(keep)
		sum.Skipped += len(page) - len(keep)
		startAt += len(page)
		emit(Progress{Phase: "issues", Fetched: sum.Fetched, Total: total, Stage: "Fetching issues"})
		if len(page) == 0 {
			break
		}
	}

	next := issuerepo.SyncState{LastSynced: start.Format(time.RFC3339), LastFull: state.LastFull}
	if full {
		next.LastFull = next.LastSynced
	}
	if err := e.repo.SetSyncState(ctx, profileID, next); err != nil {
		return e.fail(ctx, profileID, state, pages, sum, err, emit)
	}
	sum.Elapsed = e.Now().Sub(start).Round(time.Millisecond).String()
	emit(Progress{Phase: "issues", Fetched: sum.Fetched, Total: total, Done: true, Stage: "Done"})
	return sum, nil
}

// fail records the error without advancing last_synced, sends the terminal
// frame, and wraps the cause as a PartialSyncError when pages had landed.
func (e *Engine) fail(ctx context.Context, profileID string, state issuerepo.SyncState, pages int, sum Summary, cause error, emit func(Progress)) (Summary, error) {
	state.LastError = cause.Error()
	_ = e.repo.SetSyncState(ctx, profileID, state)
	emit(Progress{Phase: "issues", Fetched: sum.Fetched, Done: true, Stage: "Failed"})
	if pages > 0 {
		return sum, &PartialSyncError{Pages: pages, Err: cause}
	}
	return sum, cause
}
```

- [ ] **Step 4: Run the package and commit**

Run (inside `tam/`): `go vet ./internal/syncer/ && go test ./internal/syncer/ -v -count=1`
Expected: PASS for all five tests.

```bash
git add tam/internal/syncer
git commit -m "feat(tam): the issue sync engine with partial-failure handling"
```

---

### Task 7: The bound App methods

Wire the store, the backends, and the engine into the Wails-bound `App`, then regenerate the bindings so Task 9 can import them.

**Files:**
- Create: `tam/app_issues.go`
- Modify: `tam/app.go` (three fields, two lines in `initStore`)
- Regenerate: `tam/frontend/wailsjs/go/main/App.d.ts`, `App.js`, `tam/frontend/wailsjs/go/models.ts`
- Test: `go build ./... && go vet ./...` inside `tam/`, then `wails generate module`

**Interfaces:**
- Consumes: `corejira.NewClient`, `WithCACert`, `WithInsecureTLS` (Task 1); `issuerepo` (Tasks 2, 3); `demobackend.New` (Task 4); `jirabackend.New` (Task 5); `syncer` (Task 6); `profile.Manager.Get`, `profile.CredentialStore.Load`, `settings.Manager.Get` (core).
- Produces (bound methods, exact signatures): `SyncIssues(profileID string, full bool) (syncer.Summary, error)`, `GetSyncState(profileID string) (issuerepo.SyncState, error)`, `ListIssues(profileID string, q issuerepo.IssueQuery) (issuerepo.IssuePage, error)`, `GetIssueDetail(profileID, key string) (backend.IssueDetail, error)`, `ListLinkedTests(profileID, key string) ([]issuerepo.LinkedTest, error)`, `ListSprints(profileID string) ([]issuerepo.SprintRef, error)`, `GetProfileSetting(profileID, key string) (string, error)`, `SetProfileSetting(profileID, key, value string) error`. The progress event is `tam:sync-progress` with a `syncer.Progress` payload.

- [ ] **Step 1: Fields on App**

In `tam/app.go`, add these imports:

```go
	"sync"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
```

Add three fields to the `App` struct, after `settings   *settings.Manager`:

```go
	repo       *issuerepo.Repository
	backendMu  sync.Mutex
	backends   map[string]backend.IssueBackend
```

In `initStore`, right after `a.dbPath = dbPath`, add:

```go
	a.repo = issuerepo.New(local.DB())
	a.backends = map[string]backend.IssueBackend{}
```

- [ ] **Step 2: The methods**

Create `tam/app_issues.go`:

```go
package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	corejira "agile-suite/core/jira"
	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
	jirabackend "agile-suite/tam/internal/backend/jira"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/suiteprofiles"
	"agile-suite/tam/internal/syncer"
)

const (
	// settingRequirementType is the per-profile key for the Jira issue type
	// name TAM treats as a requirement.
	settingRequirementType = "requirement_issue_type"
	// detailFreshFor is how long a cached detail is served without asking
	// Jira again.
	detailFreshFor = 10 * time.Minute
	// syncProgressEvent carries syncer.Progress frames to the frontend.
	syncProgressEvent = "tam:sync-progress"
)

// requireProfile is requireStore plus the profile row, so every issue
// method starts from a profile that exists.
func (a *App) requireProfile(profileID string) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	if strings.TrimSpace(profileID) == "" {
		return profile.Profile{}, errors.New("no profile selected")
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("profile %s: %w", profileID, err)
	}
	return p, nil
}

// backendFor returns the profile's backend, building it on first use. A
// demo URL gets the offline dataset; anything else gets a Jira client with
// the profile's TLS settings and the PAT from the credential store. The
// token goes into the client and nowhere else.
func (a *App) backendFor(p profile.Profile) (backend.IssueBackend, error) {
	a.backendMu.Lock()
	defer a.backendMu.Unlock()
	if b, ok := a.backends[p.ID]; ok {
		return b, nil
	}
	var b backend.IssueBackend
	if suiteprofiles.IsDemoURL(p.JiraURL) {
		b = demobackend.New(p.ProjectKey)
	} else {
		token, err := a.creds.Load(p.ID)
		if err != nil {
			return nil, fmt.Errorf("read the token for %s: %w", p.Name, err)
		}
		reqType, err := a.repo.ProfileSetting(a.ctx, p.ID, settingRequirementType)
		if err != nil {
			return nil, err
		}
		client := corejira.NewClient(p.JiraURL, token, tlsOptions(p)...)
		b = jirabackend.New(client, reqType)
	}
	a.backends[p.ID] = b
	return b, nil
}

// forgetBackend drops a cached backend so the next call rebuilds it, which
// is what changing the requirement type needs.
func (a *App) forgetBackend(profileID string) {
	a.backendMu.Lock()
	delete(a.backends, profileID)
	a.backendMu.Unlock()
}

// tlsOptions derives the client options from a profile's TLS settings.
func tlsOptions(p profile.Profile) []corejira.Option {
	var opts []corejira.Option
	if strings.TrimSpace(p.CACert) != "" {
		opts = append(opts, corejira.WithCACert(p.CACert))
	}
	if p.AllowUntrustedTLS {
		opts = append(opts, corejira.WithInsecureTLS(true))
	}
	return opts
}

func (a *App) emitProgress(p syncer.Progress) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, syncProgressEvent, p)
	}
}

// SyncIssues pulls the profile's issues, incrementally or in full, and
// emits tam:sync-progress frames while it runs. It returns when the sync
// has finished or failed.
func (a *App) SyncIssues(profileID string, full bool) (syncer.Summary, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return syncer.Summary{}, err
	}
	b, err := a.backendFor(p)
	if err != nil {
		return syncer.Summary{}, err
	}
	sum, err := syncer.New(b, a.repo).Sync(a.ctx, p.ID, p.ProjectKey, p.ScopeJQL, full, a.emitProgress)
	if err != nil {
		log.Printf("tam: sync %s (%s) failed: %v", p.Name, p.ProjectKey, err)
		return sum, err
	}
	log.Printf("tam: synced %s (%s): %d fetched, %d upserted, %d skipped in %s", p.Name, p.ProjectKey, sum.Fetched, sum.Upserted, sum.Skipped, sum.Elapsed)
	return sum, nil
}

// GetSyncState reports when the profile last synced and how many issues
// are cached.
func (a *App) GetSyncState(profileID string) (issuerepo.SyncState, error) {
	if err := a.requireStore(); err != nil {
		return issuerepo.SyncState{}, err
	}
	return a.repo.SyncState(a.ctx, profileID)
}

// ListIssues is the Backlog grid's page.
func (a *App) ListIssues(profileID string, q issuerepo.IssueQuery) (issuerepo.IssuePage, error) {
	if err := a.requireStore(); err != nil {
		return issuerepo.IssuePage{}, err
	}
	return a.repo.ListIssues(a.ctx, profileID, q)
}

// GetIssueDetail returns the cached detail when it is fresh, otherwise
// fetches it from the backend and caches it.
func (a *App) GetIssueDetail(profileID, key string) (backend.IssueDetail, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	cached, fetchedAt, ok, err := a.repo.ReadDetail(a.ctx, p.ID, key)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	if ok && time.Since(fetchedAt) < detailFreshFor {
		return cached, nil
	}
	b, err := a.backendFor(p)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	d, err := b.GetIssueDetail(a.ctx, key)
	if err != nil {
		return backend.IssueDetail{}, err
	}
	if err := a.repo.WriteDetail(a.ctx, p.ID, key, d, time.Now()); err != nil {
		return backend.IssueDetail{}, err
	}
	return d, nil
}

// ListLinkedTests returns the tests linked to key through the suite's
// requirement link type.
func (a *App) ListLinkedTests(profileID, key string) ([]issuerepo.LinkedTest, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	s, err := a.settings.Get()
	if err != nil {
		return nil, err
	}
	return a.repo.ListLinkedTests(a.ctx, profileID, key, s.RequirementLinkType)
}

// ListSprints returns the sprints seen in the cached issues.
func (a *App) ListSprints(profileID string) ([]issuerepo.SprintRef, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	sprints, err := a.repo.ListSprints(a.ctx, profileID)
	if err != nil {
		return nil, err
	}
	if sprints == nil {
		sprints = []issuerepo.SprintRef{}
	}
	return sprints, nil
}

// GetProfileSetting reads a per-profile TAM setting; "" when unset.
func (a *App) GetProfileSetting(profileID, key string) (string, error) {
	if err := a.requireStore(); err != nil {
		return "", err
	}
	if strings.TrimSpace(key) == "" {
		return "", errors.New("setting key is empty")
	}
	return a.repo.ProfileSetting(a.ctx, profileID, key)
}

// SetProfileSetting writes a per-profile TAM setting and drops the
// profile's cached backend, since the requirement type feeds into it.
func (a *App) SetProfileSetting(profileID, key, value string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("setting key is empty")
	}
	if err := a.repo.SetProfileSetting(a.ctx, profileID, key, strings.TrimSpace(value)); err != nil {
		return err
	}
	a.forgetBackend(profileID)
	return nil
}
```

- [ ] **Step 3: Build, vet, regenerate the bindings**

Run, inside `tam/`:

```bash
go build ./... && go vet ./... && go test ./internal/... -count=1
wails generate module
```

Expected: build and vet clean, every package `ok`; `tam/frontend/wailsjs/go/main/App.d.ts` now declares the eight new functions, and `tam/frontend/wailsjs/go/models.ts` gains the `backend`, `issuerepo`, and `syncer` namespaces. `git status --short --untracked-files=no` shows only `tam/app.go`, `tam/app_issues.go`, and the three regenerated `wailsjs` files. If the generator also rewrote line endings under `xtm/`, run `git checkout -- xtm/`.

- [ ] **Step 4: Commit**

```bash
git add tam/app.go tam/app_issues.go tam/frontend/wailsjs
git commit -m "feat(tam): bind issue sync, listing, detail, and profile settings to the frontend"
```

---

### Task 8: The sync reducer moves to `frontend/core`

XTM's pure `syncMachine` reducer and its test move into the shared package with a re-export shim left behind, so TAM's sync provider (Task 9) can use it. Nothing else of XTM's sync state moves.

**Files:**
- Move (git mv): `xtm/frontend/src/contexts/syncMachine.ts` → `frontend/core/src/contexts/syncMachine.ts`; `xtm/frontend/src/contexts/syncMachine.test.ts` → `frontend/core/src/contexts/syncMachine.test.ts`
- Create: `xtm/frontend/src/contexts/syncMachine.ts` (the shim, at the old path)
- Modify: `frontend/core/src/index.ts`
- Test: `npm test --workspaces --if-present` and `npm run typecheck --workspaces --if-present` from the repo root

**Interfaces:**
- Produces (from `@agile-suite/core`): `syncReducer`, `initialSyncState`, `canSync`, `canCommit`, `canSwitchProfile`, and the types `SyncProgress`, `SyncStatus`, `SyncMachineState`, `SyncAction`.

- [ ] **Step 1: Record the baseline**

From the repo root run `npm test --workspace xtm/frontend 2>&1 | grep "Tests "` and note the count (191 today) and `npm test --workspace frontend/core 2>&1 | grep "Tests "` (14 today). Their sum is the floor for Step 5.

- [ ] **Step 2: Move the two files**

From the repo root:

```bash
git mv xtm/frontend/src/contexts/syncMachine.ts      frontend/core/src/contexts/syncMachine.ts
git mv xtm/frontend/src/contexts/syncMachine.test.ts frontend/core/src/contexts/syncMachine.test.ts
```

- [ ] **Step 3: Give the reducer its own progress type**

The moved reducer imported `SyncProgress` from XTM's `api.ts`, which the shared package cannot see. In `frontend/core/src/contexts/syncMachine.ts`, replace the first line

```ts
import type { SyncProgress } from "../api";
```

with

```ts
// SyncProgress mirrors the Go syncer.Progress payload both apps emit on their
// progress event: XTM on "sync:progress", TAM on "tam:sync-progress". A frame
// with done:true is the terminal one. stage is a readable label for the
// running step, shown in the status bar.
export interface SyncProgress {
  phase: string;
  fetched: number;
  total: number;
  done: boolean;
  stage?: string;
}
```

In `frontend/core/src/contexts/syncMachine.test.ts`, change

```ts
import type { SyncProgress } from "../api";
```

to

```ts
import type { SyncProgress } from "./syncMachine";
```

Nothing else in either file changes.

- [ ] **Step 4: Export from the package and leave the shim**

Append to `frontend/core/src/index.ts`:

```ts
export {
  syncReducer,
  initialSyncState,
  canSync,
  canCommit,
  canSwitchProfile,
} from "./contexts/syncMachine";
export type {
  SyncProgress,
  SyncStatus,
  SyncMachineState,
  SyncAction,
} from "./contexts/syncMachine";
```

Create `xtm/frontend/src/contexts/syncMachine.ts` with exactly:

```ts
export {
  syncReducer,
  initialSyncState,
  canSync,
  canCommit,
  canSwitchProfile,
} from "@agile-suite/core";
export type { SyncStatus, SyncMachineState, SyncAction } from "@agile-suite/core";
```

XTM's `SyncContext.tsx` keeps importing `SyncProgress` from its own `api.ts`; that interface is structurally identical to the shared one, so the reducer accepts it unchanged.

- [ ] **Step 5: Prove nothing moved**

From the repo root:

```bash
npm test --workspaces --if-present 2>&1 | grep -E "^> |Tests "
npm run typecheck --workspaces --if-present
```

Expected: XTM's count dropped by the number of tests in `syncMachine.test.ts` and core's rose by the same number, so the two sum to the Step 1 baseline; TAM's 8 are unchanged; every typecheck is clean. Then `cd xtm/frontend && npm run build` must be clean too.

- [ ] **Step 6: Commit**

```bash
git add frontend/core/src xtm/frontend/src/contexts/syncMachine.ts
git commit -m "refactor(frontend): move the sync reducer into @agile-suite/core"
```

---

### Task 9: API layer, query hooks, the sync provider, and the status bar

The frontend plumbing: typed access to the eight new bindings, TanStack Query hooks with one key table, a TAM-local sync provider on the shared reducer, and the shell's Sync menu and status bar. The Backlog view itself is Task 10.

**Files:**
- Create: `tam/frontend/src/queries/keys.ts`, `queries/issues.ts`, `queries/invalidate.ts`, `queries/issues.test.tsx`, `contexts/SyncContext.tsx`, `contexts/SyncContext.test.tsx`, `lib/format.ts`, `lib/format.test.ts`
- Modify: `tam/frontend/src/api.ts`, `main.tsx`, `App.tsx`, `App.test.tsx`, `App.css`
- Test: `npx vitest run` and `npm run typecheck` inside `tam/frontend/`

**Interfaces:**
- Consumes: the regenerated bindings (Task 7); `syncReducer`, `initialSyncState`, `canSync`, `canSwitchProfile`, `SyncProgress`, `useNotice`, `call`, `errMsg` from `@agile-suite/core` (Tasks 5 of plan 0b, 8).
- Produces (`api.ts`): types `Issue`, `IssueType`, `Link`, `IssueDetail`, `IssueQuery`, `IssuePage`, `SprintRef`, `SyncState`, `SyncSummary`, `LinkedTest`, `SyncProgress`; the constant `ISSUE_TYPES`; functions `SyncIssues`, `GetSyncState`, `ListIssues`, `GetIssueDetail`, `ListLinkedTests`, `ListSprints`, `GetProfileSetting`, `SetProfileSetting`.
- Produces (`queries/`): `keys`, `useIssues(profileId, q)`, `useIssueDetail(profileId, key)`, `useLinkedTests(profileId, key)`, `useSprints(profileId)`, `useSyncState(profileId)`, `invalidateProfileData(qc, profileId)`.
- Produces (`contexts/SyncContext.tsx`): `SyncProvider`, `useSync()` returning `{ status, progress, syncError, canSync, canSwitchProfile, runSync }` where `runSync(full: boolean): Promise<void>`.
- Produces (`lib/format.ts`): `formatWhen(iso: string, now?: Date): string`.

- [ ] **Step 1: The API layer**

Replace `tam/frontend/src/api.ts` in full with:

```ts
// api.ts is the frontend's typed access to the Go backend. It re-exports the
// generated bindings and defines plain shapes for what they return, so state
// and test fixtures can be object literals.
//
// The bindings are re-exported through those shapes rather than raw: the
// generated wailsjs models are classes carrying every column of the shared
// Go struct, so a raw re-export would force fixtures to spell out fields TAM
// never reads. Arguments that are Go structs go through the generated
// class's createFrom so the binding receives the shape it declares.

import * as App from "../wailsjs/go/main/App";
import { issuerepo } from "../wailsjs/go/models";

export { EventsOn, BrowserOpenURL } from "../wailsjs/runtime/runtime";
export type { SyncProgress } from "@agile-suite/core";

export interface Profile {
  id: string;
  name: string;
  jiraUrl: string;
  projectKey: string;
  backend: string;
  createdAt: string;
}

export interface Settings {
  defaultProfileId: string;
  theme: string;
}

export interface HealthInfo {
  ok: boolean;
  error: string;
  dbPath: string;
  sharedPath: string;
  logPath: string;
}

export interface Diagnostics {
  version: string;
  dbPath: string;
  sharedPath: string;
  logPath: string;
  os: string;
  arch: string;
  goVersion: string;
  schemaVersion: number;
  profileCount: number;
  startupError: string;
}

export type IssueType = "task" | "epic" | "story" | "bug" | "requirement";

// ISSUE_TYPES is the five logical types in display order, with the chip
// label the grid and filter bar use.
export const ISSUE_TYPES: { id: IssueType; label: string; short: string }[] = [
  { id: "task", label: "Task", short: "Task" },
  { id: "epic", label: "Epic", short: "Epic" },
  { id: "story", label: "Story", short: "Story" },
  { id: "bug", label: "Bug", short: "Bug" },
  { id: "requirement", label: "Requirement", short: "Req" },
];

export interface Issue {
  key: string;
  id: string;
  project: string;
  type: IssueType | "";
  summary: string;
  status: string;
  assignee: string;
  reporter: string;
  priority: string;
  labels: string[];
  sprintId: string;
  sprintName: string;
  parentKey: string;
  storyPoints?: number | null;
  rank: string;
  created: string;
  updated: string;
}

export interface Link {
  direction: "inward" | "outward" | string;
  type: string;
  key: string;
  summary: string;
  issueType: string;
}

export interface IssueDetail {
  key: string;
  description: string;
  links: Link[];
  fields: Record<string, unknown>;
}

export interface IssueQuery {
  text: string;
  types: string[];
  sprintId: string;
  offset: number;
  limit: number;
}

export interface IssuePage {
  issues: Issue[];
  total: number;
}

export interface SprintRef {
  id: string;
  name: string;
}

export interface SyncState {
  lastSynced: string;
  lastFull: string;
  lastError: string;
  issueCount: number;
}

export interface SyncSummary {
  fetched: number;
  upserted: number;
  skipped: number;
  full: boolean;
  elapsed: string;
}

export interface LinkedTest {
  key: string;
  summary: string;
  linkType: string;
}

export const Health: () => Promise<HealthInfo> = App.Health;
export const GetDiagnostics: () => Promise<Diagnostics> = App.GetDiagnostics;
export const ListProfiles: () => Promise<Profile[]> = App.ListProfiles;
export const CreateProfile: (
  name: string,
  jiraUrl: string,
  projectKey: string,
  token: string,
  makeDefault: boolean,
) => Promise<Profile> = App.CreateProfile;
export const DeleteProfile: (id: string) => Promise<void> = App.DeleteProfile;
export const GetSettings: () => Promise<Settings> = App.GetSettings;
export const SetTheme: (theme: string) => Promise<void> = App.SetTheme;
export const SetDefaultProfile: (id: string) => Promise<void> =
  App.SetDefaultProfile;

export const SyncIssues: (profileId: string, full: boolean) => Promise<SyncSummary> =
  App.SyncIssues;
export const GetSyncState: (profileId: string) => Promise<SyncState> = App.GetSyncState;
export const ListIssues = (profileId: string, q: IssueQuery): Promise<IssuePage> =>
  App.ListIssues(profileId, issuerepo.IssueQuery.createFrom(q));
export const GetIssueDetail: (profileId: string, key: string) => Promise<IssueDetail> =
  App.GetIssueDetail;
export const ListLinkedTests: (profileId: string, key: string) => Promise<LinkedTest[]> =
  App.ListLinkedTests;
export const ListSprints: (profileId: string) => Promise<SprintRef[]> = App.ListSprints;
export const GetProfileSetting: (profileId: string, key: string) => Promise<string> =
  App.GetProfileSetting;
export const SetProfileSetting: (
  profileId: string,
  key: string,
  value: string,
) => Promise<void> = App.SetProfileSetting;

// isDemoUrl mirrors suiteprofiles.IsDemoURL in the backend: "demo" on its own
// or a "demo:" / "demo-" variant selects the offline dataset.
export function isDemoUrl(url?: string): boolean {
  const u = (url ?? "").trim().toLowerCase();
  return u === "demo" || u.startsWith("demo:") || u.startsWith("demo-");
}
```

Run `npm run typecheck` inside `tam/frontend/`. Expected: clean. If `tsc` reports that a plain shape is not assignable to a generated class parameter, that binding needs the same `createFrom` treatment `ListIssues` has; do not widen the plain shapes.

- [ ] **Step 2: Failing tests for the formatter and the hooks**

Create `tam/frontend/src/lib/format.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { formatWhen } from "./format";

describe("formatWhen", () => {
  const now = new Date("2026-09-05T14:00:00Z");
  it("is empty for an empty stamp", () => {
    expect(formatWhen("", now)).toBe("");
  });
  it("says today with the time for a same-day stamp", () => {
    const stamp = new Date("2026-09-05T10:42:00Z").toISOString();
    const expected = new Date(stamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    expect(formatWhen(stamp, now)).toBe(`today ${expected}`);
  });
  it("shows the date and time otherwise", () => {
    const stamp = "2026-09-01T08:00:00Z";
    const d = new Date(stamp);
    const expected = `${d.toLocaleDateString()} ${d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
    expect(formatWhen(stamp, now)).toBe(expected);
  });
  it("returns the raw text for something that is not a date", () => {
    expect(formatWhen("never", now)).toBe("never");
  });
});
```

Create `tam/frontend/src/queries/issues.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import { useIssues, useSyncState } from "./issues";
import { invalidateProfileData } from "./invalidate";
import { keys } from "./keys";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListIssues: vi.fn(),
    GetSyncState: vi.fn(),
  };
});

const query = { text: "", types: [], sprintId: "", offset: 0, limit: 25 };

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0 });
});

function wrapper(qc = createQueryClient()) {
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

describe("issue queries", () => {
  it("does not fetch without a profile", async () => {
    const { result } = renderHook(() => useIssues("", query), { wrapper: wrapper() });
    await new Promise((r) => setTimeout(r, 10));
    expect(result.current.fetchStatus).toBe("idle");
    expect(api.ListIssues).not.toHaveBeenCalled();
  });

  it("fetches the page for the profile and query", async () => {
    vi.mocked(api.ListIssues).mockResolvedValue({
      issues: [{ key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", rank: "", created: "", updated: "" }],
      total: 1,
    });
    const { result } = renderHook(() => useIssues("p1", query), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.data?.total).toBe(1));
    expect(api.ListIssues).toHaveBeenCalledWith("p1", query);
  });

  it("invalidateProfileData refetches the issue and sync-state families", async () => {
    const qc = createQueryClient();
    const { result } = renderHook(() => ({ issues: useIssues("p1", query), state: useSyncState("p1") }), { wrapper: wrapper(qc) });
    await waitFor(() => expect(result.current.state.data).toBeDefined());
    expect(api.ListIssues).toHaveBeenCalledTimes(1);
    expect(api.GetSyncState).toHaveBeenCalledTimes(1);
    invalidateProfileData(qc, "p1");
    await waitFor(() => expect(api.ListIssues).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(api.GetSyncState).toHaveBeenCalledTimes(2));
  });

  it("keys are profile-scoped", () => {
    expect(keys.issues("p1", query)[0]).toBe("p1");
    expect(keys.issue("p1", "PLAT-1")).toEqual(["p1", "issue", "PLAT-1"]);
    expect(keys.linkedTests("p1", "PLAT-1")).toEqual(["p1", "issue", "PLAT-1", "tests"]);
  });
});
```

- [ ] **Step 3: Run them to see the failure**

Run (inside `tam/frontend/`): `npx vitest run src/lib src/queries`
Expected: FAIL, the modules do not exist.

- [ ] **Step 4: The formatter and the query layer**

Create `tam/frontend/src/lib/format.ts`:

```ts
// formatWhen renders a sync timestamp for the status bar: "today HH:MM" for
// a same-day stamp, the local date and time otherwise, "" for none, and the
// raw text when it is not a date at all.
export function formatWhen(iso: string, now: Date = new Date()): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  const sameDay =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate();
  return sameDay ? `today ${time}` : `${d.toLocaleDateString()} ${time}`;
}
```

Create `tam/frontend/src/queries/keys.ts`:

```ts
import type { IssueQuery } from "../api";

// keys is the single source of query keys, so an invalidation can never
// drift from the read it must refresh. Every key starts with the profile id.
export const keys = {
  issues: (profileId: string, q: IssueQuery) => [profileId, "issues", q] as const,
  issue: (profileId: string, key: string) => [profileId, "issue", key] as const,
  linkedTests: (profileId: string, key: string) =>
    [profileId, "issue", key, "tests"] as const,
  sprints: (profileId: string) => [profileId, "sprints"] as const,
  syncState: (profileId: string) => [profileId, "syncState"] as const,
};
```

Create `tam/frontend/src/queries/issues.ts`:

```ts
import { useQuery } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import {
  GetIssueDetail,
  GetSyncState,
  ListIssues,
  ListLinkedTests,
  ListSprints,
} from "../api";
import type { IssueQuery } from "../api";
import { keys } from "./keys";

// useIssues loads one page of the Backlog. placeholderData keeps the previous
// page on screen while the next one loads, so paging does not flash.
export function useIssues(profileId: string, q: IssueQuery) {
  return useQuery({
    queryKey: keys.issues(profileId, q),
    queryFn: () => call(() => ListIssues(profileId, q)),
    enabled: !!profileId,
    placeholderData: (prev) => prev,
  });
}

// useIssueDetail runs when the panel opens. The backend serves a fresh cache
// without a network call, so staleTime is short and the refetch cheap.
export function useIssueDetail(profileId: string, key: string) {
  return useQuery({
    queryKey: keys.issue(profileId, key),
    queryFn: () => call(() => GetIssueDetail(profileId, key)),
    enabled: !!profileId && !!key,
    retry: false,
  });
}

export function useLinkedTests(profileId: string, key: string) {
  return useQuery({
    queryKey: keys.linkedTests(profileId, key),
    queryFn: () => call(() => ListLinkedTests(profileId, key)),
    enabled: !!profileId && !!key,
  });
}

export function useSprints(profileId: string) {
  return useQuery({
    queryKey: keys.sprints(profileId),
    queryFn: () => call(() => ListSprints(profileId)),
    enabled: !!profileId,
  });
}

export function useSyncState(profileId: string) {
  return useQuery({
    queryKey: keys.syncState(profileId),
    queryFn: () => call(() => GetSyncState(profileId)),
    enabled: !!profileId,
  });
}
```

Create `tam/frontend/src/queries/invalidate.ts`:

```ts
import type { QueryClient } from "@tanstack/react-query";
import { keys } from "./keys";

// invalidateProfileData refreshes everything a sync can change for one
// profile: every issues page, the sprint list, and the sync state. Issue
// details are left alone; the backend's own cache decides their freshness.
export function invalidateProfileData(qc: QueryClient, profileId: string) {
  if (!profileId) return;
  for (const queryKey of [
    [profileId, "issues"] as const,
    keys.sprints(profileId),
    keys.syncState(profileId),
  ]) {
    qc.invalidateQueries({ queryKey });
  }
}
```

Run (inside `tam/frontend/`): `npx vitest run src/lib src/queries`
Expected: PASS, eight tests.

- [ ] **Step 5: Failing test for the sync provider**

Create `tam/frontend/src/contexts/SyncContext.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { SyncProgress } from "../api";
import { profileBackend } from "../profileBackend";
import { SyncProvider, useSync } from "./SyncContext";
import { useSyncState } from "../queries/issues";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    SyncIssues: vi.fn(),
    GetSyncState: vi.fn(),
    EventsOn: vi.fn(() => () => {}),
  };
});

let progressListener: ((p: SyncProgress) => void) | null = null;

beforeEach(() => {
  vi.clearAllMocks();
  progressListener = null;
  vi.mocked(api.EventsOn).mockImplementation((name: string, cb: (p: SyncProgress) => void) => {
    if (name === "tam:sync-progress") progressListener = cb;
    return () => {};
  });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0 });
});

function Probe() {
  const { status, progress, syncError, canSync, runSync } = useSync();
  const state = useSyncState("p1");
  return (
    <div>
      <span data-testid="status">{status}</span>
      <span data-testid="progress">{progress ? `${progress.fetched}/${progress.total}` : "none"}</span>
      <span data-testid="error">{syncError}</span>
      <span data-testid="count">{state.data?.issueCount ?? "?"}</span>
      <button onClick={() => void runSync(false)} disabled={!canSync}>Sync</button>
      <button onClick={() => void runSync(true)}>Full sync</button>
    </div>
  );
}

// ProfileProvider does not load on mount (the shell calls reload after
// Health), so the test loads the profile the same way.
function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderProbe() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <SyncProvider>
            <Loader />
            <Probe />
          </SyncProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

describe("SyncProvider", () => {
  it("runs a sync, shows progress frames, and refreshes the sync state", async () => {
    let finish: (v: api.SyncSummary) => void = () => {};
    vi.mocked(api.SyncIssues).mockImplementation(
      () => new Promise<api.SyncSummary>((resolve) => { finish = resolve; }),
    );
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));
    await userEvent.click(screen.getByRole("button", { name: "Sync" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("syncing"));
    expect(screen.getByRole("button", { name: "Sync" })).toBeDisabled();
    act(() => progressListener?.({ phase: "issues", fetched: 25, total: 60, done: false, stage: "Fetching issues" }));
    expect(screen.getByTestId("progress")).toHaveTextContent("25/60");
    vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "2026-09-05T10:42:00Z", lastFull: "", lastError: "", issueCount: 60 });
    await act(async () => { finish({ fetched: 60, upserted: 60, skipped: 0, full: false, elapsed: "1s" }); });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("60"));
    expect(api.SyncIssues).toHaveBeenCalledWith("p1", false);
  });

  it("records a failure and returns to idle", async () => {
    vi.mocked(api.SyncIssues).mockRejectedValue(new Error("connection test failed: 401"));
    renderProbe();
    await userEvent.click(screen.getByRole("button", { name: "Full sync" }));
    await waitFor(() => expect(screen.getByTestId("error")).toHaveTextContent("connection test failed: 401"));
    expect(screen.getByTestId("status")).toHaveTextContent("idle");
    expect(api.SyncIssues).toHaveBeenCalledWith("p1", true);
    expect(screen.getByRole("dialog")).toHaveTextContent("Sync failed");
  });
});
```

- [ ] **Step 6: The provider**

Create `tam/frontend/src/contexts/SyncContext.tsx`:

```tsx
import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef } from "react";
import type { ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  syncReducer,
  initialSyncState,
  canSync as canSyncSel,
  canSwitchProfile as canSwitchProfileSel,
  useNotice,
  useProfile,
  call,
  errMsg,
} from "@agile-suite/core";
import type { SyncProgress, SyncStatus } from "@agile-suite/core";
import { EventsOn, SyncIssues } from "../api";
import type { Profile, Settings } from "../api";
import { invalidateProfileData } from "../queries/invalidate";

// SyncContext owns TAM's sync lifecycle on the shared reducer: it subscribes
// to the progress event, runs the bound SyncIssues call, refreshes the
// profile's queries afterwards, and reports failures with a notice. Unlike
// XTM's provider it also owns the orchestration, because TAM has no commit
// path yet and the call is one line.

const PROGRESS_EVENT = "tam:sync-progress";

interface SyncApi {
  status: SyncStatus;
  progress: SyncProgress | null;
  syncError: string;
  canSync: boolean;
  canSwitchProfile: boolean;
  // runSync pulls the active profile's issues; full clears and refetches.
  runSync: (full: boolean) => Promise<void>;
}

const SyncContext = createContext<SyncApi | null>(null);

export function useSync(): SyncApi {
  const ctx = useContext(SyncContext);
  if (!ctx) {
    throw new Error("useSync must be used within a SyncProvider");
  }
  return ctx;
}

export function SyncProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(syncReducer, initialSyncState);
  const { activeId } = useProfile<Profile, Settings>();
  const qc = useQueryClient();
  const { notice } = useNotice();
  // The reducer refuses a second SYNC_START, but the bound call must not run
  // twice either, so the guard reads the latest status through a ref.
  const statusRef = useRef(state.status);
  statusRef.current = state.status;

  useEffect(
    () =>
      EventsOn(PROGRESS_EVENT, (p: SyncProgress) =>
        dispatch({ type: "SYNC_PROGRESS", progress: p }),
      ),
    [],
  );

  const runSync = useCallback(
    async (full: boolean) => {
      if (!activeId || statusRef.current !== "idle") return;
      dispatch({
        type: "SYNC_START",
        clearError: true,
        initialProgress: { phase: "issues", fetched: 0, total: 0, done: false, stage: "Starting" },
      });
      try {
        await call(() => SyncIssues(activeId, full));
      } catch (e) {
        const message = errMsg(e);
        dispatch({ type: "SYNC_ERROR", message });
        void notice({ title: "Sync failed", message, tone: "error" });
      } finally {
        dispatch({ type: "SYNC_END" });
        invalidateProfileData(qc, activeId);
      }
    },
    [activeId, qc, notice],
  );

  const api = useMemo<SyncApi>(
    () => ({
      status: state.status,
      progress: state.progress,
      syncError: state.syncError,
      canSync: canSyncSel(state) && !!activeId,
      canSwitchProfile: canSwitchProfileSel(state),
      runSync,
    }),
    [state, activeId, runSync],
  );

  return <SyncContext.Provider value={api}>{children}</SyncContext.Provider>;
}
```

Run (inside `tam/frontend/`): `npx vitest run src/contexts`
Expected: PASS, two tests.

- [ ] **Step 7: The shell: Sync menu, status bar, provider**

In `tam/frontend/src/main.tsx`, import the provider and wrap the view provider:

```tsx
import { SyncProvider } from "./contexts/SyncContext";
```

and change the tree so `SyncProvider` sits directly inside `ProfileProvider`:

```tsx
        <ProfileProvider backend={profileBackend}>
          <SyncProvider>
            <ViewProvider>
              <ModalProvider>
                <App />
              </ModalProvider>
            </ViewProvider>
          </SyncProvider>
        </ProfileProvider>
```

In `tam/frontend/src/App.tsx`:

1. Add imports:

```tsx
import { useSync } from "./contexts/SyncContext";
import { useSyncState } from "./queries/issues";
import { formatWhen } from "./lib/format";
```

2. Inside the component, after the `useModal()` line, add:

```tsx
  const { progress, syncError, canSync, canSwitchProfile, runSync } = useSync();
  const syncState = useSyncState(activeId);
```

3. Give the profile `<select>` a `disabled={!canSwitchProfile}` attribute.

4. In the topbar's right cluster, before the existing Theme `Menu`, add:

```tsx
          <Menu
            label="Sync"
            align="right"
            triggerClassName="topbar-btn topbar-btn-primary"
            items={[
              { key: "sync", label: "Sync changes", onClick: () => void runSync(false), disabled: !canSync },
              { key: "full", label: "Full sync", title: "Clears the cached issues and fetches everything", onClick: () => void runSync(true), disabled: !canSync },
            ]}
          />
```

5. Replace the whole `<footer className="statusbar">` block with:

```tsx
      <footer className="statusbar">
        <span className={`dot ${health?.ok ? "dot-ok" : "dot-warn"}`} aria-hidden="true" />
        <span>{health?.ok ? "Local store ready · tam.db" : "Starting up"}</span>
        {!startupFailed && profileError ? (
          <span className="error-text">Profiles could not be loaded: {profileError}</span>
        ) : activeProfile ? (
          <span data-testid="sync-summary">
            {syncState.data
              ? syncState.data.lastSynced
                ? `${syncState.data.issueCount.toLocaleString()} issues, last synced ${formatWhen(syncState.data.lastSynced)}`
                : "Not synced yet"
              : ""}
          </span>
        ) : (
          <span className="muted">Profiles shared with XTM · agile-suite/profiles.db</span>
        )}
        {progress && (
          <span className="chip chip-sync" role="status">
            {progress.total > 0
              ? `Syncing: ${progress.fetched} of ${progress.total}`
              : progress.stage || "Syncing"}
          </span>
        )}
        {(syncError || syncState.data?.lastError) && !progress && (
          <span className="error-text" data-testid="sync-error">
            Last sync failed: {syncError || syncState.data?.lastError}
          </span>
        )}
        <span className="muted statusbar-right">Theme: {theme}</span>
      </footer>
```

6. Append to `tam/frontend/src/App.css`:

```css
.topbar-btn-primary { background: var(--accent); color: #fff; border-color: var(--accent); }
.topbar-btn-primary:hover { background: var(--accent-hover); }
.chip-sync { background: var(--accent-soft); color: var(--accent); border: 1px solid var(--accent); }
```

- [ ] **Step 8: Update the shell test**

In `tam/frontend/src/App.test.tsx`:

1. Add to the `vi.mock("./api", ...)` return object:

```ts
    SyncIssues: vi.fn(),
    GetSyncState: vi.fn(),
    ListIssues: vi.fn(),
    GetIssueDetail: vi.fn(),
    ListLinkedTests: vi.fn(),
    ListSprints: vi.fn(),
    GetProfileSetting: vi.fn(),
    SetProfileSetting: vi.fn(),
```

2. Import the provider and wrap it into `renderApp` directly inside `ProfileProvider`:

```tsx
import { SyncProvider } from "./contexts/SyncContext";
```

```tsx
        <ProfileProvider backend={profileBackend}>
          <SyncProvider>
            <ViewProvider>
              <ModalProvider>
                <App />
              </ModalProvider>
            </ViewProvider>
          </SyncProvider>
        </ProfileProvider>
```

3. Extend `beforeEach` with:

```ts
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0 });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
```

4. Add two tests to the `describe("App shell")` block:

```tsx
  it("syncs from the topbar and refreshes the status bar", async () => {
    vi.mocked(api.SyncIssues).mockImplementation(async () => {
      vi.mocked(api.GetSyncState).mockResolvedValue({
        lastSynced: new Date().toISOString(), lastFull: "", lastError: "", issueCount: 60,
      });
      return { fetched: 60, upserted: 60, skipped: 0, full: false, elapsed: "1s" };
    });
    renderApp();
    await waitFor(() => expect(screen.getByTestId("sync-summary")).toHaveTextContent("Not synced yet"));
    await userEvent.click(screen.getByRole("button", { name: "Sync" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Sync changes" }));
    await waitFor(() => expect(screen.getByTestId("sync-summary")).toHaveTextContent(/60 issues, last synced today/));
    expect(api.SyncIssues).toHaveBeenCalledWith("p1", false);
  });

  it("shows the last sync error in the status bar", async () => {
    vi.mocked(api.GetSyncState).mockResolvedValue({
      lastSynced: "2026-09-05T10:42:00Z", lastFull: "", lastError: "jira: 502 Bad Gateway", issueCount: 12,
    });
    renderApp();
    await waitFor(() => expect(screen.getByTestId("sync-error")).toHaveTextContent("jira: 502 Bad Gateway"));
  });
```

If the shared `Menu` renders its items with a role other than `menuitem`, read `frontend/core/src/components/Menu.tsx` and use the role it renders (`getByRole("button", { name: "Sync changes" })` if items are plain buttons). Do not change `Menu`.

- [ ] **Step 9: Run everything and commit**

Run (inside `tam/frontend/`): `npx vitest run && npm run typecheck && npm run build`
Expected: 20 tests passing (the 8 existing plus 4 formatter, 4 query, 2 provider, and 2 new shell tests), typecheck and build clean.

```bash
git add tam/frontend/src tam/frontend/package.json
git commit -m "feat(tam): sync from the topbar with progress and last-sync state in the status bar"
```

---

### Task 10: The Backlog view and the issue table

The mockup's grid: search, five type chips, a sprint dropdown, 25-row paging, and the virtualised table. Row selection is kept here so Task 11 can hang the panel off it.

**Files:**
- Create: `tam/frontend/src/components/TypeChip.tsx`, `IssueTable.tsx`, `BacklogView.tsx`, `BacklogView.test.tsx`
- Modify: `tam/frontend/package.json` (adds `@tanstack/react-virtual`), `package-lock.json` (root), `tam/frontend/src/App.tsx`, `App.css`
- Test: `npx vitest run` and `npm run typecheck` inside `tam/frontend/`

**Interfaces:**
- Consumes: `useIssues`, `useSprints` (Task 9); `Issue`, `ISSUE_TYPES`, `IssueQuery` (Task 9); `useProfile` from `@agile-suite/core`.
- Produces: `TypeChip({ type })`, `IssueTable({ issues, selectedKey, onSelect })`, `BacklogView()`. `BacklogView` keeps `selectedKey` state and renders the panel slot Task 11 fills (`{selected && <IssueDetailPanel ... />}`).

- [ ] **Step 1: The dependency**

Add `"@tanstack/react-virtual": "^3.14.10"` to `dependencies` in `tam/frontend/package.json` (the version XTM pins; npm hoists one copy). From the repo root run `npm install` (not `npm ci`) so the lock records the new edge. Expected: the lock file changes in one place and `node_modules/@tanstack/react-virtual` already exists.

- [ ] **Step 2: Failing view test**

Create `tam/frontend/src/components/BacklogView.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { Issue } from "../api";
import { profileBackend } from "../profileBackend";
import { BacklogView } from "./BacklogView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    ListIssues: vi.fn(),
    ListSprints: vi.fn(),
    GetIssueDetail: vi.fn(),
    ListLinkedTests: vi.fn(),
  };
});

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", assignee: "", reporter: "",
    priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
    ...over,
  };
}

const rows: Issue[] = [
  issue({ key: "PLAT-412", type: "story", summary: "Checkout: apply promo code at payment step", status: "In Progress", assignee: "R. Anand", sprintId: "12", sprintName: "Sprint 12", storyPoints: 5 }),
  issue({ key: "PLAT-409", type: "task", summary: "Rotate payment gateway API keys", sprintId: "12", sprintName: "Sprint 12", storyPoints: 2 }),
  issue({ key: "PLAT-388", type: "requirement", summary: "Promo codes must be single-use per customer", status: "Approved", assignee: "PO" }),
];

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderView() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <Loader />
          <BacklogView />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: rows, total: 1248 });
  vi.mocked(api.ListSprints).mockResolvedValue([{ id: "12", name: "Sprint 12" }, { id: "13", name: "Sprint 13" }]);
  vi.mocked(api.GetIssueDetail).mockResolvedValue({ key: "PLAT-412", description: "", links: [], fields: {} });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
});

const lastQuery = () => vi.mocked(api.ListIssues).mock.calls.at(-1)?.[1];

describe("BacklogView", () => {
  it("renders the page with the seven columns and the count", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    const header = screen.getByRole("row", { name: /key type summary status assignee sprint pts/i });
    expect(header).toBeInTheDocument();
    const row = screen.getByRole("row", { name: /PLAT-412/ });
    expect(within(row).getByText("Story")).toBeInTheDocument();
    expect(within(row).getByText("In Progress")).toBeInTheDocument();
    expect(within(row).getByText("R. Anand")).toBeInTheDocument();
    expect(within(row).getByText("12")).toBeInTheDocument();
    expect(within(row).getByText("5")).toBeInTheDocument();
    expect(screen.getByText("Showing 1 to 25 of 1,248")).toBeInTheDocument();
    expect(lastQuery()).toEqual({ text: "", types: [], sprintId: "", offset: 0, limit: 25 });
  });

  it("filters by text, type chips, and sprint", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    await userEvent.type(screen.getByRole("searchbox", { name: "Search issues" }), "promo");
    await waitFor(() => expect(lastQuery()?.text).toBe("promo"));
    expect(lastQuery()?.offset).toBe(0);
    await userEvent.click(screen.getByRole("button", { name: "Story", pressed: false }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["story"]));
    await userEvent.click(screen.getByRole("button", { name: "Bug", pressed: false }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["story", "bug"]));
    await userEvent.click(screen.getByRole("button", { name: "Story", pressed: true }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["bug"]));
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Sprint" }), "13");
    await waitFor(() => expect(lastQuery()?.sprintId).toBe("13"));
  });

  it("pages forward and back and resets the page when the filter changes", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() => expect(lastQuery()?.offset).toBe(25));
    expect(screen.getByText("Showing 26 to 50 of 1,248")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Previous page" }));
    await waitFor(() => expect(lastQuery()?.offset).toBe(0));
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() => expect(lastQuery()?.offset).toBe(25));
    await userEvent.click(screen.getByRole("button", { name: "Bug", pressed: false }));
    await waitFor(() => expect(lastQuery()).toMatchObject({ types: ["bug"], offset: 0 }));
  });

  it("selects a row on click", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-409")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("row", { name: /PLAT-409/ }));
    expect(screen.getByRole("row", { name: /PLAT-409/ })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("row", { name: /PLAT-412/ })).toHaveAttribute("aria-selected", "false");
  });

  it("explains an empty cache", async () => {
    vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
    renderView();
    await waitFor(() => expect(screen.getByText(/No issues cached yet/)).toBeInTheDocument());
  });
});
```

- [ ] **Step 3: Run to see the failure**

Run (inside `tam/frontend/`): `npx vitest run src/components/BacklogView`
Expected: FAIL, the module does not exist.

- [ ] **Step 4: The chip and the table**

Create `tam/frontend/src/components/TypeChip.tsx`:

```tsx
import { ISSUE_TYPES } from "../api";

// TypeChip is the coloured type pill the grid, the filter bar, and the panel
// share. Unknown types (none should reach the UI) show their raw name.
export function TypeChip({ type }: { type: string }) {
  const t = ISSUE_TYPES.find((x) => x.id === type);
  return <span className={`chip chip-type chip-type-${type || "none"}`}>{t?.short ?? type}</span>;
}
```

Create `tam/frontend/src/components/IssueTable.tsx`:

```tsx
import { useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import type { Issue } from "../api";
import { TypeChip } from "./TypeChip";

const ROW_HEIGHT = 34;

interface Props {
  issues: Issue[];
  selectedKey: string;
  onSelect: (key: string) => void;
}

// IssueTable renders one page of issues with the mockup's seven columns.
// Rows are virtualised so a 500-row page stays light; the overscan is wide
// enough that a viewport with no measured height (jsdom in tests) still
// renders a full 25-row page.
export function IssueTable({ issues, selectedKey, onSelect }: Props) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: issues.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 30,
  });

  return (
    <div className="issue-table" role="table" aria-label="Issues">
      <div className="issue-row issue-head" role="row" aria-label="key type summary status assignee sprint pts">
        <span role="columnheader">KEY</span>
        <span role="columnheader">TYPE</span>
        <span role="columnheader">SUMMARY</span>
        <span role="columnheader">STATUS</span>
        <span role="columnheader">ASSIGNEE</span>
        <span role="columnheader">SPRINT</span>
        <span role="columnheader">PTS</span>
      </div>
      <div className="issue-body" ref={scrollRef} role="rowgroup">
        <div style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
          {virtualizer.getVirtualItems().map((v) => {
            const iss = issues[v.index];
            const selected = iss.key === selectedKey;
            return (
              <div
                key={iss.key}
                role="row"
                aria-selected={selected}
                aria-label={`${iss.key} ${iss.summary}`}
                className={`issue-row${selected ? " issue-row-selected" : v.index % 2 ? " issue-row-alt" : ""}`}
                style={{ position: "absolute", top: 0, left: 0, right: 0, height: v.size, transform: `translateY(${v.start}px)` }}
                onClick={() => onSelect(iss.key)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onSelect(iss.key);
                  }
                }}
                tabIndex={0}
              >
                <span role="cell" className="issue-key">{iss.key}</span>
                <span role="cell"><TypeChip type={iss.type} /></span>
                <span role="cell" className="issue-summary" title={iss.summary}>{iss.summary}</span>
                <span role="cell"><span className={`chip chip-status chip-status-${statusClass(iss.status)}`}>{iss.status}</span></span>
                <span role="cell">{iss.assignee || "-"}</span>
                <span role="cell" title={iss.sprintName}>{iss.sprintId || "-"}</span>
                <span role="cell">{iss.storyPoints ?? "-"}</span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// statusClass buckets a Jira status name into the three colours the
// mockup uses: done, in progress, and everything else.
function statusClass(status: string): "done" | "active" | "todo" {
  const s = status.toLowerCase();
  if (s === "done" || s === "closed" || s === "resolved" || s === "approved") return "done";
  if (s.includes("progress") || s === "in review") return "active";
  return "todo";
}
```

- [ ] **Step 5: The view**

Create `tam/frontend/src/components/BacklogView.tsx`:

```tsx
import { useEffect, useMemo, useState } from "react";
import { useProfile } from "@agile-suite/core";
import { ISSUE_TYPES } from "../api";
import type { IssueQuery, Profile, Settings } from "../api";
import { useIssues, useSprints } from "../queries/issues";
import { IssueTable } from "./IssueTable";

const PAGE_SIZE = 25;
const SEARCH_DELAY_MS = 250;

// useDebounced returns value after it has stopped changing for delay ms, so
// typing in the search box does not query the backend on every keystroke.
function useDebounced<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(t);
  }, [value, delay]);
  return debounced;
}

// BacklogView is the issue grid with its filter bar and pager. Filter and
// page state live here and reset when the profile changes. Selection is
// kept here too, so the detail panel can sit beside the table.
export function BacklogView() {
  const { activeId } = useProfile<Profile, Settings>();
  const [text, setText] = useState("");
  const [types, setTypes] = useState<string[]>([]);
  const [sprintId, setSprintId] = useState("");
  const [page, setPage] = useState(0);
  const [selectedKey, setSelectedKey] = useState("");
  const search = useDebounced(text, SEARCH_DELAY_MS);

  useEffect(() => {
    setText("");
    setTypes([]);
    setSprintId("");
    setPage(0);
    setSelectedKey("");
  }, [activeId]);

  // A filter change goes back to the first page.
  useEffect(() => {
    setPage(0);
  }, [search, types, sprintId]);

  const query = useMemo<IssueQuery>(
    () => ({ text: search, types, sprintId, offset: page * PAGE_SIZE, limit: PAGE_SIZE }),
    [search, types, sprintId, page],
  );
  const issues = useIssues(activeId, query);
  const sprints = useSprints(activeId);

  const total = issues.data?.total ?? 0;
  const rows = issues.data?.issues ?? [];
  const first = total === 0 ? 0 : page * PAGE_SIZE + 1;
  const last = Math.min(total, (page + 1) * PAGE_SIZE);
  const lastPage = Math.max(0, Math.ceil(total / PAGE_SIZE) - 1);
  const filtered = search !== "" || types.length > 0 || sprintId !== "";

  function toggleType(id: string) {
    setTypes((cur) => (cur.includes(id) ? cur.filter((t) => t !== id) : [...cur, id]));
  }

  return (
    <section className="backlog" aria-labelledby="view-title">
      <div className="filter-bar">
        <input
          type="search"
          className="detail-input filter-search"
          aria-label="Search issues"
          placeholder="Search summary, key, label"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <div className="type-filter" role="group" aria-label="Issue types">
          {ISSUE_TYPES.map((t) => (
            <button
              key={t.id}
              type="button"
              className={`chip chip-type chip-type-${t.id} chip-toggle${types.includes(t.id) ? " chip-on" : ""}`}
              aria-pressed={types.includes(t.id)}
              onClick={() => toggleType(t.id)}
            >
              {t.short}
            </button>
          ))}
        </div>
        <select
          className="detail-input filter-sprint"
          aria-label="Sprint"
          value={sprintId}
          onChange={(e) => setSprintId(e.target.value)}
        >
          <option value="">All sprints</option>
          {(sprints.data ?? []).map((s) => (
            <option key={s.id} value={s.id}>{s.name}</option>
          ))}
        </select>
        <span className="muted small filter-note">Read-only until plan 1b</span>
      </div>

      <div className="backlog-body">
        <div className="backlog-grid">
          {issues.isError ? (
            <p className="error-text" data-testid="issues-error">Could not load issues: {issues.error.message}</p>
          ) : total === 0 && !issues.isPending ? (
            <p className="muted backlog-empty">
              {filtered
                ? "No issues match this filter."
                : "No issues cached yet. Use Sync in the topbar to pull this project's issues."}
            </p>
          ) : (
            <IssueTable issues={rows} selectedKey={selectedKey} onSelect={setSelectedKey} />
          )}
          <div className="pager">
            <span className="muted small">{`Showing ${first.toLocaleString()} to ${last.toLocaleString()} of ${total.toLocaleString()}`}</span>
            <span className="pager-buttons">
              <button type="button" className="btn" aria-label="Previous page" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>Prev</button>
              <span className="muted small">{`${page + 1} / ${lastPage + 1}`}</span>
              <button type="button" className="btn" aria-label="Next page" disabled={page >= lastPage} onClick={() => setPage((p) => Math.min(lastPage, p + 1))}>Next</button>
            </span>
          </div>
        </div>
        {/* The detail panel arrives in the next task and mounts here. */}
      </div>
    </section>
  );
}
```

In `tam/frontend/src/App.tsx`, import the view:

```tsx
import { BacklogView } from "./components/BacklogView";
```

and replace `<Placeholder view={current} />` with:

```tsx
            {current.id === "backlog" ? <BacklogView /> : <Placeholder view={current} />}
```

Append to `tam/frontend/src/App.css`:

```css
/* Backlog: filter bar, grid, pager */
.filter-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; flex-wrap: wrap; }
.filter-search { width: 300px; }
.filter-sprint { width: 140px; }
.filter-note { margin-left: auto; }
.type-filter { display: flex; gap: 6px; }
.chip-type { font-size: 10px; font-weight: 600; padding: 2px 8px; border-radius: 8px; border: 1px solid transparent; }
.chip-type-task { background: #dbeafe; color: #1e40af; }
.chip-type-story { background: #dcfce7; color: #166534; }
.chip-type-bug { background: #fee2e2; color: #991b1b; }
.chip-type-epic { background: #ede9fe; color: #5b21b6; }
.chip-type-requirement { background: #fef3c7; color: #92400e; }
.chip-type-none { background: var(--surface-3); color: var(--text-muted); }
.chip-toggle { cursor: pointer; opacity: 0.55; padding: 4px 10px; border-radius: 12px; }
.chip-toggle.chip-on, .chip-toggle:hover { opacity: 1; }
.chip-toggle.chip-on { border-color: currentColor; }
.chip-status { font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 3px; }
.chip-status-todo { background: var(--surface-3); color: var(--text); }
.chip-status-active { background: #dbeafe; color: #1e40af; }
.chip-status-done { background: #dcfce7; color: #166534; }

.backlog-body { display: flex; gap: 12px; align-items: flex-start; }
.backlog-grid { flex: 1; min-width: 0; display: flex; flex-direction: column; background: var(--surface); border: 1px solid var(--border); border-radius: 6px; }
.backlog-empty { padding: 32px; text-align: center; }
.issue-table { display: flex; flex-direction: column; font-size: 12px; }
.issue-row { display: grid; grid-template-columns: 84px 56px 1fr 96px 96px 52px 40px; align-items: center; gap: 8px; padding: 0 10px; height: 34px; border-bottom: 1px solid var(--border-subtle); }
.issue-head { height: 30px; background: var(--surface-2); color: var(--text-muted); font-size: 11px; font-weight: 600; position: sticky; top: 0; }
.issue-body { max-height: calc(100vh - 300px); overflow: auto; }
.issue-row-alt { background: var(--bg-subtle); }
.issue-row:not(.issue-head):hover { background: var(--row-hover); cursor: pointer; }
.issue-row-selected, .issue-row-selected:hover { background: var(--accent-soft); }
.issue-key { font-weight: 600; }
.issue-summary { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pager { display: flex; align-items: center; justify-content: space-between; padding: 8px 10px; }
.pager-buttons { display: flex; align-items: center; gap: 8px; }
```

Check that `--surface-3`, `--bg-subtle`, and `--border-subtle` exist in `frontend/core/styles/tokens.css` (they do today); if a token is missing, use `--surface-2` and `--border` instead rather than adding tokens.

- [ ] **Step 6: Run the tests and commit**

Run (inside `tam/frontend/`): `npx vitest run && npm run typecheck && npm run build`
Expected: 25 tests passing (20 from Task 9 plus these 5), typecheck and build clean. The shell tests still pass because their `ListIssues` and `ListSprints` mocks return empty results.

```bash
git add package-lock.json tam/frontend/package.json tam/frontend/src
git commit -m "feat(tam): the Backlog grid with search, type chips, sprint filter, and paging"
```

---

### Task 11: The issue detail panel

The read-only panel beside the grid: header, three tabs, the cached row's fields at once, the lazy detail with its loading and error states, a refresh, and the Tests tab that makes the requirement-to-test seam visible.

**Files:**
- Create: `tam/frontend/src/components/IssueDetailPanel.tsx`, `IssueDetailPanel.test.tsx`
- Modify: `tam/frontend/src/components/BacklogView.tsx` (mounts the panel), `App.css`
- Test: `npx vitest run` and `npm run typecheck` inside `tam/frontend/`

**Interfaces:**
- Consumes: `useIssueDetail`, `useLinkedTests` (Task 9); `Issue`, `IssueDetail`, `Link`, `LinkedTest` (Task 9); `TypeChip` (Task 10); `formatWhen` (Task 9).
- Produces: `IssueDetailPanel({ profileId, issue, onClose })`.

- [ ] **Step 1: Failing panel tests**

Create `tam/frontend/src/components/IssueDetailPanel.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { Issue } from "../api";
import { IssueDetailPanel } from "./IssueDetailPanel";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetIssueDetail: vi.fn(), ListLinkedTests: vi.fn() };
});

const story: Issue = {
  key: "PLAT-412", id: "1", project: "PLAT", type: "story", summary: "Checkout: apply promo code at payment step",
  status: "In Progress", assignee: "R. Anand", reporter: "PO", priority: "High", labels: ["checkout", "promo"],
  sprintId: "12", sprintName: "Sprint 12 - Checkout polish", parentKey: "PLAT-350", storyPoints: 5, rank: "",
  created: "2026-08-01T09:00:00Z", updated: "2026-09-05T09:58:00Z",
};

function renderPanel(onClose = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <IssueDetailPanel profileId="p1" issue={story} onClose={onClose} />
    </QueryClientProvider>,
  );
  return onClose;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetIssueDetail).mockResolvedValue({
    key: "PLAT-412",
    description: "As a shopper I can enter a promo code on the payment step.",
    links: [
      { direction: "inward", type: "Tested By", key: "XT-1018", summary: "Promo code applies discount", issueType: "Test" },
      { direction: "outward", type: "Relates", key: "PLAT-388", summary: "Promo codes must be single-use", issueType: "Requirement" },
    ],
    fields: {},
  });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([
    { key: "XT-1018", summary: "Promo code applies discount", linkType: "Tested By" },
    { key: "XT-1019", summary: "Expired promo code rejected", linkType: "Tested By" },
  ]);
});

describe("IssueDetailPanel", () => {
  it("shows the cached fields at once and the description once fetched", async () => {
    renderPanel();
    expect(screen.getByRole("heading", { name: "PLAT-412" })).toBeInTheDocument();
    expect(screen.getByText("Checkout: apply promo code at payment step")).toBeInTheDocument();
    const details = screen.getByRole("tabpanel", { name: "Details" });
    expect(within(details).getByText("In Progress")).toBeInTheDocument();
    expect(within(details).getByText("R. Anand")).toBeInTheDocument();
    expect(within(details).getByText("Sprint 12 - Checkout polish")).toBeInTheDocument();
    expect(within(details).getByText("5")).toBeInTheDocument();
    expect(within(details).getByText("PLAT-350")).toBeInTheDocument();
    expect(within(details).getByText("checkout")).toBeInTheDocument();
    expect(within(details).getByText("Loading description")).toBeInTheDocument();
    await waitFor(() => expect(within(details).getByText(/As a shopper I can enter a promo code/)).toBeInTheDocument());
    expect(api.GetIssueDetail).toHaveBeenCalledWith("p1", "PLAT-412");
  });

  it("switches to Links and Tests", async () => {
    renderPanel();
    await userEvent.click(screen.getByRole("tab", { name: "Links" }));
    const links = screen.getByRole("tabpanel", { name: "Links" });
    await waitFor(() => expect(within(links).getByText("XT-1018")).toBeInTheDocument());
    expect(within(links).getByText("Tested By")).toBeInTheDocument();
    expect(within(links).getByText("PLAT-388")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "Tests" }));
    const tests = screen.getByRole("tabpanel", { name: "Tests" });
    await waitFor(() => expect(within(tests).getByText("XT-1019")).toBeInTheDocument());
    expect(within(tests).getByText(/via XTM, link: Tested By/)).toBeInTheDocument();
    expect(api.ListLinkedTests).toHaveBeenCalledWith("p1", "PLAT-412");
  });

  it("keeps the cached fields and offers a retry when the detail fetch fails", async () => {
    vi.mocked(api.GetIssueDetail).mockRejectedValueOnce(new Error("jira: 502 Bad Gateway"));
    renderPanel();
    await waitFor(() => expect(screen.getByTestId("detail-error")).toHaveTextContent("jira: 502 Bad Gateway"));
    expect(screen.getByText("R. Anand")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.getByText(/As a shopper I can enter a promo code/)).toBeInTheDocument());
    expect(api.GetIssueDetail).toHaveBeenCalledTimes(2);
  });

  it("says when there are no linked tests and closes", async () => {
    vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
    const onClose = renderPanel();
    await userEvent.click(screen.getByRole("tab", { name: "Tests" }));
    await waitFor(() => expect(screen.getByText("No linked tests.")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run to see the failure**

Run (inside `tam/frontend/`): `npx vitest run src/components/IssueDetailPanel`
Expected: FAIL, the module does not exist.

- [ ] **Step 3: The panel**

Create `tam/frontend/src/components/IssueDetailPanel.tsx`:

```tsx
import { useState } from "react";
import type { Issue, Link } from "../api";
import { useIssueDetail, useLinkedTests } from "../queries/issues";
import { formatWhen } from "../lib/format";
import { TypeChip } from "./TypeChip";

type Tab = "details" | "links" | "tests";

const TABS: { id: Tab; label: string }[] = [
  { id: "details", label: "Details" },
  { id: "links", label: "Links" },
  { id: "tests", label: "Tests" },
];

interface Props {
  profileId: string;
  issue: Issue;
  onClose: () => void;
}

// IssueDetailPanel shows one issue beside the grid. The grid row's fields
// render at once; the description, links, and linked tests load through the
// backend's detail cache. Nothing here writes; the actions arrive in plan 1b.
export function IssueDetailPanel({ profileId, issue, onClose }: Props) {
  const [tab, setTab] = useState<Tab>("details");
  const detail = useIssueDetail(profileId, issue.key);
  const tests = useLinkedTests(profileId, issue.key);

  return (
    <aside className="detail-panel" aria-labelledby="detail-title">
      <div className="detail-head">
        <h2 id="detail-title" className="detail-key">{issue.key}</h2>
        <TypeChip type={issue.type} />
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>
      <p className="detail-summary">{issue.summary}</p>

      <div className="tabs" role="tablist" aria-label="Issue sections">
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            id={`tab-${t.id}`}
            aria-selected={tab === t.id}
            aria-controls={`panel-${t.id}`}
            className={`tab${tab === t.id ? " tab-active" : ""}`}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "details" && (
        <div role="tabpanel" id="panel-details" aria-labelledby="tab-details" className="tab-panel">
          <dl className="field-list">
            <dt>Status</dt><dd>{issue.status || "-"}</dd>
            <dt>Assignee</dt><dd>{issue.assignee || "-"}</dd>
            <dt>Sprint</dt><dd>{issue.sprintName || "-"}</dd>
            <dt>Story points</dt><dd>{issue.storyPoints ?? "-"}</dd>
            <dt>{issue.type === "epic" ? "Parent" : "Epic"}</dt><dd>{issue.parentKey ? <span className="accent-text">{issue.parentKey}</span> : "-"}</dd>
            <dt>Priority</dt><dd>{issue.priority || "-"}</dd>
            <dt>Labels</dt>
            <dd>{issue.labels.length ? issue.labels.map((l) => <span key={l} className="chip chip-label">{l}</span>) : "-"}</dd>
            <dt>Updated</dt><dd>{formatWhen(issue.updated) || "-"}</dd>
          </dl>
          <div className="detail-section-head">
            <h3>Description</h3>
            <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()} disabled={detail.isFetching}>
              {detail.isFetching ? "Refreshing" : "Refresh"}
            </button>
          </div>
          {detail.isPending ? (
            <p className="muted">Loading description</p>
          ) : detail.isError ? (
            <p className="error-text" data-testid="detail-error">
              Could not load the details: {detail.error.message}{" "}
              <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()}>Retry</button>
            </p>
          ) : (
            <p className="detail-description">{detail.data.description || <span className="muted">No description.</span>}</p>
          )}
          <p className="muted small detail-note">Fields load from the cache first, then refresh from Jira when the panel opens.</p>
        </div>
      )}

      {tab === "links" && (
        <div role="tabpanel" id="panel-links" aria-labelledby="tab-links" className="tab-panel">
          {detail.isPending ? (
            <p className="muted">Loading links</p>
          ) : detail.isError ? (
            <p className="error-text" data-testid="detail-error">Could not load the links: {detail.error.message}</p>
          ) : detail.data.links.length === 0 ? (
            <p className="muted">No links.</p>
          ) : (
            <LinkGroups links={detail.data.links} />
          )}
        </div>
      )}

      {tab === "tests" && (
        <div role="tabpanel" id="panel-tests" aria-labelledby="tab-tests" className="tab-panel">
          <div className="detail-section-head">
            <h3>Covered by tests</h3>
            {tests.data && tests.data.length > 0 && (
              <span className="muted small">via XTM, link: {tests.data[0].linkType}</span>
            )}
          </div>
          {tests.isPending ? (
            <p className="muted">Loading tests</p>
          ) : tests.isError ? (
            <p className="error-text">Could not load the linked tests: {tests.error.message}</p>
          ) : tests.data.length === 0 ? (
            <p className="muted">No linked tests.</p>
          ) : (
            <ul className="linked-list">
              {tests.data.map((t) => (
                <li key={t.key} className="linked-row">
                  <span className="accent-text linked-key">{t.key}</span>
                  <span>{t.summary}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </aside>
  );
}

// LinkGroups lists links grouped by type, then direction, in the order the
// store returns them.
function LinkGroups({ links }: { links: Link[] }) {
  const groups = new Map<string, Link[]>();
  for (const l of links) {
    const k = `${l.type} (${l.direction})`;
    groups.set(k, [...(groups.get(k) ?? []), l]);
  }
  return (
    <div className="link-groups">
      {[...groups.entries()].map(([label, items]) => (
        <div key={label} className="link-group">
          <h3 className="link-group-title">{label.replace(/ \((inward|outward)\)$/, "")} <span className="muted small">{label.match(/\((inward|outward)\)$/)?.[1]}</span></h3>
          <ul className="linked-list">
            {items.map((l) => (
              <li key={`${l.direction}-${l.key}`} className="linked-row">
                <span className="accent-text linked-key">{l.key}</span>
                <span>{l.summary}</span>
                <span className="muted small">{l.issueType}</span>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 4: Mount it in the Backlog**

In `tam/frontend/src/components/BacklogView.tsx`, import the panel:

```tsx
import { IssueDetailPanel } from "./IssueDetailPanel";
```

compute the selected row after `rows`:

```tsx
  const selected = rows.find((r) => r.key === selectedKey);
```

and replace the comment `{/* The detail panel arrives in the next task and mounts here. */}` with:

```tsx
        {selected && (
          <IssueDetailPanel profileId={activeId} issue={selected} onClose={() => setSelectedKey("")} />
        )}
```

Append to `tam/frontend/src/App.css`:

```css
/* Detail panel */
.detail-panel { width: 352px; flex: none; background: var(--surface); border: 1px solid var(--border); border-radius: 6px; padding: 16px; max-height: calc(100vh - 200px); overflow: auto; }
.detail-head { display: flex; align-items: center; gap: 8px; }
.detail-key { font-size: 15px; }
.detail-close { margin-left: auto; }
.detail-summary { font-size: 14px; margin: 8px 0 12px; }
.tabs { display: flex; gap: 16px; border-bottom: 1px solid var(--border); margin-bottom: 12px; }
.tab { background: none; border: none; border-bottom: 2px solid transparent; padding: 6px 0; color: var(--text-muted); cursor: pointer; font-size: 11px; font-weight: 600; }
.tab-active { color: var(--accent); border-bottom-color: var(--accent); }
.field-list { display: grid; grid-template-columns: 96px 1fr; gap: 10px 8px; font-size: 12px; margin-bottom: 12px; }
.field-list dt { color: var(--text-muted); }
.chip-label { background: var(--surface-3); color: var(--text); font-size: 10px; padding: 2px 8px; border-radius: 8px; margin-right: 4px; }
.accent-text { color: var(--accent); }
.detail-section-head { display: flex; align-items: center; justify-content: space-between; margin: 12px 0 6px; }
.detail-section-head h3 { font-size: 12px; }
.detail-description { white-space: pre-wrap; font-size: 12px; }
.detail-note { margin-top: 16px; }
.linked-list { list-style: none; display: flex; flex-direction: column; gap: 6px; }
.linked-row { display: flex; gap: 10px; align-items: baseline; padding: 6px 10px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 4px; font-size: 12px; }
.linked-key { font-weight: 600; min-width: 64px; }
.link-group { margin-bottom: 12px; }
.link-group-title { font-size: 12px; margin-bottom: 6px; }
```

- [ ] **Step 5: Run the tests and commit**

Run (inside `tam/frontend/`): `npx vitest run && npm run typecheck && npm run build`
Expected: 29 tests passing (25 plus these 4), typecheck and build clean. The BacklogView tests still pass: their `GetIssueDetail` and `ListLinkedTests` mocks are already in place for the row-selection case.

```bash
git add tam/frontend/src
git commit -m "feat(tam): the read-only issue detail panel with Details, Links, and Tests"
```

---

### Task 12: The requirement type setting, docs, and the packaged app

The one per-profile setting 1a needs, the guides brought up to date, and proof the whole app packages with `wails build`.

**Files:**
- Modify: `tam/frontend/src/components/ProfilesModal.tsx`, `ProfilesModal.test.tsx`, `tam/CLAUDE.md`, `README.md` (root), `CLAUDE.md` (root)
- Test: `npx vitest run` inside `tam/frontend/`; `wails build` inside `tam/`

**Interfaces:**
- Consumes: `GetProfileSetting`, `SetProfileSetting` (Task 9); the `requirement_issue_type` key (Task 7).

- [ ] **Step 1: Failing modal test**

In `tam/frontend/src/components/ProfilesModal.test.tsx`, add `GetProfileSetting: vi.fn()` and `SetProfileSetting: vi.fn()` to the mocked `../api` object, add to `beforeEach`:

```ts
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.SetProfileSetting).mockResolvedValue();
```

and append this test inside the `describe("ProfilesModal")` block:

```tsx
  it("edits the requirement issue type per profile", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([
      { id: "p1", name: "Acme Platform", jiraUrl: "https://jira.acme.example", projectKey: "PLAT", backend: "xray", createdAt: "" },
    ]);
    vi.mocked(api.GetProfileSetting).mockResolvedValue("Business Requirement");
    renderModal();
    const field = await screen.findByRole("textbox", { name: "Requirement issue type for Acme Platform" });
    await waitFor(() => expect(field).toHaveValue("Business Requirement"));
    await userEvent.clear(field);
    await userEvent.type(field, "Req");
    await userEvent.tab();
    await waitFor(() =>
      expect(api.SetProfileSetting).toHaveBeenCalledWith("p1", "requirement_issue_type", "Req"),
    );
  });
```

Run (inside `tam/frontend/`): `npx vitest run src/components/ProfilesModal`
Expected: FAIL, no such textbox.

- [ ] **Step 2: The field**

In `tam/frontend/src/components/ProfilesModal.tsx`:

1. Extend the imports:

```tsx
import { useEffect, useState } from "react";
import { CreateProfile, DeleteProfile, GetProfileSetting, SetProfileSetting, isDemoUrl } from "../api";
```

2. Add this constant and component above `ProfilesModal`:

```tsx
const REQUIREMENT_TYPE_KEY = "requirement_issue_type";

// RequirementTypeField edits the one TAM setting a profile has today: the
// Jira issue type name TAM treats as a requirement. It saves on blur or
// Enter and shows the backend's default as the placeholder.
function RequirementTypeField({ profile }: { profile: Profile }) {
  const [value, setValue] = useState("");
  const [saved, setSaved] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    GetProfileSetting(profile.id, REQUIREMENT_TYPE_KEY)
      .then((v) => {
        if (live) {
          setValue(v);
          setSaved(v);
        }
      })
      .catch((e) => live && setError(errMsg(e)));
    return () => {
      live = false;
    };
  }, [profile.id]);

  async function save() {
    const next = value.trim();
    if (next === saved) return;
    try {
      await SetProfileSetting(profile.id, REQUIREMENT_TYPE_KEY, next);
      setSaved(next);
      setError("");
    } catch (e) {
      setError(errMsg(e));
    }
  }

  return (
    <span className="profile-setting">
      <input
        className="detail-input detail-input-inline"
        aria-label={`Requirement issue type for ${profile.name}`}
        placeholder="Requirement"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onBlur={() => void save()}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            void save();
          }
        }}
      />
      {error && <span className="error-text small">{error}</span>}
    </span>
  );
}
```

3. In the profile list, add the field to each row between the URL span and the "Make default" button:

```tsx
              <RequirementTypeField profile={p} />
```

4. Under the `<ul className="profile-list">`, replace the "Kiwi TCMS profiles" paragraph with:

```tsx
        <p className="muted small">
          Kiwi TCMS profiles from XTM are not listed; TAM talks to Jira only. The requirement
          field is the Jira issue type name TAM syncs as a requirement; leave it empty for
          "Requirement".
        </p>
```

Append to `tam/frontend/src/App.css`:

```css
.profile-setting { display: inline-flex; flex-direction: column; gap: 2px; }
.profile-setting .detail-input-inline { width: 150px; }
```

Run (inside `tam/frontend/`): `npx vitest run && npm run typecheck`
Expected: 30 tests passing, typecheck clean.

- [ ] **Step 3: Docs**

`tam/CLAUDE.md`: replace the Status section with:

```markdown
## Status

Plan 1a (issues, read path): sync by project into `tam.db`, the Backlog
grid, and a read-only detail panel, on the demo dataset or a live Jira DC.
Plan 1b adds the journal, create and edit, Commit, Excel import, and links.
```

and replace the Layout block with:

```
    main.go              Wails entry point, window, menu
    app.go               App struct: startup, profiles, settings
    app_issues.go        the issue methods: sync, list, detail, per-profile settings
    internal/tamstore/   TAM's own SQLite file (schema version 2: issue, issue_link, sync_state, profile_setting)
    internal/backend/    IssueBackend seam and DTOs; backend/jira on core/jira, backend/demo on internal/demo
    internal/demo/       the Acme Platform (PLAT) dataset behind a "demo" profile
    internal/issuerepo/  the store layer: issue cache, detail cache, links, sync state, profile settings
    internal/syncer/     the paging engine; emits tam:sync-progress through app_issues.go
    internal/suiteprofiles/  which shared profiles TAM shows, demo detection, validation
    frontend/            React app on @agile-suite/core (see ../frontend/core)
      src/api.ts         typed access to the bindings; plain shapes for fixtures
      src/queries/       TanStack Query keys, hooks, and the post-sync invalidation
      src/contexts/      SyncContext on the shared sync reducer
      src/components/    BacklogView, IssueTable, IssueDetailPanel, ProfilesModal, AboutModal
      wailsjs/           GENERATED bindings, do not hand-edit
```

Add to the Conventions section:

```markdown
Sync scope is `project = KEY AND issuetype in (Task, Epic, Story, Bug, <requirement
type>)` plus the profile's scope JQL; incremental syncs add `updated >=` the last
sync minus an hour. The requirement type name is the per-profile setting
`requirement_issue_type`. The Sprint and Epic Link field shapes are marked
`NOTE(tam)` in `internal/backend/jira/fields.go` until verified on a real
instance.
```

Root `CLAUDE.md`: in the bullet list, extend the `core/` bullet so it reads "the shared Go spine (store runner, profiles, connections, settings, credentials, and the Jira transport in `core/jira`)". Root `README.md`: in the TAM paragraph, replace the words that describe the scaffold stage with one sentence saying TAM now syncs a project's issues and shows them in a Backlog with a detail panel, and that writes arrive with plan 1b.

Run the humanizer pass over every prose change.

- [ ] **Step 4: Package the app**

Inside `tam/`:

```bash
wails build
```

Expected: `tam/build/bin/task-activity-manager.exe`. Then `git status --short --untracked-files=no`; if `wails build` rewrote line endings under `xtm/` or `tam/frontend/wailsjs`, revert them with `git checkout -- xtm/ tam/frontend/wailsjs` (the bindings were already regenerated and committed in Task 7; a second generate is byte-identical apart from line endings). `tam/frontend/package.json.md5` will have changed because Task 10 added a dependency; keep that change.

- [ ] **Step 5: Whole-branch verification**

From the repo root:

```bash
npm test --workspaces --if-present 2>&1 | grep -E "^> |Tests "
npm run typecheck --workspaces --if-present
(cd core && go test ./... -count=1)
(cd tam && go vet ./... && go test ./internal/... -count=1)
(cd xtm && go vet ./... && go test ./internal/... -count=1)
```

Expected: core and XTM Vitest counts sum to the Task 8 baseline, TAM reports 30, every typecheck clean, every Go package `ok`.

- [ ] **Step 6: Commit**

```bash
git add tam/frontend/src tam/frontend/package.json.md5 tam/CLAUDE.md README.md CLAUDE.md
git commit -m "feat(tam): per-profile requirement type, and the guides for the read path"
```

Push the branch and confirm the four CI jobs (test, frontend, build-windows, build-macos) are green before opening the PR.

## Human smoke test before merging

Neither the extraction PR nor this one can be signed off from tests alone:

1. XTM under `wails dev` after Task 1: sync a demo profile, open a test, file a throwaway bug against the demo, run a commit. Every Xray path goes through the embedded transport now.
2. TAM under `wails dev`: create "Demo team" (URL `demo`, key `DEMO`), Sync, watch the progress chip, filter by Story and by Sprint 12, search "promo", page forward, open PLAT-412, read the three tabs, refresh the description, switch the theme, then Full sync. Set a requirement type on the profile and confirm the next sync still lists requirements.
3. Against a real Jira DC, with a throwaway PAT: Sync, then check the Sprint and PTS columns and the Epic field in the panel. That is the `NOTE(tam)` verification for the Sprint and Epic Link shapes; record the outcome in the fields file's comment.

## Self-review notes

- **Spec coverage.** §3.1 is Task 1; §3.2 Task 8; §4.1 Task 2; §4.2 Task 5; §4.3 and §4.4 Task 4; §4.5 Tasks 2 and 3; §4.6 Task 6; §4.7 Task 7; §5 Task 2; §6 Tasks 5 and 6; §7 Tasks 9 to 12; §8 Tasks 6, 9, and 11; §9 is honoured by `detail_json`, `updated`, the interface's size, the shared reducer's commit states, and the reserved `pending` slot (the table has no pending column yet; 1b adds the flag to the row shape); §10 is the test steps of every task.
- **Things this plan deliberately leaves out.** A TAM icon, a release workflow, the `.error-text` rule's twin in XTM's `App.css`, and the `mock:` demo prefix. Each is its own change.
- **Type consistency.** `backend.Issue` is the row shape in `issuerepo`, the return shape of both backends, and (as `Issue`) the frontend's; `issuerepo.IssueQuery` fields match `api.ts`'s `IssueQuery` and `ListIssues` converts through the generated class; `syncer.Progress`'s JSON matches the shared `SyncProgress`; the eight bound names are identical in Task 7, `api.ts`, and the mocks.
- **Risky spots for the implementer.** The eight XTM test constructors (Task 1 step 6); `wails generate module` must run after Task 7 or `api.ts` will not compile; the virtualiser under jsdom relies on the overscan (Task 10); `Menu` item roles in the shell test (Task 9 step 8).
