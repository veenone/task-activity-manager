# TAM Rituals, local first: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ritual pages live in `tam.db`, are edited in TAM through a rich text editor over Confluence storage format, and reach Confluence only through a Rituals Sync button that creates the page tree, pulls, pushes, and records conflicts.

**Architecture:** Go stores raw storage XHTML and never interprets it beyond rendering templates and comparing strings. `internal/ritualtemplate` renders five deterministic pages per sprint, `internal/ritualrepo` holds each page's local body, base body and conflict body, and `internal/ritualsync` runs one pass per board against a `confluence.Pages` interface that the HTTP client and an in-memory demo fake both satisfy. The frontend converts storage XHTML to TipTap JSON and back in a pure `lib/storage` module, keeps everything it does not model as opaque nodes carrying their raw XML, and saves locally on a debounce with no lock.

**Tech Stack:** Go 1.x (Wails v2, modernc SQLite through `core/store`), React 19, TypeScript, TipTap 3.31.3, Vitest + jsdom.

**Spec:** `docs/superpowers/specs/2026-09-14-tam-rituals-local-first-design.md`

## Global Constraints

- Before Task 2 starts, the working tree must be clean of the user's own uncommitted changes to `tam/frontend/wailsjs/**` and `tam/go.mod`. They predate this plan. Ask the user to commit or stash them; never discard or overwrite them.
- Work on branch `feat/tam-rituals-local-first` (already created, spec committed there).
- **Schema version is 12, not 11.** The spec says "schema version 11"; `tamstore.Schema.Version` was already 11 (a repair migration) when this plan was written, so the ritual sync columns arrive at version 12. Everything else in the spec's storage section holds unchanged.
- UI text uses no em dashes (`tam/CLAUDE.md` conventions). Middle dot ` · ` and ellipsis `…` are fine.
- Page titles: sprint overview = sprint name; ritual page = `<sprint name> · <label>`, labels `Planning`, `Standup`, `Review`, `Retrospective`. Overview label (UI only) = `Overview`.
- Ritual types: `_sprint`, `planning`, `standup`, `review`, `retro`, in that order everywhere a list is shown or walked.
- Statuses: `local`, `synced`, `unsynced`, `conflict`, `gone`.
- The only JQL TAM emits or previews: `sprint = N ORDER BY Rank`, `sprint = N AND statusCategory = Done`, `sprint = N AND statusCategory != Done`.
- Local saves (`SaveRitualBody`, resolutions, forget, delete, ensure) take **no** lock. `SyncRituals` takes `a.acquire(p.ID, "rituals")` in Go and `runRitualsSync` in the frontend.
- Save debounce: 800 ms.
- TipTap packages pinned exactly to `3.31.3`: `@tiptap/core`, `@tiptap/pm`, `@tiptap/react`, `@tiptap/starter-kit`, `@tiptap/extension-table`, `@tiptap/extension-list`. (TipTap 3 ships task lists in `@tiptap/extension-list`; the spec's `extension-task-list`/`task-item` names are the TipTap 2 packages.)
- Sentences, verbatim:
  - Root unreadable: `The Confluence root page <id> could not be read: <reason>`
  - Foreign title: `A page titled "<title>" already exists outside the rituals root. Rename one of them.`
  - Conflict banner: `Confluence has a newer version (v<N>). Your local edits are kept until you choose.`
  - Gone banner: `This page was deleted or moved in Confluence.`
  - Read only: `This page has content TAM cannot edit safely. Edit it in Confluence; Sync will bring the changes back.`
  - Macro caveat: `From TAM's cache. Done here means the status name; Confluence shows the live result.`
  - Unconfigured: `Add Confluence in Profile settings to sync.`
  - Not configured (Go): `Confluence is not configured for this profile`
- Test counts are counts to beat. Record the starting counts in Task 2 Step 1; the final gate reports how many were removed with the wizard, `ritualdefaults` and the association code, rather than hiding the drop.

## File map

**Go, created**
- `core/confluence/pages.go`: `Pages` interface, `StoredPage`, `ErrNotFound`, `ErrVersionConflict`, the four write/read calls on `*Client`.
- `core/confluence/pages_test.go`
- `tam/internal/ritualtemplate/ritualtemplate.go`, `ritualtemplate_test.go`, `testdata/{sprint,planning,standup,review,retro}.xml` (golden files, also read by the frontend corpus test).
- `tam/internal/ritualrepo/documents.go`, `documents_test.go`
- `tam/internal/demo/confluence.go`, `confluence_test.go`
- `tam/internal/ritualsync/ensure.go`, `run.go`, `ensure_test.go`, `run_test.go`, `harness_test.go`
- `tam/app_ritualsync_test.go`
- `docs/superpowers/plans/assets/2026-09-14-confluence-write-probe.md`

**Go, modified**
- `core/confluence/client.go` (shared `send`, package comment), `tam/internal/tamstore/tamstore.go` (+ test), `tam/app.go` (App field), `tam/app_rituals.go`, `tam/app_confluence.go`, `.gitattributes`.

**Go, removed in Task 17**
- `tam/internal/ritualdefaults/`, `tam/internal/ritualrepo/demo_seed.go`, the `Draft` API in `ritualrepo.go`, the association and page-cache methods in `core/profile/profile.go`, `ListChildPages`/`GetPage`/`DemoPages` in `core/confluence`, the old bindings.

**Frontend, created** (under `tam/frontend/src/`)
- `lib/storage/xml.ts`, `parse.ts`, `serialize.ts`, `normalize.ts`, `storage.test.ts`
- `lib/ritualText.ts`, `lib/ritualText.test.ts`
- `lib/standupLog.ts`, `lib/standupLog.test.ts`
- `components/ritual-editor/extensions.ts`, `context.ts`, `OpaqueViews.tsx`, `MacroPreview.tsx`, `RitualEditor.tsx`, `RitualEditor.test.tsx`, `MacroPreview.test.tsx`

**Frontend, modified**
- `api.ts`, `contexts/SyncContext.tsx` (+ test), `components/RitualsView.tsx` (rewritten) + test, `components/ProfileForm.tsx`, `App.test.tsx`, `App.css`, `package.json`.

**Frontend, removed in Task 17**
- `components/RitualWizard.tsx`, `components/RitualWizard.test.tsx`, `queries/rituals.test.tsx`.

---

## Part A: the probe

### Task 1: Run the Confluence write probe against the real instance

This task is run **by the user**, not by an agent. An agent executing this plan writes the probe file (Step 1), commits it, then stops and asks the user to run it and paste the answers. Do not start Task 2 until the answers are recorded. Phase 4's probe was written and never run; that must not repeat.

**Files:**
- Create: `docs/superpowers/plans/assets/2026-09-14-confluence-write-probe.md`

- [ ] **Step 1: Write the probe file**

````markdown
# Probing Confluence's page write calls

Four probes against your own Confluence Data Center, fifteen minutes, before TAM's ritual
sync is built on assumptions about them. The answers decide one optional setting and confirm
three error mappings.

## Before you start

Your personal access token goes in the `Authorization` header and nowhere else. Do not paste
it into this file.

```bash
CONF=https://confluence.example.com   # base URL, no trailing slash
read -rs PAT                          # paste the token, it will not echo
SPACE=TEAM                            # a space you can create pages in
ROOT=123456                           # the rituals root page id from Profile settings
```

Use a scratch area under the root. Probe 4 creates a second page you should delete afterwards.

---

## Probe 1: does a Jira Issues macro render without serverId?

```bash
curl -sS -i -X POST "$CONF/rest/api/content?expand=body.storage,version,ancestors" \
  -H "Authorization: Bearer $PAT" -H "Content-Type: application/json" \
  -d '{"type":"page","title":"TAM probe page","space":{"key":"'"$SPACE"'"},"ancestors":[{"id":"'"$ROOT"'"}],"body":{"storage":{"representation":"storage","value":"<h2>Issues</h2><ac:structured-macro ac:name=\"jira\"><ac:parameter ac:name=\"jqlQuery\">project = YOURKEY ORDER BY Rank</ac:parameter><ac:parameter ac:name=\"columns\">key,summary,type,status,assignee,story points</ac:parameter><ac:parameter ac:name=\"maximumIssues\">50</ac:parameter></ac:structured-macro><h2>Tasks</h2><ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>first task</ac:task-body></ac:task></ac:task-list>"}}}'
```

Replace `YOURKEY` with a Jira project the application link can see. Note the page `id` and
`version.number` from the response, then open the page in a browser.

**Assumed:** `200`, the macro renders an issue table including a Story Points column.

**Record:** status code; page id; version; does the table render; is the Story Points column
there; if not rendered, the exact message Confluence shows in place of the macro.

**What it changes.** If the macro needs `serverId`, TAM adds a per-profile setting
`confluence_jira_server_id`, read by the templates (ask the user for it in Profile settings).
If `story points` is not accepted as a column name, the Planning template drops it.

## Probe 2: what does the web editor do to a page TAM wrote?

Open the page in the web editor. Tick the task. Type a paragraph above the macro, a paragraph
below it, and a new bullet between the two headings. Save. Then:

```bash
curl -sS "$CONF/rest/api/content/PAGEID?expand=body.storage,version" \
  -H "Authorization: Bearer $PAT"
```

**Assumed:** version 2; the macro and the task list come back intact; the task status reads
`complete`; Confluence adds `ac:task-id` and possibly `ac:macro-id` attributes.

**Record:** paste the full `body.storage.value` below. It joins the frontend round-trip corpus
in Task 12 as `probeRoundTrip`.

**What it changes.** Any element shape `lib/storage` does not map stays opaque (safe, not
editable). If task lists come back in a shape other than `ac:task` / `ac:task-status` /
`ac:task-body`, add that shape to the parser in Task 12 or leave it opaque; decide then.

## Probe 3: an update with a stale version number

```bash
curl -sS -i -X PUT "$CONF/rest/api/content/PAGEID" \
  -H "Authorization: Bearer $PAT" -H "Content-Type: application/json" \
  -d '{"id":"PAGEID","type":"page","title":"TAM probe page","version":{"number":2},"body":{"storage":{"representation":"storage","value":"<p>stale write</p>"}}}'
```

(Version 2 is stale because the page is at 2 after probe 2; the next valid number is 3.)

**Assumed:** `409 Conflict`, a JSON body with a `message` naming the current version.

**Record:** status code and body. If it is not 409, `ErrVersionConflict` in Task 2 maps the
code that did come back.

## Probe 4: a duplicate title in the same space

```bash
curl -sS -i -X POST "$CONF/rest/api/content" \
  -H "Authorization: Bearer $PAT" -H "Content-Type: application/json" \
  -d '{"type":"page","title":"TAM probe page","space":{"key":"'"$SPACE"'"},"body":{"storage":{"representation":"storage","value":"<p>duplicate</p>"}}}'
```

**Assumed:** `400 Bad Request` with a message saying a page with this title already exists.

**Record:** status code and message. Nothing in the plan branches on it; the sync reports
Confluence's own words in the failure list.

## Answers

| probe | answer |
|---|---|
| 1 status / renders / story points column | |
| 2 round-tripped storage body | |
| 3 status and body | |
| 4 status and message | |

Delete the probe page afterwards.
````

- [ ] **Step 2: Commit the probe file**

```bash
git add docs/superpowers/plans/assets/2026-09-14-confluence-write-probe.md
git commit -m "docs(tam): a probe for the Confluence page write calls"
```

- [ ] **Step 3: Stop and ask the user to run the probe**

Say: "Task 1 needs your Confluence instance. Please run the four probes in `docs/superpowers/plans/assets/2026-09-14-confluence-write-probe.md` and paste the answers." Record the answers in the file's table, commit (`docs(tam): the Confluence write probe answers`), and apply the "What it changes" notes before continuing:
- Probe 1 no `serverId` needed: nothing changes.
- Probe 1 `serverId` needed: add to Task 4 a `ServerID string` field on `SprintInfo`, emitted as `<ac:parameter ac:name="serverId">…</ac:parameter>` when non-empty, and to Task 10 a read of profile setting `confluence_jira_server_id` into it.
- Probe 1 story points column rejected: in Task 4 drop `,story points` from the Planning columns and regenerate the golden files.
- Probe 3 not 409: change the `ErrVersionConflict` code in Task 2 to what came back.

---

## Part B: Go

### Task 2: The Confluence page transport

**Files:**
- Create: `core/confluence/pages.go`, `core/confluence/pages_test.go`
- Modify: `core/confluence/client.go` (package comment; replace `get` with `send`)

**Interfaces:**
- Produces:
  - `type StoredPage struct { ID, Title string; Version int; Body string; AncestorIDs []string }`
  - `type Pages interface { GetPageStorage(ctx, id string) (StoredPage, error); FindPageByTitle(ctx, spaceKey, title string) (StoredPage, bool, error); CreatePage(ctx, spaceKey, parentID, title, body string) (StoredPage, error); UpdatePage(ctx, id, title, body string, version int) (StoredPage, error) }`
  - `var ErrNotFound, ErrVersionConflict error`, matched through `errors.Is` on `*HTTPError`.
  - `UpdatePage` sends `version.number = version` exactly as given; callers pass base plus one.

- [ ] **Step 1: Record the starting test counts**

Run from the repo root and write the three numbers into your task notes (they are compared at the final gate):

```bash
npm test --workspaces --if-present 2>&1 | grep -E "Tests +[0-9]+"
cd tam && go test ./... 2>&1 | tail -5
```

- [ ] **Step 2: Write the failing tests**

`core/confluence/pages_test.go`:

```go
package confluence

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const storedJSON = `{"id":"42","title":"Sprint 14 · Planning","version":{"number":3},
	"body":{"storage":{"value":"<p>plan</p>"}},"ancestors":[{"id":"1"},{"id":"7"}]}`

func TestGetPageStorageReadsBodyVersionAndAncestors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/content/42" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("expand"); got != "body.storage,version,ancestors" {
			t.Errorf("expand = %q", got)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(storedJSON))
	}))
	defer srv.Close()

	p, err := NewClient(srv.URL, "secret", "", false).GetPageStorage(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "42" || p.Version != 3 || p.Body != "<p>plan</p>" || len(p.AncestorIDs) != 2 || p.AncestorIDs[1] != "7" {
		t.Fatalf("page = %+v", p)
	}
}

func TestFindPageByTitleSearchesTheSpace(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q, _ := url.ParseQuery(r.URL.RawQuery)
		if r.URL.Path != "/rest/api/content" || q.Get("spaceKey") != "PLAT" || q.Get("type") != "page" {
			t.Errorf("request = %s ? %s", r.URL.Path, r.URL.RawQuery)
		}
		if q.Get("title") == "Missing" {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		if q.Get("title") != "Sprint 14 · Planning" {
			t.Errorf("title = %q", q.Get("title"))
		}
		_, _ = w.Write([]byte(`{"results":[` + storedJSON + `]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "secret", "", false)

	p, ok, err := c.FindPageByTitle(context.Background(), "PLAT", "Sprint 14 · Planning")
	if err != nil || !ok || p.ID != "42" {
		t.Fatalf("found = %+v, %v, %v", p, ok, err)
	}
	_, ok, err = c.FindPageByTitle(context.Background(), "PLAT", "Missing")
	if err != nil || ok {
		t.Fatalf("missing = %v, %v", ok, err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestCreatePageSendsSpaceParentAndStorageBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/content" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		raw, _ := io.ReadAll(r.Body)
		var got struct {
			Type      string `json:"type"`
			Title     string `json:"title"`
			Space     struct{ Key string } `json:"space"`
			Ancestors []struct{ ID string } `json:"ancestors"`
			Body      struct {
				Storage struct{ Value, Representation string } `json:"storage"`
			} `json:"body"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("payload %s: %v", raw, err)
		}
		if got.Type != "page" || got.Title != "Sprint 14" || got.Space.Key != "PLAT" ||
			len(got.Ancestors) != 1 || got.Ancestors[0].ID != "1" ||
			got.Body.Storage.Value != "<p>overview</p>" || got.Body.Storage.Representation != "storage" {
			t.Errorf("payload = %s", raw)
		}
		// Confluence answers a create without the body when expand is ignored.
		_, _ = w.Write([]byte(`{"id":"99","title":"Sprint 14","version":{"number":1}}`))
	}))
	defer srv.Close()

	p, err := NewClient(srv.URL, "secret", "", false).CreatePage(context.Background(), "PLAT", "1", "Sprint 14", "<p>overview</p>")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "99" || p.Version != 1 || p.Body != "<p>overview</p>" {
		t.Fatalf("created = %+v", p)
	}
}

func TestUpdatePageSendsTheVersionItIsGiven(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/content/42" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var got struct {
			ID      string `json:"id"`
			Version struct{ Number int } `json:"version"`
		}
		_ = json.Unmarshal(raw, &got)
		if got.ID != "42" || got.Version.Number != 4 {
			t.Errorf("payload = %s", raw)
		}
		_, _ = w.Write([]byte(`{"id":"42","title":"T","version":{"number":4},"body":{"storage":{"value":"<p>v4</p>"}}}`))
	}))
	defer srv.Close()

	p, err := NewClient(srv.URL, "secret", "", false).UpdatePage(context.Background(), "42", "T", "<p>v4</p>", 4)
	if err != nil || p.Version != 4 {
		t.Fatalf("updated = %+v, %v", p, err)
	}
}

func TestConflictAndNotFoundAreMatchable(t *testing.T) {
	for _, tc := range []struct {
		code      int
		want, not error
	}{
		{http.StatusConflict, ErrVersionConflict, ErrNotFound},
		{http.StatusNotFound, ErrNotFound, ErrVersionConflict},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
			_, _ = w.Write([]byte(`{"message":"nope"}`))
		}))
		_, err := NewClient(srv.URL, "secret", "", false).UpdatePage(context.Background(), "42", "T", "<p/>", 2)
		srv.Close()
		if !errors.Is(err, tc.want) || errors.Is(err, tc.not) {
			t.Errorf("code %d: err = %v", tc.code, err)
		}
	}
}
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `cd core && go test ./confluence/ -run 'Storage|Title|CreatePage|UpdatePage|Matchable'`
Expected: FAIL, `GetPageStorage` (and the others) undefined.

- [ ] **Step 4: Replace `get` with `send` in `client.go`**

In `core/confluence/client.go`, change the package comment and replace the `get` function (keep `responseDecoder` as is). Add `"bytes"` to the imports.

```go
// Package confluence is the Confluence Data Center transport behind TAM's
// Rituals view: reading a page's storage body, finding a page by title, and
// creating and updating pages. TAM's ritualsync decides when each is called;
// nothing here keeps state between calls.
package confluence
```

```go
func (c *Client) get(ctx context.Context, path string) responseDecoder {
	return c.send(ctx, http.MethodGet, path, nil)
}

// send is every request this client makes. A nil payload sends no body; any
// other payload is JSON. A non-2xx answer becomes *HTTPError carrying
// Confluence's own message, which is what errors.Is matches ErrNotFound and
// ErrVersionConflict against.
func (c *Client) send(ctx context.Context, method, path string, payload any) responseDecoder {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return responseDecoder{err: err}
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return responseDecoder{err: err}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return responseDecoder{err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(b, &e)
		return responseDecoder{err: &HTTPError{Code: resp.StatusCode, Status: resp.Status, Message: e.Message}}
	}
	return responseDecoder{body: resp.Body}
}
```

- [ ] **Step 5: Write `pages.go`**

```go
package confluence

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

var (
	// ErrNotFound matches a 404 from any call through errors.Is. The ritual
	// sync reads it as a page deleted or moved out of reach in Confluence.
	ErrNotFound = errors.New("confluence: not found")
	// ErrVersionConflict matches a 409, Confluence's answer to an update whose
	// version number is not the page's current version plus one.
	ErrVersionConflict = errors.New("confluence: version conflict")
)

// Is lets errors.Is match an HTTP answer against the two sentinels by code,
// so a caller never compares status numbers itself.
func (e *HTTPError) Is(target error) bool {
	switch target {
	case ErrNotFound:
		return e.Code == http.StatusNotFound
	case ErrVersionConflict:
		return e.Code == http.StatusConflict
	}
	return false
}

// StoredPage is a page the way the ritual sync reads and writes it: the
// storage body, the version an update must build on, and the ancestor ids an
// adoption checks against the rituals root. Ancestors run root first.
type StoredPage struct {
	ID          string
	Title       string
	Version     int
	Body        string
	AncestorIDs []string
}

// Pages is what the ritual sync needs from Confluence. The HTTP client below
// satisfies it, and so does TAM's in-memory demo space.
type Pages interface {
	GetPageStorage(ctx context.Context, id string) (StoredPage, error)
	FindPageByTitle(ctx context.Context, spaceKey, title string) (StoredPage, bool, error)
	CreatePage(ctx context.Context, spaceKey, parentID, title, body string) (StoredPage, error)
	UpdatePage(ctx context.Context, id, title, body string, version int) (StoredPage, error)
}

var _ Pages = (*Client)(nil)

const storedExpand = "body.storage,version,ancestors"

type rawStoredPage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Ancestors []struct {
		ID string `json:"id"`
	} `json:"ancestors"`
}

func (r rawStoredPage) stored() StoredPage {
	p := StoredPage{ID: r.ID, Title: r.Title, Version: r.Version.Number, Body: r.Body.Storage.Value, AncestorIDs: []string{}}
	for _, a := range r.Ancestors {
		p.AncestorIDs = append(p.AncestorIDs, a.ID)
	}
	return p
}

type storageValue struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

func storageBody(body string) map[string]storageValue {
	return map[string]storageValue{"storage": {Value: body, Representation: "storage"}}
}

// GetPageStorage reads one page's storage body, version and ancestors.
func (c *Client) GetPageStorage(ctx context.Context, id string) (StoredPage, error) {
	var r rawStoredPage
	if err := c.get(ctx, "/rest/api/content/"+url.PathEscape(id)+"?expand="+storedExpand).Decode(&r); err != nil {
		return StoredPage{}, err
	}
	return r.stored(), nil
}

// FindPageByTitle looks for a page anywhere in the space. Confluence Data
// Center keeps titles unique within a space, so there is at most one answer,
// and it may sit outside the rituals root: the caller checks AncestorIDs.
func (c *Client) FindPageByTitle(ctx context.Context, spaceKey, title string) (StoredPage, bool, error) {
	q := url.Values{}
	q.Set("spaceKey", spaceKey)
	q.Set("title", title)
	q.Set("type", "page")
	q.Set("expand", storedExpand)
	var r struct {
		Results []rawStoredPage `json:"results"`
	}
	if err := c.get(ctx, "/rest/api/content?"+q.Encode()).Decode(&r); err != nil {
		return StoredPage{}, false, err
	}
	if len(r.Results) == 0 {
		return StoredPage{}, false, nil
	}
	return r.Results[0].stored(), true, nil
}

// CreatePage creates a page under parentID. A response that leaves the body
// out answers with the body that was sent, since that is what now exists.
func (c *Client) CreatePage(ctx context.Context, spaceKey, parentID, title, body string) (StoredPage, error) {
	payload := map[string]any{
		"type":      "page",
		"title":     title,
		"space":     map[string]string{"key": spaceKey},
		"ancestors": []map[string]string{{"id": parentID}},
		"body":      storageBody(body),
	}
	var r rawStoredPage
	if err := c.send(ctx, http.MethodPost, "/rest/api/content?expand="+storedExpand, payload).Decode(&r); err != nil {
		return StoredPage{}, err
	}
	p := r.stored()
	if p.Body == "" {
		p.Body = body
	}
	return p, nil
}

// UpdatePage replaces a page's title and body. version is sent exactly as
// given, and must be the page's current version plus one: Confluence refuses
// anything else with 409, which is the optimistic concurrency the sync relies
// on instead of inventing its own.
func (c *Client) UpdatePage(ctx context.Context, id, title, body string, version int) (StoredPage, error) {
	payload := map[string]any{
		"id":      id,
		"type":    "page",
		"title":   title,
		"version": map[string]int{"number": version},
		"body":    storageBody(body),
	}
	var r rawStoredPage
	if err := c.send(ctx, http.MethodPut, "/rest/api/content/"+url.PathEscape(id)+"?expand="+storedExpand, payload).Decode(&r); err != nil {
		return StoredPage{}, err
	}
	p := r.stored()
	if p.Body == "" {
		p.Body = body
	}
	return p, nil
}
```

- [ ] **Step 6: Run the package tests**

Run: `cd core && go test ./confluence/ && go vet ./confluence/`
Expected: PASS (the three existing client tests still pass through `get`).

- [ ] **Step 7: Commit**

```bash
git add core/confluence
git commit -m "feat(core): read, find, create and update Confluence pages"
```

---

### Task 3: Schema version 12, the ritual sync columns

**Files:**
- Modify: `tam/internal/tamstore/tamstore.go` (DDL, `Schema.Version`, migration 12, package comment)
- Modify: `tam/internal/tamstore/tamstore_test.go`
- Modify: `tam/internal/ritualrepo/migration_test.go` (its hard-coded `11`)

**Interfaces:**
- Produces: `ritual_document` columns `base_body TEXT NOT NULL DEFAULT ''`, `conflict_body TEXT NOT NULL DEFAULT ''`, `conflict_version INTEGER NOT NULL DEFAULT 0`.

- [ ] **Step 1: Write the failing test** (append to `tamstore_test.go`)

```go
// Version 12 adds the three columns the ritual sync compares and resolves
// with. It rewrites no rows: a body written before it keeps its text, and the
// ensure step upgrades rows that have none.
func TestSchemaVersionTwelveAddsTheRitualSyncColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE ritual_document DROP COLUMN base_body`,
		`ALTER TABLE ritual_document DROP COLUMN conflict_body`,
		`ALTER TABLE ritual_document DROP COLUMN conflict_version`,
		`INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, body) VALUES ('p', 1, 14, 'planning', '<p>kept</p>')`,
		`UPDATE meta SET value = '11' WHERE key = 'schema_version'`,
	} {
		if _, err := db.DB().Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	_ = db.Close()

	db, err = tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	var n int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('ritual_document')
		WHERE name IN ('base_body', 'conflict_body', 'conflict_version')`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("sync columns = %d, %v", n, err)
	}
	var body string
	if err := db.DB().QueryRow(`SELECT body FROM ritual_document WHERE sprint_id = 14`).Scan(&body); err != nil || body != "<p>kept</p>" {
		t.Fatalf("body = %q, %v", body, err)
	}
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version || v != 12 {
		t.Errorf("schema version = %d, want 12", v)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `cd tam && go test ./internal/tamstore/ -run Twelve`
Expected: FAIL at the first `DROP COLUMN` (no such column).

- [ ] **Step 3: Add the columns and the migration**

In `ritualDocumentDDL`, after `confluence_version INTEGER NOT NULL DEFAULT 0,`:

```sql
	base_body         TEXT NOT NULL DEFAULT '',
	conflict_body     TEXT NOT NULL DEFAULT '',
	conflict_version  INTEGER NOT NULL DEFAULT 0,
```

Set `Version: 12` and append to `Migrations`, after the version 11 entry:

```go
	}, {
		Version: 12,
		// The ritual sync keeps three more facts per page: the body as of the
		// last synced version, and a newer remote body a Sync found while local
		// edits were pending, with its version. Column adds in version 7's
		// shape; a fresh database has them from ritualDocumentDDL already, which
		// AddColumnIfMissing treats as success. No row is rewritten: this
		// migration knows no sprint names to render a template from, so rows
		// written before it are upgraded by ritualsync.Ensure instead.
		Apply: func(db *sql.DB) error {
			for _, column := range []string{
				"base_body TEXT NOT NULL DEFAULT ''",
				"conflict_body TEXT NOT NULL DEFAULT ''",
				"conflict_version INTEGER NOT NULL DEFAULT 0",
			} {
				if err := store.AddColumnIfMissing(db, "ritual_document", column); err != nil {
					return err
				}
			}
			return nil
		},
```

Append to the package comment, after the version 11 sentence: `Version 12 adds ritual_document's base_body, conflict_body and conflict_version, the three facts the ritual sync compares and resolves with.`

In `tam/internal/ritualrepo/migration_test.go`, replace `version != 11` with `version != tamstore.Schema.Version`.

- [ ] **Step 4: Run the store and ritual tests**

Run: `cd tam && go test ./internal/tamstore/ ./internal/ritualrepo/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/tamstore tam/internal/ritualrepo/migration_test.go
git commit -m "feat(tam): the columns a ritual sync compares and resolves with"
```

---

### Task 4: The ritual templates

**Files:**
- Create: `tam/internal/ritualtemplate/ritualtemplate.go`, `tam/internal/ritualtemplate/ritualtemplate_test.go`
- Create (generated in Step 5): `tam/internal/ritualtemplate/testdata/{sprint,planning,standup,review,retro}.xml`
- Modify: `.gitattributes` at the repo root (create if absent)

**Interfaces:**
- Produces:
  - consts `Sprint = "_sprint"`, `Planning`, `Standup`, `Review`, `Retro`; `var Types = []string{Sprint, Planning, Standup, Review, Retro}`
  - `func Known(ritualType string) bool`, `func Label(ritualType string) string`
  - `type SprintInfo struct { ID int; Name, Goal, StartDate, EndDate, BoardName string }`
  - `func Title(ritualType string, s SprintInfo) string`
  - `func Render(ritualType string, s SprintInfo, loc *time.Location) string`
  - `func StandupEntry(day time.Time) string`
  - `type Note struct { Key, Remark string }`; `func EarlierNotes(remark string, notes []Note) string`
  - `type Filter int` with `All`, `Done`, `NotDone`; `func JQL(sprintID int, f Filter) string`; `func ParseJQL(jql string) (int, Filter, bool)`

- [ ] **Step 1: Write the failing tests**

```go
package ritualtemplate

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var golden = SprintInfo{
	ID: 14, Name: "Sprint 14", Goal: "Ship promo codes & the VAT fix",
	StartDate: "2026-09-14T09:00:00.000+0000", EndDate: "2026-09-25T17:00:00.000+0000",
	BoardName: "PLAT board",
}

// The golden files are the frontend's round-trip corpus as well, so a change
// to a template is a change the storage converter is re-tested against.
// Regenerate with: UPDATE_GOLDEN=1 go test ./internal/ritualtemplate/
func TestRenderMatchesTheGoldenFiles(t *testing.T) {
	for _, typ := range Types {
		got := Render(typ, golden, time.UTC)
		path := filepath.Join("testdata", strings.TrimPrefix(typ, "_")+".xml")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run with UPDATE_GOLDEN=1 once)", path, err)
		}
		if got != string(want) {
			t.Errorf("%s drifted from its golden file:\n got %s\nwant %s", typ, got, want)
		}
	}
}

// Adoption compares a stored body against a fresh render, so the same sprint
// must always render the same bytes.
func TestRenderIsDeterministic(t *testing.T) {
	for _, typ := range Types {
		if Render(typ, golden, time.UTC) != Render(typ, golden, time.UTC) {
			t.Errorf("%s rendered differently twice", typ)
		}
	}
}

func TestRenderIsWellFormedOnceItsPrefixesAreDeclared(t *testing.T) {
	for _, typ := range Types {
		doc := `<root xmlns:ac="http://atlassian.com/content" xmlns:ri="http://atlassian.com/resource/identifier">` +
			Render(typ, golden, time.UTC) + `</root>`
		dec := xml.NewDecoder(strings.NewReader(doc))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s is not well formed: %v", typ, err)
			}
		}
	}
}

func TestRenderEmitsOnlyTheThreeJQLForms(t *testing.T) {
	cases := map[string][]string{
		Planning: {"sprint = 14 ORDER BY Rank"},
		Standup:  {"sprint = 14 AND statusCategory != Done"},
		Review:   {"sprint = 14 AND statusCategory = Done", "sprint = 14 AND statusCategory != Done"},
		Retro:    {"sprint = 14 AND statusCategory != Done"},
	}
	for typ, wants := range cases {
		body := Render(typ, golden, time.UTC)
		for _, want := range wants {
			if !strings.Contains(body, `<ac:parameter ac:name="jqlQuery">`+want+`</ac:parameter>`) {
				t.Errorf("%s is missing %q", typ, want)
			}
		}
	}
}

func TestRenderEscapesAndCarriesTheSprintFacts(t *testing.T) {
	body := Render(Sprint, golden, time.UTC)
	for _, want := range []string{"Ship promo codes &amp; the VAT fix", "PLAT board", "14 Sep 2026 to 25 Sep 2026", `ac:name="children"`} {
		if !strings.Contains(body, want) {
			t.Errorf("overview missing %q in %s", want, body)
		}
	}
	undated := golden
	undated.StartDate, undated.EndDate = "", ""
	if !strings.Contains(Render(Sprint, undated, time.UTC), "Dates not set") {
		t.Error("an undated sprint should say its dates are not set")
	}
	if strings.Contains(Render(Standup, undated, time.UTC), "<h3>") {
		t.Error("an undated standup should seed no daily entry")
	}
}

func TestTitlesAndLabels(t *testing.T) {
	if got := Title(Sprint, golden); got != "Sprint 14" {
		t.Errorf("overview title = %q", got)
	}
	if got := Title(Retro, golden); got != "Sprint 14 · Retrospective" {
		t.Errorf("retro title = %q", got)
	}
	if got := Title(Planning, SprintInfo{ID: 9}); got != "Sprint 9 · Planning" {
		t.Errorf("nameless title = %q", got)
	}
	if !Known(" Review ") || Known("party") || Label(Sprint) != "Overview" {
		t.Error("Known/Label disagree with the type list")
	}
}

func TestStandupEntryIsTheDayHeadingAndThreeSections(t *testing.T) {
	entry := StandupEntry(time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	if !strings.HasPrefix(entry, "<h3>Tue 15 Sep 2026</h3>") {
		t.Fatalf("entry = %s", entry)
	}
	for _, want := range []string{"<strong>Yesterday</strong>", "<strong>Today</strong>", "<strong>Blockers</strong>", "<ac:task-list>"} {
		if !strings.Contains(entry, want) {
			t.Errorf("entry missing %q", want)
		}
	}
}

func TestEarlierNotesKeepsOnlyWhatSomebodyWrote(t *testing.T) {
	if EarlierNotes("  ", []Note{{Key: "PLAT-1"}}) != "" {
		t.Error("nothing written should add nothing")
	}
	got := EarlierNotes("short sprint", []Note{{Key: "PLAT-1", Remark: "demoed"}, {Key: "PLAT-2"}})
	want := "<h2>Earlier draft notes</h2><p>short sprint</p><ul><li>PLAT-1: demoed</li></ul>"
	if got != want {
		t.Errorf("notes = %s", got)
	}
}

func TestParseJQLReadsTheThreeFormsAndNothingElse(t *testing.T) {
	for _, f := range []Filter{All, Done, NotDone} {
		id, got, ok := ParseJQL(JQL(14, f))
		if !ok || id != 14 || got != f {
			t.Errorf("round trip of %v = %d %v %v", f, id, got, ok)
		}
	}
	if _, _, ok := ParseJQL("  SPRINT = 3  and statuscategory != done "); !ok {
		t.Error("case and spacing should not matter")
	}
	for _, other := range []string{"project = PLAT", "sprint = 14 AND assignee = currentUser()", "sprint in openSprints()"} {
		if _, _, ok := ParseJQL(other); ok {
			t.Errorf("%q should not be previewable", other)
		}
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd tam && go test ./internal/ritualtemplate/`
Expected: FAIL to compile, `Render` undefined.

- [ ] **Step 3: Write the package**

```go
// Package ritualtemplate renders the five pages a sprint's rituals start
// from, in Confluence storage format. It does no I/O and reads no clock:
// ritualsync compares a stored body against a fresh render to tell a page
// nobody has touched from one somebody wrote in, and that comparison is only
// honest if the same sprint always renders the same bytes.
package ritualtemplate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"agile-suite/tam/internal/sprintdate"
)

const (
	Sprint   = "_sprint"
	Planning = "planning"
	Standup  = "standup"
	Review   = "review"
	Retro    = "retro"
)

// Types is every page a sprint gets, the overview first because Sync has to
// create it before any ritual page can sit under it.
var Types = []string{Sprint, Planning, Standup, Review, Retro}

var labels = map[string]string{
	Sprint: "Overview", Planning: "Planning", Standup: "Standup", Review: "Review", Retro: "Retrospective",
}

func normalize(ritualType string) string { return strings.ToLower(strings.TrimSpace(ritualType)) }

// Known reports whether ritualType names one of the five pages.
func Known(ritualType string) bool { _, ok := labels[normalize(ritualType)]; return ok }

// Label is a page's name on screen and in its title.
func Label(ritualType string) string { return labels[normalize(ritualType)] }

// SprintInfo is everything a template reads, all of it from the board cache.
type SprintInfo struct {
	ID        int
	Name      string
	Goal      string
	StartDate string
	EndDate   string
	BoardName string
}

func (s SprintInfo) name() string {
	if n := strings.TrimSpace(s.Name); n != "" {
		return n
	}
	return "Sprint " + strconv.Itoa(s.ID)
}

// Title is the page title Sync creates and matches by. Confluence keeps
// titles unique within a space, which is why ritual pages carry the sprint.
func Title(ritualType string, s SprintInfo) string {
	if normalize(ritualType) == Sprint {
		return s.name()
	}
	return s.name() + " · " + Label(ritualType)
}

// Filter is which of a sprint's issues a Jira Issues macro lists.
type Filter int

const (
	All Filter = iota
	Done
	NotDone
)

// JQL is the query a macro carries. These three forms are the only ones TAM
// writes, because they are the only ones it can preview from its cache.
func JQL(sprintID int, f Filter) string {
	switch f {
	case Done:
		return fmt.Sprintf("sprint = %d AND statusCategory = Done", sprintID)
	case NotDone:
		return fmt.Sprintf("sprint = %d AND statusCategory != Done", sprintID)
	}
	return fmt.Sprintf("sprint = %d ORDER BY Rank", sprintID)
}

var jqlForm = regexp.MustCompile(`(?i)^\s*sprint\s*=\s*(\d+)\s*(?:AND\s+statusCategory\s*(!=|=)\s*Done|ORDER\s+BY\s+Rank)?\s*$`)

// ParseJQL reads a macro's query back. ok is false for anything but the
// three forms JQL writes, however reasonable the query.
func ParseJQL(jql string) (int, Filter, bool) {
	m := jqlForm.FindStringSubmatch(jql)
	if m == nil {
		return 0, All, false
	}
	id, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, All, false
	}
	switch m[2] {
	case "=":
		return id, Done, true
	case "!=":
		return id, NotDone, true
	}
	return id, All, true
}

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func esc(s string) string { return escaper.Replace(s) }

type page struct{ strings.Builder }

func (p *page) h2(text string)   { p.WriteString("<h2>" + esc(text) + "</h2>") }
func (p *page) para(text string) { p.WriteString("<p>" + esc(text) + "</p>") }
func (p *page) emptyList()       { p.WriteString("<ul><li></li></ul>") }
func (p *page) tasks()           { p.WriteString(taskList) }

const taskList = "<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body></ac:task-body></ac:task></ac:task-list>"

func (p *page) table(headers ...string) {
	p.WriteString("<table><tbody><tr>")
	for _, h := range headers {
		p.WriteString("<th>" + esc(h) + "</th>")
	}
	p.WriteString("</tr>")
	for row := 0; row < 3; row++ {
		p.WriteString("<tr>" + strings.Repeat("<td></td>", len(headers)) + "</tr>")
	}
	p.WriteString("</tbody></table>")
}

func (p *page) jira(jql string, storyPoints bool) {
	columns := "key,summary,type,status,assignee"
	if storyPoints {
		columns += ",story points"
	}
	p.WriteString(`<ac:structured-macro ac:name="jira">` +
		`<ac:parameter ac:name="jqlQuery">` + esc(jql) + `</ac:parameter>` +
		`<ac:parameter ac:name="columns">` + columns + `</ac:parameter>` +
		`<ac:parameter ac:name="maximumIssues">50</ac:parameter>` +
		`</ac:structured-macro>`)
}

func day(value string, loc *time.Location) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	t, err := sprintdate.Parse(value)
	if err != nil {
		return time.Time{}, false
	}
	return t.In(loc), true
}

func dates(s SprintInfo, loc *time.Location) string {
	start, okStart := day(s.StartDate, loc)
	end, okEnd := day(s.EndDate, loc)
	if !okStart && !okEnd {
		return "Dates not set"
	}
	format := func(t time.Time, ok bool) string {
		if !ok {
			return "not set"
		}
		return t.Format("2 Jan 2006")
	}
	return format(start, okStart) + " to " + format(end, okEnd)
}

// Render is the page a ritual starts as. loc decides which local day a
// sprint date falls on; nil reads as UTC.
func Render(ritualType string, s SprintInfo, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	var p page
	switch normalize(ritualType) {
	case Sprint:
		p.WriteString("<p><strong>Board:</strong> " + esc(s.BoardName) + "</p>")
		p.WriteString("<p><strong>Dates:</strong> " + esc(dates(s, loc)) + "</p>")
		p.h2("Sprint goal")
		p.para(s.Goal)
		p.h2("Rituals")
		p.WriteString(`<ac:structured-macro ac:name="children" />`)
	case Planning:
		p.h2("Sprint goal")
		p.para(s.Goal)
		p.h2("Capacity")
		p.table("Member", "Days available", "Notes")
		p.h2("Committed scope")
		p.jira(JQL(s.ID, All), true)
		p.h2("Risks and dependencies")
		p.emptyList()
		p.h2("Decisions")
		p.emptyList()
		p.h2("Action items")
		p.tasks()
	case Standup:
		p.h2("Blockers and work in flight")
		p.jira(JQL(s.ID, NotDone), false)
		p.h2("Daily log")
		if start, ok := day(s.StartDate, loc); ok {
			p.WriteString(StandupEntry(start))
		}
	case Review:
		p.h2("Sprint goal")
		p.para(s.Goal)
		p.para("Met / Partly met / Not met")
		p.h2("Completed")
		p.jira(JQL(s.ID, Done), false)
		p.h2("Not completed")
		p.jira(JQL(s.ID, NotDone), false)
		p.h2("Demo notes")
		p.emptyList()
		p.h2("Stakeholder feedback")
		p.table("Who", "Feedback", "Follow up")
		p.h2("Follow ups")
		p.tasks()
	case Retro:
		p.h2("What went well")
		p.emptyList()
		p.h2("What did not")
		p.emptyList()
		p.h2("What we will try")
		p.emptyList()
		p.h2("Carried over")
		p.jira(JQL(s.ID, NotDone), false)
		p.h2("Action items")
		p.tasks()
	}
	return p.String()
}

// StandupEntry is one day of the standup's daily log. The editor's "Add
// today's entry" inserts exactly this, so the log and the template agree.
func StandupEntry(day time.Time) string {
	return "<h3>" + day.Format("Mon 2 Jan 2006") + "</h3>" +
		"<p><strong>Yesterday</strong></p><ul><li></li></ul>" +
		"<p><strong>Today</strong></p><ul><li></li></ul>" +
		"<p><strong>Blockers</strong></p>" + taskList
}

// Note is one remark the retired wizard stored against an issue.
type Note struct {
	Key    string
	Remark string
}

// EarlierNotes carries what somebody typed into the retired wizard onto the
// page that replaces it. It answers "" when nothing was written.
func EarlierNotes(remark string, notes []Note) string {
	var kept []Note
	for _, n := range notes {
		if strings.TrimSpace(n.Remark) != "" {
			kept = append(kept, n)
		}
	}
	if strings.TrimSpace(remark) == "" && len(kept) == 0 {
		return ""
	}
	var p page
	p.h2("Earlier draft notes")
	if strings.TrimSpace(remark) != "" {
		p.para(remark)
	}
	if len(kept) > 0 {
		p.WriteString("<ul>")
		for _, n := range kept {
			p.WriteString("<li>" + esc(n.Key+": "+n.Remark) + "</li>")
		}
		p.WriteString("</ul>")
	}
	return p.String()
}
```

- [ ] **Step 4: Keep golden files byte-exact on Windows checkouts**

Append to the repo root `.gitattributes` (create the file if it does not exist):

```
tam/internal/ritualtemplate/testdata/*.xml -text
```

Without it `core.autocrlf` rewrites the golden files to CRLF on checkout and both the Go and frontend comparisons fail.

- [ ] **Step 5: Generate the golden files, then run the tests**

```bash
cd tam
mkdir -p internal/ritualtemplate/testdata
UPDATE_GOLDEN=1 go test ./internal/ritualtemplate/ -run Golden
go test ./internal/ritualtemplate/
```

Expected: PASS. Open `testdata/planning.xml` and check by eye that it reads as the spec's Planning page.

- [ ] **Step 6: Commit**

```bash
git add .gitattributes tam/internal/ritualtemplate
git commit -m "feat(tam): the five pages a sprint's rituals start from"
```

---

### Task 5: The ritual document store

**Files:**
- Create: `tam/internal/ritualrepo/documents.go`, `tam/internal/ritualrepo/documents_test.go`

The old `Draft` API in `ritualrepo.go` stays untouched until Task 17; both coexist on the same table.

**Interfaces:**
- Consumes: Task 3 columns.
- Produces:
  - consts `StatusLocal`, `StatusSynced`, `StatusUnsynced`, `StatusConflict`, `StatusGone`
  - `type Key struct { ProfileID string; BoardID, SprintID int; RitualType string }`
  - `type Document struct` with JSON fields `profileId, boardId, sprintId, ritualType, title, body, baseBody, pageId, version, conflictBody, conflictVersion, status, updatedAt, syncedAt`; methods `Key() Key`, `Dirty() bool`
  - `type Legacy struct { RitualType, Remark string; Issues []Issue }`
  - `(*Repository)`: `Document(ctx, Key) (Document, bool, error)`, `Documents(ctx, profileID string, boardID, sprintID int) ([]Document, error)`, `ProfileDocuments(ctx, profileID string) ([]Document, error)`, `BoardSprintIDs(ctx, profileID string, boardID int) ([]int, error)`, `NeedsTemplate(ctx, profileID string, boardID, sprintID int, types []string) ([]Legacy, error)`, `WriteTemplate(ctx, Key, title, body, now string) error`, `SaveBody(ctx, Key, body, now string) (Document, error)`, `ApplyCreated(ctx, Key, pageID, pushed string, version int, now string) error`, `ApplyPushed(ctx, Key, pushed string, version int, now string) error`, `ApplyPulled(ctx, Key, pageID, readBody, remote string, version int, now string) (bool, error)`, `ApplyConflict(ctx, Key, pageID, remote string, version int) error`, `MarkGone(ctx, Key) error`, `ResolveMine(ctx, Key, now string) error`, `ResolveTheirs(ctx, Key, now string) error`, `ForgetPage(ctx, Key, now string) error`, `DeleteDocument(ctx, Key) error`

- [ ] **Step 1: Write the failing tests**

```go
package ritualrepo_test

import (
	"context"
	"path/filepath"
	"testing"

	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/tamstore"
)

func newDocs(t *testing.T) (*ritualrepo.Repository, context.Context) {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return ritualrepo.New(db.DB()), context.Background()
}

var planning = ritualrepo.Key{ProfileID: "p1", BoardID: 1, SprintID: 14, RitualType: "planning"}

func mustDoc(t *testing.T, r *ritualrepo.Repository, ctx context.Context, k ritualrepo.Key) ritualrepo.Document {
	t.Helper()
	d, ok, err := r.Document(ctx, k)
	if err != nil || !ok {
		t.Fatalf("document %+v: ok=%v err=%v", k, ok, err)
	}
	return d
}

func TestWriteTemplateCreatesALocalRowAndLeavesAWrittenOneAlone(t *testing.T) {
	r, ctx := newDocs(t)
	if err := r.WriteTemplate(ctx, planning, "Sprint 14 · Planning", "<p>template</p>", "t1"); err != nil {
		t.Fatal(err)
	}
	d := mustDoc(t, r, ctx, planning)
	if d.Status != ritualrepo.StatusLocal || d.Body != "<p>template</p>" || d.BaseBody != "" || !d.Dirty() {
		t.Fatalf("fresh = %+v", d)
	}
	if _, err := r.SaveBody(ctx, planning, "<p>mine</p>", "t2"); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteTemplate(ctx, planning, "Sprint 14 · Planning", "<p>template again</p>", "t3"); err != nil {
		t.Fatal(err)
	}
	if got := mustDoc(t, r, ctx, planning).Body; got != "<p>mine</p>" {
		t.Fatalf("a written body was overwritten: %q", got)
	}
}

func TestNeedsTemplateFindsMissingAndEmptyRowsWithTheirLegacyText(t *testing.T) {
	r, ctx := newDocs(t)
	db := dbOf(t, r, ctx)
	for _, stmt := range []string{
		`INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, remark, issues_json)
		 VALUES ('p1', 1, 14, 'review', 'short sprint', '[{"key":"PLAT-1","remark":"demoed"}]')`,
		`INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, body) VALUES ('p1', 1, 14, 'retro', '<p>written</p>')`,
		`INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, confluence_page_id) VALUES ('p1', 1, 14, 'standup', '77')`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	need, err := r.NeedsTemplate(ctx, "p1", 1, 14, []string{"_sprint", "planning", "standup", "review", "retro"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ritualrepo.Legacy{}
	for _, l := range need {
		got[l.RitualType] = l
	}
	if len(got) != 3 {
		t.Fatalf("need = %+v", need)
	}
	if _, ok := got["retro"]; ok {
		t.Error("a row with a body needs no template")
	}
	if _, ok := got["standup"]; ok {
		t.Error("a row with a page id needs no template")
	}
	if l := got["review"]; l.Remark != "short sprint" || len(l.Issues) != 1 || l.Issues[0].Remark != "demoed" {
		t.Errorf("legacy review = %+v", l)
	}
}

func TestSaveBodyStatusFollowsThePageAndTheBase(t *testing.T) {
	r, ctx := newDocs(t)
	_ = r.WriteTemplate(ctx, planning, "T", "<p>a</p>", "t1")
	if d, _ := r.SaveBody(ctx, planning, "<p>b</p>", "t2"); d.Status != ritualrepo.StatusLocal {
		t.Fatalf("no page yet = %s", d.Status)
	}
	if err := r.ApplyCreated(ctx, planning, "42", "<p>b</p>", 1, "t3"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.Status != ritualrepo.StatusSynced || d.Dirty() || d.PageID != "42" {
		t.Fatalf("after create = %+v", d)
	}
	if d, _ := r.SaveBody(ctx, planning, "<p>c</p>", "t4"); d.Status != ritualrepo.StatusUnsynced || !d.Dirty() {
		t.Fatalf("edited = %+v", d)
	}
	if d, _ := r.SaveBody(ctx, planning, "<p>b</p>", "t5"); d.Status != ritualrepo.StatusSynced {
		t.Fatalf("undone back to base = %s", d.Status)
	}
	if _, err := r.SaveBody(ctx, ritualrepo.Key{ProfileID: "p1", BoardID: 1, SprintID: 99, RitualType: "planning"}, "x", "t6"); err == nil {
		t.Fatal("saving a row that does not exist should fail")
	}
}

func TestApplyPulledRefusesABodyThatChangedSinceItWasRead(t *testing.T) {
	r, ctx := newDocs(t)
	_ = r.WriteTemplate(ctx, planning, "T", "<p>read</p>", "t1")
	_, _ = r.SaveBody(ctx, planning, "<p>typed after the read</p>", "t2")
	pulled, err := r.ApplyPulled(ctx, planning, "42", "<p>read</p>", "<p>remote</p>", 2, "t3")
	if err != nil || pulled {
		t.Fatalf("pulled = %v, %v", pulled, err)
	}
	if got := mustDoc(t, r, ctx, planning).Body; got != "<p>typed after the read</p>" {
		t.Fatalf("a pull overwrote a newer save: %q", got)
	}
	pulled, _ = r.ApplyPulled(ctx, planning, "42", "<p>typed after the read</p>", "<p>remote</p>", 2, "t4")
	if d := mustDoc(t, r, ctx, planning); !pulled || d.Body != "<p>remote</p>" || d.BaseBody != "<p>remote</p>" || d.Version != 2 || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("clean pull = %+v", d)
	}
}

func TestApplyPushedLeavesAKeystrokeSavedMidPushUnsynced(t *testing.T) {
	r, ctx := newDocs(t)
	_ = r.WriteTemplate(ctx, planning, "T", "<p>v1</p>", "t1")
	_ = r.ApplyCreated(ctx, planning, "42", "<p>v1</p>", 1, "t2")
	_, _ = r.SaveBody(ctx, planning, "<p>pushed</p>", "t3")
	_, _ = r.SaveBody(ctx, planning, "<p>typed during the push</p>", "t4")
	if err := r.ApplyPushed(ctx, planning, "<p>pushed</p>", 2, "t5"); err != nil {
		t.Fatal(err)
	}
	d := mustDoc(t, r, ctx, planning)
	if d.Body != "<p>typed during the push</p>" || d.BaseBody != "<p>pushed</p>" || d.Version != 2 || d.Status != ritualrepo.StatusUnsynced {
		t.Fatalf("after push = %+v", d)
	}
}

func TestResolveMineRebasesAndTheirsTakesTheRemote(t *testing.T) {
	r, ctx := newDocs(t)
	setup := func() {
		_ = r.DeleteDocument(ctx, planning)
		_ = r.WriteTemplate(ctx, planning, "T", "<p>v1</p>", "t1")
		_ = r.ApplyCreated(ctx, planning, "42", "<p>v1</p>", 1, "t2")
		_, _ = r.SaveBody(ctx, planning, "<p>mine</p>", "t3")
		_ = r.ApplyConflict(ctx, planning, "42", "<p>theirs</p>", 3)
	}
	setup()
	if d := mustDoc(t, r, ctx, planning); d.Status != ritualrepo.StatusConflict || d.ConflictVersion != 3 {
		t.Fatalf("conflict = %+v", d)
	}
	if _, err := r.SaveBody(ctx, planning, "<p>mine, more</p>", "t4"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.Status != ritualrepo.StatusConflict {
		t.Fatalf("a save must not clear a conflict: %s", d.Status)
	}
	if err := r.ResolveMine(ctx, planning, "t5"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.Body != "<p>mine, more</p>" || d.BaseBody != "<p>theirs</p>" || d.Version != 3 || d.ConflictBody != "" || d.Status != ritualrepo.StatusUnsynced {
		t.Fatalf("mine = %+v", d)
	}
	if err := r.ResolveMine(ctx, planning, "t6"); err == nil {
		t.Fatal("resolving a row with no conflict should fail")
	}

	setup()
	if err := r.ResolveTheirs(ctx, planning, "t7"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.Body != "<p>theirs</p>" || d.BaseBody != "<p>theirs</p>" || d.Version != 3 || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("theirs = %+v", d)
	}
}

func TestForgetPageOnlyActsOnAGoneRow(t *testing.T) {
	r, ctx := newDocs(t)
	_ = r.WriteTemplate(ctx, planning, "T", "<p>v1</p>", "t1")
	_ = r.ApplyCreated(ctx, planning, "42", "<p>v1</p>", 1, "t2")
	if err := r.ForgetPage(ctx, planning, "t3"); err == nil {
		t.Fatal("forgetting a live page should fail")
	}
	_ = r.MarkGone(ctx, planning)
	if err := r.ForgetPage(ctx, planning, "t4"); err != nil {
		t.Fatal(err)
	}
	if d := mustDoc(t, r, ctx, planning); d.PageID != "" || d.Version != 0 || d.Body != "<p>v1</p>" || d.Status != ritualrepo.StatusLocal {
		t.Fatalf("forgotten = %+v", d)
	}
}

func TestListingByBoardSprintAndProfile(t *testing.T) {
	r, ctx := newDocs(t)
	for _, k := range []ritualrepo.Key{
		planning,
		{ProfileID: "p1", BoardID: 1, SprintID: 14, RitualType: "_sprint"},
		{ProfileID: "p1", BoardID: 1, SprintID: 12, RitualType: "retro"},
		{ProfileID: "p1", BoardID: 2, SprintID: 14, RitualType: "retro"},
	} {
		_ = r.WriteTemplate(ctx, k, "T", "<p/>", "t")
	}
	docs, _ := r.Documents(ctx, "p1", 1, 14)
	if len(docs) != 2 {
		t.Fatalf("sprint 14 on board 1 = %d", len(docs))
	}
	ids, _ := r.BoardSprintIDs(ctx, "p1", 1)
	if len(ids) != 2 || ids[0] != 12 || ids[1] != 14 {
		t.Fatalf("sprint ids = %v", ids)
	}
	_ = r.ApplyCreated(ctx, planning, "42", "<p/>", 1, "t")
	all, _ := r.ProfileDocuments(ctx, "p1")
	if len(all) != 4 {
		t.Fatalf("profile documents = %d", len(all))
	}
}
```

`dbOf` needs the handle; add to the same test file:

```go
func dbOf(t *testing.T, r *ritualrepo.Repository, ctx context.Context) interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
} {
	t.Helper()
	return r.DB()
}
```

and import `"database/sql"`. `Repository.DB()` is added in Step 3.

- [ ] **Step 2: Run to see it fail**

Run: `cd tam && go test ./internal/ritualrepo/ -run 'Template|SaveBody|Apply|Resolve|Forget|Listing'`
Expected: FAIL to compile.

- [ ] **Step 3: Write `documents.go`**

```go
package ritualrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// The statuses a ritual document moves through. Nothing in the sync reads
// status to decide what is dirty (Dirty compares bodies); status is what the
// view draws, written by whichever operation changed it.
const (
	StatusLocal    = "local"
	StatusSynced   = "synced"
	StatusUnsynced = "unsynced"
	StatusConflict = "conflict"
	StatusGone     = "gone"
)

// Key names one ritual document.
type Key struct {
	ProfileID  string
	BoardID    int
	SprintID   int
	RitualType string
}

func (k Key) args() []any {
	return []any{k.ProfileID, k.BoardID, k.SprintID, normalizeRitualType(k.RitualType)}
}

// Document is one ritual page as TAM keeps it. Body is the local page,
// BaseBody the page as of Version, the last remote TAM synced with, and
// ConflictBody a newer remote a Sync found while local edits were pending.
type Document struct {
	ProfileID       string `json:"profileId"`
	BoardID         int    `json:"boardId"`
	SprintID        int    `json:"sprintId"`
	RitualType      string `json:"ritualType"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	BaseBody        string `json:"baseBody"`
	PageID          string `json:"pageId"`
	Version         int    `json:"version"`
	ConflictBody    string `json:"conflictBody"`
	ConflictVersion int    `json:"conflictVersion"`
	Status          string `json:"status"`
	UpdatedAt       string `json:"updatedAt"`
	SyncedAt        string `json:"syncedAt"`
}

// Key is the document's own key.
func (d Document) Key() Key {
	return Key{ProfileID: d.ProfileID, BoardID: d.BoardID, SprintID: d.SprintID, RitualType: d.RitualType}
}

// Dirty is computed and never stored: a page Confluence has never seen, or a
// body that differs from the last one synced. A flag would drift from the
// bodies; this comparison cannot.
func (d Document) Dirty() bool { return d.PageID == "" || d.Body != d.BaseBody }

// Legacy is a document the ensure step has to render a template for, with
// whatever the retired wizard stored on it.
type Legacy struct {
	RitualType string
	Remark     string
	Issues     []Issue
}

// DB is the handle this repository runs on, for tests that need to write a
// row shape no method writes any more.
func (r *Repository) DB() *sql.DB { return r.db }

const documentColumns = `profile_id, board_id, sprint_id, ritual_type, title, body, base_body,
	confluence_page_id, confluence_version, conflict_body, conflict_version, status, updated_at, published_at`

const keyWhere = ` WHERE profile_id = ? AND board_id = ? AND sprint_id = ? AND ritual_type = ?`

type scanner interface{ Scan(...any) error }

func scanDocument(row scanner) (Document, error) {
	var d Document
	err := row.Scan(&d.ProfileID, &d.BoardID, &d.SprintID, &d.RitualType, &d.Title, &d.Body, &d.BaseBody,
		&d.PageID, &d.Version, &d.ConflictBody, &d.ConflictVersion, &d.Status, &d.UpdatedAt, &d.SyncedAt)
	return d, err
}

// Document reads one document; ok is false when none is stored.
func (r *Repository) Document(ctx context.Context, k Key) (Document, bool, error) {
	d, err := scanDocument(r.db.QueryRowContext(ctx, `SELECT `+documentColumns+` FROM ritual_document`+keyWhere, k.args()...))
	if errors.Is(err, sql.ErrNoRows) {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("read %s ritual for sprint %d: %w", k.RitualType, k.SprintID, err)
	}
	return d, true, nil
}

func (r *Repository) documents(ctx context.Context, where string, args ...any) ([]Document, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+documentColumns+` FROM ritual_document `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("list ritual documents: %w", err)
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ritual document: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Documents lists one sprint's documents on one board.
func (r *Repository) Documents(ctx context.Context, profileID string, boardID, sprintID int) ([]Document, error) {
	return r.documents(ctx, `WHERE profile_id = ? AND board_id = ? AND sprint_id = ? ORDER BY ritual_type`, profileID, boardID, sprintID)
}

// ProfileDocuments lists every document of a profile that has a page, which
// is what the demo space is rebuilt from when the app restarts.
func (r *Repository) ProfileDocuments(ctx context.Context, profileID string) ([]Document, error) {
	return r.documents(ctx, `WHERE profile_id = ? AND confluence_page_id <> '' ORDER BY board_id, sprint_id, ritual_type`, profileID)
}

// BoardSprintIDs lists the sprints of a board that hold any document, so a
// closed sprint that already has pages keeps syncing them.
func (r *Repository) BoardSprintIDs(ctx context.Context, profileID string, boardID int) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT sprint_id FROM ritual_document WHERE profile_id = ? AND board_id = ? ORDER BY sprint_id`, profileID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list ritual sprints for board %d: %w", boardID, err)
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// NeedsTemplate answers which of types have no usable document: no row, or a
// row with neither a body nor a page, which is every row written before the
// ritual sync existed. Each carries the retired wizard's remark and issues so
// the ensure step can keep what somebody typed. A malformed legacy column
// reads as no issues rather than failing, because it must not stop a sprint
// opening.
func (r *Repository) NeedsTemplate(ctx context.Context, profileID string, boardID, sprintID int, types []string) ([]Legacy, error) {
	out := []Legacy{}
	for _, t := range types {
		t = normalizeRitualType(t)
		var body, pageID, remark, issuesJSON string
		err := r.db.QueryRowContext(ctx, `SELECT body, confluence_page_id, remark, issues_json FROM ritual_document`+keyWhere,
			profileID, boardID, sprintID, t).Scan(&body, &pageID, &remark, &issuesJSON)
		if errors.Is(err, sql.ErrNoRows) {
			out = append(out, Legacy{RitualType: t, Issues: []Issue{}})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("check %s ritual for sprint %d: %w", t, sprintID, err)
		}
		if body != "" || pageID != "" {
			continue
		}
		issues, err := DecodeIssues(issuesJSON)
		if err != nil {
			issues = []Issue{}
		}
		out = append(out, Legacy{RitualType: t, Remark: remark, Issues: issues})
	}
	return out, nil
}

// WriteTemplate stores a rendered template as a local document. It never
// replaces a row that already has a body or a page.
func (r *Repository) WriteTemplate(ctx context.Context, k Key, title, body, now string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, title, body, base_body, status, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, '', 'local', ?)
		ON CONFLICT(profile_id, board_id, sprint_id, ritual_type) DO UPDATE SET
			title = excluded.title, body = excluded.body, base_body = '', status = 'local', updated_at = excluded.updated_at
		WHERE ritual_document.body = '' AND ritual_document.confluence_page_id = ''`,
		k.ProfileID, k.BoardID, k.SprintID, normalizeRitualType(k.RitualType), title, body, now)
	if err != nil {
		return fmt.Errorf("write %s template for sprint %d: %w", k.RitualType, k.SprintID, err)
	}
	return nil
}

func (r *Repository) update(ctx context.Context, what string, k Key, set string, setArgs []any, extraWhere string, extraArgs ...any) (int64, error) {
	args := append(append(setArgs, k.args()...), extraArgs...)
	res, err := r.db.ExecContext(ctx, `UPDATE ritual_document SET `+set+keyWhere+extraWhere, args...)
	if err != nil {
		return 0, fmt.Errorf("%s %s ritual for sprint %d: %w", what, k.RitualType, k.SprintID, err)
	}
	return res.RowsAffected()
}

// SaveBody is the editor's local save. It takes no lock: the sync's
// compare-and-set writes are what make a save during a pass safe. A row in
// conflict or gone keeps that status, since a save resolves neither.
func (r *Repository) SaveBody(ctx context.Context, k Key, body, now string) (Document, error) {
	n, err := r.update(ctx, "save", k, `body = ?, updated_at = ?,
		status = CASE
			WHEN status IN ('conflict', 'gone') THEN status
			WHEN confluence_page_id = '' THEN 'local'
			WHEN base_body = ? THEN 'synced'
			ELSE 'unsynced' END`, []any{body, now, body}, "")
	if err != nil {
		return Document{}, err
	}
	if n == 0 {
		return Document{}, fmt.Errorf("no %s ritual is stored for sprint %d", k.RitualType, k.SprintID)
	}
	d, _, err := r.Document(ctx, k)
	return d, err
}

// ApplyCreated records a page Sync created from pushed. body is left alone,
// so a save that landed while the create was in flight stays unsynced.
func (r *Repository) ApplyCreated(ctx context.Context, k Key, pageID, pushed string, version int, now string) error {
	_, err := r.update(ctx, "record created", k, `confluence_page_id = ?, base_body = ?, confluence_version = ?, published_at = ?,
		status = CASE WHEN body = ? THEN 'synced' ELSE 'unsynced' END`, []any{pageID, pushed, version, now, pushed}, "")
	return err
}

// ApplyPushed records a push of pushed at version, with ApplyCreated's rule
// for a save that landed during it.
func (r *Repository) ApplyPushed(ctx context.Context, k Key, pushed string, version int, now string) error {
	_, err := r.update(ctx, "record pushed", k, `base_body = ?, confluence_version = ?, published_at = ?,
		status = CASE WHEN body = ? THEN 'synced' ELSE 'unsynced' END`, []any{pushed, version, now, pushed}, "")
	return err
}

// ApplyPulled takes a remote body, but only while the local body is still
// readBody, the value the pass read. false means a save landed in between
// and nothing was written; the caller records a conflict instead.
func (r *Repository) ApplyPulled(ctx context.Context, k Key, pageID, readBody, remote string, version int, now string) (bool, error) {
	n, err := r.update(ctx, "record pulled", k, `confluence_page_id = ?, body = ?, base_body = ?, confluence_version = ?,
		conflict_body = '', conflict_version = 0, status = 'synced', published_at = ?`,
		[]any{pageID, remote, remote, version, now}, ` AND body = ?`, readBody)
	return n == 1, err
}

// ApplyConflict keeps a newer remote beside the local body. It never touches
// body, so it needs no compare-and-set.
func (r *Repository) ApplyConflict(ctx context.Context, k Key, pageID, remote string, version int) error {
	_, err := r.update(ctx, "record conflict", k, `confluence_page_id = ?, conflict_body = ?, conflict_version = ?, status = 'conflict'`,
		[]any{pageID, remote, version}, "")
	return err
}

// MarkGone records a page Confluence no longer answers for.
func (r *Repository) MarkGone(ctx context.Context, k Key) error {
	_, err := r.update(ctx, "mark gone", k, `status = 'gone'`, nil, "")
	return err
}

// ResolveMine rebases local edits onto the newer remote, so the next Sync
// pushes them over it. It is RebaseMoves for a page.
func (r *Repository) ResolveMine(ctx context.Context, k Key, now string) error {
	n, err := r.update(ctx, "keep mine", k, `base_body = conflict_body, confluence_version = conflict_version,
		conflict_body = '', conflict_version = 0, updated_at = ?,
		status = CASE WHEN body = conflict_body THEN 'synced' ELSE 'unsynced' END`, []any{now}, ` AND status = 'conflict'`)
	if err == nil && n == 0 {
		err = fmt.Errorf("the %s ritual for sprint %d has no conflict to resolve", k.RitualType, k.SprintID)
	}
	return err
}

// ResolveTheirs discards local edits for the newer remote.
func (r *Repository) ResolveTheirs(ctx context.Context, k Key, now string) error {
	n, err := r.update(ctx, "take theirs", k, `body = conflict_body, base_body = conflict_body, confluence_version = conflict_version,
		conflict_body = '', conflict_version = 0, updated_at = ?, status = 'synced'`, []any{now}, ` AND status = 'conflict'`)
	if err == nil && n == 0 {
		err = fmt.Errorf("the %s ritual for sprint %d has no conflict to resolve", k.RitualType, k.SprintID)
	}
	return err
}

// ForgetPage lets a gone page be created again on the next Sync, keeping the
// local body it will be created from.
func (r *Repository) ForgetPage(ctx context.Context, k Key, now string) error {
	n, err := r.update(ctx, "forget page", k, `confluence_page_id = '', confluence_version = 0, base_body = '',
		conflict_body = '', conflict_version = 0, updated_at = ?, status = 'local'`, []any{now}, ` AND status = 'gone'`)
	if err == nil && n == 0 {
		err = fmt.Errorf("the %s ritual for sprint %d is not gone from Confluence", k.RitualType, k.SprintID)
	}
	return err
}

// DeleteDocument removes the local copy only. A page in Confluence is left
// exactly where it is.
func (r *Repository) DeleteDocument(ctx context.Context, k Key) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ritual_document`+keyWhere, k.args()...)
	if err != nil {
		return fmt.Errorf("delete %s ritual for sprint %d: %w", k.RitualType, k.SprintID, err)
	}
	return nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `cd tam && go test ./internal/ritualrepo/ && go vet ./internal/ritualrepo/`
Expected: PASS, the old draft tests included.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/ritualrepo/documents.go tam/internal/ritualrepo/documents_test.go
git commit -m "feat(tam): a ritual page's local body, base and conflict"
```

---

### Task 6: The demo Confluence space

**Files:**
- Create: `tam/internal/demo/confluence.go`, `tam/internal/demo/confluence_test.go`

**Interfaces:**
- Consumes: `confluence.Pages`, `confluence.StoredPage`, `confluence.HTTPError` (Task 2).
- Produces:
  - `func NewConfluence(space, rootID string, stageConflict bool) *Confluence`, satisfying `confluence.Pages`
  - `(*Confluence) Seed(parentID, title, body string) string` (parent `""` = top level of the space)
  - `(*Confluence) Restore(id, parentID, title, body string, version int)`
  - `(*Confluence) EditRemote(id, body string)`, `Remove(id string)`, `Page(id string) (confluence.StoredPage, bool)`
  - `(*Confluence) FailNext(op, target string, err error)`; ops `"get"`, `"find"`, `"create"`, `"update"`; target = page id, or title for `find`/`create`
  - `(*Confluence) After(op, target string, fn func())`: runs `fn` once, after that call has computed its answer and released the lock, before it returns

- [ ] **Step 1: Write the failing tests**

```go
package demo

import (
	"context"
	"errors"
	"testing"

	"agile-suite/core/confluence"
)

func TestTheDemoSpaceTracksAncestorsAndRefusesDuplicateTitles(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", false)
	sprint, err := c.CreatePage(ctx, "DEMO", "root", "Sprint 14", "<p/>")
	if err != nil || sprint.Version != 1 {
		t.Fatalf("create = %+v, %v", sprint, err)
	}
	child, _ := c.CreatePage(ctx, "DEMO", sprint.ID, "Sprint 14 · Planning", "<p>plan</p>")
	found, ok, err := c.FindPageByTitle(ctx, "DEMO", "Sprint 14 · Planning")
	if err != nil || !ok || found.ID != child.ID || len(found.AncestorIDs) != 2 || found.AncestorIDs[0] != "root" || found.AncestorIDs[1] != sprint.ID {
		t.Fatalf("found = %+v", found)
	}
	if _, err := c.CreatePage(ctx, "DEMO", "root", "Sprint 14", "<p/>"); err == nil {
		t.Fatal("a duplicate title should be refused")
	}
	if _, err := c.CreatePage(ctx, "DEMO", "nope", "Orphan", "<p/>"); !errors.Is(err, confluence.ErrNotFound) {
		t.Fatalf("missing parent = %v", err)
	}
}

func TestTheDemoSpaceEnforcesVersionPlusOne(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", false)
	p, _ := c.CreatePage(ctx, "DEMO", "root", "Sprint 14", "<p>v1</p>")
	if _, err := c.UpdatePage(ctx, p.ID, "Sprint 14", "<p>stale</p>", 1); !errors.Is(err, confluence.ErrVersionConflict) {
		t.Fatalf("stale update = %v", err)
	}
	u, err := c.UpdatePage(ctx, p.ID, "Sprint 14", "<p>v2</p>", 2)
	if err != nil || u.Version != 2 || u.Body != "<p>v2</p>" {
		t.Fatalf("update = %+v, %v", u, err)
	}
	c.Remove(p.ID)
	if _, err := c.GetPageStorage(ctx, p.ID); !errors.Is(err, confluence.ErrNotFound) {
		t.Fatalf("removed = %v", err)
	}
}

func TestFailNextAndAfterFireOnce(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", false)
	boom := errors.New("boom")
	c.FailNext("get", "root", boom)
	if _, err := c.GetPageStorage(ctx, "root"); !errors.Is(err, boom) {
		t.Fatalf("first get = %v", err)
	}
	if _, err := c.GetPageStorage(ctx, "root"); err != nil {
		t.Fatalf("second get = %v", err)
	}
	calls := 0
	c.After("get", "root", func() { calls++ })
	_, _ = c.GetPageStorage(ctx, "root")
	_, _ = c.GetPageStorage(ctx, "root")
	if calls != 1 {
		t.Fatalf("after ran %d times", calls)
	}
}

func TestTheStagedConflictBumpsOnlyTheFirstStandup(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", true)
	first, _ := c.CreatePage(ctx, "DEMO", "root", "Sprint 14 · Standup", "<p>mine</p>")
	if first.Version != 1 {
		t.Fatalf("the create itself answers version 1, got %d", first.Version)
	}
	if p, _ := c.Page(first.ID); p.Version != 2 {
		t.Fatalf("the staged edit should leave version 2, got %d", p.Version)
	}
	second, _ := c.CreatePage(ctx, "DEMO", "root", "Sprint 15 · Standup", "<p>mine</p>")
	if p, _ := c.Page(second.ID); p.Version != 1 {
		t.Fatalf("only the first standup is staged, got %d", p.Version)
	}
}

func TestRestoreRebuildsAPageAndKeepsIdsUnique(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", false)
	c.Restore("1500", "root", "Sprint 14", "<p>kept</p>", 4)
	p, ok := c.Page("1500")
	if !ok || p.Version != 4 || p.Body != "<p>kept</p>" {
		t.Fatalf("restored = %+v", p)
	}
	next, _ := c.CreatePage(ctx, "DEMO", "root", "Sprint 15", "<p/>")
	if next.ID == "1500" {
		t.Fatal("a new page reused a restored id")
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd tam && go test ./internal/demo/ -run 'DemoSpace|FailNext|Staged|Restore'`
Expected: FAIL to compile.

- [ ] **Step 3: Write `confluence.go`**

```go
package demo

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"agile-suite/core/confluence"
)

// Confluence is an in-memory Confluence space. A demo profile's Rituals Sync
// runs against it, and so do ritualsync's tests, which is why it carries
// FailNext and After: the two ways a test makes Confluence misbehave at a
// precise moment.
//
// stageConflict bumps the first Standup page it creates straight after the
// create, with a paragraph "a teammate" added, so a demo user who edits the
// Standup and syncs again meets a conflict. It is the demo Commit's staged
// conflict on <project>-412, for pages.
type Confluence struct {
	mu       sync.Mutex
	space    string
	pages    map[string]*fakePage
	next     int
	failures map[string]error
	after    map[string]func()
	stage    bool
	staged   bool
}

type fakePage struct {
	id, parent, title, body string
	version                 int
}

var _ confluence.Pages = (*Confluence)(nil)

// DemoRootTitle is the demo root page's title.
const DemoRootTitle = "Team rituals"

// NewConfluence answers a space holding only its root page.
func NewConfluence(space, rootID string, stageConflict bool) *Confluence {
	c := &Confluence{space: space, pages: map[string]*fakePage{}, next: 1000,
		failures: map[string]error{}, after: map[string]func(){}, stage: stageConflict}
	c.pages[rootID] = &fakePage{id: rootID, title: DemoRootTitle, body: "<p>Ritual pages for this team.</p>", version: 1}
	return c
}

// FailNext makes the next op on target fail with err.
func (c *Confluence) FailNext(op, target string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures[op+"|"+target] = err
}

// After runs fn once, after the next op on target has computed its answer and
// before it returns it.
func (c *Confluence) After(op, target string, fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.after[op+"|"+target] = fn
}

func (c *Confluence) failure(op, target string) error {
	key := op + "|" + target
	err, ok := c.failures[key]
	if ok {
		delete(c.failures, key)
	}
	return err
}

func (c *Confluence) afterHook(op, target string) func() {
	key := op + "|" + target
	fn, ok := c.after[key]
	if !ok {
		return func() {}
	}
	delete(c.after, key)
	return fn
}

func (c *Confluence) stored(p *fakePage) confluence.StoredPage {
	ancestors := []string{}
	for parent := p.parent; parent != ""; {
		ancestors = append([]string{parent}, ancestors...)
		pp, ok := c.pages[parent]
		if !ok {
			break
		}
		parent = pp.parent
	}
	return confluence.StoredPage{ID: p.id, Title: p.title, Version: p.version, Body: p.body, AncestorIDs: ancestors}
}

func notFound() error {
	return &confluence.HTTPError{Code: http.StatusNotFound, Status: "404 Not Found", Message: "No content found with id"}
}

// GetPageStorage reads a page.
func (c *Confluence) GetPageStorage(_ context.Context, id string) (confluence.StoredPage, error) {
	c.mu.Lock()
	if err := c.failure("get", id); err != nil {
		c.mu.Unlock()
		return confluence.StoredPage{}, err
	}
	p, ok := c.pages[id]
	var out confluence.StoredPage
	if ok {
		out = c.stored(p)
	}
	hook := c.afterHook("get", id)
	c.mu.Unlock()
	hook()
	if !ok {
		return confluence.StoredPage{}, notFound()
	}
	return out, nil
}

// FindPageByTitle looks through the whole space, lowest id first.
func (c *Confluence) FindPageByTitle(_ context.Context, spaceKey, title string) (confluence.StoredPage, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.failure("find", title); err != nil {
		return confluence.StoredPage{}, false, err
	}
	if spaceKey != c.space {
		return confluence.StoredPage{}, false, nil
	}
	ids := make([]string, 0, len(c.pages))
	for id := range c.pages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if c.pages[id].title == title {
			return c.stored(c.pages[id]), true, nil
		}
	}
	return confluence.StoredPage{}, false, nil
}

func (c *Confluence) titleTaken(title string) bool {
	for _, p := range c.pages {
		if p.title == title {
			return true
		}
	}
	return false
}

// CreatePage creates a page under parentID.
func (c *Confluence) CreatePage(_ context.Context, spaceKey, parentID, title, body string) (confluence.StoredPage, error) {
	c.mu.Lock()
	if err := c.failure("create", title); err != nil {
		c.mu.Unlock()
		return confluence.StoredPage{}, err
	}
	if spaceKey != c.space {
		c.mu.Unlock()
		return confluence.StoredPage{}, notFound()
	}
	if c.titleTaken(title) {
		c.mu.Unlock()
		return confluence.StoredPage{}, &confluence.HTTPError{Code: http.StatusBadRequest, Status: "400 Bad Request",
			Message: "A page with this title already exists: A page already exists with the title " + title + " in the space with key " + spaceKey}
	}
	if _, ok := c.pages[parentID]; !ok {
		c.mu.Unlock()
		return confluence.StoredPage{}, notFound()
	}
	p := &fakePage{id: strconv.Itoa(c.next), parent: parentID, title: title, body: body, version: 1}
	c.next++
	c.pages[p.id] = p
	out := c.stored(p)
	if c.stage && !c.staged && strings.HasSuffix(title, " · Standup") {
		c.staged = true
		p.version = 2
		p.body += "<p>Added in Confluence by a teammate: the payment sandbox is down until Thursday.</p>"
	}
	hook := c.afterHook("create", title)
	c.mu.Unlock()
	hook()
	return out, nil
}

// UpdatePage replaces a page, refusing any version but current plus one.
func (c *Confluence) UpdatePage(_ context.Context, id, title, body string, version int) (confluence.StoredPage, error) {
	c.mu.Lock()
	if err := c.failure("update", id); err != nil {
		c.mu.Unlock()
		return confluence.StoredPage{}, err
	}
	p, ok := c.pages[id]
	if !ok {
		c.mu.Unlock()
		return confluence.StoredPage{}, notFound()
	}
	if version != p.version+1 {
		current := p.version
		c.mu.Unlock()
		return confluence.StoredPage{}, &confluence.HTTPError{Code: http.StatusConflict, Status: "409 Conflict",
			Message: "Version must be incremented on update. Current version is: " + strconv.Itoa(current)}
	}
	p.title, p.body, p.version = title, body, version
	out := c.stored(p)
	hook := c.afterHook("update", id)
	c.mu.Unlock()
	hook()
	return out, nil
}

// Seed adds a page as somebody writing in Confluence would, with no failure
// or staging applied. parentID "" puts it at the top of the space.
func (c *Confluence) Seed(parentID, title, body string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := &fakePage{id: strconv.Itoa(c.next), parent: parentID, title: title, body: body, version: 1}
	c.next++
	c.pages[p.id] = p
	return p.id
}

// Restore puts back a page the app knew about before a restart emptied this
// space. A restored Standup counts as already staged.
func (c *Confluence) Restore(id, parentID, title, body string, version int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pages[id] = &fakePage{id: id, parent: parentID, title: title, body: body, version: version}
	if n, err := strconv.Atoi(id); err == nil && n >= c.next {
		c.next = n + 1
	}
	if strings.HasSuffix(title, " · Standup") {
		c.staged = true
	}
}

// EditRemote changes a page the way a teammate editing it in Confluence would.
func (c *Confluence) EditRemote(id, body string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.pages[id]; ok {
		p.body = body
		p.version++
	}
}

// Remove deletes a page from the space.
func (c *Confluence) Remove(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pages, id)
}

// Page reads a page without any failure or hook applying.
func (c *Confluence) Page(id string) (confluence.StoredPage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.pages[id]
	if !ok {
		return confluence.StoredPage{}, false
	}
	return c.stored(p), true
}
```

- [ ] **Step 4: Run the tests**

Run: `cd tam && go test ./internal/demo/ && go vet ./internal/demo/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/demo/confluence.go tam/internal/demo/confluence_test.go
git commit -m "feat(tam): an in-memory Confluence space for the demo and the sync tests"
```

---

### Task 7: The ensure step and the sprint selection

**Files:**
- Create: `tam/internal/ritualsync/ensure.go`, `tam/internal/ritualsync/harness_test.go`, `tam/internal/ritualsync/ensure_test.go`

**Interfaces:**
- Consumes: `ritualtemplate` (Task 4), `ritualrepo` documents API (Task 5), `demo.Confluence` (Task 6, harness only), `boardrepo.Sprint`.
- Produces:
  - `type Sprint struct { Info ritualtemplate.SprintInfo; State string }`
  - `func Info(s boardrepo.Sprint, boardName string) ritualtemplate.SprintInfo`
  - `func Sprints(cached []boardrepo.Sprint, boardName string, withRows []int) []Sprint`
  - `func Ensure(ctx context.Context, docs *ritualrepo.Repository, profileID string, boardID int, s Sprint, loc *time.Location, now time.Time) error`

- [ ] **Step 1: Write the test harness**

`harness_test.go` (shared by Tasks 7 to 9):

```go
package ritualsync

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/tam/internal/demo"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
	"agile-suite/tam/internal/tamstore"
)

const (
	testProfile = "p1"
	testBoard   = 1
)

var sprint14 = Sprint{State: "active", Info: ritualtemplate.SprintInfo{
	ID: 14, Name: "Sprint 14", Goal: "Ship promo codes",
	StartDate: "2026-09-14T09:00:00.000+0000", EndDate: "2026-09-25T17:00:00.000+0000", BoardName: "PLAT board",
}}

type harness struct {
	t    *testing.T
	ctx  context.Context
	docs *ritualrepo.Repository
	fake *demo.Confluence
	cfg  Config
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &harness{
		t: t, ctx: context.Background(), docs: ritualrepo.New(db.DB()),
		fake: demo.NewConfluence("PLAT", "root", false),
		cfg: Config{SpaceKey: "PLAT", RootID: "root", Location: time.UTC,
			Now: func() time.Time { return time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC) }},
	}
}

func (h *harness) key(sprintID int, ritualType string) ritualrepo.Key {
	return ritualrepo.Key{ProfileID: testProfile, BoardID: testBoard, SprintID: sprintID, RitualType: ritualType}
}

func (h *harness) doc(sprintID int, ritualType string) ritualrepo.Document {
	h.t.Helper()
	d, ok, err := h.docs.Document(h.ctx, h.key(sprintID, ritualType))
	if err != nil || !ok {
		h.t.Fatalf("document %d %s: ok=%v err=%v", sprintID, ritualType, ok, err)
	}
	return d
}

func (h *harness) ensure(s Sprint) {
	h.t.Helper()
	if err := Ensure(h.ctx, h.docs, testProfile, testBoard, s, time.UTC, h.cfg.Now()); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) save(sprintID int, ritualType, body string) {
	h.t.Helper()
	if _, err := h.docs.SaveBody(h.ctx, h.key(sprintID, ritualType), body, "t"); err != nil {
		h.t.Fatal(err)
	}
}
```

`Config` and `Run` do not exist until Task 8. For this task, add a minimal `Config` to `ensure.go` in Step 3 so the harness compiles; Task 8 extends the same type.

- [ ] **Step 2: Write the failing tests** (`ensure_test.go`)

```go
package ritualsync

import (
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

func TestEnsureWritesTheFiveTemplatesLocally(t *testing.T) {
	h := newHarness(t)
	h.ensure(sprint14)
	docs, _ := h.docs.Documents(h.ctx, testProfile, testBoard, 14)
	if len(docs) != 5 {
		t.Fatalf("documents = %d", len(docs))
	}
	for _, typ := range ritualtemplate.Types {
		d := h.doc(14, typ)
		if d.Status != ritualrepo.StatusLocal || d.Title != ritualtemplate.Title(typ, sprint14.Info) ||
			d.Body != ritualtemplate.Render(typ, sprint14.Info, time.UTC) {
			t.Errorf("%s = %+v", typ, d)
		}
	}
}

func TestEnsureTwiceLeavesAnEditedPageAlone(t *testing.T) {
	h := newHarness(t)
	h.ensure(sprint14)
	h.save(14, "planning", "<p>mine</p>")
	h.ensure(sprint14)
	if got := h.doc(14, "planning").Body; got != "<p>mine</p>" {
		t.Fatalf("planning = %q", got)
	}
}

func TestEnsureCarriesTheWizardsRemarksForward(t *testing.T) {
	h := newHarness(t)
	if _, err := h.docs.DB().ExecContext(h.ctx, `INSERT INTO ritual_document
		(profile_id, board_id, sprint_id, ritual_type, remark, issues_json)
		VALUES ('p1', 1, 14, 'review', 'short sprint', '[{"key":"PLAT-1","remark":"demoed"}]')`); err != nil {
		t.Fatal(err)
	}
	h.ensure(sprint14)
	body := h.doc(14, "review").Body
	if !strings.HasPrefix(body, ritualtemplate.Render("review", sprint14.Info, time.UTC)) ||
		!strings.HasSuffix(body, "<h2>Earlier draft notes</h2><p>short sprint</p><ul><li>PLAT-1: demoed</li></ul>") {
		t.Fatalf("review = %s", body)
	}
}

func TestEnsureGivesAClosedSprintNothing(t *testing.T) {
	h := newHarness(t)
	closed := sprint14
	closed.State = "closed"
	h.ensure(closed)
	if docs, _ := h.docs.Documents(h.ctx, testProfile, testBoard, 14); len(docs) != 0 {
		t.Fatalf("a closed sprint got %d documents", len(docs))
	}
}

func TestSprintsCoversOpenSprintsAndClosedOnesThatHavePages(t *testing.T) {
	cached := []boardrepo.Sprint{
		{ID: 14, Name: "Sprint 14", State: "active"},
		{ID: 15, Name: "Sprint 15", State: "future"},
		{ID: 12, Name: "Sprint 12", State: "closed"},
		{ID: 11, Name: "Sprint 11", State: "closed"},
	}
	got := Sprints(cached, "PLAT board", []int{12})
	var ids []int
	for _, s := range got {
		ids = append(ids, s.Info.ID)
		if s.Info.BoardName != "PLAT board" {
			t.Errorf("board name = %q", s.Info.BoardName)
		}
	}
	if len(ids) != 3 || ids[0] != 14 || ids[1] != 15 || ids[2] != 12 {
		t.Fatalf("sprints = %v", ids)
	}
}
```

- [ ] **Step 3: Write `ensure.go`**

```go
// Package ritualsync keeps a board's ritual pages in step with Confluence.
// Ensure writes a sprint's pages locally from templates, with no network,
// and Run is the Sync pass: create, adopt, pull, push, and record conflicts.
// internal/ritualtemplate stays pure and internal/ritualrepo stays storage;
// every decision about what a Sync does to a page lives here.
package ritualsync

import (
	"context"
	"time"

	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

// Config is what a pass needs besides its sprints: where the pages live, and
// the clock and zone the templates and timestamps read.
type Config struct {
	SpaceKey string
	RootID   string
	Location *time.Location
	Now      func() time.Time
}

// Sprint is one sprint a pass covers.
type Sprint struct {
	Info  ritualtemplate.SprintInfo
	State string
}

// Info is a cached sprint in the shape the templates read.
func Info(s boardrepo.Sprint, boardName string) ritualtemplate.SprintInfo {
	return ritualtemplate.SprintInfo{ID: s.ID, Name: s.Name, Goal: s.Goal, StartDate: s.StartDate, EndDate: s.EndDate, BoardName: boardName}
}

// Sprints picks the sprints a pass covers, in the cache's own order: every
// active or future sprint, and a closed one only when it already holds
// documents. A closed sprint never gets new pages from a template, and a
// first Sync must not write five pages for every sprint in a board's history.
func Sprints(cached []boardrepo.Sprint, boardName string, withRows []int) []Sprint {
	has := map[int]bool{}
	for _, id := range withRows {
		has[id] = true
	}
	out := []Sprint{}
	for _, s := range cached {
		if s.State == "closed" && !has[s.ID] {
			continue
		}
		out = append(out, Sprint{Info: Info(s, boardName), State: s.State})
	}
	return out
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// Ensure writes whichever of a sprint's five documents are missing, each
// rendered from its template, locally and with no network call. A row the
// retired wizard left with a remark gets that remark carried onto its page.
// A closed sprint is left as it is.
func Ensure(ctx context.Context, docs *ritualrepo.Repository, profileID string, boardID int, s Sprint, loc *time.Location, now time.Time) error {
	if s.State == "closed" {
		return nil
	}
	need, err := docs.NeedsTemplate(ctx, profileID, boardID, s.Info.ID, ritualtemplate.Types)
	if err != nil {
		return err
	}
	for _, l := range need {
		notes := make([]ritualtemplate.Note, len(l.Issues))
		for i, issue := range l.Issues {
			notes[i] = ritualtemplate.Note{Key: issue.Key, Remark: issue.Remark}
		}
		body := ritualtemplate.Render(l.RitualType, s.Info, loc) + ritualtemplate.EarlierNotes(l.Remark, notes)
		k := ritualrepo.Key{ProfileID: profileID, BoardID: boardID, SprintID: s.Info.ID, RitualType: l.RitualType}
		if err := docs.WriteTemplate(ctx, k, ritualtemplate.Title(l.RitualType, s.Info), body, stamp(now)); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd tam && go test ./internal/ritualsync/ && go vet ./internal/ritualsync/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/ritualsync
git commit -m "feat(tam): a sprint's ritual pages written locally from templates"
```

---

### Task 8: The Sync pass, placing pages

**Files:**
- Create: `tam/internal/ritualsync/run.go`, `tam/internal/ritualsync/run_test.go`

**Interfaces:**
- Consumes: Tasks 2, 5, 6, 7.
- Produces:
  - `type PageFailure struct { SprintName, Title, Reason string }` (JSON `sprintName`, `title`, `reason`)
  - `type Result struct { Created, Pulled, Pushed, Conflicts, Gone int; Failed []PageFailure; SyncedAt string }` (JSON `created, pulled, pushed, conflicts, gone, failed, syncedAt`)
  - `func Run(ctx context.Context, pages confluence.Pages, docs *ritualrepo.Repository, cfg Config, profileID string, boardID int, sprints []Sprint) (Result, error)`: the Go error is only ever the unreadable root, a cancelled context, or the local store failing.

- [ ] **Step 1: Write the failing tests** (`run_test.go`, placement half)

```go
package ritualsync

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"agile-suite/core/confluence"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

func (h *harness) run(sprints ...Sprint) Result {
	h.t.Helper()
	res, err := Run(h.ctx, h.fake, h.docs, h.cfg, testProfile, testBoard, sprints)
	if err != nil {
		h.t.Fatalf("run: %v", err)
	}
	return res
}

func TestFirstSyncCreatesTheSprintPageThenItsRituals(t *testing.T) {
	h := newHarness(t)
	res := h.run(sprint14)
	if res.Created != 5 || len(res.Failed) != 0 || res.SyncedAt == "" {
		t.Fatalf("result = %+v", res)
	}
	overview := h.doc(14, ritualtemplate.Sprint)
	page, _ := h.fake.Page(overview.PageID)
	if page.Title != "Sprint 14" || len(page.AncestorIDs) != 1 || page.AncestorIDs[0] != "root" {
		t.Fatalf("overview page = %+v", page)
	}
	for _, typ := range ritualtemplate.Types[1:] {
		d := h.doc(14, typ)
		p, ok := h.fake.Page(d.PageID)
		if !ok || p.AncestorIDs[len(p.AncestorIDs)-1] != overview.PageID {
			t.Errorf("%s is not under its sprint page: %+v", typ, p)
		}
		if d.Status != ritualrepo.StatusSynced || d.Version != 1 || d.BaseBody != d.Body {
			t.Errorf("%s = %+v", typ, d)
		}
	}
}

func TestAPageAlreadyUnderTheRootIsAdoptedNotDuplicated(t *testing.T) {
	h := newHarness(t)
	id := h.fake.Seed("root", "Sprint 14", "<p>written by hand</p>")
	res := h.run(sprint14)
	if res.Created != 4 || res.Pulled != 1 {
		t.Fatalf("result = %+v", res)
	}
	d := h.doc(14, ritualtemplate.Sprint)
	if d.PageID != id || d.Body != "<p>written by hand</p>" || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("overview = %+v", d)
	}
}

func TestAdoptingOverLocalEditsIsAConflict(t *testing.T) {
	h := newHarness(t)
	h.ensure(sprint14)
	h.save(14, ritualtemplate.Sprint, "<p>mine</p>")
	id := h.fake.Seed("root", "Sprint 14", "<p>theirs</p>")
	res := h.run(sprint14)
	d := h.doc(14, ritualtemplate.Sprint)
	if res.Conflicts != 1 || d.Status != ritualrepo.StatusConflict || d.PageID != id ||
		d.Body != "<p>mine</p>" || d.ConflictBody != "<p>theirs</p>" {
		t.Fatalf("result = %+v, overview = %+v", res, d)
	}
	// The rituals still land under the adopted page.
	if res.Created != 4 {
		t.Fatalf("created = %d", res.Created)
	}
}

func TestATitleTakenOutsideTheRootIsRefusedAndItsRitualsWait(t *testing.T) {
	h := newHarness(t)
	h.fake.Seed("", "Sprint 14", "<p>another team</p>")
	res := h.run(sprint14)
	if res.Created != 0 || len(res.Failed) != 1 {
		t.Fatalf("result = %+v", res)
	}
	want := `A page titled "Sprint 14" already exists outside the rituals root. Rename one of them.`
	if f := res.Failed[0]; f.Reason != want || f.Title != "Sprint 14" || f.SprintName != "Sprint 14" {
		t.Fatalf("failure = %+v", f)
	}
	if d := h.doc(14, "planning"); d.PageID != "" {
		t.Fatalf("a ritual was placed without its sprint page: %+v", d)
	}
}

func TestClosedSprintsGetNoNewPages(t *testing.T) {
	h := newHarness(t)
	closed := sprint14
	closed.State = "closed"
	if res := h.run(closed); res.Created != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestAnUnreadableRootRefusesThePass(t *testing.T) {
	h := newHarness(t)
	h.fake.FailNext("get", "root", &confluence.HTTPError{Code: http.StatusForbidden, Status: "403 Forbidden"})
	_, err := Run(h.ctx, h.fake, h.docs, h.cfg, testProfile, testBoard, []Sprint{sprint14})
	if err == nil || !strings.HasPrefix(err.Error(), "The Confluence root page root could not be read: ") {
		t.Fatalf("err = %v", err)
	}
	if docs, _ := h.docs.Documents(h.ctx, testProfile, testBoard, 14); len(docs) != 0 {
		t.Fatal("a refused pass should not have started")
	}
}

func TestAFailedCreateIsReportedAndTheRestContinue(t *testing.T) {
	h := newHarness(t)
	h.fake.FailNext("create", "Sprint 14 · Review", errors.New("boom"))
	res := h.run(sprint14)
	if res.Created != 4 || len(res.Failed) != 1 || res.Failed[0].Reason != "boom" || res.Failed[0].Title != "Sprint 14 · Review" {
		t.Fatalf("result = %+v", res)
	}
	if d := h.doc(14, "review"); d.PageID != "" || d.Status != ritualrepo.StatusLocal {
		t.Fatalf("review = %+v", d)
	}
}
```

- [ ] **Step 2: Run to see it fail**

Run: `cd tam && go test ./internal/ritualsync/`
Expected: FAIL to compile, `Run` undefined.

- [ ] **Step 3: Write `run.go`**

```go
package ritualsync

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"agile-suite/core/confluence"
	"agile-suite/tam/internal/errtext"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

// PageFailure is one page a pass could not bring into step, with the reason
// in one readable line.
type PageFailure struct {
	SprintName string `json:"sprintName"`
	Title      string `json:"title"`
	Reason     string `json:"reason"`
}

// Result is what a pass did. It travels as a value, not a Go error, because
// Wails fills in a bound method's value or its error and never both: a pass
// that created four pages and failed one has to deliver all five facts.
type Result struct {
	Created   int           `json:"created"`
	Pulled    int           `json:"pulled"`
	Pushed    int           `json:"pushed"`
	Conflicts int           `json:"conflicts"`
	Gone      int           `json:"gone"`
	Failed    []PageFailure `json:"failed"`
	SyncedAt  string        `json:"syncedAt"`
}

type pass struct {
	ctx   context.Context
	pages confluence.Pages
	docs  *ritualrepo.Repository
	cfg   Config
	res   *Result
}

func (p *pass) now() string { return stamp(p.cfg.Now()) }

func (p *pass) fail(sp Sprint, d ritualrepo.Document, reason string) {
	p.res.Failed = append(p.res.Failed, PageFailure{SprintName: sp.Info.Name, Title: d.Title, Reason: reason})
}

// Run is one Sync pass over a board's sprints. Every write it makes to a
// document is either blind to body or conditional on body still holding what
// the pass read, so a save made while the pass runs is never overwritten and
// never claimed as synced.
func Run(ctx context.Context, pages confluence.Pages, docs *ritualrepo.Repository, cfg Config, profileID string, boardID int, sprints []Sprint) (Result, error) {
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	res := Result{Failed: []PageFailure{}}
	if _, err := pages.GetPageStorage(ctx, cfg.RootID); err != nil {
		return res, fmt.Errorf("The Confluence root page %s could not be read: %s", cfg.RootID, errtext.Line(err))
	}
	p := &pass{ctx: ctx, pages: pages, docs: docs, cfg: cfg, res: &res}
	for _, sp := range sprints {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if err := Ensure(ctx, docs, profileID, boardID, sp, cfg.Location, cfg.Now()); err != nil {
			return res, err
		}
		list, err := docs.Documents(ctx, profileID, boardID, sp.Info.ID)
		if err != nil {
			return res, err
		}
		byType := map[string]ritualrepo.Document{}
		for _, d := range list {
			byType[d.RitualType] = d
		}
		overview, ok := byType[ritualtemplate.Sprint]
		if !ok {
			// A closed sprint whose overview was removed locally has nothing
			// its rituals could be created under.
			continue
		}
		parentID, ok, err := p.page(sp, overview, cfg.RootID)
		if err != nil {
			return res, err
		}
		if !ok {
			continue
		}
		for _, t := range ritualtemplate.Types[1:] {
			d, ok := byType[t]
			if !ok {
				continue
			}
			if _, _, err := p.page(sp, d, parentID); err != nil {
				return res, err
			}
		}
	}
	res.SyncedAt = stamp(cfg.Now())
	return res, nil
}

// page brings one document and its Confluence page into step and answers
// with the page id children hang under. ok is false when there is no page
// to create children beneath: a refused title, a failed create, or a page
// gone from Confluence. The error is only ever the local store's; everything
// Confluence does wrong is recorded in the result instead.
func (p *pass) page(sp Sprint, d ritualrepo.Document, parentID string) (string, bool, error) {
	if d.Status == ritualrepo.StatusGone {
		return "", false, nil
	}
	if d.PageID == "" {
		return p.place(sp, d, parentID)
	}
	return p.reconcile(sp, d)
}

// place finds a home for a document with no page: adopt a page of the same
// title under parentID, refuse one anywhere else in the space, or create it.
func (p *pass) place(sp Sprint, d ritualrepo.Document, parentID string) (string, bool, error) {
	k := d.Key()
	found, exists, err := p.pages.FindPageByTitle(p.ctx, p.cfg.SpaceKey, d.Title)
	if err != nil {
		p.fail(sp, d, errtext.Line(err))
		return "", false, nil
	}
	if !exists {
		created, err := p.pages.CreatePage(p.ctx, p.cfg.SpaceKey, parentID, d.Title, d.Body)
		if err != nil {
			p.fail(sp, d, errtext.Line(err))
			return "", false, nil
		}
		if err := p.docs.ApplyCreated(p.ctx, k, created.ID, d.Body, created.Version, p.now()); err != nil {
			return "", false, err
		}
		p.res.Created++
		return created.ID, true, nil
	}
	if !slices.Contains(found.AncestorIDs, parentID) {
		p.fail(sp, d, fmt.Sprintf("A page titled %q already exists outside the rituals root. Rename one of them.", d.Title))
		return "", false, nil
	}
	// An untouched template takes the page somebody already wrote. Anything
	// else is somebody's text on both sides, and the user decides.
	if d.Body == ritualtemplate.Render(d.RitualType, sp.Info, p.cfg.Location) {
		pulled, err := p.docs.ApplyPulled(p.ctx, k, found.ID, d.Body, found.Body, found.Version, p.now())
		if err != nil {
			return "", false, err
		}
		if pulled {
			p.res.Pulled++
			return found.ID, true, nil
		}
	}
	if err := p.docs.ApplyConflict(p.ctx, k, found.ID, found.Body, found.Version); err != nil {
		return "", false, err
	}
	p.res.Conflicts++
	return found.ID, true, nil
}

// reconcile is Task 9's half: a document that already has a page. Until then
// it leaves the page alone, which is what an unchanged page wants anyway.
func (p *pass) reconcile(sp Sprint, d ritualrepo.Document) (string, bool, error) {
	_ = errors.Is
	return d.PageID, true, nil
}
```

(The `_ = errors.Is` line keeps the import used until Task 9 replaces `reconcile`; Task 9 removes it.)

- [ ] **Step 4: Run the tests**

Run: `cd tam && go test ./internal/ritualsync/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/ritualsync
git commit -m "feat(tam): a ritual sync that creates and adopts a sprint's pages"
```

---

### Task 9: The Sync pass, pages that already exist

**Files:**
- Modify: `tam/internal/ritualsync/run.go` (replace `reconcile`, add `pull` and `push`)
- Modify: `tam/internal/ritualsync/run_test.go` (append)

**Interfaces:**
- Consumes: Task 8's `pass`, Task 6's `After`/`FailNext`/`EditRemote`/`Remove`.
- Produces: no new exported names.

- [ ] **Step 1: Write the failing tests** (append to `run_test.go`)

```go
func TestAnUntouchedSyncedPageIsLeftAlone(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	res := h.run(sprint14)
	if res.Created+res.Pulled+res.Pushed+res.Conflicts+res.Gone != 0 || len(res.Failed) != 0 {
		t.Fatalf("second pass = %+v", res)
	}
}

func TestALocalEditIsPushedAtTheNextVersion(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	h.save(14, "planning", "<p>plan</p>")
	res := h.run(sprint14)
	d := h.doc(14, "planning")
	page, _ := h.fake.Page(d.PageID)
	if res.Pushed != 1 || page.Body != "<p>plan</p>" || page.Version != 2 ||
		d.Version != 2 || d.BaseBody != "<p>plan</p>" || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("result = %+v, doc = %+v, page = %+v", res, d, page)
	}
}

func TestARemoteEditIsPulledIntoACleanPage(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "retro").PageID
	h.fake.EditRemote(id, "<p>theirs</p>")
	res := h.run(sprint14)
	if d := h.doc(14, "retro"); res.Pulled != 1 || d.Body != "<p>theirs</p>" || d.Version != 2 || d.Status != ritualrepo.StatusSynced {
		t.Fatalf("result = %+v, doc = %+v", res, d)
	}
}

func TestARemoteEditAgainstLocalEditsIsAConflictAndIsNeverPushed(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "retro").PageID
	h.fake.EditRemote(id, "<p>theirs</p>")
	h.save(14, "retro", "<p>mine</p>")
	if res := h.run(sprint14); res.Conflicts != 1 {
		t.Fatalf("result = %+v", res)
	}
	h.fake.EditRemote(id, "<p>theirs again</p>")
	res := h.run(sprint14)
	d := h.doc(14, "retro")
	page, _ := h.fake.Page(id)
	if res.Conflicts != 1 || res.Pushed != 0 || d.ConflictVersion != 3 || d.ConflictBody != "<p>theirs again</p>" ||
		d.Body != "<p>mine</p>" || page.Body != "<p>theirs again</p>" {
		t.Fatalf("result = %+v, doc = %+v, page = %+v", res, d, page)
	}
	// Keep mine, and the next pass pushes over the newer version.
	if err := h.docs.ResolveMine(h.ctx, h.key(14, "retro"), "t"); err != nil {
		t.Fatal(err)
	}
	if res := h.run(sprint14); res.Pushed != 1 {
		t.Fatalf("after keep mine = %+v", res)
	}
	if page, _ := h.fake.Page(id); page.Body != "<p>mine</p>" || page.Version != 4 {
		t.Fatalf("page = %+v", page)
	}
}

func TestAPageDeletedInConfluenceGoesGoneAndIsNotRecreated(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	h.fake.Remove(h.doc(14, "review").PageID)
	if res := h.run(sprint14); res.Gone != 1 || h.doc(14, "review").Status != ritualrepo.StatusGone {
		t.Fatalf("result = %+v", res)
	}
	if res := h.run(sprint14); res.Gone != 0 || res.Created != 0 {
		t.Fatalf("a gone page must be skipped, not recreated: %+v", res)
	}
}

func TestAVersionRaceOnPushBecomesAConflict(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "planning").PageID
	h.save(14, "planning", "<p>mine</p>")
	// The teammate saves after the pass read version 1 and before it pushes.
	h.fake.After("get", id, func() { h.fake.EditRemote(id, "<p>raced</p>") })
	res := h.run(sprint14)
	d := h.doc(14, "planning")
	if res.Conflicts != 1 || res.Pushed != 0 || d.Status != ritualrepo.StatusConflict || d.ConflictBody != "<p>raced</p>" {
		t.Fatalf("result = %+v, doc = %+v", res, d)
	}
}

func TestASaveDuringAPushStaysUnsynced(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "planning").PageID
	h.save(14, "planning", "<p>pushed</p>")
	h.fake.After("update", id, func() { h.save(14, "planning", "<p>typed during the push</p>") })
	res := h.run(sprint14)
	d := h.doc(14, "planning")
	if res.Pushed != 1 || d.Body != "<p>typed during the push</p>" || d.BaseBody != "<p>pushed</p>" || d.Status != ritualrepo.StatusUnsynced {
		t.Fatalf("result = %+v, doc = %+v", res, d)
	}
	if res := h.run(sprint14); res.Pushed != 1 {
		t.Fatalf("the next pass should push what was typed: %+v", res)
	}
}

func TestASaveDuringAPullBecomesAConflictNotAnOverwrite(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	id := h.doc(14, "retro").PageID
	h.fake.EditRemote(id, "<p>theirs</p>")
	h.fake.After("get", id, func() { h.save(14, "retro", "<p>typed during the pull</p>") })
	res := h.run(sprint14)
	d := h.doc(14, "retro")
	if res.Conflicts != 1 || d.Body != "<p>typed during the pull</p>" || d.ConflictBody != "<p>theirs</p>" {
		t.Fatalf("result = %+v, doc = %+v", res, d)
	}
}

func TestOneFailingPageLeavesTheOthersSynced(t *testing.T) {
	h := newHarness(t)
	h.run(sprint14)
	h.save(14, "planning", "<p>plan</p>")
	h.save(14, "review", "<p>review</p>")
	h.fake.FailNext("update", h.doc(14, "planning").PageID, errors.New("boom"))
	res := h.run(sprint14)
	if res.Pushed != 1 || len(res.Failed) != 1 || res.Failed[0].Title != "Sprint 14 · Planning" || res.Failed[0].Reason != "boom" {
		t.Fatalf("result = %+v", res)
	}
	if h.doc(14, "planning").Status != ritualrepo.StatusUnsynced || h.doc(14, "review").Status != ritualrepo.StatusSynced {
		t.Fatal("statuses after a partial pass are wrong")
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd tam && go test ./internal/ritualsync/ -run 'Untouched|LocalEdit|RemoteEdit|Deleted|Race|During|OneFailing'`
Expected: FAIL (the placeholder `reconcile` pushes and pulls nothing).

- [ ] **Step 3: Replace `reconcile`, add `pull` and `push`** in `run.go`

Delete the placeholder `reconcile` (and its `_ = errors.Is` line), and add:

```go
// reconcile brings a document that already has a page into step. The table
// in section 2 of the design is this switch, row for row.
func (p *pass) reconcile(sp Sprint, d ritualrepo.Document) (string, bool, error) {
	k := d.Key()
	remote, err := p.pages.GetPageStorage(p.ctx, d.PageID)
	if errors.Is(err, confluence.ErrNotFound) {
		if err := p.docs.MarkGone(p.ctx, k); err != nil {
			return "", false, err
		}
		p.res.Gone++
		return "", false, nil
	}
	if err != nil {
		p.fail(sp, d, errtext.Line(err))
		// The page id is still good for children even when this read failed.
		return d.PageID, true, nil
	}
	switch {
	case d.Status == ritualrepo.StatusConflict:
		// Never pushed while in conflict; follow a remote that moved again.
		if remote.Version > d.ConflictVersion {
			if err := p.docs.ApplyConflict(p.ctx, k, d.PageID, remote.Body, remote.Version); err != nil {
				return "", false, err
			}
		}
		p.res.Conflicts++
	case remote.Version > d.Version && !d.Dirty():
		if err := p.pull(k, d, remote); err != nil {
			return "", false, err
		}
	case remote.Version > d.Version:
		if err := p.docs.ApplyConflict(p.ctx, k, d.PageID, remote.Body, remote.Version); err != nil {
			return "", false, err
		}
		p.res.Conflicts++
	case d.Dirty():
		if err := p.push(sp, k, d); err != nil {
			return "", false, err
		}
	}
	return d.PageID, true, nil
}

// pull takes a newer remote into a clean document, and records a conflict
// instead when a save landed after the pass read the row.
func (p *pass) pull(k ritualrepo.Key, d ritualrepo.Document, remote confluence.StoredPage) error {
	pulled, err := p.docs.ApplyPulled(p.ctx, k, d.PageID, d.Body, remote.Body, remote.Version, p.now())
	if err != nil {
		return err
	}
	if pulled {
		p.res.Pulled++
		return nil
	}
	if err := p.docs.ApplyConflict(p.ctx, k, d.PageID, remote.Body, remote.Version); err != nil {
		return err
	}
	p.res.Conflicts++
	return nil
}

// push sends the body the pass read at base plus one. A 409 means the page
// moved between the read and the write, so it is read again and recorded as
// a conflict. The base is set to what was pushed, never to what Confluence
// answers with: Confluence normalises storage on save, and a base taken from
// its answer would leave every pushed page dirty forever.
func (p *pass) push(sp Sprint, k ritualrepo.Key, d ritualrepo.Document) error {
	updated, err := p.pages.UpdatePage(p.ctx, d.PageID, d.Title, d.Body, d.Version+1)
	if errors.Is(err, confluence.ErrVersionConflict) {
		again, getErr := p.pages.GetPageStorage(p.ctx, d.PageID)
		if getErr != nil {
			p.fail(sp, d, errtext.Line(getErr))
			return nil
		}
		if err := p.docs.ApplyConflict(p.ctx, k, d.PageID, again.Body, again.Version); err != nil {
			return err
		}
		p.res.Conflicts++
		return nil
	}
	if err != nil {
		p.fail(sp, d, errtext.Line(err))
		return nil
	}
	if err := p.docs.ApplyPushed(p.ctx, k, d.Body, updated.Version, p.now()); err != nil {
		return err
	}
	p.res.Pushed++
	return nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `cd tam && go test ./internal/ritualsync/ && go vet ./internal/ritualsync/`
Expected: PASS, every test in Tasks 7 to 9.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/ritualsync
git commit -m "feat(tam): a ritual sync that pulls, pushes and keeps conflicts"
```

---

### Task 10: The App bindings

**Files:**
- Modify: `tam/app.go` (field `demoConfluence map[string]*demo.Confluence`, initialised in `initStore` beside `a.busy`)
- Modify: `tam/app_rituals.go` (append the new bindings; the old ones stay until Task 17)
- Create: `tam/app_ritualsync_test.go`
- Regenerate: `tam/frontend/wailsjs/**` via `wails generate module`

**Interfaces:**
- Consumes: Tasks 4 to 9, `a.acquire`/`a.release`, `a.requireProfile`, `a.requireRituals`, `a.confluenceClient`, `a.repo.ProfileSetting`/`SetProfileSetting`/`ListIssues`, `a.boards.ListSprints`/`ListBoards`.
- Produces (bound to Wails):
  - `EnsureSprintRituals(profileID string, boardID, sprintID int) ([]ritualrepo.Document, error)`
  - `ListRitualDocuments(profileID string, boardID, sprintID int) ([]ritualrepo.Document, error)`
  - `SaveRitualBody(profileID string, boardID, sprintID int, ritualType, body string) (ritualrepo.Document, error)`
  - `ResolveRitualConflict(profileID string, boardID, sprintID int, ritualType, choice string) error` (`"mine"` | `"theirs"`)
  - `ForgetRitualPage(profileID string, boardID, sprintID int, ritualType string) error`
  - `DeleteRitualDocument(profileID string, boardID, sprintID int, ritualType string) error`
  - `RitualMacroIssues(profileID, jql string) (RitualMacroPreview, error)`; `type RitualMacroPreview struct { Supported bool; JQL string; Issues []backend.Issue }` (JSON `supported, jql, issues`)
  - `StandupEntry(day string) (string, error)` (`day` = `YYYY-MM-DD`)
  - `LastRitualSync(profileID string, boardID int) (string, error)`
  - `SyncRituals(profileID string, boardID int) (ritualsync.Result, error)`

- [ ] **Step 1: Write the failing tests** (`app_ritualsync_test.go`)

```go
package main

import (
	"strings"
	"testing"

	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/ritualrepo"
)

// newRitualSyncApp is a non-demo Jira profile whose Confluence URL is "demo",
// which is what routes its pages to the in-memory space, with one scrum board
// and one active sprint in the cache.
func newRitualSyncApp(t *testing.T) (*App, profile.Profile) {
	t.Helper()
	a, p := newTestAppWithRituals(t)
	if err := a.profiles.SetConfluenceConfig(p.ID, profile.ConfluenceConfig{BaseURL: "demo", SpaceKey: "DEMO", RootPageID: "demo-root"}); err != nil {
		t.Fatal(err)
	}
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT board", Type: "scrum"}, nil,
		[]backend.Sprint{{ID: 14, BoardID: 1, Name: "Sprint 14", State: "active",
			StartDate: "2026-09-14T09:00:00.000+0000", EndDate: "2026-09-25T17:00:00.000+0000"}}, nil); err != nil {
		t.Fatal(err)
	}
	return a, p
}

func TestEnsureSprintRitualsWritesFiveLocalPagesWithoutConfluence(t *testing.T) {
	a, p := newRitualSyncApp(t)
	docs, err := a.EnsureSprintRituals(p.ID, 1, 14)
	if err != nil || len(docs) != 5 {
		t.Fatalf("docs = %d, %v", len(docs), err)
	}
	for _, d := range docs {
		if d.Status != ritualrepo.StatusLocal {
			t.Errorf("%s = %s", d.RitualType, d.Status)
		}
	}
	if len(a.demoConfluence) != 0 {
		t.Fatal("ensuring pages must not touch a Confluence transport")
	}
	if _, err := a.EnsureSprintRituals(p.ID, 1, 99); err == nil {
		t.Fatal("a sprint the cache does not hold should be refused")
	}
}

func TestSyncRitualsCreatesTheTreeAndRecordsWhen(t *testing.T) {
	a, p := newRitualSyncApp(t)
	res, err := a.SyncRituals(p.ID, 1)
	if err != nil || res.Created != 5 {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if last, err := a.LastRitualSync(p.ID, 1); err != nil || last != res.SyncedAt {
		t.Fatalf("last sync = %q, %v", last, err)
	}
	if _, ok := a.busy[p.ID]; ok {
		t.Fatal("the lock was not released")
	}
}

func TestSyncRitualsIsRefusedWhileAnotherOperationHoldsTheLock(t *testing.T) {
	a, p := newRitualSyncApp(t)
	a.busy[p.ID] = "sync"
	if _, err := a.SyncRituals(p.ID, 1); err == nil || err.Error() != "a sync is already running for this profile" {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncRitualsIsRefusedWithoutConfluence(t *testing.T) {
	a, p := newTestAppWithRituals(t)
	if _, err := a.SyncRituals(p.ID, 1); err == nil || err.Error() != "Confluence is not configured for this profile" {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveAndResolveThroughTheBindings(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.EnsureSprintRituals(p.ID, 1, 14); err != nil {
		t.Fatal(err)
	}
	d, err := a.SaveRitualBody(p.ID, 1, 14, "planning", "<p>mine</p>")
	if err != nil || d.Body != "<p>mine</p>" || d.Status != ritualrepo.StatusLocal {
		t.Fatalf("saved = %+v, %v", d, err)
	}
	if _, err := a.SaveRitualBody(p.ID, 1, 14, "party", "<p/>"); err == nil {
		t.Fatal("an unknown ritual type should be refused")
	}
	if err := a.ResolveRitualConflict(p.ID, 1, 14, "planning", "both"); err == nil {
		t.Fatal("an unknown choice should be refused")
	}
	if err := a.ResolveRitualConflict(p.ID, 1, 14, "planning", "mine"); err == nil {
		t.Fatal("a row with no conflict should be refused")
	}
}

func TestRitualMacroIssuesPreviewsTheThreeFormsFromTheCache(t *testing.T) {
	a, p := newRitualSyncApp(t)
	seedSprintIssues(t, a, p.ID, 12) // To Do, In Progress, Done, Blocked, all in sprint 12
	for jql, want := range map[string]int{
		"sprint = 12 ORDER BY Rank":               4,
		"sprint = 12 AND statusCategory = Done":   1,
		"sprint = 12 AND statusCategory != Done":  3,
	} {
		got, err := a.RitualMacroIssues(p.ID, jql)
		if err != nil || !got.Supported || len(got.Issues) != want {
			t.Errorf("%s = %d issues, supported %v, %v", jql, len(got.Issues), got.Supported, err)
		}
	}
	if got, _ := a.RitualMacroIssues(p.ID, "project = PLAT"); got.Supported || len(got.Issues) != 0 {
		t.Fatalf("other JQL = %+v", got)
	}
}

func TestStandupEntryReadsADayInput(t *testing.T) {
	a, _ := newRitualSyncApp(t)
	entry, err := a.StandupEntry("2026-09-15")
	if err != nil || !strings.HasPrefix(entry, "<h3>Tue 15 Sep 2026</h3>") {
		t.Fatalf("entry = %q, %v", entry, err)
	}
	if _, err := a.StandupEntry("tomorrow"); err == nil {
		t.Fatal("a malformed day should be refused")
	}
}

// The demo space lives in memory, so a restart empties it. Without rebuilding
// it from the pages the store already knows, every demo page would read as
// gone after reopening the app.
func TestDemoPagesSurviveARestart(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.SyncRituals(p.ID, 1); err != nil {
		t.Fatal(err)
	}
	a.demoConfluence = nil
	if _, err := a.SaveRitualBody(p.ID, 1, 14, "planning", "<p>after restart</p>"); err != nil {
		t.Fatal(err)
	}
	res, err := a.SyncRituals(p.ID, 1)
	if err != nil || res.Gone != 0 || res.Pushed != 1 {
		t.Fatalf("after restart = %+v, %v", res, err)
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `cd tam && go test . -run 'Ritual|Standup|DemoPages'`
Expected: FAIL to compile.

- [ ] **Step 3: Add the App field**

In `tam/app.go`, add to the `App` struct after `reportCancels`:

```go
	// demoConfluence holds the in-memory Confluence space of each profile
	// whose Confluence URL is "demo". Guarded by backendMu; app_rituals.go is
	// the only file that touches it.
	demoConfluence map[string]*demo.Confluence
```

Import `"agile-suite/tam/internal/demo"`, and in `initStore` after `a.busy = map[string]string{}` add `a.demoConfluence = map[string]*demo.Confluence{}`.

- [ ] **Step 4: Append the bindings to `app_rituals.go`**

Add imports: `"fmt"`, `"log"`, `"agile-suite/tam/internal/demo"`, `"agile-suite/tam/internal/errtext"`, `"agile-suite/tam/internal/ritualsync"`, `"agile-suite/tam/internal/ritualtemplate"`, `"agile-suite/tam/internal/sprintdate"`, `"agile-suite/tam/internal/suiteprofiles"`.

```go
// confluencePages answers with the page transport a profile's rituals sync
// through and the configuration it was built from. It reads only local
// configuration and the credential store: nothing here makes a request.
func (a *App) confluencePages(p profile.Profile) (profile.ConfluenceConfig, confluence.Pages, error) {
	c, err := a.profiles.ConfluenceConfig(p.ID)
	if err != nil {
		return c, nil, err
	}
	if strings.TrimSpace(c.BaseURL) == "" && suiteprofiles.IsDemoURL(p.JiraURL) {
		c = profile.ConfluenceConfig{BaseURL: "demo", SpaceKey: "DEMO", RootPageID: "demo-root"}
	}
	switch {
	case strings.TrimSpace(c.BaseURL) == "":
		return c, nil, errors.New("Confluence is not configured for this profile")
	case strings.TrimSpace(c.SpaceKey) == "" || strings.TrimSpace(c.RootPageID) == "":
		return c, nil, errors.New("Confluence needs a space key and a root page id before rituals can sync")
	}
	if strings.EqualFold(strings.TrimSpace(c.BaseURL), "demo") {
		return c, a.demoSpace(p.ID, c), nil
	}
	_, client, err := a.confluenceClient(p.ID)
	if err != nil {
		return c, nil, err
	}
	return c, client, nil
}

// demoSpace is a profile's in-memory space, rebuilt from the pages the store
// knows about the first time it is asked for after the app starts, so a
// restart does not turn every demo page gone.
func (a *App) demoSpace(profileID string, c profile.ConfluenceConfig) *demo.Confluence {
	a.backendMu.Lock()
	defer a.backendMu.Unlock()
	if a.demoConfluence == nil {
		a.demoConfluence = map[string]*demo.Confluence{}
	}
	if space, ok := a.demoConfluence[profileID]; ok {
		return space
	}
	space := demo.NewConfluence(c.SpaceKey, c.RootPageID, true)
	if docs, err := a.rituals.ProfileDocuments(a.ctx, profileID); err != nil {
		log.Printf("tam: rebuild demo ritual pages for %s: %v", profileID, err)
	} else {
		overviews := map[[2]int]string{}
		for _, d := range docs {
			if d.RitualType == ritualtemplate.Sprint {
				overviews[[2]int{d.BoardID, d.SprintID}] = d.PageID
				space.Restore(d.PageID, c.RootPageID, d.Title, d.BaseBody, d.Version)
			}
		}
		for _, d := range docs {
			if parent, ok := overviews[[2]int{d.BoardID, d.SprintID}]; ok && d.RitualType != ritualtemplate.Sprint {
				space.Restore(d.PageID, parent, d.Title, d.BaseBody, d.Version)
			}
		}
	}
	a.demoConfluence[profileID] = space
	return space
}

func (a *App) boardName(profileID string, boardID int) string {
	boards, err := a.boards.ListBoards(a.ctx, profileID)
	if err != nil {
		return ""
	}
	for _, b := range boards {
		if b.ID == boardID {
			return b.Name
		}
	}
	return ""
}

func (a *App) ritualSprint(profileID string, boardID, sprintID int) (ritualsync.Sprint, error) {
	cached, err := a.boards.ListSprints(a.ctx, profileID, boardID)
	if err != nil {
		return ritualsync.Sprint{}, err
	}
	for _, s := range cached {
		if s.ID == sprintID {
			return ritualsync.Sprint{Info: ritualsync.Info(s, a.boardName(profileID, boardID)), State: s.State}, nil
		}
	}
	return ritualsync.Sprint{}, fmt.Errorf("sprint %d is not in this board's cache; refresh the board first", sprintID)
}

func ritualKey(profileID string, boardID, sprintID int, ritualType string) (ritualrepo.Key, error) {
	if !ritualtemplate.Known(ritualType) {
		return ritualrepo.Key{}, errors.New("unknown ritual type")
	}
	return ritualrepo.Key{ProfileID: profileID, BoardID: boardID, SprintID: sprintID, RitualType: ritualType}, nil
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// EnsureSprintRituals writes whichever of a sprint's five pages are missing,
// from templates, and lists the sprint's pages. Local only, no lock: this is
// what lets a planning page be written with no network at all.
func (a *App) EnsureSprintRituals(profileID string, boardID, sprintID int) ([]ritualrepo.Document, error) {
	if _, err := a.requireProfile(profileID); err != nil {
		return nil, err
	}
	if err := a.requireRituals(); err != nil {
		return nil, err
	}
	sp, err := a.ritualSprint(profileID, boardID, sprintID)
	if err != nil {
		return nil, err
	}
	if err := ritualsync.Ensure(a.ctx, a.rituals, profileID, boardID, sp, time.Local, time.Now()); err != nil {
		return nil, err
	}
	return a.rituals.Documents(a.ctx, profileID, boardID, sprintID)
}

// ListRitualDocuments reads a sprint's pages from tam.db, and nothing else.
func (a *App) ListRitualDocuments(profileID string, boardID, sprintID int) ([]ritualrepo.Document, error) {
	if _, err := a.requireProfile(profileID); err != nil {
		return nil, err
	}
	if err := a.requireRituals(); err != nil {
		return nil, err
	}
	return a.rituals.Documents(a.ctx, profileID, boardID, sprintID)
}

// SaveRitualBody is the editor's local save. It takes no lock, the way a
// board move takes none; the sync's compare-and-set is what makes that safe.
func (a *App) SaveRitualBody(profileID string, boardID, sprintID int, ritualType, body string) (ritualrepo.Document, error) {
	if _, err := a.requireProfile(profileID); err != nil {
		return ritualrepo.Document{}, err
	}
	if err := a.requireRituals(); err != nil {
		return ritualrepo.Document{}, err
	}
	k, err := ritualKey(profileID, boardID, sprintID, ritualType)
	if err != nil {
		return ritualrepo.Document{}, err
	}
	return a.rituals.SaveBody(a.ctx, k, body, nowStamp())
}

// ResolveRitualConflict keeps the local body ("mine", pushed on the next
// Sync) or takes Confluence's ("theirs"). Local, no network.
func (a *App) ResolveRitualConflict(profileID string, boardID, sprintID int, ritualType, choice string) error {
	if _, err := a.requireProfile(profileID); err != nil {
		return err
	}
	if err := a.requireRituals(); err != nil {
		return err
	}
	k, err := ritualKey(profileID, boardID, sprintID, ritualType)
	if err != nil {
		return err
	}
	switch choice {
	case "mine":
		return a.rituals.ResolveMine(a.ctx, k, nowStamp())
	case "theirs":
		return a.rituals.ResolveTheirs(a.ctx, k, nowStamp())
	}
	return fmt.Errorf("unknown conflict choice %q", choice)
}

// ForgetRitualPage lets a page gone from Confluence be created again on the
// next Sync, from the local body.
func (a *App) ForgetRitualPage(profileID string, boardID, sprintID int, ritualType string) error {
	if _, err := a.requireProfile(profileID); err != nil {
		return err
	}
	if err := a.requireRituals(); err != nil {
		return err
	}
	k, err := ritualKey(profileID, boardID, sprintID, ritualType)
	if err != nil {
		return err
	}
	return a.rituals.ForgetPage(a.ctx, k, nowStamp())
}

// DeleteRitualDocument removes the local copy. The Confluence page is left
// alone; the next time the sprint opens, a fresh template takes its place.
func (a *App) DeleteRitualDocument(profileID string, boardID, sprintID int, ritualType string) error {
	if _, err := a.requireProfile(profileID); err != nil {
		return err
	}
	if err := a.requireRituals(); err != nil {
		return err
	}
	k, err := ritualKey(profileID, boardID, sprintID, ritualType)
	if err != nil {
		return err
	}
	return a.rituals.DeleteDocument(a.ctx, k)
}

// RitualMacroPreview is what a Jira Issues macro shows inside the editor.
type RitualMacroPreview struct {
	Supported bool            `json:"supported"`
	JQL       string          `json:"jql"`
	Issues    []backend.Issue `json:"issues"`
}

// RitualMacroIssues answers a macro's query from the issue cache, for the
// three forms the templates write. Done here is backend.IsDone, the status
// name rule, which is not Jira's statusCategory: the editor says so under
// every preview.
func (a *App) RitualMacroIssues(profileID, jql string) (RitualMacroPreview, error) {
	out := RitualMacroPreview{JQL: jql, Issues: []backend.Issue{}}
	if _, err := a.requireProfile(profileID); err != nil {
		return out, err
	}
	sprintID, filter, ok := ritualtemplate.ParseJQL(jql)
	if !ok {
		return out, nil
	}
	out.Supported = true
	page, err := a.repo.ListIssues(a.ctx, profileID, issuerepo.IssueQuery{SprintID: strconv.Itoa(sprintID), Limit: 500})
	if err != nil {
		return out, err
	}
	for _, issue := range page.Issues {
		done := backend.IsDone(issue.Status)
		if (filter == ritualtemplate.Done && !done) || (filter == ritualtemplate.NotDone && done) {
			continue
		}
		out.Issues = append(out.Issues, issue)
	}
	return out, nil
}

// StandupEntry is one day of the standup log for a YYYY-MM-DD day, the same
// fragment the template seeds, so "Add today's entry" and the template agree.
func (a *App) StandupEntry(day string) (string, error) {
	t, err := time.ParseInLocation(sprintdate.Day, strings.TrimSpace(day), time.Local)
	if err != nil {
		return "", fmt.Errorf("read standup day %q: %w", day, err)
	}
	return ritualtemplate.StandupEntry(t), nil
}

func lastRitualSyncKey(boardID int) string { return "rituals_last_sync:" + strconv.Itoa(boardID) }

// LastRitualSync is when a board's rituals last finished a Sync, "" if never.
func (a *App) LastRitualSync(profileID string, boardID int) (string, error) {
	if _, err := a.requireProfile(profileID); err != nil {
		return "", err
	}
	return a.repo.ProfileSetting(a.ctx, profileID, lastRitualSyncKey(boardID))
}

// SyncRituals is the Rituals view's Sync: one pass over a board's open
// sprints, and closed ones that already have pages. It holds the profile
// lock under "rituals". Configuration and credentials are refused before
// the lock is taken, since neither needs it; per-page trouble travels in the
// result.
func (a *App) SyncRituals(profileID string, boardID int) (ritualsync.Result, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return ritualsync.Result{}, err
	}
	if err := a.requireRituals(); err != nil {
		return ritualsync.Result{}, err
	}
	cfg, pages, err := a.confluencePages(p)
	if err != nil {
		return ritualsync.Result{}, err
	}
	if err := a.acquire(p.ID, "rituals"); err != nil {
		return ritualsync.Result{}, err
	}
	defer a.release(p.ID)
	log.Printf("tam: rituals sync for %s board %d starting", p.ID, boardID)

	cached, err := a.boards.ListSprints(a.ctx, p.ID, boardID)
	if err != nil {
		return ritualsync.Result{}, err
	}
	withRows, err := a.rituals.BoardSprintIDs(a.ctx, p.ID, boardID)
	if err != nil {
		return ritualsync.Result{}, err
	}
	res, err := ritualsync.Run(a.ctx, pages, a.rituals, ritualsync.Config{
		SpaceKey: cfg.SpaceKey, RootID: cfg.RootPageID, Location: time.Local, Now: time.Now,
	}, p.ID, boardID, ritualsync.Sprints(cached, a.boardName(p.ID, boardID), withRows))
	if err != nil {
		log.Printf("tam: rituals sync for %s board %d refused: %v", p.ID, boardID, err)
		return ritualsync.Result{}, errors.New(errtext.Line(err))
	}
	if err := a.repo.SetProfileSetting(a.ctx, p.ID, lastRitualSyncKey(boardID), res.SyncedAt); err != nil {
		log.Printf("tam: record rituals sync time for %s: %v", p.ID, err)
	}
	log.Printf("tam: rituals sync for %s board %d done: %d created, %d pulled, %d pushed, %d conflicts, %d gone, %d failed",
		p.ID, boardID, res.Created, res.Pulled, res.Pushed, res.Conflicts, res.Gone, len(res.Failed))
	return res, nil
}
```

(`requireProfile` returns `profile.Profile`; `app_rituals.go` already imports `profile`, `confluence`, `backend`, `issuerepo`, `ritualrepo`, `errors`, `strconv`, `strings`, `time`.)

- [ ] **Step 5: Run the App tests and vet**

Run: `cd tam && go test . && go vet .`
Expected: PASS, old ritual tests included.

- [ ] **Step 6: Regenerate the Wails bindings**

Run: `cd tam && wails generate module`
Expected: `frontend/wailsjs/go/main/App.d.ts` gains `SyncRituals`, `SaveRitualBody`, and the rest; `models.ts` gains `ritualrepo.Document`, `ritualsync.Result`, `main.RitualMacroPreview`. Check with `git diff --stat tam/frontend/wailsjs`.

Run: `cd tam/frontend && npm run typecheck`
Expected: PASS (nothing uses the new bindings yet).

- [ ] **Step 7: Commit**

```bash
git add tam/app.go tam/app_rituals.go tam/app_ritualsync_test.go tam/frontend/wailsjs
git commit -m "feat(tam): bindings for local ritual pages and their Sync"
```

---

## Part C: frontend

All `npm` commands run from the repo root unless a step says otherwise; `npx vitest run <path>` runs from `tam/frontend`.

### Task 11: The editor dependencies and the storage XML layer

**Files:**
- Modify: `tam/frontend/package.json` (via `npm install`)
- Create: `tam/frontend/src/lib/storage/xml.ts`, `tam/frontend/src/lib/storage/normalize.ts`, `tam/frontend/src/lib/storage/xml.test.ts`

**Interfaces:**
- Produces:
  - `NAMESPACES`, `isBlank(s: string): boolean`, `toNumericEntities(body: string): string`, `parseXml(body: string): Element | null`, `stripNamespaces(xml: string): string`, `outerXml(node: Node): string`, `escapeText(s: string): string`, `escapeAttr(s: string): string`
  - `normalizeStorage(body: string): string`

- [ ] **Step 1: Install TipTap, pinned**

```bash
npm install --save-exact --workspace tam-frontend @tiptap/core@3.31.3 @tiptap/pm@3.31.3 @tiptap/react@3.31.3 @tiptap/starter-kit@3.31.3 @tiptap/extension-table@3.31.3 @tiptap/extension-list@3.31.3
```

Expected: `tam/frontend/package.json` dependencies list the six at `3.31.3`, root `package-lock.json` updated.

- [ ] **Step 2: Write the failing tests** (`xml.test.ts`)

```ts
import { describe, expect, it } from "vitest";
import { escapeAttr, escapeText, isBlank, outerXml, parseXml, toNumericEntities } from "./xml";
import { normalizeStorage } from "./normalize";

describe("storage XML", () => {
  it("rewrites HTML named entities as numeric references and leaves XML's own five alone", () => {
    expect(toNumericEntities("a&nbsp;b &mdash; &amp; &lt; &gt; &quot; &apos;")).toBe("a&#160;b &#8212; &amp; &lt; &gt; &quot; &apos;");
  });

  it("leaves an entity nobody knows, so the parse fails rather than guessing", () => {
    expect(toNumericEntities("&bogus;")).toBe("&bogus;");
    expect(parseXml("<p>&bogus;</p>")).toBeNull();
  });

  it("reads the ac: and ri: prefixes Confluence never declares", () => {
    const root = parseXml('<ac:link><ri:user ri:userkey="u1"/></ac:link>');
    expect(root?.firstElementChild?.nodeName).toBe("ac:link");
  });

  it("refuses malformed XML", () => {
    expect(parseXml("<p>unclosed")).toBeNull();
  });

  it("writes a fragment back without the declarations the wrapper added, CDATA intact", () => {
    const xml = '<ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[if a < b && c > d {}]]></ac:plain-text-body></ac:structured-macro>';
    const root = parseXml(xml)!;
    expect(outerXml(root.firstChild!)).toBe(xml);
  });

  it("escapes text and attribute values", () => {
    expect(escapeText(`a & <b> "c"`)).toBe(`a &amp; &lt;b&gt; "c"`);
    expect(escapeAttr(`a & "c"`)).toBe("a &amp; &quot;c&quot;");
  });

  it("treats only ASCII whitespace as blank, so a non-breaking space is content", () => {
    expect(isBlank(" \n\t")).toBe(true);
    expect(isBlank(" ")).toBe(false);
  });

  it("normalises whitespace between elements, attribute order and self-closing form, and nothing else", () => {
    expect(normalizeStorage('<p b="2" a="1">x</p>\n  <br/>')).toBe(normalizeStorage('<p a="1" b="2">x</p><br></br>'));
    expect(normalizeStorage("<p>a b</p>")).not.toBe(normalizeStorage("<p>ab</p>"));
    expect(normalizeStorage("<p>&nbsp;</p>")).toBe(normalizeStorage("<p> </p>"));
  });
});
```

- [ ] **Step 3: Run to see it fail**

Run: `cd tam/frontend && npx vitest run src/lib/storage/xml.test.ts`
Expected: FAIL, cannot resolve `./xml`.

- [ ] **Step 4: Write `xml.ts`**

```ts
// Confluence storage format is XML in all but two habits: it uses the ac:,
// ri: and at: prefixes without declaring them, and it writes HTML named
// entities XML does not define. parseXml undoes both so the browser's own XML
// parser can read a page, and outerXml undoes the declarations XMLSerializer
// adds back when a fragment is written out, so an element TAM does not model
// goes back to Confluence exactly as it came.

export const NAMESPACES: Record<string, string> = {
  ac: "http://atlassian.com/content",
  ri: "http://atlassian.com/resource/identifier",
  at: "http://atlassian.com/template",
};

const XML_ENTITIES = new Set(["amp", "lt", "gt", "quot", "apos"]);
const decoded = new Map<string, string>();

// isBlank is ASCII whitespace only. String.prototype.trim also strips a
// non-breaking space, which in a page is content somebody typed.
export function isBlank(s: string): boolean {
  return /^[ \t\r\n]*$/.test(s);
}

export function toNumericEntities(body: string): string {
  return body.replace(/&([a-zA-Z][a-zA-Z0-9]*);/g, (whole, name: string) => {
    if (XML_ENTITIES.has(name)) return whole;
    let text = decoded.get(name);
    if (text === undefined) {
      text = new DOMParser().parseFromString(`<!doctype html><body>&${name};`, "text/html").body.textContent ?? "";
      decoded.set(name, text);
    }
    // An entity the HTML parser does not know comes back as its own text.
    if (text === "" || text === whole) return whole;
    return Array.from(text).map((ch) => `&#${ch.codePointAt(0)};`).join("");
  });
}

export function parseXml(body: string): Element | null {
  const declarations = Object.entries(NAMESPACES).map(([prefix, uri]) => `xmlns:${prefix}="${uri}"`).join(" ");
  const doc = new DOMParser().parseFromString(`<tam-root ${declarations}>${toNumericEntities(body)}</tam-root>`, "application/xml");
  if (doc.getElementsByTagName("parsererror").length > 0) return null;
  return doc.documentElement;
}

export function stripNamespaces(xml: string): string {
  return xml.replace(/\s+xmlns:(ac|ri|at)="[^"]*"/g, "");
}

export function outerXml(node: Node): string {
  return stripNamespaces(new XMLSerializer().serializeToString(node));
}

export function escapeText(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

export function escapeAttr(s: string): string {
  return escapeText(s).replace(/"/g, "&quot;");
}
```

- [ ] **Step 5: Write `normalize.ts`**

```ts
import { escapeAttr, escapeText, isBlank, parseXml } from "./xml";

// normalizeStorage is the equality the round-trip contract is stated in:
// whitespace between elements, attribute order, and self-closing form do not
// count, and nothing else is forgiven. It exists for tests and for nothing
// the app decides on.
export function normalizeStorage(body: string): string {
  const root = parseXml(body);
  return root ? children(root) : `unparseable:${body}`;
}

function children(el: Element): string {
  const nodes = Array.from(el.childNodes);
  const hasElement = nodes.some((c) => c.nodeType === Node.ELEMENT_NODE);
  return nodes.map((c) => {
    switch (c.nodeType) {
      case Node.ELEMENT_NODE: {
        const e = c as Element;
        const attrs = Array.from(e.attributes).map((a) => `${a.name}="${escapeAttr(a.value)}"`).sort().join(" ");
        return `<${e.nodeName}${attrs ? ` ${attrs}` : ""}>${children(e)}</${e.nodeName}>`;
      }
      case Node.CDATA_SECTION_NODE:
        return `<![CDATA[${c.textContent ?? ""}]]>`;
      case Node.TEXT_NODE: {
        const text = c.textContent ?? "";
        return hasElement && isBlank(text) ? "" : escapeText(text);
      }
      case Node.COMMENT_NODE:
        return `<!--${c.textContent ?? ""}-->`;
    }
    return "";
  }).join("");
}
```

- [ ] **Step 6: Run the tests and the typecheck**

Run: `cd tam/frontend && npx vitest run src/lib/storage/xml.test.ts && npm run typecheck`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add package-lock.json tam/frontend/package.json tam/frontend/src/lib/storage
git commit -m "feat(tam): read and write Confluence storage format as XML"
```

---

### Task 12: Storage format to editor document and back

**Files:**
- Create: `tam/frontend/src/lib/storage/parse.ts`, `tam/frontend/src/lib/storage/serialize.ts`, `tam/frontend/src/lib/storage/storage.test.ts`

**Interfaces:**
- Consumes: Task 11 `xml.ts`, `normalize.ts`; Task 4 golden files.
- Produces:
  - `type ParseResult = { ok: true; doc: JSONContent } | { ok: false; reason: string }`; `parseStorage(body: string): ParseResult`
  - `serializeStorage(doc: JSONContent): string`
  - The JSON shape the editor schema (Task 14) must declare:
    - `paragraph` attrs `bare` (true = written without `<p>`), `extra`
    - `heading` attrs `level`, `extra`; `bulletList`/`orderedList`/`listItem`/`blockquote`/`tableRow`/`taskList` attr `extra`
    - `table` attrs `extra`, `colgroup` (raw XML), `tbody` (boolean); `tableCell`/`tableHeader` attrs `colspan`, `rowspan`, `extra`
    - `taskItem` attrs `checked`, `taskId`, `extraXml`
    - marks `bold`/`italic`/`strike` attr `tag`; `link` attrs `href`, `extra`; `underline`, `code`
    - `opaqueBlock`, `opaqueInline` attrs `xml`, `label`; `hardBreak`; `horizontalRule`

- [ ] **Step 1: Write the failing tests** (`storage.test.ts`)

```ts
import { describe, expect, it } from "vitest";
import type { JSONContent } from "@tiptap/core";
import { parseStorage } from "./parse";
import { serializeStorage } from "./serialize";
import { normalizeStorage } from "./normalize";
import sprint from "../../../../internal/ritualtemplate/testdata/sprint.xml?raw";
import planning from "../../../../internal/ritualtemplate/testdata/planning.xml?raw";
import standup from "../../../../internal/ritualtemplate/testdata/standup.xml?raw";
import review from "../../../../internal/ritualtemplate/testdata/review.xml?raw";
import retro from "../../../../internal/ritualtemplate/testdata/retro.xml?raw";

const handWritten: Record<string, string> = {
  layout: `<ac:layout><ac:layout-section ac:type="two_equal"><ac:layout-cell><p>Left</p></ac:layout-cell><ac:layout-cell><p>Right</p></ac:layout-cell></ac:layout-section></ac:layout>`,
  codeMacro: `<p>Before</p><ac:structured-macro ac:name="code" ac:schema-version="1" ac:macro-id="abc"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><![CDATA[if a < b && c > d { return }]]></ac:plain-text-body></ac:structured-macro><p>After</p>`,
  mention: `<p>Ask <ac:link><ri:user ri:userkey="8a7f808a"/></ac:link> about <strong>infra</strong> access.</p>`,
  colouredSpan: `<p>Status is <span style="color: rgb(255,0,0);">red</span> today.</p>`,
  mergedCells: `<table><colgroup><col/><col/></colgroup><tbody><tr><th colspan="2">Capacity</th></tr><tr><td>Dian</td><td><p>8 days</p></td></tr></tbody></table>`,
  entities: `<p>Owner&nbsp;notes &mdash; keep &amp; share &lt;tags&gt;</p>`,
  nestedTasks: `<ac:task-list><ac:task><ac:task-id>1</ac:task-id><ac:task-status>complete</ac:task-status><ac:task-body>Parent<ac:task-list><ac:task><ac:task-id>2</ac:task-id><ac:task-status>incomplete</ac:task-status><ac:task-body>Child</ac:task-body></ac:task></ac:task-list></ac:task-body></ac:task></ac:task-list>`,
  image: `<p><ac:image ac:height="250"><ri:attachment ri:filename="board.png"/></ac:image></p>`,
  emoticon: `<p>Shipped <ac:emoticon ac:name="tick"/></p>`,
  mixedList: `<ul><li>Item<ul><li>Sub item</li></ul></li><li><p>Para item</p></li></ul>`,
  bareRoot: `Loose text <em>here</em>`,
  inlineMacro: `<p>State: <ac:structured-macro ac:name="status"><ac:parameter ac:name="colour">Green</ac:parameter><ac:parameter ac:name="title">On track</ac:parameter></ac:structured-macro></p>`,
  marksAndLinks: `<p>a <strong>b <em>c</em></strong> <a href="https://example.com" title="x">d</a><br/>e <s>gone</s> <u>under</u> <code>x()</code></p>`,
  headingsAndRule: `<h1>One</h1><h4 style="text-align: center;">Four</h4><hr/><blockquote><p>Quoted</p></blockquote>`,
};

const corpus: Record<string, string> = { sprint, planning, standup, review, retro, ...handWritten };

function roundTrip(body: string): string {
  const parsed = parseStorage(body);
  if (!parsed.ok) throw new Error(parsed.reason);
  return serializeStorage(parsed.doc);
}

function findAll(node: JSONContent, type: string, out: JSONContent[] = []): JSONContent[] {
  if (node.type === type) out.push(node);
  for (const child of node.content ?? []) findAll(child, type, out);
  return out;
}

describe("storage round trip", () => {
  for (const [name, body] of Object.entries(corpus)) {
    it(`round-trips ${name}`, () => {
      expect(normalizeStorage(roundTrip(body))).toBe(normalizeStorage(body));
    });
  }

  it("keeps opaque XML byte for byte", () => {
    const out = roundTrip(handWritten.codeMacro);
    expect(out).toContain('<ac:structured-macro ac:name="code" ac:schema-version="1" ac:macro-id="abc">');
    expect(out).toContain("<![CDATA[if a < b && c > d { return }]]>");
    expect(out).not.toContain("xmlns");
  });

  it("labels a macro by its name and keeps a layout whole", () => {
    const parsed = parseStorage(standup);
    if (!parsed.ok) throw new Error(parsed.reason);
    expect(findAll(parsed.doc, "opaqueBlock").map((n) => n.attrs?.label)).toContain("jira");
    const layout = parseStorage(handWritten.layout);
    if (!layout.ok) throw new Error(layout.reason);
    expect(layout.doc.content).toHaveLength(1);
    expect(layout.doc.content?.[0].attrs?.label).toBe("ac:layout");
  });

  it("maps marks and links onto editable text", () => {
    const parsed = parseStorage(`<p>a <strong>b <em>c</em></strong></p>`);
    if (!parsed.ok) throw new Error(parsed.reason);
    expect(parsed.doc.content?.[0]).toEqual({
      type: "paragraph",
      attrs: { extra: null },
      content: [
        { type: "text", text: "a " },
        { type: "text", text: "b ", marks: [{ type: "bold", attrs: { tag: "strong" } }] },
        { type: "text", text: "c", marks: [{ type: "bold", attrs: { tag: "strong" } }, { type: "italic", attrs: { tag: "em" } }] },
      ],
    });
  });

  it("reads a task's status, id and body", () => {
    const parsed = parseStorage(handWritten.nestedTasks);
    if (!parsed.ok) throw new Error(parsed.reason);
    const [parent, child] = findAll(parsed.doc, "taskItem");
    expect(parent.attrs).toEqual({ checked: true, taskId: "1", extraXml: "" });
    expect(child.attrs).toEqual({ checked: false, taskId: "2", extraXml: "" });
  });

  it("reports a page it cannot read instead of guessing", () => {
    expect(parseStorage("<p>unclosed").ok).toBe(false);
    expect(parseStorage("<p>&bogus;</p>").ok).toBe(false);
  });

  it("writes two bare paragraphs an edit put side by side as real paragraphs", () => {
    const doc: JSONContent = { type: "doc", content: [
      { type: "paragraph", attrs: { bare: true }, content: [{ type: "text", text: "a" }] },
      { type: "paragraph", attrs: { bare: true }, content: [{ type: "text", text: "b" }] },
    ] };
    expect(serializeStorage(doc)).toBe("<p>a</p><p>b</p>");
  });

  it("writes what the editor creates in the shapes Confluence expects", () => {
    const doc: JSONContent = { type: "doc", content: [
      { type: "taskList", content: [{ type: "taskItem", attrs: { checked: false }, content: [{ type: "paragraph", content: [{ type: "text", text: "call infra" }] }] }] },
      { type: "table", content: [{ type: "tableRow", content: [{ type: "tableHeader", attrs: { colspan: 1, rowspan: 1 }, content: [{ type: "paragraph" }] }] }] },
    ] };
    expect(serializeStorage(doc)).toBe(
      "<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body><p>call infra</p></ac:task-body></ac:task></ac:task-list>" +
      "<table><tbody><tr><th><p></p></th></tr></tbody></table>",
    );
  });
});
```

If Task 1 recorded a round-tripped storage body for probe 2, add it to `handWritten` as `probeRoundTrip: String.raw\`<paste the body verbatim>\``. If that entry fails, the failure names the shape to either map in `parse.ts` or confirm stays opaque; do not weaken `normalizeStorage` to make it pass.

- [ ] **Step 2: Run to see it fail**

Run: `cd tam/frontend && npx vitest run src/lib/storage/storage.test.ts`
Expected: FAIL, cannot resolve `./parse`.

- [ ] **Step 3: Write `parse.ts`**

```ts
import type { JSONContent } from "@tiptap/core";
import { isBlank, outerXml, parseXml } from "./xml";

export type ParseResult = { ok: true; doc: JSONContent } | { ok: false; reason: string };

type Mark = { type: string; attrs?: Record<string, unknown> };
type Attrs = Record<string, string> | null;

// Elements that always start a block of their own.
const BLOCK = new Set(["p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "table", "blockquote", "hr", "ac:task-list"]);
// Elements that always sit inside a line of text. Anything in neither set
// (a macro, an image, an element nobody anticipated) is placed by its
// neighbours: inline beside text, a block of its own otherwise.
const INLINE = new Set(["strong", "b", "em", "i", "u", "s", "del", "code", "a", "br", "span", "sub", "sup", "ac:link", "ac:emoticon", "time", "ac:inline-comment-marker"]);
const MARKS: Record<string, string> = { strong: "bold", b: "bold", em: "italic", i: "italic", u: "underline", s: "strike", del: "strike", code: "code" };
const TAGGED = new Set(["bold", "italic", "strike"]);

// parseStorage turns a page into the editor's document. Every element it
// does not model becomes an opaque node holding its raw XML, so nothing on a
// page is ever dropped: the fallback is the rule, not a list of known
// unknowns.
export function parseStorage(body: string): ParseResult {
  const root = parseXml(body);
  if (!root) return { ok: false, reason: "The page is not well-formed storage format." };
  return { ok: true, doc: { type: "doc", content: nonEmpty(blockChildren(root)) } };
}

function node(type: string, attrs?: Record<string, unknown> | null, content?: JSONContent[]): JSONContent {
  const n: JSONContent = { type };
  if (attrs) n.attrs = attrs;
  if (content && content.length) n.content = content;
  return n;
}

const isElement = (n: Node): n is Element => n.nodeType === Node.ELEMENT_NODE;
const isText = (n: Node) => n.nodeType === Node.TEXT_NODE;

function attrsOf(el: Element, skip: string[] = []): Attrs {
  const out: Record<string, string> = {};
  for (const a of Array.from(el.attributes)) if (!skip.includes(a.name)) out[a.name] = a.value;
  return Object.keys(out).length ? out : null;
}

function label(n: Node): string {
  if (!isElement(n)) return n.nodeType === Node.COMMENT_NODE ? "comment" : "content";
  if (n.nodeName === "ac:structured-macro") return n.getAttribute("ac:name") ?? "macro";
  return n.nodeName;
}

function opaque(type: "opaqueBlock" | "opaqueInline", n: Node): JSONContent {
  return node(type, { xml: outerXml(n), label: label(n) });
}

// A container whose blocks the editor requires at least one of.
const nonEmpty = (blocks: JSONContent[]) => (blocks.length ? blocks : [node("paragraph", { bare: true })]);
// A list item and a task item must open with a paragraph; a bare empty one
// writes back as nothing.
const paragraphFirst = (blocks: JSONContent[]) =>
  blocks[0]?.type === "paragraph" ? blocks : [node("paragraph", { bare: true }), ...blocks];

function definitelyInline(n: Node): boolean {
  if (isText(n)) return !isBlank(n.textContent ?? "");
  if (!isElement(n)) return false;
  return INLINE.has(n.nodeName) || n.nodeName.startsWith("ri:");
}

// blockChildren reads a container that holds blocks. A run of inline content
// between blocks becomes a bare paragraph, written back without <p>, which
// is how "<li>Item<ul>...</ul></li>" survives. A run holding no text is its
// elements, each a block of its own; blank text between blocks is dropped.
function blockChildren(parent: Element): JSONContent[] {
  const out: JSONContent[] = [];
  let run: Node[] = [];
  const flush = () => {
    if (run.some(definitelyInline)) out.push(node("paragraph", { bare: true }, inlineNodes(run, [])));
    else for (const n of run) if (isElement(n) || n.nodeType === Node.COMMENT_NODE || n.nodeType === Node.CDATA_SECTION_NODE) out.push(opaque("opaqueBlock", n));
    run = [];
  };
  for (const child of Array.from(parent.childNodes)) {
    if (isElement(child) && BLOCK.has(child.nodeName)) {
      flush();
      out.push(block(child));
    } else {
      run.push(child);
    }
  }
  flush();
  return out;
}

function elementChildren(el: Element): Element[] | null {
  const out: Element[] = [];
  for (const c of Array.from(el.childNodes)) {
    if (isElement(c)) out.push(c);
    else if (isText(c) && isBlank(c.textContent ?? "")) continue;
    else return null;
  }
  return out;
}

function block(el: Element): JSONContent {
  const name = el.nodeName;
  if (name === "p") return node("paragraph", { extra: attrsOf(el) }, inlineNodes(Array.from(el.childNodes), []));
  if (/^h[1-6]$/.test(name)) return node("heading", { level: Number(name[1]), extra: attrsOf(el) }, inlineNodes(Array.from(el.childNodes), []));
  if (name === "ul" || name === "ol") return list(el);
  if (name === "blockquote") return node("blockquote", { extra: attrsOf(el) }, nonEmpty(blockChildren(el)));
  if (name === "hr") return el.attributes.length || el.childNodes.length ? opaque("opaqueBlock", el) : node("horizontalRule");
  if (name === "table") return table(el);
  if (name === "ac:task-list") return taskList(el);
  return opaque("opaqueBlock", el);
}

function list(el: Element): JSONContent {
  const items = elementChildren(el);
  if (!items || items.length === 0 || items.some((i) => i.nodeName !== "li")) return opaque("opaqueBlock", el);
  return node(el.nodeName === "ul" ? "bulletList" : "orderedList", { extra: attrsOf(el) },
    items.map((li) => node("listItem", { extra: attrsOf(li) }, paragraphFirst(nonEmpty(blockChildren(li))))));
}

// table models the shape Confluence writes: an optional colgroup, then rows
// directly or inside one tbody, cells of td and th. Anything else (thead, a
// caption, a second tbody) keeps the whole table opaque rather than
// rearranging it.
function table(el: Element): JSONContent {
  const children = elementChildren(el);
  if (!children) return opaque("opaqueBlock", el);
  let colgroup = "";
  let tbody = false;
  let rows: Element[] = [];
  for (const c of children) {
    if (c.nodeName === "colgroup" && !colgroup && !tbody && rows.length === 0) {
      colgroup = outerXml(c);
    } else if (c.nodeName === "tbody" && !tbody && rows.length === 0 && c.attributes.length === 0) {
      const inner = elementChildren(c);
      if (!inner) return opaque("opaqueBlock", el);
      tbody = true;
      rows = inner;
    } else if (c.nodeName === "tr" && !tbody) {
      rows.push(c);
    } else {
      return opaque("opaqueBlock", el);
    }
  }
  if (rows.length === 0 || rows.some((r) => r.nodeName !== "tr")) return opaque("opaqueBlock", el);
  const rowNodes: JSONContent[] = [];
  for (const r of rows) {
    const cells = elementChildren(r);
    if (!cells || cells.length === 0 || cells.some((c) => c.nodeName !== "td" && c.nodeName !== "th")) return opaque("opaqueBlock", el);
    rowNodes.push(node("tableRow", { extra: attrsOf(r) }, cells.map((c) => node(c.nodeName === "th" ? "tableHeader" : "tableCell", {
      colspan: Number(c.getAttribute("colspan") ?? 1),
      rowspan: Number(c.getAttribute("rowspan") ?? 1),
      extra: attrsOf(c, ["colspan", "rowspan"]),
    }, nonEmpty(blockChildren(c))))));
  }
  return node("table", { extra: attrsOf(el), colgroup, tbody }, rowNodes);
}

// taskList reads ac:task-list. A task's own children are an optional task-id,
// any elements Confluence adds before the status (kept as raw XML in their
// place), the status, and the body. Any other shape stays opaque.
function taskList(el: Element): JSONContent {
  const tasks = elementChildren(el);
  if (!tasks || tasks.length === 0 || tasks.some((t) => t.nodeName !== "ac:task")) return opaque("opaqueBlock", el);
  const items: JSONContent[] = [];
  for (const t of tasks) {
    const parts = elementChildren(t);
    if (!parts) return opaque("opaqueBlock", el);
    let taskId = "";
    let checked = false;
    let extraXml = "";
    let sawStatus = false;
    let body: Element | null = null;
    for (const p of parts) {
      if (p.nodeName === "ac:task-id" && !sawStatus && !body) taskId = p.textContent ?? "";
      else if (p.nodeName === "ac:task-status" && !sawStatus && !body) {
        sawStatus = true;
        checked = (p.textContent ?? "").trim() === "complete";
      } else if (p.nodeName === "ac:task-body" && sawStatus && !body) body = p;
      else if (!sawStatus && !body) extraXml += outerXml(p);
      else return opaque("opaqueBlock", el);
    }
    if (!body) return opaque("opaqueBlock", el);
    items.push(node("taskItem", { checked, taskId, extraXml }, paragraphFirst(nonEmpty(blockChildren(body)))));
  }
  return node("taskList", { extra: attrsOf(el) }, items);
}

function inlineNodes(nodes: Node[], marks: Mark[]): JSONContent[] {
  const out: JSONContent[] = [];
  const withMarks = (n: JSONContent): JSONContent => (marks.length ? { ...n, marks } : n);
  for (const n of nodes) {
    if (isText(n)) {
      const text = n.textContent ?? "";
      if (text) out.push(withMarks({ type: "text", text }));
      continue;
    }
    if (!isElement(n)) {
      out.push(withMarks(opaque("opaqueInline", n)));
      continue;
    }
    const name = n.nodeName;
    if (name === "br" && n.attributes.length === 0 && n.childNodes.length === 0) {
      out.push(withMarks({ type: "hardBreak" }));
      continue;
    }
    if (name === "a" && n.hasAttribute("href")) {
      out.push(...inlineNodes(Array.from(n.childNodes), [...marks, { type: "link", attrs: { href: n.getAttribute("href"), extra: attrsOf(n, ["href"]) } }]));
      continue;
    }
    const mark = MARKS[name];
    if (mark && n.attributes.length === 0) {
      out.push(...inlineNodes(Array.from(n.childNodes), [...marks, TAGGED.has(mark) ? { type: mark, attrs: { tag: name } } : { type: mark }]));
      continue;
    }
    out.push(withMarks(opaque("opaqueInline", n)));
  }
  return out;
}
```

- [ ] **Step 4: Write `serialize.ts`**

```ts
import type { JSONContent } from "@tiptap/core";
import { escapeAttr, escapeText } from "./xml";

type Mark = { type: string; attrs?: Record<string, unknown> };

// serializeStorage is parseStorage's inverse. Opaque nodes print the XML they
// were read from, unchanged; everything else is written in the shape
// Confluence writes itself.
export function serializeStorage(doc: JSONContent): string {
  return blocks(doc.content ?? []);
}

function attrs(extra: unknown, more: Record<string, string> = {}): string {
  const all: Record<string, string> = { ...((extra as Record<string, string> | null) ?? {}), ...more };
  return Object.entries(all).map(([k, v]) => ` ${k}="${escapeAttr(String(v))}"`).join("");
}

const isBare = (n: JSONContent | undefined) => n?.type === "paragraph" && n.attrs?.bare === true;

// A bare paragraph is written without <p> only when no other bare paragraph
// sits beside it. Parsing never puts two side by side; an edit that did
// would otherwise run their text together.
function blocks(nodes: JSONContent[]): string {
  return nodes.map((n, i) => (isBare(n) && !isBare(nodes[i - 1]) && !isBare(nodes[i + 1]) ? inline(n.content ?? []) : block(n))).join("");
}

function block(n: JSONContent): string {
  const a = n.attrs ?? {};
  const kids = n.content ?? [];
  switch (n.type) {
    case "paragraph": return `<p${attrs(a.extra)}>${inline(kids)}</p>`;
    case "heading": return `<h${a.level}${attrs(a.extra)}>${inline(kids)}</h${a.level}>`;
    case "bulletList": return `<ul${attrs(a.extra)}>${blocks(kids)}</ul>`;
    case "orderedList": return `<ol${attrs(a.extra)}>${blocks(kids)}</ol>`;
    case "listItem": return `<li${attrs(a.extra)}>${blocks(kids)}</li>`;
    case "blockquote": return `<blockquote${attrs(a.extra)}>${blocks(kids)}</blockquote>`;
    case "horizontalRule": return "<hr />";
    case "table": {
      const rows = blocks(kids);
      // A table made in the editor carries no tbody attribute; Confluence
      // writes one, so absent means yes.
      const tbody = a.tbody !== false;
      return `<table${attrs(a.extra)}>${a.colgroup ?? ""}${tbody ? `<tbody>${rows}</tbody>` : rows}</table>`;
    }
    case "tableRow": return `<tr${attrs(a.extra)}>${blocks(kids)}</tr>`;
    case "tableCell":
    case "tableHeader": {
      const tag = n.type === "tableHeader" ? "th" : "td";
      const spans: Record<string, string> = {};
      if (a.colspan && a.colspan !== 1) spans.colspan = String(a.colspan);
      if (a.rowspan && a.rowspan !== 1) spans.rowspan = String(a.rowspan);
      return `<${tag}${attrs(a.extra, spans)}>${blocks(kids)}</${tag}>`;
    }
    case "taskList": return `<ac:task-list${attrs(a.extra)}>${blocks(kids)}</ac:task-list>`;
    case "taskItem": {
      const id = a.taskId ? `<ac:task-id>${escapeText(String(a.taskId))}</ac:task-id>` : "";
      return `<ac:task>${id}${a.extraXml ?? ""}<ac:task-status>${a.checked ? "complete" : "incomplete"}</ac:task-status><ac:task-body>${blocks(kids)}</ac:task-body></ac:task>`;
    }
    case "opaqueBlock": return String(a.xml ?? "");
  }
  return "";
}

function tagOf(m: Mark, fallback: string): string {
  const tag = m.attrs?.tag;
  return typeof tag === "string" && tag ? tag : fallback;
}

function openTag(m: Mark): string {
  switch (m.type) {
    case "bold": return `<${tagOf(m, "strong")}>`;
    case "italic": return `<${tagOf(m, "em")}>`;
    case "strike": return `<${tagOf(m, "s")}>`;
    case "underline": return "<u>";
    case "code": return "<code>";
    case "link": return `<a${attrs(m.attrs?.extra, { href: String(m.attrs?.href ?? "") })}>`;
  }
  return "";
}

function closeTag(m: Mark): string {
  switch (m.type) {
    case "bold": return `</${tagOf(m, "strong")}>`;
    case "italic": return `</${tagOf(m, "em")}>`;
    case "strike": return `</${tagOf(m, "s")}>`;
    case "underline": return "</u>";
    case "code": return "</code>";
    case "link": return "</a>";
  }
  return "";
}

// markKey compares only what a mark writes, so a link the editor decorated
// with its own target and rel still closes and reopens where the page did.
function markKey(m: Mark): string {
  return `${m.type}|${m.type === "link" ? `${m.attrs?.href}|${JSON.stringify(m.attrs?.extra ?? null)}` : tagOf(m, "")}`;
}

// inline keeps a stack of open marks and closes only the ones that end, so
// "<strong>b <em>c</em></strong>" comes back as it went in rather than as a
// strong per text node.
function inline(nodes: JSONContent[]): string {
  let out = "";
  let open: Mark[] = [];
  for (const n of nodes) {
    const marks = ((n.marks ?? []) as Mark[]).filter((m) => openTag(m) !== "");
    let common = 0;
    while (common < open.length && common < marks.length && markKey(open[common]) === markKey(marks[common])) common++;
    for (let i = open.length - 1; i >= common; i--) out += closeTag(open[i]);
    for (let i = common; i < marks.length; i++) out += openTag(marks[i]);
    open = marks;
    if (n.type === "text") out += escapeText(n.text ?? "");
    else if (n.type === "hardBreak") out += "<br />";
    else if (n.type === "opaqueInline") out += String(n.attrs?.xml ?? "");
  }
  for (let i = open.length - 1; i >= 0; i--) out += closeTag(open[i]);
  return out;
}
```

- [ ] **Step 5: Run the tests and the typecheck**

Run: `cd tam/frontend && npx vitest run src/lib/storage && npm run typecheck`
Expected: PASS. If a hand-written corpus page fails, fix `parse.ts`/`serialize.ts`, never the page or `normalizeStorage`.

- [ ] **Step 6: Commit**

```bash
git add tam/frontend/src/lib/storage
git commit -m "feat(tam): Confluence storage format to an editor document and back"
```

---

### Task 13: Bindings, ritual sentences, and the Sync lock

**Files:**
- Modify: `tam/frontend/src/api.ts` (add types and bindings; old ones stay until Task 17)
- Create: `tam/frontend/src/lib/ritualText.ts`, `tam/frontend/src/lib/ritualText.test.ts`
- Modify: `tam/frontend/src/contexts/SyncContext.tsx`, `tam/frontend/src/contexts/SyncContext.test.tsx`

**Interfaces:**
- Consumes: Task 10 bindings (generated).
- Produces:
  - `api.ts`: `type RitualStatus`, `interface RitualDocument`, `interface RitualPageFailure`, `interface RitualSyncResult`, `interface RitualMacroPreview`; `EnsureSprintRituals`, `ListRitualDocuments`, `SaveRitualBody`, `ResolveRitualConflict`, `ForgetRitualPage`, `DeleteRitualDocument`, `RitualMacroIssues`, `StandupEntry`, `LastRitualSync`, `SyncRituals`
  - `ritualText.ts`: `RITUAL_ORDER`, `RITUAL_LABEL`, `STATUS_LABEL`, `syncSummary(r)`, `pendingLine(docs, lastSync)`, `conflictSentence(version)`, `editorStatusLine(input)`, `clock(iso)`, and the constants `GONE_SENTENCE`, `READ_ONLY_SENTENCE`, `MACRO_CAVEAT`, `UNCONFIGURED_SENTENCE`, `CLOSED_EMPTY_SENTENCE`, `NO_SCRUM_BOARD_SENTENCE`, `ENTRY_EXISTS`, `DAILY_LOG_MISSING`
  - `SyncContext`: `LockedOperation` gains `"rituals"`; `runRitualsSync(boardId: number): Promise<RitualSyncResult>`

- [ ] **Step 1: Add the types and bindings to `api.ts`**

After the existing ritual bindings (around `ScaffoldSprintRituals`), add:

```ts
// A ritual page as tam.db keeps it. body is the local page in Confluence
// storage format; baseBody the page as of version, the last one synced;
// conflictBody a newer remote a Sync found while local edits were pending.
export type RitualStatus = "local" | "synced" | "unsynced" | "conflict" | "gone";
export interface RitualDocument {
  profileId: string;
  boardId: number;
  sprintId: number;
  ritualType: string;
  title: string;
  body: string;
  baseBody: string;
  pageId: string;
  version: number;
  conflictBody: string;
  conflictVersion: number;
  status: RitualStatus;
  updatedAt: string;
  syncedAt: string;
}
export interface RitualPageFailure { sprintName: string; title: string; reason: string }
export interface RitualSyncResult {
  created: number;
  pulled: number;
  pushed: number;
  conflicts: number;
  gone: number;
  failed: RitualPageFailure[];
  syncedAt: string;
}
export interface RitualMacroPreview { supported: boolean; jql: string; issues: Issue[] }

export const EnsureSprintRituals: (profileId: string, boardId: number, sprintId: number) => Promise<RitualDocument[]> = App.EnsureSprintRituals as any;
export const ListRitualDocuments: (profileId: string, boardId: number, sprintId: number) => Promise<RitualDocument[]> = App.ListRitualDocuments as any;
export const SaveRitualBody: (profileId: string, boardId: number, sprintId: number, ritualType: string, body: string) => Promise<RitualDocument> = App.SaveRitualBody as any;
export const ResolveRitualConflict: (profileId: string, boardId: number, sprintId: number, ritualType: string, choice: "mine" | "theirs") => Promise<void> = App.ResolveRitualConflict;
export const ForgetRitualPage: (profileId: string, boardId: number, sprintId: number, ritualType: string) => Promise<void> = App.ForgetRitualPage;
export const DeleteRitualDocument: (profileId: string, boardId: number, sprintId: number, ritualType: string) => Promise<void> = App.DeleteRitualDocument;
export const RitualMacroIssues: (profileId: string, jql: string) => Promise<RitualMacroPreview> = App.RitualMacroIssues as any;
export const StandupEntry: (day: string) => Promise<string> = App.StandupEntry;
export const LastRitualSync: (profileId: string, boardId: number) => Promise<string> = App.LastRitualSync;
export const SyncRituals: (profileId: string, boardId: number) => Promise<RitualSyncResult> = App.SyncRituals as any;
```

- [ ] **Step 2: Write the failing `ritualText` tests**

```ts
import { describe, expect, it } from "vitest";
import type { RitualDocument } from "../api";
import { conflictSentence, editorStatusLine, pendingLine, syncSummary } from "./ritualText";

const doc = (status: RitualDocument["status"]): RitualDocument => ({
  profileId: "p1", boardId: 1, sprintId: 14, ritualType: "planning", title: "T", body: "", baseBody: "",
  pageId: "", version: 0, conflictBody: "", conflictVersion: 0, status, updatedAt: "2026-09-14T09:05:00Z", syncedAt: "2026-09-14T09:00:00Z",
});

describe("ritual text", () => {
  it("says what a Sync did, in the order it matters", () => {
    expect(syncSummary({ created: 5, pulled: 1, pushed: 2, conflicts: 1, gone: 0, failed: [{ sprintName: "S", title: "T", reason: "r" }], syncedAt: "" }))
      .toBe("Sync finished: 5 created, 2 pushed, 1 pulled, 1 in conflict, 1 failed.");
    expect(syncSummary({ created: 0, pulled: 0, pushed: 0, conflicts: 0, gone: 0, failed: [], syncedAt: "" }))
      .toBe("Sync finished. Everything was already in step.");
  });

  it("counts local and unsynced pages as unsynced, and conflicts apart", () => {
    const line = pendingLine([doc("local"), doc("unsynced"), doc("synced"), doc("conflict")], "");
    expect(line).toBe("Not synced yet · 2 unsynced · 1 conflict");
    expect(pendingLine([doc("synced")], "2026-09-14T09:00:00Z")).toMatch(/^Last synced .+/);
  });

  it("names the newer version in the conflict sentence", () => {
    expect(conflictSentence(8)).toBe("Confluence has a newer version (v8). Your local edits are kept until you choose.");
  });

  it("picks the editor status line from what is happening now", () => {
    expect(editorStatusLine({ doc: doc("unsynced"), saving: true, saveError: "", savedAt: "" })).toBe("Saving…");
    expect(editorStatusLine({ doc: doc("unsynced"), saving: false, saveError: "disk full", savedAt: "" })).toBe("Not saved: disk full");
    expect(editorStatusLine({ doc: doc("conflict"), saving: false, saveError: "", savedAt: "" })).toBe("Conflict with Confluence");
    expect(editorStatusLine({ doc: doc("synced"), saving: false, saveError: "", savedAt: "" })).toMatch(/^Synced .+/);
    expect(editorStatusLine({ doc: doc("local"), saving: false, saveError: "", savedAt: "" })).toMatch(/^Saved locally .+ · not synced$/);
  });

  it("uses no em dash anywhere", async () => {
    const text = await import("./ritualText");
    for (const value of Object.values(text)) if (typeof value === "string") expect(value).not.toContain(String.fromCharCode(0x2014));
  });
});
```

- [ ] **Step 3: Run to see it fail**

Run: `cd tam/frontend && npx vitest run src/lib/ritualText.test.ts`
Expected: FAIL, cannot resolve `./ritualText`.

- [ ] **Step 4: Write `ritualText.ts`**

```ts
import type { RitualDocument, RitualStatus, RitualSyncResult } from "../api";

// Every sentence the Rituals view and its editor print, in one place and
// testable without rendering anything, the way reportText does for Reports.

export const RITUAL_ORDER = ["_sprint", "planning", "standup", "review", "retro"] as const;

export const RITUAL_LABEL: Record<string, string> = {
  _sprint: "Overview", planning: "Planning", standup: "Standup", review: "Review", retro: "Retrospective",
};

export const STATUS_LABEL: Record<RitualStatus, string> = {
  local: "Local", synced: "Synced", unsynced: "Unsynced", conflict: "Conflict", gone: "Gone",
};

export const GONE_SENTENCE = "This page was deleted or moved in Confluence.";
export const READ_ONLY_SENTENCE = "This page has content TAM cannot edit safely. Edit it in Confluence; Sync will bring the changes back.";
export const MACRO_CAVEAT = "From TAM's cache. Done here means the status name; Confluence shows the live result.";
export const UNCONFIGURED_SENTENCE = "Add Confluence in Profile settings to sync.";
export const CLOSED_EMPTY_SENTENCE = "This sprint closed before it had ritual pages.";
export const NO_SCRUM_BOARD_SENTENCE = "No scrum board has been synced for this project, so there are no sprints to hold rituals.";
export const ENTRY_EXISTS = "Today's entry is already in the log.";
export const DAILY_LOG_MISSING = "The Daily log heading is gone, so there is nowhere to add today's entry.";

export function clock(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function conflictSentence(version: number): string {
  return `Confluence has a newer version (v${version}). Your local edits are kept until you choose.`;
}

export function syncSummary(r: RitualSyncResult): string {
  const parts: string[] = [];
  if (r.created) parts.push(`${r.created} created`);
  if (r.pushed) parts.push(`${r.pushed} pushed`);
  if (r.pulled) parts.push(`${r.pulled} pulled`);
  if (r.conflicts) parts.push(`${r.conflicts} in conflict`);
  if (r.gone) parts.push(`${r.gone} gone from Confluence`);
  if (r.failed.length) parts.push(`${r.failed.length} failed`);
  return parts.length ? `Sync finished: ${parts.join(", ")}.` : "Sync finished. Everything was already in step.";
}

export function pendingLine(docs: RitualDocument[], lastSync: string): string {
  const unsynced = docs.filter((d) => d.status === "local" || d.status === "unsynced").length;
  const conflicts = docs.filter((d) => d.status === "conflict").length;
  return [
    lastSync ? `Last synced ${clock(lastSync)}` : "Not synced yet",
    unsynced ? `${unsynced} unsynced` : "",
    conflicts ? `${conflicts} ${conflicts === 1 ? "conflict" : "conflicts"}` : "",
  ].filter(Boolean).join(" · ");
}

export function editorStatusLine(input: { doc: RitualDocument; saving: boolean; saveError: string; savedAt: string }): string {
  const { doc, saving, saveError, savedAt } = input;
  if (saving) return "Saving…";
  if (saveError) return `Not saved: ${saveError}`;
  if (doc.status === "conflict") return "Conflict with Confluence";
  if (doc.status === "gone") return "Gone from Confluence";
  if (doc.status === "synced") return `Synced ${clock(doc.syncedAt)}`;
  return `Saved locally ${clock(savedAt || doc.updatedAt)} · not synced`;
}
```

- [ ] **Step 5: Add `runRitualsSync` to `SyncContext`**

In `SyncContext.tsx`:
- import `SyncRituals` beside `SyncBoards`, and type `RitualSyncResult`;
- `export type LockedOperation = "sync" | "commit" | "boards refresh" | "sprint" | "report" | "rituals";`
- add to `SyncApi`, after `runReport`:

```ts
  // runRitualsSync is the Rituals view's Sync. It is several requests with no
  // modal holding focus, so it drives the banner the way a boards refresh
  // does rather than staying quiet: a Sync button left enabled and inert
  // beside it is the bug "One lock, both ends" was written about.
  runRitualsSync: (boardId: number) => Promise<RitualSyncResult>;
```

- add after `runReport`'s definition:

```ts
  const runRitualsSync = useCallback(async (boardId: number): Promise<RitualSyncResult> => {
    if (!activeId) throw new Error("no profile selected");
    if (statusRef.current !== "idle") throw busyRefusal();
    take("rituals");
    dispatch({
      type: "SYNC_START",
      clearError: true,
      initialProgress: { phase: "rituals", fetched: 0, total: 0, done: false, stage: "Syncing rituals with Confluence" },
    });
    try {
      return await call(() => SyncRituals(activeId, boardId));
    } finally {
      release();
      dispatch({ type: "SYNC_END" });
    }
  }, [activeId, busyRefusal, take, release]);
```

- add `runRitualsSync` to the context value object and its `useMemo` dependency list.

- [ ] **Step 6: Write the failing `SyncContext` test**

In `SyncContext.test.tsx`: add `SyncRituals: vi.fn(),` to the `vi.mock("../api", …)` factory; in `Probe`, destructure `running` and `runRitualsSync`, and add:

```tsx
      <span data-testid="running">{running ?? "none"}</span>
      <button onClick={() => void runRitualsSync(1).catch(() => {})}>Sync rituals</button>
```

Then add inside `describe("SyncProvider", …)`:

```tsx
  it("holds the lock under rituals while a rituals sync runs", async () => {
    let finish: (v: api.RitualSyncResult) => void = () => {};
    vi.mocked(api.SyncRituals).mockImplementation(
      () => new Promise<api.RitualSyncResult>((resolve) => { finish = resolve; }),
    );
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));

    await userEvent.click(screen.getByRole("button", { name: "Sync rituals" }));
    await waitFor(() => expect(screen.getByTestId("running")).toHaveTextContent("rituals"));
    expect(screen.getByTestId("status")).toHaveTextContent("syncing");
    expect(screen.getByTestId("stage")).toHaveTextContent("Syncing rituals with Confluence");
    expect(screen.getByRole("button", { name: "Sync" })).toBeDisabled();
    expect(api.SyncRituals).toHaveBeenCalledWith("p1", 1);

    await act(async () => {
      finish({ created: 5, pulled: 0, pushed: 0, conflicts: 0, gone: 0, failed: [], syncedAt: "2026-09-14T10:00:00Z" });
    });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
    expect(screen.getByTestId("running")).toHaveTextContent("none");
  });
```

- [ ] **Step 7: Run the tests and the typecheck**

Run: `cd tam/frontend && npx vitest run src/lib/ritualText.test.ts src/contexts/SyncContext.test.tsx && npm run typecheck`
Expected: PASS. (Components that mock `useSync` with a partial object keep compiling because their mocks are untyped `vi.hoisted` objects.)

- [ ] **Step 8: Commit**

```bash
git add tam/frontend/src/api.ts tam/frontend/src/lib/ritualText.ts tam/frontend/src/lib/ritualText.test.ts tam/frontend/src/contexts
git commit -m "feat(tam): the ritual bindings, their sentences, and the Sync lock"
```

---

### Task 14: The ritual editor

**Files:**
- Create: `tam/frontend/src/components/ritual-editor/context.ts`, `extensions.ts`, `OpaqueViews.tsx`, `RitualEditor.tsx`, `RitualEditor.test.tsx`
- Create: `tam/frontend/src/lib/standupLog.ts`, `tam/frontend/src/lib/standupLog.test.ts`
- Modify: `tam/frontend/src/App.css` (append the editor styles)

**Interfaces:**
- Consumes: Tasks 12 and 13.
- Produces:
  - `RitualEditorContext` (`{ profileId: string }`)
  - `ritualExtensions()` (the schema Task 12's JSON declares)
  - `OpaqueBlockView`, `OpaqueInlineView`
  - `findTodaysEntry(doc: PMNode, dayHeading: string): LogPlace`, `localDay(d: Date): string`, `DAILY_LOG_HEADING`
  - `RitualEditor` props `{ profileId: string; doc: RitualDocument; body?: string; readOnly?: boolean; pageUrl?: string; onSaved?: (doc: RitualDocument) => void; saveDelayMs?: number; ref?: Ref<RitualEditorHandle> }`; `interface RitualEditorHandle { flush: () => Promise<void>; editor: Editor | null }`; `SAVE_DELAY_MS = 800`

- [ ] **Step 1: Write `context.ts` and `extensions.ts`**

`context.ts`:

```ts
import { createContext } from "react";

// The profile a node view reads the cache for. Node views render through
// portals under EditorContent, so React context reaches them.
export const RitualEditorContext = createContext<{ profileId: string }>({ profileId: "" });
```

`extensions.ts`:

```ts
import { Extension, Node as TiptapNode, mergeAttributes } from "@tiptap/core";
import { ReactNodeViewRenderer } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { Table, TableCell, TableHeader, TableRow } from "@tiptap/extension-table";
import { TaskItem, TaskList } from "@tiptap/extension-list";
import { OpaqueBlockView, OpaqueInlineView } from "./OpaqueViews";

// hidden declares an attribute lib/storage writes that the page's HTML never
// shows. keepOnSplit is off so a new paragraph or task made by pressing
// Enter does not inherit a bare flag, an attribute bag, or a task id.
const hidden = (fallback: unknown) => ({ default: fallback, rendered: false, keepOnSplit: false });

// StorageAttributes declares every attribute lib/storage/parse puts on a
// node or a mark. ProseMirror silently drops an attribute its schema does
// not declare, so without this a page would lose its colgroup, its task ids,
// and every attribute TAM does not model the first time anybody edited it.
const StorageAttributes = Extension.create({
  name: "storageAttributes",
  addGlobalAttributes() {
    return [
      { types: ["paragraph"], attributes: { bare: hidden(null) } },
      { types: ["paragraph", "heading", "bulletList", "orderedList", "listItem", "blockquote", "tableRow", "tableCell", "tableHeader", "taskList"], attributes: { extra: hidden(null) } },
      { types: ["table"], attributes: { extra: hidden(null), colgroup: hidden(""), tbody: hidden(true) } },
      { types: ["taskItem"], attributes: { taskId: hidden(""), extraXml: hidden("") } },
      { types: ["bold", "italic", "strike"], attributes: { tag: hidden(null) } },
      { types: ["link"], attributes: { extra: hidden(null) } },
    ];
  },
});

const xmlAttributes = () => ({
  xml: { default: "", parseHTML: (el: HTMLElement) => el.getAttribute("data-xml") ?? "", renderHTML: (a: Record<string, unknown>) => ({ "data-xml": a.xml }) },
  label: { default: "", parseHTML: (el: HTMLElement) => el.getAttribute("data-label") ?? "", renderHTML: (a: Record<string, unknown>) => ({ "data-label": a.label }) },
});

// OpaqueBlock is content TAM does not model: a macro, a layout, anything.
// It can be moved and deleted whole, and never edited inside.
export const OpaqueBlock = TiptapNode.create({
  name: "opaqueBlock",
  group: "block",
  atom: true,
  selectable: true,
  draggable: true,
  addAttributes: xmlAttributes,
  parseHTML: () => [{ tag: "div[data-opaque-block]" }],
  renderHTML: ({ HTMLAttributes }) => ["div", mergeAttributes(HTMLAttributes, { "data-opaque-block": "" })],
  addNodeView: () => ReactNodeViewRenderer(OpaqueBlockView),
});

export const OpaqueInline = TiptapNode.create({
  name: "opaqueInline",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,
  addAttributes: xmlAttributes,
  parseHTML: () => [{ tag: "span[data-opaque-inline]" }],
  renderHTML: ({ HTMLAttributes }) => ["span", mergeAttributes(HTMLAttributes, { "data-opaque-inline": "" })],
  addNodeView: () => ReactNodeViewRenderer(OpaqueInlineView),
});

export function ritualExtensions() {
  return [
    // No code block (Confluence writes code as a macro, which stays opaque)
    // and no trailing node: that plugin appends a paragraph on load, which
    // would change a page nobody touched.
    StarterKit.configure({
      heading: { levels: [1, 2, 3, 4, 5, 6] },
      codeBlock: false,
      trailingNode: false,
      link: { openOnClick: false, autolink: false },
    }),
    Table.configure({ resizable: false }),
    TableRow,
    TableHeader,
    TableCell,
    TaskList,
    TaskItem.configure({ nested: true }),
    OpaqueBlock,
    OpaqueInline,
    StorageAttributes,
  ];
}
```

If `npm run typecheck` reports `trailingNode` is not a StarterKit option in 3.31.3, remove that line: the `touched` guard in `RitualEditor` (Step 5) already stops a load-time transaction from saving.

- [ ] **Step 2: Write `OpaqueViews.tsx`**

```tsx
import { NodeViewWrapper } from "@tiptap/react";
import type { NodeViewProps } from "@tiptap/react";

export function opaqueTitle(label: string): string {
  return label === "jira" ? "Jira issues" : `Confluence: ${label || "content"}`;
}

export function OpaqueBlockView({ node, selected }: NodeViewProps) {
  const label = String(node.attrs.label ?? "");
  return (
    <NodeViewWrapper className={`ritual-opaque${selected ? " is-selected" : ""}`} contentEditable={false} data-drag-handle>
      <span className="ritual-opaque-label">{opaqueTitle(label)}</span>
      <span className="muted small">Edit this in Confluence. TAM keeps it exactly as it is.</span>
    </NodeViewWrapper>
  );
}

export function OpaqueInlineView({ node }: NodeViewProps) {
  return (
    <NodeViewWrapper as="span" className="ritual-opaque-inline" contentEditable={false} title="Kept exactly as it is. Edit it in Confluence.">
      {String(node.attrs.label || "content")}
    </NodeViewWrapper>
  );
}
```

- [ ] **Step 3: Write the failing `standupLog` test, then `standupLog.ts`**

`standupLog.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { getSchema } from "@tiptap/core";
import { ritualExtensions } from "../components/ritual-editor/extensions";
import { parseStorage } from "./storage/parse";
import { findTodaysEntry, localDay } from "./standupLog";

const schema = getSchema(ritualExtensions());

function docOf(body: string) {
  const parsed = parseStorage(body);
  if (!parsed.ok) throw new Error(parsed.reason);
  return schema.nodeFromJSON(parsed.doc);
}

describe("standup log", () => {
  const log = "<h2>Blockers and work in flight</h2><p>x</p><h2>Daily log</h2><h3>Mon 14 Sep 2026</h3><p>y</p>";

  it("puts a new day straight under the Daily log heading", () => {
    const doc = docOf(log);
    const place = findTodaysEntry(doc, "Tue 15 Sep 2026");
    expect(place.kind).toBe("insert");
    if (place.kind === "insert") expect(doc.nodeAt(place.pos)?.textContent).toBe("Mon 14 Sep 2026");
  });

  it("refuses a second entry for the same day", () => {
    expect(findTodaysEntry(docOf(log), "Mon 14 Sep 2026").kind).toBe("exists");
  });

  it("does not count a same-named heading outside the log", () => {
    expect(findTodaysEntry(docOf("<h2>Daily log</h2><h2>Notes</h2><h3>Mon 14 Sep 2026</h3>"), "Mon 14 Sep 2026").kind).toBe("insert");
  });

  it("refuses when the Daily log heading is gone", () => {
    expect(findTodaysEntry(docOf("<h2>Something else</h2>"), "Mon 14 Sep 2026").kind).toBe("missing");
  });

  it("formats a local day for the binding", () => {
    expect(localDay(new Date(2026, 8, 5))).toBe("2026-09-05");
  });
});
```

`standupLog.ts`:

```ts
import type { Node as PMNode } from "@tiptap/pm/model";

export const DAILY_LOG_HEADING = "Daily log";

export type LogPlace = { kind: "insert"; pos: number } | { kind: "exists" } | { kind: "missing" };

// findTodaysEntry answers where "Add today's entry" puts a day: straight
// under the Daily log heading, so the newest day reads first. It refuses a
// second entry for the same day, and a page whose heading is gone, since a
// guessed place in a page somebody reorganised lands wherever the guess did.
export function findTodaysEntry(doc: PMNode, dayHeading: string): LogPlace {
  let insertAt = -1;
  let inLog = false;
  let exists = false;
  doc.forEach((child, offset) => {
    if (child.type.name !== "heading") return;
    const level = Number(child.attrs.level);
    const text = child.textContent.trim();
    if (level <= 2) {
      inLog = level === 2 && text === DAILY_LOG_HEADING;
      if (inLog && insertAt < 0) insertAt = offset + child.nodeSize;
      return;
    }
    if (inLog && level === 3 && text === dayHeading) exists = true;
  });
  if (insertAt < 0) return { kind: "missing" };
  return exists ? { kind: "exists" } : { kind: "insert", pos: insertAt };
}

export function localDay(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
```

Run: `cd tam/frontend && npx vitest run src/lib/standupLog.test.ts`
Expected: PASS.

- [ ] **Step 4: Write the failing `RitualEditor` tests**

```tsx
import { createRef } from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "../../api";
import { READ_ONLY_SENTENCE, ENTRY_EXISTS } from "../../lib/ritualText";
import { RitualEditor } from "./RitualEditor";
import type { RitualEditorHandle } from "./RitualEditor";

vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return { ...actual, SaveRitualBody: vi.fn(), StandupEntry: vi.fn(), RitualMacroIssues: vi.fn(), BrowserOpenURL: vi.fn() };
});

// jsdom has no layout; ProseMirror asks for rectangles when it scrolls a
// selection into view.
beforeAll(() => {
  Range.prototype.getBoundingClientRect = () => new DOMRect();
  Range.prototype.getClientRects = () => ({ length: 0, item: () => null, [Symbol.iterator]: [][Symbol.iterator] }) as unknown as DOMRectList;
  document.elementFromPoint = () => null;
});

function doc(over: Partial<api.RitualDocument> = {}): api.RitualDocument {
  return {
    profileId: "p1", boardId: 1, sprintId: 14, ritualType: "planning", title: "Sprint 14 · Planning",
    body: "<h2>Decisions</h2><ul><li>ship it</li></ul>", baseBody: "", pageId: "", version: 0,
    conflictBody: "", conflictVersion: 0, status: "local", updatedAt: "2026-09-14T09:00:00Z", syncedAt: "", ...over,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.SaveRitualBody).mockImplementation(async (_p, _b, _s, _t, body) => doc({ body, status: "local" }));
});

const pause = (ms: number) => act(() => new Promise((r) => setTimeout(r, ms)));

describe("RitualEditor", () => {
  it("never saves a page just because it was opened", async () => {
    render(<RitualEditor profileId="p1" doc={doc()} saveDelayMs={10} />);
    expect(await screen.findByText("ship it")).toBeInTheDocument();
    await pause(50);
    expect(api.SaveRitualBody).not.toHaveBeenCalled();
  });

  it("saves once after an edit pauses, and hands the saved row back", async () => {
    const ref = createRef<RitualEditorHandle>();
    const onSaved = vi.fn();
    const { container } = render(<RitualEditor ref={ref} profileId="p1" doc={doc()} onSaved={onSaved} saveDelayMs={10} />);
    await screen.findByText("ship it");
    fireEvent.keyDown(container.querySelector(".ProseMirror")!, { key: "a" });
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>hello</p>"); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.SaveRitualBody).mock.calls[0]).toEqual(["p1", 1, 14, "planning", "<h2>Decisions</h2><ul><li>ship it</li></ul><p>hello</p>"]);
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });

  it("flushes a pending edit immediately when asked", async () => {
    const ref = createRef<RitualEditorHandle>();
    const { container } = render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={60_000} />);
    await screen.findByText("ship it");
    fireEvent.keyDown(container.querySelector(".ProseMirror")!, { key: "a" });
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>now</p>"); });
    await act(() => ref.current!.flush());
    expect(api.SaveRitualBody).toHaveBeenCalledTimes(1);
  });

  it("opens a page it cannot read as read only, with the page link", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ body: "<p>&bogus;</p>" })} pageUrl="https://c.example.com/pages/viewpage.action?pageId=42" />);
    expect(screen.getByText(READ_ONLY_SENTENCE)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Open in Confluence" }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://c.example.com/pages/viewpage.action?pageId=42");
    expect(document.querySelector(".ProseMirror")).toBeNull();
  });

  it("draws content it does not model as a locked block", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ body: '<ac:structured-macro ac:name="panel"><ac:rich-text-body><p>x</p></ac:rich-text-body></ac:structured-macro>' })} />);
    expect(await screen.findByText("Confluence: panel")).toBeInTheDocument();
  });

  it("adds today's standup entry under the log once, and refuses a second", async () => {
    vi.mocked(api.StandupEntry).mockResolvedValue("<h3>Tue 15 Sep 2026</h3><p><strong>Yesterday</strong></p><ul><li></li></ul>");
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" saveDelayMs={10}
      doc={doc({ ritualType: "standup", body: "<h2>Daily log</h2><h3>Mon 14 Sep 2026</h3><p>y</p>" })} />);
    await screen.findByText("Mon 14 Sep 2026");
    await userEvent.click(screen.getByRole("button", { name: "Add today's entry" }));
    await waitFor(() => expect(ref.current!.editor!.state.doc.child(1).textContent).toBe("Tue 15 Sep 2026"));
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalled());
    await userEvent.click(screen.getByRole("button", { name: "Add today's entry" }));
    expect(await screen.findByText(ENTRY_EXISTS)).toBeInTheDocument();
  });

  it("offers Add today's entry only on the standup", async () => {
    render(<RitualEditor profileId="p1" doc={doc()} />);
    await screen.findByText("ship it");
    expect(screen.queryByRole("button", { name: "Add today's entry" })).toBeNull();
  });
});
```

Run: `cd tam/frontend && npx vitest run src/components/ritual-editor/RitualEditor.test.tsx`
Expected: FAIL, cannot resolve `./RitualEditor`.

- [ ] **Step 5: Write `RitualEditor.tsx`**

```tsx
import { useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import type { KeyboardEvent, Ref } from "react";
import { EditorContent, useEditor, useEditorState } from "@tiptap/react";
import type { ChainedCommands, Editor } from "@tiptap/core";
import { announce, errMsg } from "@agile-suite/core";
import { BrowserOpenURL, SaveRitualBody, StandupEntry } from "../../api";
import type { RitualDocument } from "../../api";
import { parseStorage } from "../../lib/storage/parse";
import { serializeStorage } from "../../lib/storage/serialize";
import { sanitizeHtml } from "../../lib/sanitizeHtml";
import { findTodaysEntry, localDay } from "../../lib/standupLog";
import { DAILY_LOG_MISSING, ENTRY_EXISTS, READ_ONLY_SENTENCE, editorStatusLine } from "../../lib/ritualText";
import { RitualEditorContext } from "./context";
import { ritualExtensions } from "./extensions";

export const SAVE_DELAY_MS = 800;

export interface RitualEditorHandle {
  flush: () => Promise<void>;
  editor: Editor | null;
}

interface Props {
  profileId: string;
  doc: RitualDocument;
  // body defaults to doc.body; the conflict view passes conflictBody with readOnly.
  body?: string;
  readOnly?: boolean;
  pageUrl?: string;
  onSaved?: (doc: RitualDocument) => void;
  saveDelayMs?: number;
  ref?: Ref<RitualEditorHandle>;
}

type Run = (chain: ChainedCommands) => ChainedCommands;

// RitualEditor edits one ritual page. It saves locally, 800 ms after the last
// edit, on Ctrl+S, on flush, and on unmount; it never talks to Confluence.
// Only an edit a person made counts: loading content, and anything a plugin
// does on load, never saves, so opening a page never makes it unsynced.
export function RitualEditor({ profileId, doc, body = doc.body, readOnly = false, pageUrl, onSaved, saveDelayMs = SAVE_DELAY_MS, ref }: Props) {
  const parsed = useMemo(() => parseStorage(body), [body]);
  const editable = !readOnly && parsed.ok;
  const extensions = useMemo(() => ritualExtensions(), []);
  const touched = useRef(false);
  const pending = useRef<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const [saving, setSaving] = useState(false);
  const [savedAt, setSavedAt] = useState("");
  const [saveError, setSaveError] = useState("");
  const [notice, setNotice] = useState("");

  const save = useCallback(async () => {
    clearTimeout(timer.current);
    const next = pending.current;
    if (next === null) return;
    pending.current = null;
    setSaving(true);
    setSaveError("");
    try {
      const saved = await SaveRitualBody(profileId, doc.boardId, doc.sprintId, doc.ritualType, next);
      setSavedAt(new Date().toISOString());
      onSaved?.(saved);
    } catch (e) {
      // Keep the text for the next try, unless a newer edit already replaced it.
      if (pending.current === null) pending.current = next;
      setSaveError(errMsg(e));
    } finally {
      setSaving(false);
    }
  }, [profileId, doc.boardId, doc.sprintId, doc.ritualType, onSaved]);

  const saveRef = useRef(save);
  useEffect(() => { saveRef.current = save; }, [save]);

  const editor = useEditor({
    extensions,
    content: parsed.ok ? parsed.doc : "",
    editable,
    onUpdate: ({ editor: current }) => {
      if (!editable || !touched.current) return;
      pending.current = serializeStorage(current.getJSON());
      clearTimeout(timer.current);
      timer.current = setTimeout(() => void saveRef.current(), saveDelayMs);
    },
  }, [body, editable]);

  useImperativeHandle(ref, () => ({ flush: () => saveRef.current(), editor }), [editor]);

  // Leaving a page, a sprint, or the view saves what was typed.
  useEffect(() => () => {
    if (pending.current !== null) void saveRef.current();
  }, []);

  const act = (run: Run) => {
    if (!editor) return;
    touched.current = true;
    run(editor.chain().focus()).run();
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      void save();
      return;
    }
    touched.current = true;
  };

  async function addTodaysEntry() {
    if (!editor) return;
    setNotice("");
    const fragment = parseStorage(await StandupEntry(localDay(new Date())));
    if (!fragment.ok || !fragment.doc.content?.length) return;
    const heading = editor.schema.nodeFromJSON(fragment.doc.content[0]).textContent.trim();
    const place = findTodaysEntry(editor.state.doc, heading);
    if (place.kind !== "insert") {
      const sentence = place.kind === "exists" ? ENTRY_EXISTS : DAILY_LOG_MISSING;
      setNotice(sentence);
      announce(sentence);
      return;
    }
    touched.current = true;
    editor.chain().insertContentAt(place.pos, fragment.doc.content).run();
  }

  if (!parsed.ok) {
    return (
      <div className="ritual-readonly">
        <p className="warn-text">{READ_ONLY_SENTENCE}</p>
        {pageUrl && <button className="btn btn-ghost" onClick={() => BrowserOpenURL(pageUrl)}>Open in Confluence</button>}
        <div className="ritual-page-body" dangerouslySetInnerHTML={{ __html: sanitizeHtml(body) }} />
      </div>
    );
  }

  return (
    <RitualEditorContext.Provider value={{ profileId }}>
      <div
        className={`ritual-editor${readOnly ? " ritual-editor-readonly" : ""}`}
        onKeyDownCapture={onKeyDown}
        onPasteCapture={() => { touched.current = true; }}
        onDropCapture={() => { touched.current = true; }}
      >
        {editable && editor && <Toolbar editor={editor} act={act} standup={doc.ritualType === "standup"} onAddEntry={() => void addTodaysEntry()} />}
        {notice && <p className="muted small ritual-notice" role="status">{notice}</p>}
        <EditorContent editor={editor} className="ritual-editor-content" />
        {editable && <p className="ritual-status-line muted small">{editorStatusLine({ doc, saving, saveError, savedAt })}</p>}
      </div>
    </RitualEditorContext.Provider>
  );
}

function Toolbar({ editor, act, standup, onAddEntry }: { editor: Editor; act: (run: Run) => void; standup: boolean; onAddEntry: () => void }) {
  const active = useEditorState({
    editor,
    selector: ({ editor: e }) => ({
      bold: e.isActive("bold"),
      italic: e.isActive("italic"),
      h2: e.isActive("heading", { level: 2 }),
      h3: e.isActive("heading", { level: 3 }),
      bullet: e.isActive("bulletList"),
      ordered: e.isActive("orderedList"),
      tasks: e.isActive("taskList"),
      link: e.isActive("link"),
    }),
  });
  const [href, setHref] = useState<string | null>(null);

  const buttons: { label: string; text: string; run: Run; pressed?: boolean }[] = [
    { label: "Bold", text: "B", run: (c) => c.toggleBold(), pressed: active.bold },
    { label: "Italic", text: "I", run: (c) => c.toggleItalic(), pressed: active.italic },
    { label: "Heading 2", text: "H2", run: (c) => c.toggleHeading({ level: 2 }), pressed: active.h2 },
    { label: "Heading 3", text: "H3", run: (c) => c.toggleHeading({ level: 3 }), pressed: active.h3 },
    { label: "Bullet list", text: "• List", run: (c) => c.toggleBulletList(), pressed: active.bullet },
    { label: "Numbered list", text: "1. List", run: (c) => c.toggleOrderedList(), pressed: active.ordered },
    { label: "Task list", text: "☐ Tasks", run: (c) => c.toggleTaskList(), pressed: active.tasks },
    { label: "Insert table", text: "Table", run: (c) => c.insertTable({ rows: 3, cols: 3, withHeaderRow: true }) },
    { label: "Undo", text: "Undo", run: (c) => c.undo() },
    { label: "Redo", text: "Redo", run: (c) => c.redo() },
  ];

  function applyLink() {
    const value = (href ?? "").trim();
    act((c) => (value ? c.extendMarkRange("link").setLink({ href: value }) : c.extendMarkRange("link").unsetLink()));
    setHref(null);
  }

  return (
    <div className="ritual-toolbar" role="toolbar" aria-label="Formatting">
      {buttons.map((b) => (
        <button key={b.label} type="button" className="btn btn-ghost" aria-label={b.label}
          aria-pressed={b.pressed === undefined ? undefined : b.pressed}
          onMouseDown={(e) => e.preventDefault()} onClick={() => act(b.run)}>
          {b.text}
        </button>
      ))}
      <button type="button" className="btn btn-ghost" aria-label="Link" aria-pressed={active.link}
        onMouseDown={(e) => e.preventDefault()}
        onClick={() => setHref(href === null ? String(editor.getAttributes("link").href ?? "") : null)}>
        Link
      </button>
      {href !== null && (
        <span className="ritual-link-input">
          <input aria-label="Link address" value={href} placeholder="https://" onChange={(e) => setHref(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); applyLink(); } if (e.key === "Escape") setHref(null); }} />
          <button type="button" className="btn btn-ghost" onClick={applyLink}>Apply</button>
        </span>
      )}
      {standup && <button type="button" className="btn btn-ghost ritual-add-entry" onClick={onAddEntry}>Add today's entry</button>}
    </div>
  );
}
```

- [ ] **Step 6: Append the editor styles to `App.css`**

```css
/* The ritual editor: a toolbar, one scroller, and a status line. */
.ritual-page { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
.ritual-page-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 8px 12px; border-bottom: 1px solid var(--border); }
.ritual-page-head h3 { margin: 0; font-size: 15px; }
.ritual-editor { display: flex; flex-direction: column; flex: 1; min-height: 0; }
.ritual-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 2px; padding: 4px 8px; border-bottom: 1px solid var(--border); background: var(--surface-2); }
.ritual-toolbar .btn[aria-pressed="true"] { background: var(--accent-soft); color: var(--accent); }
.ritual-toolbar .ritual-add-entry { margin-left: auto; }
.ritual-link-input { display: inline-flex; gap: 4px; align-items: center; }
.ritual-link-input input { width: 220px; }
.ritual-editor-content { flex: 1; min-height: 0; overflow: auto; padding: 12px 16px; }
.ritual-editor-content .ProseMirror { outline: none; min-height: 100%; }
.ritual-editor-content table { border-collapse: collapse; margin: 8px 0; }
.ritual-editor-content th, .ritual-editor-content td { border: 1px solid var(--border); padding: 4px 8px; min-width: 64px; vertical-align: top; }
.ritual-editor-content th { background: var(--surface-2); }
.ritual-editor-content ul[data-type="taskList"] { list-style: none; padding-left: 2px; }
.ritual-editor-content ul[data-type="taskList"] li { display: flex; gap: 6px; align-items: flex-start; }
.ritual-editor-content ul[data-type="taskList"] li > div { flex: 1; }
.ritual-opaque { display: block; border: 1px dashed var(--border-strong); border-radius: 6px; padding: 8px 10px; margin: 8px 0; background: var(--surface-sunken); }
.ritual-opaque.is-selected { outline: 2px solid var(--accent); }
.ritual-opaque-label { display: block; font-weight: 600; font-size: 12px; margin-bottom: 4px; color: var(--text-strong); }
.ritual-opaque-inline { border: 1px dashed var(--border-strong); border-radius: 4px; padding: 0 4px; font-size: 12px; background: var(--surface-sunken); }
.ritual-status-line { margin: 0; padding: 4px 16px; border-top: 1px solid var(--border-subtle); }
.ritual-notice { margin: 0; padding: 4px 16px; }
.ritual-readonly { padding: 12px 16px; overflow: auto; min-height: 0; }
```

- [ ] **Step 7: Run the editor tests and the typecheck**

Run: `cd tam/frontend && npx vitest run src/components/ritual-editor src/lib/standupLog.test.ts && npm run typecheck`
Expected: PASS.

If the save test's serialized body differs only because TipTap normalised the inserted paragraph (for example an `extra` of `null` written as an empty attribute), fix `serialize.ts` so an absent or null `extra` writes nothing, rather than loosening the assertion.

- [ ] **Step 8: Commit**

```bash
git add tam/frontend/src/components/ritual-editor tam/frontend/src/lib/standupLog.ts tam/frontend/src/lib/standupLog.test.ts tam/frontend/src/App.css
git commit -m "feat(tam): an editor for ritual pages that saves locally"
```

---

### Task 15: The Jira macro preview

**Files:**
- Create: `tam/frontend/src/components/ritual-editor/MacroPreview.tsx`, `tam/frontend/src/components/ritual-editor/MacroPreview.test.tsx`
- Modify: `tam/frontend/src/components/ritual-editor/OpaqueViews.tsx`

**Interfaces:**
- Consumes: `RitualMacroIssues` (Task 13), `RitualEditorContext` (Task 14), `parseXml` (Task 11), `MACRO_CAVEAT`.
- Produces: `macroJql(xml: string): string`, `MacroPreview({ xml })`, `PREVIEW_LIMIT = 50`.

- [ ] **Step 1: Write the failing tests**

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import * as api from "../../api";
import { MACRO_CAVEAT } from "../../lib/ritualText";
import { RitualEditorContext } from "./context";
import { MacroPreview, macroJql } from "./MacroPreview";

vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return { ...actual, RitualMacroIssues: vi.fn() };
});

const macro = (jql: string) =>
  `<ac:structured-macro ac:name="jira"><ac:parameter ac:name="jqlQuery">${jql}</ac:parameter><ac:parameter ac:name="columns">key,summary</ac:parameter></ac:structured-macro>`;

function renderPreview(xml: string) {
  return render(
    <RitualEditorContext.Provider value={{ profileId: "p1" }}>
      <MacroPreview xml={xml} />
    </RitualEditorContext.Provider>,
  );
}

beforeEach(() => vi.clearAllMocks());

describe("MacroPreview", () => {
  it("reads the query out of the macro", () => {
    expect(macroJql(macro("sprint = 14 AND statusCategory != Done"))).toBe("sprint = 14 AND statusCategory != Done");
    expect(macroJql('<ac:structured-macro ac:name="jira"/>')).toBe("");
  });

  it("previews a form it knows from the cache, with the caveat", async () => {
    vi.mocked(api.RitualMacroIssues).mockResolvedValue({
      supported: true, jql: "sprint = 14 ORDER BY Rank",
      issues: [
        { key: "PLAT-1", summary: "Checkout", status: "Done", assignee: "R. Anand" },
        { key: "PLAT-2", summary: "Payments", status: "In Progress", assignee: "" },
      ] as api.Issue[],
    });
    renderPreview(macro("sprint = 14 ORDER BY Rank"));
    expect(await screen.findByText("PLAT-2")).toBeInTheDocument();
    expect(screen.getByText(MACRO_CAVEAT)).toBeInTheDocument();
    expect(api.RitualMacroIssues).toHaveBeenCalledWith("p1", "sprint = 14 ORDER BY Rank");
  });

  it("shows any other query as text Confluence renders", async () => {
    vi.mocked(api.RitualMacroIssues).mockResolvedValue({ supported: false, jql: "project = PLAT", issues: [] });
    renderPreview(macro("project = PLAT"));
    expect(await screen.findByText("project = PLAT")).toBeInTheDocument();
    expect(screen.getByText("Rendered in Confluence")).toBeInTheDocument();
    expect(screen.queryByText(MACRO_CAVEAT)).toBeNull();
  });
});
```

Run: `cd tam/frontend && npx vitest run src/components/ritual-editor/MacroPreview.test.tsx`
Expected: FAIL, cannot resolve `./MacroPreview`.

- [ ] **Step 2: Write `MacroPreview.tsx`**

```tsx
import { useContext, useEffect, useState } from "react";
import { RitualMacroIssues } from "../../api";
import type { RitualMacroPreview } from "../../api";
import { MACRO_CAVEAT } from "../../lib/ritualText";
import { parseXml } from "../../lib/storage/xml";
import { RitualEditorContext } from "./context";

export const PREVIEW_LIMIT = 50;

export function macroJql(xml: string): string {
  const macro = parseXml(xml)?.firstElementChild;
  if (!macro) return "";
  for (const p of Array.from(macro.children)) {
    if (p.nodeName === "ac:parameter" && p.getAttribute("ac:name") === "jqlQuery") return (p.textContent ?? "").trim();
  }
  return "";
}

// MacroPreview draws a Jira Issues macro from TAM's cache, so a ritual page
// shows its issues offline. Only the three forms the templates write are
// answered; the caveat says the cache's "done" is not Jira's statusCategory.
export function MacroPreview({ xml }: { xml: string }) {
  const { profileId } = useContext(RitualEditorContext);
  const jql = macroJql(xml);
  const [preview, setPreview] = useState<RitualMacroPreview | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    setPreview(null);
    setError("");
    if (!profileId || !jql) return;
    RitualMacroIssues(profileId, jql)
      .then((p) => { if (live) setPreview(p); })
      .catch((e) => { if (live) setError(String(e)); });
    return () => { live = false; };
  }, [profileId, jql]);

  if (!jql) return <span className="muted small">Rendered in Confluence</span>;
  if (error) return <span className="warn-text small">{error}</span>;
  if (!preview) return <span className="muted small">Reading the cache</span>;
  if (!preview.supported) {
    return <span className="small"><code>{jql}</code> <span className="muted">Rendered in Confluence</span></span>;
  }
  const shown = preview.issues.slice(0, PREVIEW_LIMIT);
  return (
    <div className="ritual-macro-preview">
      <code className="small">{jql}</code>
      {shown.length === 0 ? (
        <p className="muted small">No cached issues match.</p>
      ) : (
        <table className="ritual-macro-table">
          <thead><tr><th>Key</th><th>Summary</th><th>Status</th><th>Assignee</th></tr></thead>
          <tbody>
            {shown.map((i) => <tr key={i.key}><td>{i.key}</td><td>{i.summary}</td><td>{i.status}</td><td>{i.assignee}</td></tr>)}
          </tbody>
        </table>
      )}
      <p className="muted small">{MACRO_CAVEAT}</p>
    </div>
  );
}
```

- [ ] **Step 3: Use it for `jira` blocks** in `OpaqueViews.tsx`

Import `MacroPreview` and replace the body of `OpaqueBlockView`'s wrapper:

```tsx
      <span className="ritual-opaque-label">{opaqueTitle(label)}</span>
      {label === "jira"
        ? <MacroPreview xml={String(node.attrs.xml ?? "")} />
        : <span className="muted small">Edit this in Confluence. TAM keeps it exactly as it is.</span>}
```

Append to `App.css`:

```css
.ritual-macro-table { border-collapse: collapse; margin: 6px 0; font-size: 12px; }
.ritual-macro-table th, .ritual-macro-table td { border: 1px solid var(--border-subtle); padding: 2px 6px; text-align: left; }
```

- [ ] **Step 4: Run the editor folder's tests**

Run: `cd tam/frontend && npx vitest run src/components/ritual-editor && npm run typecheck`
Expected: PASS. (`RitualEditor.test.tsx` mocks `RitualMacroIssues`; its bodies hold no `jira` macro, so the mock is never awaited.)

- [ ] **Step 5: Commit**

```bash
git add tam/frontend/src/components/ritual-editor tam/frontend/src/App.css
git commit -m "feat(tam): Jira issue macros previewed from the cache"
```

---

### Task 16: The Rituals view, rebuilt

**Files:**
- Rewrite: `tam/frontend/src/components/RitualsView.tsx`
- Rewrite: `tam/frontend/src/components/RitualsView.test.tsx`
- Modify: `tam/frontend/src/App.css` (replace the old `.rituals-*`/`.ritual-slot*`/`.ritual-link*`/`.ritual-template*` rules)

**Interfaces:**
- Consumes: Tasks 13 to 15; `useSync().runRitualsSync`, `useSync().running`; `GetConfluenceConfig`, `ListBoards`, `ListBoardSprints`, `EnsureSprintRituals`, `LastRitualSync`, `ResolveRitualConflict`, `ForgetRitualPage`, `DeleteRitualDocument`, `BrowserOpenURL`.
- Produces: `RitualsView()` (same export name `App.tsx` already renders).

- [ ] **Step 1: Write the failing tests** (replace `RitualsView.test.tsx` entirely)

```tsx
import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { CLOSED_EMPTY_SENTENCE, GONE_SENTENCE, NO_SCRUM_BOARD_SENTENCE, UNCONFIGURED_SENTENCE } from "../lib/ritualText";
import { RitualsView } from "./RitualsView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(), GetSettings: vi.fn(), GetConfluenceConfig: vi.fn(), ListBoards: vi.fn(), ListBoardSprints: vi.fn(),
    EnsureSprintRituals: vi.fn(), LastRitualSync: vi.fn(), ResolveRitualConflict: vi.fn(), ForgetRitualPage: vi.fn(),
    DeleteRitualDocument: vi.fn(), BrowserOpenURL: vi.fn(),
  };
});

const sync = vi.hoisted(() => ({ running: null as string | null, runRitualsSync: vi.fn() }));
vi.mock("../contexts/SyncContext", () => ({ useSync: () => sync }));

// The editor has its own tests; here it only has to show which body and mode it was handed.
vi.mock("./ritual-editor/RitualEditor", () => ({
  RitualEditor: (p: { doc: api.RitualDocument; body?: string; readOnly?: boolean }) => (
    <div data-testid="editor" data-type={p.doc.ritualType} data-readonly={String(!!p.readOnly)}>{p.body ?? p.doc.body}</div>
  ),
}));

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
          <RitualsView />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

function docFor(ritualType: string, over: Partial<api.RitualDocument> = {}): api.RitualDocument {
  return {
    profileId: "p1", boardId: 1, sprintId: 14, ritualType, title: `Sprint 14 · ${ritualType}`, body: `<p>${ritualType} body</p>`,
    baseBody: "", pageId: "", version: 0, conflictBody: "", conflictVersion: 0, status: "local",
    updatedAt: "2026-09-14T09:00:00Z", syncedAt: "", ...over,
  };
}
const five = (over: Record<string, Partial<api.RitualDocument>> = {}) =>
  ["_sprint", "planning", "standup", "review", "retro"].map((t) => docFor(t, over[t]));

beforeEach(() => {
  vi.clearAllMocks();
  sync.running = null;
  vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Acme", jiraUrl: "https://jira.example.com", projectKey: "PLAT", backend: "jira", createdAt: "" }]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetConfluenceConfig).mockResolvedValue({ baseURL: "https://confluence.example.com", spaceKey: "PLAT", rootPageID: "100" });
  vi.mocked(api.ListBoards).mockResolvedValue([{ id: 1, name: "PLAT board", type: "scrum" }] as api.Board[]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue([
    { id: 13, boardId: 1, name: "Sprint 13", state: "closed", startDate: "", endDate: "", goal: "", completeDate: "" },
    { id: 14, boardId: 1, name: "Sprint 14", state: "active", startDate: "", endDate: "", goal: "", completeDate: "" },
  ] as api.Sprint[]);
  vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five());
  vi.mocked(api.LastRitualSync).mockResolvedValue("");
});

describe("RitualsView", () => {
  it("lists the active sprint's five pages with their statuses and opens Planning", async () => {
    vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five({ review: { status: "conflict" }, retro: { status: "synced" } }));
    renderView();
    const nav = await screen.findByRole("navigation", { name: "Ritual documents" });
    for (const label of ["Overview", "Planning", "Standup", "Review", "Retrospective"]) {
      expect(within(nav).getByRole("button", { name: new RegExp(label) })).toBeInTheDocument();
    }
    expect(within(nav).getByRole("button", { name: /Review/ })).toHaveTextContent("Conflict");
    expect(api.EnsureSprintRituals).toHaveBeenCalledWith("p1", 1, 14);
    expect(screen.getByTestId("editor")).toHaveAttribute("data-type", "planning");
    expect(screen.getByText("Not synced yet · 3 unsynced · 1 conflict")).toBeInTheDocument();
  });

  it("keeps editing available without Confluence and says how to sync", async () => {
    vi.mocked(api.GetConfluenceConfig).mockResolvedValue({ baseURL: "", spaceKey: "", rootPageID: "" });
    renderView();
    expect(await screen.findByTestId("editor")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sync rituals" })).toBeDisabled();
    expect(screen.getByText(UNCONFIGURED_SENTENCE)).toBeInTheDocument();
  });

  it("syncs through the lock, reports what happened, and reloads", async () => {
    sync.runRitualsSync.mockResolvedValue({ created: 5, pulled: 0, pushed: 0, conflicts: 0, gone: 0,
      failed: [{ sprintName: "Sprint 14", title: "Sprint 14 · Review", reason: "403 Forbidden" }], syncedAt: "2026-09-14T10:00:00Z" });
    renderView();
    await screen.findByTestId("editor");
    await userEvent.click(screen.getByRole("button", { name: "Sync rituals" }));
    expect(sync.runRitualsSync).toHaveBeenCalledWith(1);
    expect(await screen.findByText("Sync finished: 5 created, 1 failed.")).toBeInTheDocument();
    expect(screen.getByText("403 Forbidden", { exact: false })).toBeInTheDocument();
    await waitFor(() => expect(api.EnsureSprintRituals).toHaveBeenCalledTimes(2));
  });

  it("shows theirs read only, keeps mine, and confirms before taking theirs", async () => {
    vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five({
      planning: { status: "conflict", pageId: "42", version: 1, conflictVersion: 3, conflictBody: "<p>their planning</p>" },
    }));
    renderView();
    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent("Confluence has a newer version (v3). Your local edits are kept until you choose.");

    await userEvent.click(within(banner).getByRole("button", { name: "View theirs" }));
    expect(screen.getByTestId("editor")).toHaveTextContent("their planning");
    expect(screen.getByTestId("editor")).toHaveAttribute("data-readonly", "true");

    await userEvent.click(within(banner).getByRole("button", { name: "Keep mine" }));
    await waitFor(() => expect(api.ResolveRitualConflict).toHaveBeenCalledWith("p1", 1, 14, "planning", "mine"));

    await userEvent.click(within(screen.getByRole("alert")).getByRole("button", { name: "Take theirs" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Take theirs" }));
    await waitFor(() => expect(api.ResolveRitualConflict).toHaveBeenCalledWith("p1", 1, 14, "planning", "theirs"));
  });

  it("offers to recreate a page gone from Confluence or remove the local copy", async () => {
    vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five({ planning: { status: "gone", pageId: "42" } }));
    renderView();
    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent(GONE_SENTENCE);
    await userEvent.click(within(banner).getByRole("button", { name: "Recreate on next Sync" }));
    await waitFor(() => expect(api.ForgetRitualPage).toHaveBeenCalledWith("p1", 1, 14, "planning"));
    await userEvent.click(within(screen.getByRole("alert")).getByRole("button", { name: "Remove local copy" }));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(api.DeleteRitualDocument).toHaveBeenCalledWith("p1", 1, 14, "planning"));
  });

  it("links a synced page to Confluence", async () => {
    vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five({ planning: { status: "synced", pageId: "42" } }));
    renderView();
    await userEvent.click(await screen.findByRole("button", { name: "Open in Confluence" }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://confluence.example.com/pages/viewpage.action?pageId=42");
  });

  it("says a closed sprint without pages has none", async () => {
    vi.mocked(api.EnsureSprintRituals).mockImplementation(async (_p, _b, sprintId) => (sprintId === 13 ? [] : five()));
    renderView();
    await screen.findByTestId("editor");
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Ritual sprint" }), "13");
    expect(await screen.findByText(CLOSED_EMPTY_SENTENCE)).toBeInTheDocument();
  });

  it("says when there is no scrum board", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([{ id: 2, name: "Kanban", type: "kanban" }] as api.Board[]);
    renderView();
    expect(await screen.findByText(NO_SCRUM_BOARD_SENTENCE)).toBeInTheDocument();
  });
});
```

Run: `cd tam/frontend && npx vitest run src/components/RitualsView.test.tsx`
Expected: FAIL (the old view renders none of this).

- [ ] **Step 2: Rewrite `RitualsView.tsx`**

```tsx
import { useCallback, useEffect, useRef, useState } from "react";
import { announce, errMsg, useConfirm, useProfile } from "@agile-suite/core";
import {
  BrowserOpenURL, DeleteRitualDocument, EnsureSprintRituals, ForgetRitualPage, GetConfluenceConfig,
  LastRitualSync, ListBoards, ListBoardSprints, ResolveRitualConflict,
} from "../api";
import type { Board, ConfluenceConfig, Profile, RitualDocument, RitualSyncResult, Settings, Sprint } from "../api";
import { useSync } from "../contexts/SyncContext";
import {
  CLOSED_EMPTY_SENTENCE, GONE_SENTENCE, NO_SCRUM_BOARD_SENTENCE, RITUAL_LABEL, RITUAL_ORDER, STATUS_LABEL,
  UNCONFIGURED_SENTENCE, conflictSentence, pendingLine, syncSummary,
} from "../lib/ritualText";
import { RitualEditor } from "./ritual-editor/RitualEditor";
import type { RitualEditorHandle } from "./ritual-editor/RitualEditor";

// RitualsView reads nothing but tam.db. Opening it, picking a sprint, and
// editing a page make no Confluence call; Sync rituals is the one press that
// does, through the profile lock.
export function RitualsView() {
  const { activeId } = useProfile<Profile, Settings>();
  const { confirm } = useConfirm();
  const { runRitualsSync, running } = useSync();
  const [config, setConfig] = useState<ConfluenceConfig | null>(null);
  const [boards, setBoards] = useState<Board[] | null>(null);
  const [boardId, setBoardId] = useState(0);
  const [sprints, setSprints] = useState<Sprint[]>([]);
  const [sprintId, setSprintId] = useState(0);
  const [docs, setDocs] = useState<RitualDocument[] | null>(null);
  const [selected, setSelected] = useState<string>("planning");
  const [lastSync, setLastSync] = useState("");
  const [result, setResult] = useState<RitualSyncResult | null>(null);
  const [error, setError] = useState("");
  const [viewTheirs, setViewTheirs] = useState(false);
  const editorRef = useRef<RitualEditorHandle>(null);

  useEffect(() => {
    let live = true;
    setConfig(null); setBoards(null); setBoardId(0); setError(""); setResult(null);
    GetConfluenceConfig(activeId).then((c) => { if (live) setConfig(c); }).catch((e) => { if (live) setError(errMsg(e)); });
    ListBoards(activeId).then((all) => {
      if (!live) return;
      const scrum = all.filter((b) => b.type === "scrum");
      setBoards(scrum);
      setBoardId(scrum[0]?.id ?? 0);
    }).catch((e) => { if (live) { setBoards([]); setError(errMsg(e)); } });
    return () => { live = false; };
  }, [activeId]);

  useEffect(() => {
    let live = true;
    setSprints([]); setSprintId(0); setLastSync("");
    if (!boardId) return;
    ListBoardSprints(activeId, boardId).then((list) => {
      if (!live) return;
      setSprints(list);
      setSprintId(list.find((s) => s.state === "active")?.id ?? list[0]?.id ?? 0);
    }).catch((e) => { if (live) setError(errMsg(e)); });
    LastRitualSync(activeId, boardId).then((at) => { if (live) setLastSync(at); }).catch(() => {});
    return () => { live = false; };
  }, [activeId, boardId]);

  const loadDocs = useCallback(async () => {
    if (!boardId || !sprintId) return;
    setDocs(await EnsureSprintRituals(activeId, boardId, sprintId));
  }, [activeId, boardId, sprintId]);

  useEffect(() => {
    let live = true;
    setDocs(null); setViewTheirs(false);
    if (!boardId || !sprintId) return;
    EnsureSprintRituals(activeId, boardId, sprintId)
      .then((list) => { if (live) setDocs(list); })
      .catch((e) => { if (live) setError(errMsg(e)); });
    return () => { live = false; };
  }, [activeId, boardId, sprintId]);

  const onSaved = useCallback((saved: RitualDocument) => {
    setDocs((list) => list?.map((d) => (d.ritualType === saved.ritualType ? saved : d)) ?? list);
  }, []);

  async function sync() {
    setError(""); setResult(null);
    try {
      await editorRef.current?.flush();
      const res = await runRitualsSync(boardId);
      setResult(res);
      setLastSync(res.syncedAt);
      announce(syncSummary(res));
      await loadDocs();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function resolve(doc: RitualDocument, choice: "mine" | "theirs") {
    if (choice === "theirs") {
      const ok = await confirm({
        title: "Take the Confluence version?",
        message: "Your local edits to this page will be replaced by the version in Confluence. This cannot be undone.",
        confirmLabel: "Take theirs",
        cancelLabel: "Keep editing",
        danger: true,
      });
      if (!ok) return;
    }
    setError("");
    try {
      await editorRef.current?.flush();
      await ResolveRitualConflict(activeId, doc.boardId, doc.sprintId, doc.ritualType, choice);
      announce(choice === "mine" ? "Kept your version. The next Sync pushes it." : "Took the Confluence version.");
      setViewTheirs(false);
      await loadDocs();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function forget(doc: RitualDocument) {
    setError("");
    try {
      await ForgetRitualPage(activeId, doc.boardId, doc.sprintId, doc.ritualType);
      announce("The page will be created again on the next Sync.");
      await loadDocs();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function removeLocal(doc: RitualDocument) {
    const ok = await confirm({
      title: `Remove the local ${RITUAL_LABEL[doc.ritualType] ?? doc.ritualType}?`,
      message: "This deletes TAM's copy of the page. A fresh page from the template takes its place. It cannot be undone.",
      confirmLabel: "Remove",
      cancelLabel: "Keep it",
      danger: true,
    });
    if (!ok) return;
    setError("");
    try {
      await DeleteRitualDocument(activeId, doc.boardId, doc.sprintId, doc.ritualType);
      announce("Removed the local copy.");
      await loadDocs();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  if (boards === null) {
    return <section className="backlog rituals-view" aria-label="Rituals"><p className="muted" role="status">Loading rituals</p></section>;
  }
  if (boards.length === 0) {
    return (
      <section className="backlog rituals-view" aria-label="Rituals">
        {error && <p className="error-text" role="alert">{error}</p>}
        <p className="muted">{NO_SCRUM_BOARD_SENTENCE}</p>
      </section>
    );
  }

  const configured = !!config && !!config.baseURL.trim() && !!config.spaceKey.trim() && !!config.rootPageID.trim();
  const demoSpace = config?.baseURL.trim().toLowerCase() === "demo";
  const sprint = sprints.find((s) => s.id === sprintId);
  const doc = docs?.find((d) => d.ritualType === selected) ?? null;
  const pageUrl = doc?.pageId && configured && !demoSpace
    ? `${config!.baseURL.trim().replace(/\/+$/, "")}/pages/viewpage.action?pageId=${encodeURIComponent(doc.pageId)}`
    : "";
  const syncing = running === "rituals";

  return (
    <section className="backlog rituals-view" aria-label="Rituals">
      <div className="board-head rituals-toolbar">
        {boards.length > 1 ? (
          <label className="board-picker">
            <span>Board</span>
            <select aria-label="Ritual board" value={boardId} onChange={(e) => setBoardId(Number(e.target.value))}>
              {boards.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
            </select>
          </label>
        ) : <h3 className="board-head-name">{boards[0].name}</h3>}
        {sprints.length > 0 && (
          <label className="board-picker">
            <span>Sprint</span>
            <select aria-label="Ritual sprint" value={sprintId} onChange={(e) => setSprintId(Number(e.target.value))}>
              {sprints.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
          </label>
        )}
        <button className="btn" onClick={() => void sync()} disabled={!configured || !boardId || syncing}>
          {syncing ? "Syncing rituals" : "Sync rituals"}
        </button>
        <span className="muted small">{configured ? pendingLine(docs ?? [], lastSync) : UNCONFIGURED_SENTENCE}</span>
      </div>

      {error && <p className="error-text" role="alert">{error}</p>}
      {result && (
        <div className="ritual-sync-result" role="status">
          <p>{syncSummary(result)}</p>
          {result.failed.length > 0 && (
            <ul>{result.failed.map((f) => <li key={`${f.sprintName}|${f.title}`}><strong>{f.title}</strong>: {f.reason}</li>)}</ul>
          )}
        </div>
      )}

      {docs === null ? (
        sprintId ? <p className="muted" role="status">Loading this sprint's rituals</p> : null
      ) : docs.length === 0 ? (
        <p className="muted">{sprint?.state === "closed" ? CLOSED_EMPTY_SENTENCE : "This sprint has no ritual pages yet."}</p>
      ) : (
        <div className="rituals-layout">
          <nav aria-label="Ritual documents" className="ritual-docs">
            <ul>
              {RITUAL_ORDER.map((type) => {
                const d = docs.find((x) => x.ritualType === type);
                if (!d) return null;
                return (
                  <li key={type}>
                    <button
                      className={`folder-item ritual-doc${selected === type ? " folder-selected" : ""}`}
                      aria-current={selected === type ? "page" : undefined}
                      onClick={() => { setSelected(type); setViewTheirs(false); }}
                    >
                      <span className="ritual-doc-label">{RITUAL_LABEL[type]}</span>
                      <span className={`ritual-chip ritual-chip-${d.status}`}>{STATUS_LABEL[d.status]}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </nav>
          <article aria-label="Ritual page" className="ritual-page">
            {doc && (
              <>
                <div className="ritual-page-head">
                  <h3>{doc.title}</h3>
                  {pageUrl && <button className="btn btn-ghost" onClick={() => BrowserOpenURL(pageUrl)}>Open in Confluence</button>}
                </div>
                {doc.status === "conflict" && (
                  <div className="ritual-banner" role="alert">
                    <p>{conflictSentence(doc.conflictVersion)}</p>
                    <div className="row">
                      <button className="btn btn-ghost" aria-pressed={viewTheirs} onClick={() => setViewTheirs((v) => !v)}>
                        {viewTheirs ? "View mine" : "View theirs"}
                      </button>
                      <button className="btn" onClick={() => void resolve(doc, "mine")}>Keep mine</button>
                      <button className="btn btn-ghost" onClick={() => void resolve(doc, "theirs")}>Take theirs</button>
                    </div>
                  </div>
                )}
                {doc.status === "gone" && (
                  <div className="ritual-banner" role="alert">
                    <p>{GONE_SENTENCE}</p>
                    <div className="row">
                      <button className="btn" onClick={() => void forget(doc)}>Recreate on next Sync</button>
                      <button className="btn btn-ghost" onClick={() => void removeLocal(doc)}>Remove local copy</button>
                    </div>
                  </div>
                )}
                {viewTheirs && doc.status === "conflict" ? (
                  <RitualEditor key={`theirs:${doc.ritualType}:${doc.conflictVersion}`} profileId={activeId} doc={doc}
                    body={doc.conflictBody} readOnly pageUrl={pageUrl} />
                ) : (
                  // Keyed by version and page id, so a Sync that pulls, pushes or
                  // creates remounts the editor on the new body, and a local save,
                  // which changes neither, does not.
                  <RitualEditor key={`${doc.boardId}:${doc.sprintId}:${doc.ritualType}:${doc.version}:${doc.pageId}`}
                    ref={editorRef} profileId={activeId} doc={doc} pageUrl={pageUrl} onSaved={onSaved} />
                )}
              </>
            )}
          </article>
        </div>
      )}
    </section>
  );
}
```

- [ ] **Step 3: Replace the old view styles in `App.css`**

Delete every rule whose selector starts `.ritual-slot`, `.ritual-link`, `.ritual-template`, `.ritual-missing`, `.ritual-report-context`, `.ritual-page-meta`, `.ritual-association`, `.ritual-icon`, `.ritual-wizard`, `.ritual-issues`, and the existing `.rituals-layout` rules (around `App.css:689-731`). Then append:

```css
.rituals-view { display: flex; flex-direction: column; min-height: 0; flex: 1; gap: 8px; }
.rituals-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
.rituals-layout { display: grid; grid-template-columns: minmax(200px, 240px) minmax(0, 1fr); flex: 1; min-height: 0;
  border: 1px solid var(--border); border-radius: 6px; background: var(--surface); overflow: hidden; }
.ritual-docs { border-right: 1px solid var(--border); overflow: auto; min-height: 0; padding: 4px 0; }
.ritual-docs ul { list-style: none; margin: 0; padding: 0; }
.ritual-doc { justify-content: space-between; border: 0; background: transparent; font: inherit; color: inherit; text-align: left; }
.ritual-chip { font-size: 11px; padding: 0 6px; border-radius: 8px; border: 1px solid var(--border); color: var(--text-muted); }
.ritual-chip-synced { color: var(--ok-text); }
.ritual-chip-unsynced, .ritual-chip-local { color: var(--text-strong); }
.ritual-chip-conflict, .ritual-chip-gone { color: var(--warn-text); border-color: var(--warn-border); background: var(--warn-soft); }
.ritual-banner { margin: 0; padding: 8px 12px; border-bottom: 1px solid var(--warn-border); background: var(--warn-soft); color: var(--warn-text); }
.ritual-banner p { margin: 0 0 6px; }
.ritual-sync-result { padding: 6px 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface-2); }
.ritual-sync-result p, .ritual-sync-result ul { margin: 0; }
@media (max-width: 720px) { .rituals-layout { grid-template-columns: 1fr; } .ritual-docs { border-right: 0; border-bottom: 1px solid var(--border); } }
```

- [ ] **Step 4: Run the view tests and the typecheck**

Run: `cd tam/frontend && npx vitest run src/components/RitualsView.test.tsx && npm run typecheck`
Expected: PASS. (`RitualWizard.tsx` still exists and still compiles until Task 17; nothing imports it now.)

- [ ] **Step 5: Commit**

```bash
git add tam/frontend/src/components/RitualsView.tsx tam/frontend/src/components/RitualsView.test.tsx tam/frontend/src/App.css
git commit -m "feat(tam): a Rituals view that reads only tam.db and syncs on request"
```

---

### Task 17: Retire the wizard, the associations, and the live reads

**Files:**
- Delete: `tam/internal/ritualdefaults/`, `tam/internal/ritualrepo/demo_seed.go`, `core/confluence/demo.go`, `tam/frontend/src/components/RitualWizard.tsx`, `tam/frontend/src/components/RitualWizard.test.tsx`, `tam/frontend/src/queries/rituals.test.tsx`
- Modify: `tam/internal/ritualrepo/ritualrepo.go`, `ritualrepo_test.go`, `migration_test.go`; `tam/app.go`; `tam/app_profiles.go`; `tam/app_rituals.go`; `tam/app_rituals_test.go`; `tam/app_confluence.go`; `tam/app_confluence_test.go`; `core/confluence/client.go`, `client_test.go`; `core/profile/profile.go`; `tam/frontend/src/api.ts`; `tam/frontend/src/components/ProfileForm.tsx`; `tam/frontend/src/App.test.tsx`
- Regenerate: `tam/frontend/wailsjs/**`

**Interfaces:**
- Removes (Go bindings): `ListRitualAssociations`, `SetRitualAssociation`, `DeleteRitualAssociation`, `GetRitualPage`, `ListRitualDrafts`, `GetRitualDraft`, `SaveRitualDraft`, `DeleteRitualDraft`, `ScaffoldSprintRituals`, `ListSprintIssues`, `GetConfluencePage`, `ListConfluenceChildPages`.
- Keeps: `ritualrepo.Repository`, `New`, `Issue`, `DecodeIssues`, `normalizeRitualType`; `profile.ConfluenceConfig` and its get/set; the `confluence_association` and `confluence_page_cache` tables in `core/shareddb` and in `Manager.Delete`'s purge list; `newTestAppWithRituals` and `seedSprintIssues` in `app_rituals_test.go` (Task 10's tests use both).

- [ ] **Step 1: Go, delete the old model**

1. `rm -r tam/internal/ritualdefaults tam/internal/ritualrepo/demo_seed.go core/confluence/demo.go`
2. `tam/internal/ritualrepo/ritualrepo.go`: delete `EncodeIssues`, `Draft`, `selectDraftSQL`, `Get`, `upsertDraftSQL`, `Upsert`, `Delete`, `listDraftsSQL`, `ListDrafts`. Keep the package comment, `Issue`, `DecodeIssues`, `Repository`, `New`, `normalizeRitualType`.
3. `tam/internal/ritualrepo/ritualrepo_test.go`: delete every test that calls a removed function; keep tests of `DecodeIssues`, if any.
4. `tam/internal/ritualrepo/migration_test.go`: replace the body of the loop so it no longer calls `SeedDemo`/`Get`:

```go
	for pass := 0; pass < 2; pass++ {
		db, err := tamstore.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var title, remark, body, pageID, issues string
		var version int
		if err := db.DB().QueryRow(`SELECT title, remark, body, confluence_page_id, confluence_version, issues_json
			FROM ritual_document WHERE profile_id = 'demo' AND sprint_id = 12 AND ritual_type = 'planning'`).
			Scan(&title, &remark, &body, &pageID, &version, &issues); err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
		if title != "My plan" || remark != "Keep this remark" || body != "<p>My notes</p>" || pageID != "1234" || version != 3 {
			t.Errorf("repair changed the saved row: %s %s %s %s %d", title, remark, body, pageID, version)
		}
		if issues != `[{"key":"DEMO-412","remark":""},{"key":"DEMO-409","remark":""}]` {
			t.Errorf("lost issue selections: %s", issues)
		}
		if version, err := store.ReadSchemaVersion(db.DB()); err != nil || version != tamstore.Schema.Version {
			t.Errorf("schema = %d, error = %v", version, err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
```

   Rename the test to `TestRepairingVersionTenWithoutIssuesColumnKeepsTheRow` and drop the now-unused `ritualrepo`/`context` imports.
5. `tam/app.go`: delete the demo-seeding block in `initStore` (the `if demoProfiles, listErr := a.profiles.List(); …` block and its comment). Remove the `suiteprofiles` and `context` imports if the compiler reports them unused.
6. `tam/app_profiles.go`: in the demo branch of profile creation, delete the `if a.local != nil { … SeedDemo … }` block; keep `SetConfluenceConfig`. Remove the `ritualrepo` import.
7. `tam/app_rituals.go`: delete `ListRitualAssociations`, `SetRitualAssociation`, `DeleteRitualAssociation`, `GetRitualPage`, `isRitualDemo`, `knownRitualType`, `ListRitualDrafts`, `GetRitualDraft`, `SaveRitualDraft`, `DeleteRitualDraft`, `ScaffoldSprintRituals`, `ListSprintIssues`. Remove `encoding/json`, `ritualdefaults` and any other imports the compiler reports unused.
8. `tam/app_confluence.go`: delete `GetConfluencePage` and `ListConfluenceChildPages`. In `GetConfluenceConfig`, replace `isRitualDemo(p.JiraURL)` with `suiteprofiles.IsDemoURL(p.JiraURL)` and import `suiteprofiles`.
9. `core/confluence/client.go`: delete `Page`, `PageSpace`, `PageVersion`, `PageLinks`, `PageBody`, `PageStorage`, `PageView`, `ChildPage`, `ChildPageResult`, `GetPage`, `ListChildPages`, and the `strconv` import.
10. `core/confluence/client_test.go`: delete `TestClientGetsPageWithBearerAndDecodesBody` and `TestClientListsChildPagesWithPagination`; in `TestClientMapsHTTPError`, call `GetPageStorage` instead of `GetPage`.
11. `core/profile/profile.go`: delete `RitualAssociation`, `CacheConfluencePage`, `CachedConfluencePage`, `ListRitualAssociations`, `SetRitualAssociation`, `DeleteRitualAssociation`. Leave the three table names in `Delete`'s purge loop: the tables still hold rows written before this change.
12. `tam/app_rituals_test.go`: delete every test; keep the imports `newTestAppWithRituals` and `seedSprintIssues` need, and those two helpers.
13. `tam/app_confluence_test.go`: delete `TestConfluenceClientReportsMissingCredential`, `TestConfluenceClientReportsMissingConfiguration`, `TestConfluenceClientReadsPage`, and add:

```go
func TestConfluencePagesReportsMissingCredential(t *testing.T) {
	app, p := testConfluenceApp(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("request should not be made") }))
	if err := app.profiles.SetConfluenceConfig(p.ID, profile.ConfluenceConfig{BaseURL: "https://confluence.example.com", SpaceKey: "QA", RootPageID: "1"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.confluencePages(p); err == nil || err.Error() != "Confluence credentials are not configured for this profile" {
		t.Fatalf("error = %v", err)
	}
}

func TestConfluencePagesNeedsASpaceAndARoot(t *testing.T) {
	app, p := testConfluenceApp(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("request should not be made") }))
	if _, _, err := app.confluencePages(p); err == nil || err.Error() != "Confluence needs a space key and a root page id before rituals can sync" {
		t.Fatalf("error = %v", err)
	}
}
```

- [ ] **Step 2: Go, build, vet and test both modules**

Run: `cd core && go vet ./... && go test ./...` then `cd ../tam && go vet ./... && go test ./...`
Expected: PASS. `grep -rn "SeedDemo\|ritualdefaults\|RitualAssociation\|ListChildPages\|DemoPages\|GetRitualPage" --include=*.go core tam xtm` prints nothing.

- [ ] **Step 3: Regenerate the bindings**

Run: `cd tam && wails generate module`
Expected: the removed methods disappear from `frontend/wailsjs/go/main/App.d.ts`.

- [ ] **Step 4: Frontend, delete the old model**

1. `rm tam/frontend/src/components/RitualWizard.tsx tam/frontend/src/components/RitualWizard.test.tsx tam/frontend/src/queries/rituals.test.tsx`
2. `api.ts`: delete `ConfluencePage`, `ConfluenceChildPageResult`, `RitualAssociation`, `RitualIssue`, `RitualDraft`, `parseRitualIssues`, `encodeRitualIssues`, and the bindings `GetConfluencePage`, `ListConfluenceChildPages`, `GetRitualPage`, `ListRitualAssociations`, `SetRitualAssociation`, `DeleteRitualAssociation`, `ListRitualDrafts`, `GetRitualDraft`, `SaveRitualDraft`, `DeleteRitualDraft`, `ScaffoldSprintRituals`, `ListSprintIssues`. Drop `confluence` and `ritualrepo` from the `models` import.
3. `ProfileForm.tsx`: delete the imports `ListRitualAssociations`, `DeleteRitualAssociation`, `ListConfluenceChildPages`, `SetRitualAssociation`; the state `ritualAssociations`, `ritualPages`, `ritualPageID`, `ritualType`, `ritualBoardID`, `ritualSprintID`; the two lines that load associations and child pages after `setConfluenceLoaded(true)`; and the two JSX blocks `isEdit && ritualAssociations.length > 0 && …` and `isEdit && ritualPages.length > 0 && …`. The Confluence URL, space, root page id and token fields stay.
4. `App.test.tsx`: remove `ListRitualAssociations` from the `vi.mock` factory and its `mockResolvedValue` line; add `EnsureSprintRituals: vi.fn()` and `LastRitualSync: vi.fn()` to the factory. Replace the test at `it("opens Rituals and says when Confluence is not configured", …)` with:

```tsx
  // ListBoards answers no boards in this suite, so the Rituals view has no
  // scrum board to hold rituals and says so instead of drawing an editor.
  it("opens Rituals and says when there is no scrum board", async () => {
    renderApp();
    const rail = await screen.findByRole("navigation", { name: "Views" });
    await userEvent.click(within(rail).getByRole("button", { name: "Rituals" }));
    expect(await screen.findByText(/No scrum board has been synced for this project/)).toBeInTheDocument();
  });
```

   Keep whatever setup lines (rail toggle, `renderApp` helper name) the existing test used before its `findByText`; only the assertion and title change.

- [ ] **Step 5: Frontend, test and typecheck every workspace**

Run: `npm test --workspaces --if-present && npm run typecheck --workspaces --if-present`
Expected: PASS. `grep -rn "RitualWizard\|ListRitualAssociations\|GetRitualPage\|ScaffoldSprintRituals\|parseRitualIssues" tam/frontend/src` prints nothing.

- [ ] **Step 6: Commit**

```bash
git add -A core tam
git commit -m "refactor(tam): retire the ritual wizard, page associations and live page reads"
```

---

### Task 18: Documentation, the full gate, and the build

**Files:**
- Modify: `tam/CLAUDE.md`, `docs/user-guide/USER_GUIDE.md` (if it describes Rituals), `docs/superpowers/specs/2026-09-14-tam-rituals-local-first-design.md` (one line: schema version 12)

- [ ] **Step 1: Correct the spec's schema version**

In the spec, change the heading `### Schema version 11` to `### Schema version 12` and add under it: `Version 11 was already taken by a repair migration when this was built, so these columns arrive at 12.`

- [ ] **Step 2: Add the section to `tam/CLAUDE.md`**

Insert after the "Phase 4: where the report is kept, asked for, and read" section, in that file's own terse style:

```markdown
## Rituals, local first

Ritual page live in `tam.db` (`ritual_document`), edited in TAM, reach
Confluence only on Rituals view **Sync rituals**. View make no Confluence call
on open, sprint pick, or edit. Design =
`docs/superpowers/specs/2026-09-14-tam-rituals-local-first-design.md`.

TAM own whole page. No markers, no splice: `body` = local page in storage
XHTML, `base_body` = page as of `confluence_version`, `conflict_body` +
`conflict_version` = newer remote Sync found while local edits pending
(schema version 12). Dirty computed (`page id empty or body != base_body`),
never stored; `status` only what view draw.

`internal/ritualtemplate` render five pages per sprint (`_sprint` overview,
planning, standup, review, retro), pure, no clock. **Deterministic on
purpose**: adoption compare stored body against fresh render to tell
untouched template from page somebody wrote in. Golden files in `testdata/`
are also frontend round-trip corpus, and `.gitattributes` keep them LF.
Jira issues = Jira Issues macro with one of three JQL forms
(`JQL`/`ParseJQL`), the only ones editor preview from cache.

`ritualsync.Ensure` write missing pages from templates, local, no lock, so
planning page writable offline; closed sprint get none. `ritualsync.Run` =
Sync pass under `a.acquire(p.ID, "rituals")`: title match space wide (titles
unique per space), adopt under root, refuse elsewhere by name, create, pull,
push base+1, 409 or remote-newer-and-dirty = conflict, 404 = gone and never
silently recreated. **Every row write blind to body or compare-and-set on body
read at pass start**, so save landing mid-push stay unsynced and mid-pull
become conflict, never overwrite. Base after push = body pushed, never
Confluence answer: Confluence normalise storage on save, and base from its
answer leave page dirty forever. Per-page trouble travel in `Result.Failed`,
not Go error (Wails either/or).

Local writes take no lock (`SaveRitualBody`, resolve, forget, delete, ensure),
same as board moves; compare-and-set make that safe. Demo Confluence URL
"demo" = `internal/demo.Confluence`, in memory, rebuilt from stored pages on
first use after restart, and stage one conflict on first Standup it create.

Frontend `lib/storage` convert storage XHTML to TipTap JSON and back.
Everything not modelled = opaque node carrying raw XML, written back byte for
byte; fallback is rule, not list. Page that will not parse open read only.
Editor save only after person's edit (`touched` guard), 800 ms debounce,
Ctrl+S, flush on Sync and unmount; opening page never make it unsynced.
Editor keyed by version + page id, so Sync remount it and local save do not.
`lib/ritualText.ts` = every sentence.

Retired: wizard, `ritualdefaults`, page associations, `GetRitualPage` live
read. `confluence_association` and `confluence_page_cache` tables left in
`profiles.db`, unwritten, still purged with profile.
```

In the "One lock, both ends" section, append:

```markdown
**Rituals Sync not quiet either.** `SyncContext.runRitualsSync` drive banner
like `runBoardsRefresh`, `running` = `"rituals"`: several requests, no modal
holding focus. Editor save, resolve, forget, delete take no lock at all.
```

In the Layout section: add `app_rituals.go` (the ritual bindings: ensure, list, save, resolve, forget, delete, macro preview, standup entry, last sync, Sync), and entries for `internal/ritualtemplate/`, `internal/ritualrepo/` (documents.go), `internal/ritualsync/`, `internal/demo/confluence.go`, `frontend/src/lib/storage/`, `frontend/src/lib/ritualText.ts`, `frontend/src/lib/standupLog.ts`, `frontend/src/components/ritual-editor/`, `RitualsView`. Change `internal/tamstore/` to say schema version 12 and list `ritual_document`'s `base_body`, `conflict_body`, `conflict_version`. Remove any mention of `RitualWizard` or `ritualdefaults`.

- [ ] **Step 3: Update the user guide**

Run: `grep -n -i "ritual" docs/user-guide/USER_GUIDE.md`. If it describes Rituals, replace that section's body with:

```markdown
Rituals keeps one page per sprint ritual (Overview, Planning, Standup, Review,
Retrospective) on your machine. Pick a board and sprint, choose a page on the
left, and write. Your edits save locally as you type.

Nothing reaches Confluence until you press **Sync rituals**. Sync creates any
pages missing under the rituals root page you set in Profile settings, brings
in edits made in Confluence, and sends yours. If a page changed on both sides,
TAM keeps your text and asks: **View theirs**, **Keep mine** (sent on the next
Sync) or **Take theirs**. A page deleted in Confluence can be recreated on the
next Sync or removed locally.

Jira issue lists on a page are Confluence Jira Issues macros. In TAM they show
the issues from TAM's own cache; Confluence shows the live result.

On the Standup page, **Add today's entry** adds a dated Yesterday / Today /
Blockers section at the top of the Daily log.
```

If the guide does not mention Rituals, skip this step.

- [ ] **Step 4: Run the full gate**

```bash
cd core && go vet ./... && go test ./...
cd ../tam && go vet ./... && go test ./...
cd .. && npm test --workspaces --if-present && npm run typecheck --workspaces --if-present
```

Expected: every suite passes. Compare the test counts with Task 2 Step 1's notes and report both, naming how many tests left with the wizard, `ritualdefaults`, the associations and the live reads.

- [ ] **Step 5: Build the app**

Run: `cd tam && wails build`
Expected: `build/bin/task-activity-manager.exe` produced with no errors.

- [ ] **Step 6: Smoke the demo in the running app**

Run `cd tam && wails dev`, open a demo profile, and check by hand:
1. Rituals opens on the active sprint with five pages, all **Local**, and no network activity (nothing in the log about Confluence).
2. Type in Planning; the status line reads "Saved locally … · not synced".
3. **Sync rituals**: the banner shows "Syncing rituals with Confluence", the result reads "Sync finished: 5 created.", chips turn **Synced**.
4. Type in Standup, **Sync rituals** again: Standup turns **Conflict** (the demo's staged teammate edit); **View theirs** shows the added paragraph; **Keep mine**, then Sync pushes.
5. **Add today's entry** on Standup inserts today's dated section under Daily log; pressing it again says the entry is already there.
6. A Jira Issues block on Planning previews the sprint's cached issues with the caveat line.

Report anything that differs.

- [ ] **Step 7: Commit**

```bash
git add tam/CLAUDE.md docs
git commit -m "docs(tam): local-first rituals"
```
