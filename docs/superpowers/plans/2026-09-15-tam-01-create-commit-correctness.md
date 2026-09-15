# TAM Create and Commit Correctness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A sprint, an epic, its stories and their technical tasks drafted together commit in one press with no 400: sprints become drafts, Commit creates in dependency order and rewrites every placeholder before any payload names it, and the create dialog offers only fields on the issue type's create screen.

**Architecture:** `core/jira/createmeta.go` becomes the one createmeta reader (per-type endpoint, classic fallback on 404) and the one value shaper. TAM's Jira backend builds the dialog's field list from it, stores the offered field ids on the draft, and guards the create payload so an extra can never overwrite a base field or name a field off the screen. A sprint created in TAM is a `sprint_create` journal row plus a `draft = 1` row in `sprint` under a negative id (schema version 13). `internal/committer` runs Commit as an ordered list of phases (sprints, epics, issues, sub-tasks, edits, board moves, links), re-reading the journal after each, rewriting draft sprint ids and `TAM-NEW-n` parents through `issuerepo`, holding every dependent of a create that failed, and refusing to send any payload that still names a placeholder.

**Tech Stack:** Go (Wails v2, modernc SQLite through `core/store`), React 19, TypeScript, TanStack Query, Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-09-15-tam-01-create-commit-correctness-design.md`

## Global Constraints

- Work only in the worktree `C:\tool-projects\task-activity-manager\.claude\worktrees\tam-planning-bundles`, branch `feat/tam-bundles-01-06`. Later bundles (02 to 06) land on this same branch.
- **Schema version is 12 today; this bundle takes 13.** Version 13 adds `sprint.draft INTEGER NOT NULL DEFAULT 0`. Nothing else in the schema changes.
- UI text uses no em dashes. Middle dot ` · ` and ellipsis `…` are fine.
- Credentials live only in the OS credential manager. Nothing here reads, logs, or stores a token.
- Logic lives in `internal/` (and `core/`); `app*.go` only adapts it to Wails.
- Anything that calls a binding taking `App.acquire` must also take the frontend `SyncContext` lock (`runQuietLock`, `runSprintCeremony`, `runCommit`, and the rest). `CreateSprint`, `EditSprint`, `DeleteSprint` keep `a.acquire(p.ID, "sprint")` for drafts too, and the frontend keeps calling them through `runQuietLock`.
- Wails fills in a bound method's value or its error, never both: per-row trouble travels inside `committer.Result` (`Failures`, `Held`), never as a Go error.
- The journal is the only write path to Jira except the four sprint writes left in `internal/sprints` (`Start`, `Complete`, `Edit`, `Delete`). `internal/sprints/exceptions_test.go` is the fence and changes deliberately in Task 7.
- `boardrepo` never imports `issuerepo`. `issuerepo` writes the `sprint` table only for rows with `draft = 1` and for the one statement that turns a draft real (`RekeySprint`); every row Jira reported stays `boardrepo`'s.
- The existing board pass (sprint moves, then transitions, then ranks), rank pass, and link pass keep their semantics: satisfied rows, conflict classification, conditional deletes, batch width 20.
- Commit messages use conventional prefixes (`feat(core):`, `feat(tam):`, `fix(tam):`, `docs(tam):`, `test(tam):`) and must **never** contain `Co-Authored-By`, `Claude-Session`, or `Generated with Claude Code` lines. Use plain `git add <files>` then `git commit -m "<message>"` as separate commands.
- Test gates, all must pass at the end of every task that touches the layer:
  - `cd core && go test ./...`
  - `cd tam && go test ./...`
  - `cd tam/frontend && npx vitest run`
  - `cd frontend/core && npx vitest run`
  - `npm run typecheck --workspaces --if-present` (repo root)
- After any change to an exported Go struct or bound method that crosses Wails, run `cd tam && wails generate module` and commit `tam/frontend/wailsjs/**` with the task. Never hand-edit `wailsjs`.
- Sentences, verbatim:
  - Held reason: `waits for <label>, <why>` where `<label>` is `TAM-NEW-n` or `sprint "<name>"` and `<why>` is one of `which Jira refused`, `which is waiting for <label>`, `which could not be read`, `which could not be sent`, `which this connection cannot create`, `which Jira created as <key> but TAM could not rename; sync, then set it again`, `which Jira created but TAM could not rename; refresh the board, then move its cards again`.
  - Draft sprint refusal (Go): `this sprint is a draft in TAM; Commit creates it in Jira first`
  - Draft sprint tooltip: `Commit this sprint first`
  - Create sprint dialog subtitle: `Drafted locally. Commit creates it in Jira.`
  - Create sprint announcement: `<name> was drafted, <from> to <to>. Commit creates it in Jira.`
  - More fields toggle: `More fields (<n>)`
  - Firewall (Go): `internal error: <where> still names a local placeholder, so it was not sent to Jira; this is a TAM bug, discard the change and make it again`

## Rulings on the spec

These were ambiguous in the spec; the plan implements the ruling given.

1. **"The metadata id set is stored on the draft row"** is stored in the draft's create JSON as `IssueDraft.ScreenFields []string` (`screenFields`), not as a new `issue` column. `nil` means a draft made before this bundle or by the importer; `[]` means drafted against metadata that offered no extras.
2. **"Refuses any extra whose key is not in the metadata"** means the extra is left out of the payload with a log line, and the create proceeds. Failing the create forever over a field the user cannot edit off a draft would strand it.
3. **Epic Name is a base field.** An `Extra` carrying the Epic Name id is ignored and the summary is sent, which reverses the old "Extra's Epic Name wins" test deliberately.
4. **Shaping still reads createmeta at Commit** (as today) to know each extra's kind; only the screen check is offline. A failed read shapes extras as plain text, as today.
5. **XTM stays on its own reader.** The shared reader lands in `core/jira`; moving `xtm/internal/jira.GetBugCreateFields` onto it is a separate change.
6. **Draft sprint ids** come from a per-profile `profile_setting` sequence `draft_sprint_seq`, taking the lowest of the stored value and every negative id in `sprint`, minus one. Ids are never reused, so a stale reference to a discarded draft can never attach to a new one.
7. **Draft sprint create, edit, delete keep the `"sprint"` lock** in Go (they already take it and the frontend already wraps them in `runQuietLock`), which also serialises them against Commit's phase 1.
8. **Delete of a draft sprint is the same operation as discarding its `sprint_create` row:** every journaled move into it is reverted, every draft issue in it loses the sprint, and the draft row goes.
9. **A draft issue whose sprint is a refused draft sprint is held**, not created without its sprint, following the spec's "any draft ... that references it is skipped".
10. **Rituals ignore draft sprints**: `ritualsync.Sprints` skips them and `ritualSprint` refuses one, so no Confluence page is written for a sprint Jira may never have.
11. **`RemoveBoards` is unchanged.** A draft sprint on a board that leaves the cache goes with it; its journal row stays in Pending changes, where it can be committed or discarded.
12. **There is no TAM user guide in the repository** (`docs/user-guide/USER_GUIDE.md` is XTM's). The docs task updates `tam/CLAUDE.md`; the Outline user guide page is updated by hand after merge and is noted in Task 14.
13. **Held rows are reported in `Result.Held`, not as failures.** `TestAnEditNamingAnUncreatedDraftWaits` changes from two failures to one failure plus one held row.
14. **The demo's staged epic refusal is opt-in by summary.** The spec's manual demo test commits a sprint, epic, story and technical task "once", while its task 9 stages a refused epic. Both hold: the demo refuses, once per run, only an epic whose summary contains `refused`.
15. **Held rows are shown, not undoable.** Pending changes marks them Waiting with their reason; the existing per-row Discard is how a user drops one. No new Undo button.

## File map

**Go, created**
- `core/jira/createmeta.go`, `core/jira/createmeta_test.go`: `CreateMeta`, `MetaField`, `Kind`, `ShapeValue`, `SplitList`.
- `tam/internal/issuerepo/draftsprints.go`, `draftsprints_test.go`: `EntitySprintCreate`, `DraftSprint`, `CreateDraftSprint`, `EditDraftSprint`, `DiscardDraftSprint`, `rewriteSprintID`.
- `tam/internal/issuerepo/rekeysprint.go`, `rekeysprint_test.go`: `RekeySprint`, `MarkSprintCreatedWithoutRekey`, `rewriteParentKey`.
- `tam/internal/committer/phases.go`, `phases_test.go`: `phase`, `commitRun`, `draftOrdinal`, `createSprints`, `createDrafts`, `pushEdits`.
- `tam/internal/committer/held.go`, `firewall.go`, `firewall_internal_test.go`, `held_test.go`.
- `tam/internal/sprints/draft.go`, `draft_test.go`: `DraftSprint`, `errDraftSprint`.
- `tam/internal/ritualsync/drafts_test.go`, `tam/internal/importer/draftsprint_test.go`, `tam/app_commitplan_test.go`.
- `docs/superpowers/plans/assets/2026-09-15-createmeta-probe.md` (committed with this plan).

**Go, modified**
- `tam/internal/backend/backend.go` (`IssueDraft.ScreenFields`, `IssueBackend.CreateFields` doc)
- `tam/internal/backend/jira/writes.go`, `writes_test.go`, `jira_test.go` (fake), `subtask_test.go`
- `tam/internal/backend/demo/demo.go`, `demo_test.go`
- `tam/internal/tamstore/tamstore.go`, `tamstore_test.go`
- `tam/internal/boardrepo/boardrepo.go`, `boards.go`, `opensprints.go`, `sprintlist.go`, `replace.go`, `replace_test.go`
- `tam/internal/issuerepo/rekey.go`, `discard.go`
- `tam/internal/committer/committer.go`, `boards.go`, `ranks.go`, `committer_test.go`
- `tam/internal/sprints/sprints.go`, `manage.go`, `guards.go`, `exceptions_test.go`, `manage_test.go`, `cache_internal_test.go`
- `tam/internal/ritualsync/ensure.go`, `tam/app_rituals.go`
- `tam/app_sprintmanage.go`, `tam/app_sprintmanage_test.go`, `tam/app_writes.go`
- `tam/CLAUDE.md`

**Frontend, created** (under `tam/frontend/src/`)
- `components/MetaField.tsx`, `components/MetaField.test.tsx`
- `components/BoardsToolbar.test.tsx`

**Frontend, modified**
- `api.ts`, `lib/sprintOptions.ts`, `queries/pending.ts`, `queries/sprints.ts`, `App.css`
- `components/NewIssueModal.tsx` (+ test), `IssueDetailPanel.tsx` (+ test)
- `components/SprintRow.tsx` (+ test), `BoardsToolbar.tsx`, `BoardCeremonies.tsx`, `CreateSprintModal.tsx`, `EditSprintModal.tsx` (+ test), `SprintsView.tsx` (+ test), `BoardsView.test.tsx`, `RitualsView.tsx`, `SprintField.test.tsx`
- `components/PendingChangesModal.tsx` (+ test), `CommitBanner.tsx`
- `wailsjs/**` (generated)

## Extension points later bundles use

- **Bundle 04 (board-add commit step)** adds one entry to `phases()` in `tam/internal/committer/phases.go`, between `"sub-tasks"` and `"edits"` or between `"board moves"` and `"links"`, with a `run func(ctx context.Context, r *commitRun)`. It reads `r.rows` (already re-read), reports into `r.res`, and holds dependents through `r.deps.blockedBy` / `r.deps.hold`. No other phase changes.
- **Bundle 06 (Write/Preview for long text)** replaces the `textarea` entry of `META_INPUTS` in `tam/frontend/src/components/MetaField.tsx`. The Go side already reports long-text fields as `FieldSpec.Type == "textarea"`.

---

## Part A: the create dialog

### Task 1: The shared createmeta reader in core

**Files:**
- Create: `core/jira/createmeta.go`
- Test: `core/jira/createmeta_test.go`

**Interfaces:**
- Consumes: `(*Client).Get(ctx, path string, out any) error`, `*HTTPError{Code int}` (both in `core/jira/client.go`).
- Produces:
  - `const MetaPerType = "issuetype"`, `const MetaClassic = "classic"`
  - `const KindString = "string"`, `KindTextarea = "textarea"`, `KindOption = "option"`, `KindNumber = "number"`, `KindDate = "date"`, `KindDateTime = "datetime"`, `KindArray = "array"`, `KindUser = "user"`, `KindOther = "other"`
  - `type MetaSchema struct { Type, Items, System, Custom string }`
  - `type MetaOption struct { ID, Value, Name string }` with `func (o MetaOption) Label() string`
  - `type MetaField struct { ID, Name string; Required, HasDefaultValue bool; Operations []string; Schema MetaSchema; AllowedValues []MetaOption }` with `func (f MetaField) Kind() string`
  - `type CreateMeta struct { Source string; Fields []MetaField }` with `func (m CreateMeta) Field(id string) (MetaField, bool)`
  - `func (c *Client) CreateMeta(ctx context.Context, projectKey, issueTypeID, issueTypeName string) (CreateMeta, error)`
  - `func ShapeValue(f MetaField, v string) any`
  - `func SplitList(v string) []string`

- [ ] **Step 1: Write the failing tests**

`core/jira/createmeta_test.go`:

```go
package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// perTypeStory is Data Center 8.4's per-type answer for a Story, in two
// pages, and it does not list customfield_10253: a field absent from this
// answer is not on the create screen.
const perTypeStoryPage1 = `{"startAt":0,"maxResults":2,"total":3,"isLast":false,"values":[
	{"fieldId":"summary","name":"Summary","required":true,"schema":{"type":"string","system":"summary"},"operations":["set"]},
	{"fieldId":"customfield_10050","name":"Severity","required":true,"schema":{"type":"option","custom":"com.atlassian.jira.plugin.system.customfieldtypes:select"},"allowedValues":[{"id":"1","value":"Minor"},{"id":"3","value":"Critical"}]}
]}`

const perTypeStoryPage2 = `{"startAt":2,"maxResults":2,"total":3,"isLast":true,"values":[
	{"fieldId":"customfield_10300","name":"Acceptance criteria","required":false,"schema":{"type":"string","custom":"com.atlassian.jira.plugin.system.customfieldtypes:textarea"}}
]}`

const classicStory = `{"projects":[{"key":"TKT","issuetypes":[{"id":"10001","name":"Story","fields":{
	"summary":{"required":true,"name":"Summary","schema":{"type":"string","system":"summary"}},
	"customfield_10253":{"required":false,"name":"Team","schema":{"type":"string"}}
}}]}]}`

func TestCreateMetaReadsEveryPageOfThePerTypeEndpoint(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		if r.URL.Path != "/rest/api/2/issue/createmeta/TKT/issuetypes/10001" {
			t.Errorf("path = %s", r.URL.Path)
		}
		switch r.URL.Query().Get("startAt") {
		case "0":
			_, _ = w.Write([]byte(perTypeStoryPage1))
		case "2":
			_, _ = w.Write([]byte(perTypeStoryPage2))
		default:
			t.Errorf("startAt = %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()

	meta, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).CreateMeta(context.Background(), "TKT", "10001", "Story")
	if err != nil {
		t.Fatalf("CreateMeta: %v", err)
	}
	if meta.Source != MetaPerType || len(meta.Fields) != 3 || len(paths) != 2 {
		t.Fatalf("meta = %+v after %v", meta, paths)
	}
	if _, ok := meta.Field("customfield_10253"); ok {
		t.Error("a field the per-type answer does not list is not on the screen")
	}
	sev, ok := meta.Field("customfield_10050")
	if !ok || !sev.Required || sev.Kind() != KindOption || sev.AllowedValues[1].Label() != "Critical" {
		t.Errorf("severity = %+v", sev)
	}
	if ac, _ := meta.Field("customfield_10300"); ac.Kind() != KindTextarea || ac.Required {
		t.Errorf("acceptance criteria = %+v", ac)
	}
}

func TestCreateMetaFallsBackToTheClassicCallOnlyOnA404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/createmeta/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<html>Not Found</html>`))
			return
		}
		q := r.URL.Query()
		if r.URL.Path != "/rest/api/2/issue/createmeta" || q.Get("projectKeys") != "TKT" ||
			q.Get("issuetypeIds") != "10001" || q.Get("expand") != "projects.issuetypes.fields" {
			t.Errorf("classic request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(classicStory))
	}))
	defer srv.Close()

	meta, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).CreateMeta(context.Background(), "TKT", "10001", "Story")
	if err != nil {
		t.Fatalf("CreateMeta: %v", err)
	}
	if meta.Source != MetaClassic {
		t.Errorf("source = %q", meta.Source)
	}
	team, ok := meta.Field("customfield_10253")
	if !ok || team.ID != "customfield_10253" || team.Name != "Team" {
		t.Errorf("the classic answer carries its ids as map keys: %+v", meta.Fields)
	}
}

func TestCreateMetaAsksTheClassicCallByNameWhenTheTypeHasNoId(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/createmeta/") {
			t.Errorf("no type id, so no per-type request: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("issuetypeNames"); got != "Bug" {
			t.Errorf("issuetypeNames = %q", got)
		}
		_, _ = w.Write([]byte(`{"projects":[{"issuetypes":[{"name":"Bug","fields":{}}]}]}`))
	}))
	defer srv.Close()

	if _, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).CreateMeta(context.Background(), "PLAT", "", "Bug"); err != nil {
		t.Fatalf("CreateMeta: %v", err)
	}
}

func TestCreateMetaDoesNotFallBackOnAnythingButA404(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["no permission"]}`))
	}))
	defer srv.Close()

	_, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).CreateMeta(context.Background(), "TKT", "10001", "Story")
	if err == nil || calls != 1 {
		t.Fatalf("err = %v after %d calls, want the 403 and no classic retry", err, calls)
	}
}

func TestShapeValueForEachKind(t *testing.T) {
	opts := []MetaOption{{ID: "100", Name: "Checkout"}, {ID: "101", Name: "Payments"}}
	for _, tc := range []struct {
		name  string
		field MetaField
		in    string
		want  string
	}{
		{"option with values sends the id", MetaField{Schema: MetaSchema{Type: "option"}, AllowedValues: opts}, "100", `{"id":"100"}`},
		{"option without values sends the text", MetaField{Schema: MetaSchema{Type: "option"}}, "Needs docs", `{"value":"Needs docs"}`},
		{"array with values sends every id", MetaField{Schema: MetaSchema{Type: "array", Items: "component"}, AllowedValues: opts}, "100, 101", `[{"id":"100"},{"id":"101"}]`},
		{"array of strings is a plain list", MetaField{Schema: MetaSchema{Type: "array", Items: "string"}}, "alpha, beta", `["alpha","beta"]`},
		{"array of options without values sends values", MetaField{Schema: MetaSchema{Type: "array", Items: "option"}}, "a,b", `[{"value":"a"},{"value":"b"}]`},
		{"array of users sends names", MetaField{Schema: MetaSchema{Type: "array", Items: "user"}}, "jdoe, ranand", `[{"name":"jdoe"},{"name":"ranand"}]`},
		{"user sends a name", MetaField{Schema: MetaSchema{Type: "user"}}, " jdoe ", `{"name":"jdoe"}`},
		{"number is a number", MetaField{Schema: MetaSchema{Type: "number"}}, "2.5", `2.5`},
		{"a number that is not one goes as typed", MetaField{Schema: MetaSchema{Type: "number"}}, "soon", `"soon"`},
		{"date is the ISO day", MetaField{Schema: MetaSchema{Type: "date"}}, " 2026-09-16 ", `"2026-09-16"`},
		{"datetime is the day at midnight", MetaField{Schema: MetaSchema{Type: "datetime"}}, "2026-09-16", `"2026-09-16T00:00:00.000+0000"`},
		{"text is text", MetaField{Schema: MetaSchema{Type: "string"}}, "free text", `"free text"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(ShapeValue(tc.field, tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("ShapeValue = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestKindNamesLongTextAndTheFieldsNoFormCanFill(t *testing.T) {
	for _, tc := range []struct {
		schema MetaSchema
		want   string
	}{
		{MetaSchema{Type: "string", Custom: "com.atlassian.jira.plugin.system.customfieldtypes:textarea"}, KindTextarea},
		{MetaSchema{Type: "string", System: "environment"}, KindTextarea},
		{MetaSchema{Type: "string"}, KindString},
		{MetaSchema{Type: "version"}, KindOption},
		{MetaSchema{Type: "attachment"}, KindOther},
		{MetaSchema{Type: "array", Items: "issuelinks"}, KindOther},
		{MetaSchema{Type: "timetracking"}, KindOther},
	} {
		if got := (MetaField{Schema: tc.schema}).Kind(); got != tc.want {
			t.Errorf("Kind(%+v) = %q, want %q", tc.schema, got, tc.want)
		}
	}
	if got := SplitList(" a, ,b ,"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("SplitList = %v", got)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd core && go test ./jira/ -run 'CreateMeta|ShapeValue|KindNames'`
Expected: FAIL, `undefined: MetaOption` (and the rest).

- [ ] **Step 3: Write the reader and the shaper**

`core/jira/createmeta.go`:

```go
package jira

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Create metadata is the list of fields an issue type's create screen
// carries. Data Center 8.4 answers it per issue type, paged, and a field
// absent from that answer is not on the screen. Older instances only have
// the classic expand call, which on some versions lists fields that are not
// on the screen at all, and sending one of those gets "Field cannot be set.
// It is not on the appropriate screen". So the per-type endpoint is asked
// first and the classic call only when the per-type one answers 404.

// The two endpoints a CreateMeta can have come from.
const (
	MetaPerType = "issuetype"
	MetaClassic = "classic"
)

// The kinds a field is rendered and shaped by. They are also the FieldSpec
// types TAM's New issue dialog draws inputs for.
const (
	KindString   = "string"
	KindTextarea = "textarea"
	KindOption   = "option"
	KindNumber   = "number"
	KindDate     = "date"
	KindDateTime = "datetime"
	KindArray    = "array"
	KindUser     = "user"
	// KindOther is a field no text form can fill: an attachment, issue
	// links, time tracking.
	KindOther = "other"
)

// metaPage is how many fields one per-type page asks for, and metaMaxPages
// bounds a server that ignores startAt and answers the same page forever.
const (
	metaPage     = 50
	metaMaxPages = 40
)

// MetaSchema is a field's schema as createmeta reports it.
type MetaSchema struct {
	Type   string `json:"type"`
	Items  string `json:"items"`
	System string `json:"system"`
	Custom string `json:"custom"`
}

// MetaOption is one allowed value of a field.
type MetaOption struct {
	ID    string `json:"id"`
	Value string `json:"value"`
	Name  string `json:"name"`
}

// Label is what a person reads for the option: its value, or its name when
// Jira sent no value (components and versions carry names).
func (o MetaOption) Label() string {
	if o.Value != "" {
		return o.Value
	}
	return o.Name
}

// MetaField is one field of a create screen. ID is fieldId on the per-type
// answer and the map key on the classic one.
type MetaField struct {
	ID              string       `json:"fieldId"`
	Name            string       `json:"name"`
	Required        bool         `json:"required"`
	HasDefaultValue bool         `json:"hasDefaultValue"`
	Operations      []string     `json:"operations"`
	Schema          MetaSchema   `json:"schema"`
	AllowedValues   []MetaOption `json:"allowedValues"`
}

// CreateMeta is one issue type's create fields and the endpoint that
// answered, since only the per-type answer can be read as the screen.
type CreateMeta struct {
	Source string
	Fields []MetaField
}

// Field finds a field by id.
func (m CreateMeta) Field(id string) (MetaField, bool) {
	for _, f := range m.Fields {
		if f.ID == id {
			return f, true
		}
	}
	return MetaField{}, false
}

// Kind is how the field is rendered and shaped.
func (f MetaField) Kind() string {
	switch f.Schema.Type {
	case "string":
		if strings.HasSuffix(f.Schema.Custom, ":textarea") || f.Schema.System == "description" || f.Schema.System == "environment" {
			return KindTextarea
		}
		return KindString
	case "number":
		return KindNumber
	case "date":
		return KindDate
	case "datetime":
		return KindDateTime
	case "user":
		return KindUser
	case "array":
		switch f.Schema.Items {
		case "attachment", "issuelinks", "worklog":
			return KindOther
		}
		return KindArray
	case "option", "option-with-child", "priority", "version", "component", "resolution", "securitylevel":
		return KindOption
	case "attachment", "issuelinks", "timetracking", "worklog", "comments-page", "any":
		return KindOther
	}
	if len(f.AllowedValues) > 0 {
		return KindOption
	}
	return KindString
}

// CreateMeta reads one issue type's create fields. issueTypeID picks the
// per-type endpoint; an empty id, or a 404 from that endpoint, falls back to
// the classic expand call, by id when there is one and by issueTypeName
// otherwise. Any other failure is returned as it is: a 403 on the per-type
// endpoint will not read better through the classic one.
func (c *Client) CreateMeta(ctx context.Context, projectKey, issueTypeID, issueTypeName string) (CreateMeta, error) {
	if strings.TrimSpace(issueTypeID) != "" {
		fields, err := c.perTypeMeta(ctx, projectKey, issueTypeID)
		if err == nil {
			return CreateMeta{Source: MetaPerType, Fields: fields}, nil
		}
		var he *HTTPError
		if !errors.As(err, &he) || he.Code != http.StatusNotFound {
			return CreateMeta{}, err
		}
	}
	fields, err := c.classicMeta(ctx, projectKey, issueTypeID, issueTypeName)
	if err != nil {
		return CreateMeta{}, err
	}
	return CreateMeta{Source: MetaClassic, Fields: fields}, nil
}

func (c *Client) perTypeMeta(ctx context.Context, projectKey, issueTypeID string) ([]MetaField, error) {
	out := []MetaField{}
	start := 0
	for page := 0; page < metaMaxPages; page++ {
		q := url.Values{}
		q.Set("startAt", strconv.Itoa(start))
		q.Set("maxResults", strconv.Itoa(metaPage))
		var body struct {
			Total  int         `json:"total"`
			IsLast bool        `json:"isLast"`
			Values []MetaField `json:"values"`
		}
		path := "/rest/api/2/issue/createmeta/" + url.PathEscape(projectKey) + "/issuetypes/" + url.PathEscape(issueTypeID) + "?" + q.Encode()
		if err := c.Get(ctx, path, &body); err != nil {
			return nil, err
		}
		out = append(out, body.Values...)
		start += len(body.Values)
		switch {
		case body.IsLast, len(body.Values) == 0:
			return out, nil
		case body.Total > 0 && start >= body.Total:
			return out, nil
		case body.Total == 0 && len(body.Values) < metaPage:
			return out, nil
		}
	}
	return out, nil
}

func (c *Client) classicMeta(ctx context.Context, projectKey, issueTypeID, issueTypeName string) ([]MetaField, error) {
	q := url.Values{}
	q.Set("projectKeys", projectKey)
	if strings.TrimSpace(issueTypeID) != "" {
		q.Set("issuetypeIds", issueTypeID)
	} else {
		q.Set("issuetypeNames", issueTypeName)
	}
	q.Set("expand", "projects.issuetypes.fields")
	var body struct {
		Projects []struct {
			IssueTypes []struct {
				ID     string               `json:"id"`
				Name   string               `json:"name"`
				Fields map[string]MetaField `json:"fields"`
			} `json:"issuetypes"`
		} `json:"projects"`
	}
	if err := c.Get(ctx, "/rest/api/2/issue/createmeta?"+q.Encode(), &body); err != nil {
		return nil, err
	}
	out := []MetaField{}
	for _, p := range body.Projects {
		for _, t := range p.IssueTypes {
			matches := (issueTypeID != "" && t.ID == issueTypeID) || strings.EqualFold(t.Name, issueTypeName)
			if !matches && len(p.IssueTypes) > 1 {
				continue
			}
			for id, f := range t.Fields {
				f.ID = id
				out = append(out, f)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ShapeValue turns the text a form holds into the JSON Jira wants for f: an
// option's id when Jira listed allowed values and the typed text as a value
// when it did not, every part of a comma list for an array, a name for a
// user, a number for a number, and the ISO day for a date. Text that does not
// parse as what the field wants goes as typed, so Jira's own message is the
// one the user reads.
func ShapeValue(f MetaField, v string) any {
	switch f.Kind() {
	case KindOption:
		if len(f.AllowedValues) > 0 {
			return map[string]string{"id": v}
		}
		return map[string]string{"value": v}
	case KindArray:
		parts := SplitList(v)
		switch {
		case f.Schema.Items == "user":
			return keyed("name", parts)
		case len(f.AllowedValues) > 0:
			return keyed("id", parts)
		case f.Schema.Items == "option":
			return keyed("value", parts)
		}
		return parts
	case KindNumber:
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return n
		}
	case KindUser:
		return map[string]string{"name": strings.TrimSpace(v)}
	case KindDate:
		return strings.TrimSpace(v)
	case KindDateTime:
		day := strings.TrimSpace(v)
		if len(day) == len("2006-01-02") {
			return day + "T00:00:00.000+0000"
		}
		return day
	}
	return v
}

func keyed(key string, parts []string) []map[string]string {
	out := make([]map[string]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, map[string]string{key: p})
	}
	return out
}

// SplitList turns a form's comma list into its non-empty parts.
func SplitList(v string) []string {
	out := []string{}
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests to see them pass**

Run: `cd core && go test ./jira/`
Expected: PASS (every existing `core/jira` test too).

- [ ] **Step 5: Commit**

```bash
git add core/jira/createmeta.go core/jira/createmeta_test.go
git commit -m "feat(core): one createmeta reader, per issue type with a classic fallback"
```

---

### Task 2: The dialog's fields come from the create screen

**Files:**
- Modify: `tam/internal/backend/backend.go:70-99` (`IssueDraft`), `:462-464` (`CreateFields` doc)
- Modify: `tam/internal/backend/jira/writes.go:185-303` (replace `shapeExtra`, `splitList`, `createMeta`, `formFields`, `CreateFields`, `fieldKind`)
- Modify: `tam/internal/backend/jira/jira_test.go:31-122` (the fake's createmeta and project routes)
- Test: `tam/internal/backend/jira/writes_test.go`
- Regenerate: `tam/frontend/wailsjs/**`

**Interfaces:**
- Consumes: `corejira.CreateMeta`, `MetaField.Kind`, `MetaOption.Label`, `KindOther` (Task 1); `(*Backend).discover`, `jiraTypeNames`, `typesOrEmpty`, `fieldIDs.list()` (existing).
- Produces:
  - `backend.IssueDraft.ScreenFields []string` with JSON `screenFields`: the extra field ids the dialog offered; `nil` for a draft made before this bundle or by the importer.
  - `func (b *Backend) createMeta(ctx context.Context, projectKey, logicalType string) (corejira.CreateMeta, string, error)`: the metadata and the Jira type name it was read for.
  - `func isBaseField(f corejira.MetaField, ids fieldIDs) bool`
  - `func isBaseFieldID(id string, ids fieldIDs) bool`
  - `CreateFields` now returns required **and** optional fields (with `FieldSpec.Required` saying which), never a base field, never an optional field of `KindOther`; `FieldSpec.Type` is one of the Task 1 kinds.

- [ ] **Step 1: Teach the fake Jira the per-type endpoint**

In `tam/internal/backend/jira/jira_test.go`, inside `handler`'s `switch`, add these cases directly above the existing `case r.URL.Path == "/rest/api/2/issue/createmeta":`:

```go
		case strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/createmeta/"):
			f.searches = append(f.searches, "createmeta-type "+r.URL.Path)
			switch r.URL.Path {
			case "/rest/api/2/issue/createmeta/TKT/issuetypes/10001":
				_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":7,"isLast":true,"values":[
					{"fieldId":"summary","name":"Summary","required":true,"schema":{"type":"string","system":"summary"}},
					{"fieldId":"issuetype","name":"Issue Type","required":true,"schema":{"type":"issuetype","system":"issuetype"}},
					{"fieldId":"customfield_10014","name":"Epic Link","required":false,"schema":{"type":"any","custom":"com.pyxis.greenhopper.jira:gh-epic-link"}},
					{"fieldId":"customfield_10020","name":"Sprint","required":false,"schema":{"type":"array","items":"string","custom":"com.pyxis.greenhopper.jira:gh-sprint"}},
					{"fieldId":"customfield_10050","name":"Severity","required":true,"schema":{"type":"option"},"allowedValues":[{"id":"1","value":"Minor"},{"id":"3","value":"Critical"}]},
					{"fieldId":"customfield_10300","name":"Acceptance criteria","required":false,"schema":{"type":"string","custom":"com.atlassian.jira.plugin.system.customfieldtypes:textarea"}},
					{"fieldId":"attachment","name":"Attachment","required":false,"schema":{"type":"array","items":"attachment","system":"attachment"}}
				]}`))
			case "/rest/api/2/issue/createmeta/TKT/issuetypes/10003":
				_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":3,"isLast":true,"values":[
					{"fieldId":"parent","name":"Parent","required":true,"schema":{"type":"issuelink","system":"parent"}},
					{"fieldId":"summary","name":"Summary","required":true,"schema":{"type":"string","system":"summary"}},
					{"fieldId":"customfield_10300","name":"Acceptance criteria","required":false,"schema":{"type":"string","custom":"com.atlassian.jira.plugin.system.customfieldtypes:textarea"}}
				]}`))
			default:
				// An instance before 8.4, or a type id this fake does not know:
				// the backend falls back to the classic call.
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errorMessages":["not found"]}`))
			}
		case r.URL.Path == "/rest/api/2/project/TKT":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"10001","name":"Story"},{"id":"10003","name":"Technical task","subtask":true}]}`))
```

Inside the existing classic `case r.URL.Path == "/rest/api/2/issue/createmeta":`, directly after the `f.searches = append(...)` line, add the TKT classic answer (it lists `customfield_10253`, the field that is not on the screen):

```go
			if r.URL.Query().Get("projectKeys") == "TKT" {
				_, _ = w.Write([]byte(`{"projects":[{"key":"TKT","issuetypes":[{"id":"10001","name":"Story","fields":{
					"summary":{"required":true,"name":"Summary","schema":{"type":"string"}},
					"customfield_10253":{"required":false,"name":"Team","schema":{"type":"string"}}
				}}]}]}`))
				return
			}
```

- [ ] **Step 2: Write the failing tests**

In `tam/internal/backend/jira/writes_test.go`, replace `TestCreateFieldsKeepsOnlyRequiredUnknownFields` with the two tests below and add the third:

```go
// The classic answer this fake gives for a Bug carries one optional field,
// Environment, which the dialog now offers under More fields.
func TestCreateFieldsOffersRequiredAndOptionalFieldsBeyondTheForm(t *testing.T) {
	b, f := newBackend(t, twoFields)
	specs, err := b.CreateFields(context.Background(), "PLAT", backend.TypeBug)
	if err != nil {
		t.Fatalf("CreateFields: %v", err)
	}
	var seen []string
	for _, s := range specs {
		seen = append(seen, fmt.Sprintf("%s:%s:%v", s.ID, s.Type, s.Required))
	}
	// Sorted by name: Component/s, Environment, Keywords, Release Note,
	// Severity. Story Points and Summary are the form's own.
	want := "components:array:true,environment:string:false,customfield_10071:array:true,customfield_10070:option:true,customfield_10050:option:true"
	if strings.Join(seen, ",") != want {
		t.Errorf("specs = %v", seen)
	}
	if specs[4].Name != "Severity" || len(specs[4].AllowedValues) != 2 || specs[4].AllowedValues[1].Value != "Critical" {
		t.Errorf("severity = %+v", specs[4])
	}
	if specs[0].AllowedValues[0].Value != "Checkout" {
		t.Errorf("array options take name when value is empty: %+v", specs[0])
	}
	found := false
	for _, s := range f.searches {
		if strings.HasPrefix(s, "createmeta ") && strings.Contains(s, "projectKeys=PLAT") && strings.Contains(s, "issuetypeNames=Bug") && strings.Contains(s, "expand=projects.issuetypes.fields") {
			found = true
		}
	}
	if !found {
		t.Errorf("a type with no id in the project list is read through the classic call by name: %v", f.searches)
	}
}

// A Story on TKT is read through the per-type endpoint, which does not list
// customfield_10253, so the dialog never offers it. Epic Link and Sprint are
// the form's own, found by id or by their greenhopper type, and an optional
// attachment is nothing a text form can fill.
func TestCreateFieldsReadsTheScreenAndLeavesOutBaseAndUnfillableFields(t *testing.T) {
	b, f := newBackend(t, threeFields)
	specs, err := b.CreateFields(context.Background(), "TKT", backend.TypeStory)
	if err != nil {
		t.Fatalf("CreateFields: %v", err)
	}
	var seen []string
	for _, s := range specs {
		seen = append(seen, fmt.Sprintf("%s:%s:%v", s.ID, s.Type, s.Required))
	}
	if strings.Join(seen, ",") != "customfield_10300:textarea:false,customfield_10050:option:true" {
		t.Errorf("specs = %v", seen)
	}
	for _, s := range f.searches {
		if strings.HasPrefix(s, "createmeta ") {
			t.Errorf("the per-type endpoint answered, so the classic call is never made: %v", f.searches)
		}
	}
}

// Item 2 of the ticket: a technical task drafted from a story was asked for
// its parent a second time, because createmeta lists parent as required.
func TestCreateFieldsNeverOffersTheParentOfASubtask(t *testing.T) {
	b, _ := newBackend(t, threeFields)
	specs, err := b.CreateFields(context.Background(), "TKT", backend.TypeSubtask)
	if err != nil {
		t.Fatalf("CreateFields: %v", err)
	}
	if len(specs) != 1 || specs[0].ID != "customfield_10300" {
		t.Errorf("specs = %+v, want only Acceptance criteria", specs)
	}
}
```

Add `"fmt"` to the test file's imports.

- [ ] **Step 3: Run the tests to see them fail**

Run: `cd tam && go test ./internal/backend/jira/ -run 'CreateFields'`
Expected: FAIL. `TestCreateFieldsOffersRequiredAndOptionalFieldsBeyondTheForm` reports no `environment` row; `TestCreateFieldsReadsTheScreen...` reports the classic call and `customfield_10253` missing from the reasoning; `TestCreateFieldsNeverOffersTheParentOfASubtask` reports `parent` among the specs.

- [ ] **Step 4: Add `ScreenFields` to the draft**

In `tam/internal/backend/backend.go`, inside `IssueDraft`, directly above `Extra map[string]string`, add:

```go
	// ScreenFields are the extra field ids the create dialog offered for
	// this draft's type, read off the create screen when it was drafted.
	// The create sends no extra outside this set, which is what keeps a
	// field createmeta listed but the screen does not carry out of the
	// payload without a network call at Commit. Nil is a draft made before
	// the set existed or by the importer, whose extras are empty anyway.
	ScreenFields []string `json:"screenFields"`
```

And replace the `CreateFields` doc comment in `IssueBackend` with:

```go
	// CreateFields lists the create-screen fields of a logical type that
	// the New issue form does not already carry, required and optional,
	// with FieldSpec.Required saying which. The form's own fields (summary,
	// description, priority, labels, assignee, story points, Epic Link,
	// Epic Name, parent, sprint) never come back, whatever createmeta says.
```

- [ ] **Step 5: Rebuild `CreateFields` on the core reader**

In `tam/internal/backend/jira/writes.go`, delete `shapeExtra`, `splitList`, the `createMeta` struct, `formFields`, `CreateFields`, and `fieldKind` (lines 185 to the end of the file), and add:

```go
// baseFieldIDs are the create-meta ids TAM's own form carries or sets itself.
// An extra may never name one: the dialog does not offer them, and a create
// never lets an extra overwrite what the form set.
var baseFieldIDs = map[string]bool{
	"project": true, "issuetype": true, "summary": true, "description": true,
	"priority": true, "assignee": true, "labels": true, "reporter": true, "parent": true,
}

// agileBaseTypes are the custom field types behind the Agile fields TAM owns.
// They are matched by type as well as by the discovered ids, because
// discovery can fail on an instance where the field still sits on the screen.
var agileBaseTypes = []string{":gh-epic-link", ":gh-epic-label", ":gh-sprint", ":gh-lexo-rank"}

// isBaseFieldID says the id is one of TAM's own fields: a fixed system id,
// or the Story Points, Epic Link, Epic Name, Sprint, or Rank this instance
// discovered.
func isBaseFieldID(id string, ids fieldIDs) bool {
	if baseFieldIDs[id] {
		return true
	}
	for _, own := range ids.list() {
		if id == own {
			return true
		}
	}
	return false
}

// isBaseField is isBaseFieldID plus the schema, which also catches a parent
// field reported under another id and an Agile field discovery missed.
func isBaseField(f corejira.MetaField, ids fieldIDs) bool {
	if isBaseFieldID(f.ID, ids) || f.Schema.System == "parent" {
		return true
	}
	for _, suffix := range agileBaseTypes {
		if strings.HasSuffix(f.Schema.Custom, suffix) {
			return true
		}
	}
	return false
}

// createMeta reads the create fields of one logical type in the project, and
// answers with the Jira type name it resolved. The type's id comes from the
// project's own type list, so the per-type endpoint can be asked; a type the
// list does not name is asked for by name through the classic call.
func (b *Backend) createMeta(ctx context.Context, projectKey, logicalType string) (corejira.CreateMeta, string, error) {
	names := jiraTypeNames([]string{logicalType}, b.requirementType, b.typesOrEmpty(ctx, projectKey))
	if len(names) == 0 {
		return corejira.CreateMeta{}, "", fmt.Errorf("unknown issue type %q", logicalType)
	}
	typeID := ""
	if types, err := b.c.IssueTypes(ctx, projectKey); err == nil {
		for _, t := range types {
			if strings.EqualFold(t.Name, names[0]) {
				typeID = t.ID
				break
			}
		}
	}
	meta, err := b.c.CreateMeta(ctx, projectKey, typeID, names[0])
	return meta, names[0], err
}

// CreateFields returns the create-screen fields of the type beyond the
// form's own, required and optional, sorted by name, with their options when
// they have any. An optional field no text form can fill is left out; a
// required one is still offered as text, because leaving it out would only
// move the failure to a Jira 400 at Commit.
func (b *Backend) CreateFields(ctx context.Context, projectKey, logicalType string) ([]backend.FieldSpec, error) {
	meta, _, err := b.createMeta(ctx, projectKey, logicalType)
	if err != nil {
		return nil, err
	}
	ids := b.discover(ctx)
	out := []backend.FieldSpec{}
	for _, f := range meta.Fields {
		if isBaseField(f, ids) {
			continue
		}
		kind := f.Kind()
		if kind == corejira.KindOther {
			if !f.Required {
				continue
			}
			kind = corejira.KindString
		}
		spec := backend.FieldSpec{ID: f.ID, Name: f.Name, Type: kind, Required: f.Required, AllowedValues: []backend.FieldOption{}}
		for _, av := range f.AllowedValues {
			spec.AllowedValues = append(spec.AllowedValues, backend.FieldOption{ID: av.ID, Value: av.Label()})
		}
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}
```

`CreateIssue` still references `shapeExtra` and `CreateFields`; Task 3 rewrites that block. To keep this task compiling, replace `CreateIssue`'s `if len(d.Extra) > 0 { ... }` block (lines 148-167) with this interim version, which Task 3 replaces:

```go
	if len(d.Extra) > 0 {
		meta, _, metaErr := b.createMeta(ctx, projectKey, d.Type)
		for id, v := range d.Extra {
			if v == "" {
				continue
			}
			f, known := meta.Field(id)
			if metaErr != nil || !known {
				f = corejira.MetaField{ID: id, Schema: corejira.MetaSchema{Type: "string"}}
			}
			fields[id] = corejira.ShapeValue(f, v)
		}
	}
```

Add `corejira "agile-suite/core/jira"` to the imports and drop `strconv` (only `shapeExtra` used it).

- [ ] **Step 6: Run the backend tests**

Run: `cd tam && go test ./internal/backend/...`
Expected: PASS, including `TestCreateIssueShapesExtraFromCreateMeta` and `TestCreateIssuePostsTheDraftAndReturnsTheKey` unchanged.

- [ ] **Step 7: Regenerate the bindings and run everything Go**

Run: `cd tam && wails generate module`
Expected: `frontend/wailsjs/go/models.ts` gains `screenFields: string[];` on `backend.IssueDraft`. Check with `git diff --stat tam/frontend/wailsjs`.

Run: `cd tam && go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add tam/internal/backend/backend.go tam/internal/backend/jira/writes.go tam/internal/backend/jira/writes_test.go tam/internal/backend/jira/jira_test.go tam/frontend/wailsjs
git commit -m "feat(tam): offer the create screen's own fields, never the form's"
```

---

### Task 3: The create payload guard

**Files:**
- Modify: `tam/internal/backend/jira/writes.go:101-183` (`CreateIssue`)
- Test: `tam/internal/backend/jira/writes_test.go`, `tam/internal/backend/jira/subtask_test.go`

**Interfaces:**
- Consumes: `createMeta`, `isBaseFieldID`, `IssueDraft.ScreenFields` (Task 2); `corejira.ShapeValue`, `CreateMeta.Field`, `MetaPerType` (Task 1).
- Produces: `CreateIssue` that sends an extra only when (a) its id is not already in the payload, (b) it is not a base field, (c) it is in `d.ScreenFields` when that is non-nil, and (d) the live metadata, when it came from the per-type endpoint, still lists it.

- [ ] **Step 1: Write the failing tests reproducing both 400s**

Add to `tam/internal/backend/jira/subtask_test.go`:

```go
// Item 3 of the ticket, first half: the duplicate Parent input put a plain
// string into Extra["parent"], which overwrote the {"key": ...} object and
// Jira answered "parent: data was not an object".
func TestCreateSubtaskKeepsTheParentObjectWhateverExtraSays(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.createKey = "TKT-9"
	_, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeSubtask, Summary: "Wire the input", ParentKey: "TKT-7",
		Extra:        map[string]string{"parent": "TKT-7", "customfield_10300": "Given a promo"},
		ScreenFields: []string{"customfield_10300"},
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := f.writes[len(f.writes)-1]
	if !strings.Contains(post, `"parent":{"key":"TKT-7"}`) || strings.Contains(post, `"parent":"TKT-7"`) {
		t.Errorf("parent must stay the object the create set: %s", post)
	}
	if !strings.Contains(post, `"customfield_10300":"Given a promo"`) {
		t.Errorf("an extra on the screen is still sent: %s", post)
	}
}
```

Add to `tam/internal/backend/jira/writes_test.go`:

```go
// Item 3 of the ticket, second half: customfield_10253 came from a classic
// createmeta answer that listed a field the Story create screen does not
// carry, and Jira answered "Field cannot be set. It is not on the
// appropriate screen". A draft carries the ids its dialog offered, so the
// field stays out of the payload with no network call; and when the live
// per-type answer does not list a field either, it stays out too.
func TestCreateIssueSendsNoFieldThatIsNotOnTheScreen(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.createKey = "TKT-10"
	if _, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Promo input",
		Extra:        map[string]string{"customfield_10253": "Platform", "customfield_10050": "3"},
		ScreenFields: []string{"customfield_10050"},
	}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := f.writes[len(f.writes)-1]
	if strings.Contains(post, "customfield_10253") {
		t.Errorf("a field off the drafted screen must not be sent: %s", post)
	}
	if !strings.Contains(post, `"customfield_10050":{"id":"3"}`) {
		t.Errorf("the screen's own field is shaped and sent: %s", post)
	}

	// A draft from before the set existed: the live per-type answer decides.
	f.createKey = "TKT-11"
	if _, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Legacy draft",
		Extra: map[string]string{"customfield_10253": "Platform"},
	}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if post := f.writes[len(f.writes)-1]; strings.Contains(post, "customfield_10253") {
		t.Errorf("the per-type answer does not list the field, so it is not sent: %s", post)
	}
}

func TestCreateIssueNeverLetsAnExtraOverwriteABaseField(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.createKey = "TKT-12"
	if _, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Real summary", ParentKey: "TKT-2",
		Extra: map[string]string{"summary": "Fake summary", "customfield_10014": "TKT-99", "customfield_10016": "40"},
	}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := f.writes[len(f.writes)-1]
	for _, bad := range []string{"Fake summary", "TKT-99", `"customfield_10016":40`} {
		if strings.Contains(post, bad) {
			t.Errorf("an extra overwrote a base field (%s): %s", bad, post)
		}
	}
	if !strings.Contains(post, `"summary":"Real summary"`) || !strings.Contains(post, `"customfield_10014":"TKT-2"`) {
		t.Errorf("the form's own values stand: %s", post)
	}
}
```

In `TestCreateEpicDefaultsEpicNameAndSendsNoEpicLink`, replace the second half (from `f.createKey = "PLAT-601"` to the end of the test) with:

```go
	// Epic Name is one of TAM's own fields: an extra naming it is ignored
	// and the summary is what Jira gets, the same as with no extra at all.
	f.createKey = "PLAT-601"
	if _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeEpic, Summary: "Another epic", Extra: map[string]string{"customfield_10011": "Custom name"},
	}); err != nil {
		t.Fatal(err)
	}
	post = f.writes[len(f.writes)-1]
	if !strings.Contains(post, `"customfield_10011":"Another epic"`) || strings.Contains(post, "Custom name") {
		t.Errorf("Epic Name is the summary, whatever Extra says: %s", post)
	}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam && go test ./internal/backend/jira/ -run 'CreateSubtaskKeeps|NotOnTheScreen|OverwriteABaseField|EpicDefaults'`
Expected: FAIL on all four: `"parent":"TKT-7"` in the payload, `customfield_10253` sent, `Fake summary` sent, `Custom name` sent.

- [ ] **Step 3: Guard the extras**

In `CreateIssue`, replace the interim `if len(d.Extra) > 0 { ... }` block from Task 2 with:

```go
	if len(d.Extra) > 0 {
		b.applyExtras(ctx, projectKey, names[0], d, ids, fields)
	}
```

And move the Epic Name default so it runs regardless of extras (it already sits below the block; leave it as it is, `if _, set := fields[ids.EpicName]; !set` still holds because an extra can no longer set it).

Add below `CreateIssue`:

```go
// applyExtras writes the draft's extra fields into the payload, shaped from
// the type's create metadata. Four rules keep an extra out, each logged:
//
//   - the payload already holds the id: what the form set is never
//     overwritten, which is how Extra["parent"] once replaced the
//     {"key": ...} object with a string;
//   - the id is one of TAM's own fields, set by the form or by nothing;
//   - the draft carries the ids its dialog offered and this one is not
//     among them, which is the screen check Commit makes with no network;
//   - the metadata read now came from the per-type endpoint and no longer
//     lists the id, so the field is not on the screen today.
//
// An unreadable metadata read shapes every surviving extra as text, and
// Jira's own validation decides.
func (b *Backend) applyExtras(ctx context.Context, projectKey, typeName string, d backend.IssueDraft, ids fieldIDs, fields map[string]any) {
	meta, _, metaErr := b.createMeta(ctx, projectKey, d.Type)
	var screen map[string]bool
	if d.ScreenFields != nil {
		screen = make(map[string]bool, len(d.ScreenFields))
		for _, id := range d.ScreenFields {
			screen[id] = true
		}
	}
	extraIDs := make([]string, 0, len(d.Extra))
	for id := range d.Extra {
		extraIDs = append(extraIDs, id)
	}
	sort.Strings(extraIDs)
	for _, id := range extraIDs {
		v := d.Extra[id]
		if strings.TrimSpace(v) == "" {
			continue
		}
		if _, set := fields[id]; set || isBaseFieldID(id, ids) {
			log.Printf("tam: the %s create of %q ignores extra %s, which is one of TAM's own fields", typeName, d.Summary, id)
			continue
		}
		if screen != nil && !screen[id] {
			log.Printf("tam: the %s create of %q leaves out %s, which was not on the screen it was drafted against", typeName, d.Summary, id)
			continue
		}
		f, known := meta.Field(id)
		if metaErr == nil && meta.Source == corejira.MetaPerType && !known {
			log.Printf("tam: the %s create of %q leaves out %s, which is not on the %s create screen", typeName, d.Summary, id, typeName)
			continue
		}
		if metaErr != nil || !known {
			f = corejira.MetaField{ID: id, Schema: corejira.MetaSchema{Type: "string"}}
		}
		fields[id] = corejira.ShapeValue(f, v)
	}
}
```

Update the `CreateIssue` doc comment to:

```go
// CreateIssue POSTs the draft. TAM's own fields are set first and are never
// overwritten; extras are then shaped from the type's create metadata and
// filtered by applyExtras, so a field off the create screen is never sent.
```

- [ ] **Step 4: Run the tests to see them pass**

Run: `cd tam && go test ./internal/backend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/backend/jira/writes.go tam/internal/backend/jira/writes_test.go tam/internal/backend/jira/subtask_test.go
git commit -m "fix(tam): never let an extra overwrite a base field or leave the create screen"
```

---

### Task 4: The dialog shows More fields and states the parent

**Files:**
- Create: `tam/frontend/src/components/MetaField.tsx`, `tam/frontend/src/components/MetaField.test.tsx`
- Modify: `tam/frontend/src/components/NewIssueModal.tsx:84-161` (remove `MetaField`), `:197-201`, `:237-300`, `:477-504`
- Modify: `tam/frontend/src/api.ts:631-661` (`IssueDraft.screenFields`)
- Modify: `tam/frontend/src/components/IssueDetailPanel.tsx:154-160` (`canHoldSubtasks`)
- Modify: `tam/frontend/src/App.css` (after `.meta-fields` at line 457)
- Test: `tam/frontend/src/components/NewIssueModal.test.tsx`, `tam/frontend/src/components/IssueDetailPanel.test.tsx`

**Interfaces:**
- Consumes: `GetCreateFields` answering required and optional `FieldSpec`s (Task 2).
- Produces:
  - `export const FORM_OWNED_FIELDS: Set<string>`
  - `export function splitMetaFields(specs: FieldSpec[]): { required: FieldSpec[]; optional: FieldSpec[] }`
  - `export interface MetaInputProps { spec: FieldSpec; value: string; onChange: (v: string) => void; shared: MetaInputShared }`
  - `export const META_INPUTS: Record<string, (p: MetaInputProps) => ReactElement>` (the table bundle 06 extends)
  - `export function MetaField(props: { spec: FieldSpec; value: string; invalid: boolean; onChange: (v: string) => void }): ReactElement`
  - `IssueDraft.screenFields?: string[]` on the frontend type.

- [ ] **Step 1: Write the failing tests**

`tam/frontend/src/components/MetaField.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { FieldSpec } from "../api";
import { META_INPUTS, MetaField, splitMetaFields } from "./MetaField";

const spec = (over: Partial<FieldSpec>): FieldSpec => ({
  id: "customfield_1", name: "Field", type: "string", required: false, allowedValues: [], ...over,
});

describe("splitMetaFields", () => {
  it("drops the form's own fields and splits the rest by required", () => {
    const { required, optional } = splitMetaFields([
      spec({ id: "parent", name: "Parent", required: true }),
      spec({ id: "summary", name: "Summary", required: true }),
      spec({ id: "customfield_10050", name: "Severity", required: true }),
      spec({ id: "customfield_10300", name: "Acceptance criteria", type: "textarea" }),
    ]);
    expect(required.map((s) => s.id)).toEqual(["customfield_10050"]);
    expect(optional.map((s) => s.id)).toEqual(["customfield_10300"]);
  });
});

describe("MetaField", () => {
  it("draws long text as a text area, through the table later inputs replace", async () => {
    const onChange = vi.fn();
    render(<MetaField spec={spec({ name: "Acceptance criteria", type: "textarea" })} value="" invalid={false} onChange={onChange} />);
    const box = screen.getByLabelText("Acceptance criteria");
    expect(box.tagName).toBe("TEXTAREA");
    await userEvent.type(box, "G");
    expect(onChange).toHaveBeenCalledWith("G");
    expect(Object.keys(META_INPUTS)).toContain("textarea");
  });

  it("draws a date for date and datetime, and says a user field wants a username", () => {
    render(
      <>
        <MetaField spec={spec({ id: "a", name: "Due", type: "date" })} value="" invalid={false} onChange={vi.fn()} />
        <MetaField spec={spec({ id: "b", name: "Starts", type: "datetime" })} value="" invalid={false} onChange={vi.fn()} />
        <MetaField spec={spec({ id: "c", name: "Tester", type: "user" })} value="" invalid={false} onChange={vi.fn()} />
      </>,
    );
    expect(screen.getByLabelText("Due")).toHaveAttribute("type", "date");
    expect(screen.getByLabelText("Starts")).toHaveAttribute("type", "date");
    expect(screen.getByText("A Jira username.")).toBeInTheDocument();
  });

  it("offers a select when Jira listed values, a multi-select for an array", () => {
    render(
      <MetaField
        spec={spec({ name: "Components", type: "array", required: true, allowedValues: [{ id: "10", value: "Frontend" }, { id: "11", value: "Backend" }] })}
        value="10"
        invalid
        onChange={vi.fn()}
      />,
    );
    const select = screen.getByLabelText("Components *");
    expect(select).toHaveAttribute("multiple");
    expect(select).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText("Pick one or more.")).toBeInTheDocument();
  });
});
```

In `tam/frontend/src/components/NewIssueModal.test.tsx`:

Change `baseDraft` to carry the offered ids:

```tsx
const baseDraft = {
  type: "task", summary: "", description: "", priority: "", labels: [] as string[],
  assignee: "", storyPoints: null as number | null, parentKey: "", sprintId: "", sprintName: "", extra: {},
  screenFields: [] as string[],
};
```

In `"asks for the type's required create-meta fields and sends them as extra"`, add after the last `expect`:

```tsx
    expect(vi.mocked(api.CreateIssue).mock.calls[0][1].screenFields).toEqual(["customfield_10050"]);
```

Add these tests inside `describe("NewIssueModal", ...)`:

```tsx
  // Item 2 of the ticket: a technical task drafted from a story was asked
  // for its parent a second time, under the parent it already stated.
  it("never renders the parent or the form's own fields from create-meta", async () => {
    vi.mocked(api.GetCreateFields).mockResolvedValue([
      { id: "parent", name: "Parent", type: "string", required: true, allowedValues: [] },
      { id: "summary", name: "Summary", type: "string", required: true, allowedValues: [] },
      { id: "customfield_10300", name: "Acceptance criteria", type: "textarea", required: false, allowedValues: [] },
    ]);
    renderModal(vi.fn(), vi.fn(), "subtask", true, "PLAT-412");
    const dialog = await screen.findByRole("dialog", { name: "New technical task" });
    await submitButton(dialog);
    expect(within(dialog).queryByLabelText(/^Parent/)).not.toBeInTheDocument();
    expect(within(dialog).getAllByLabelText("Summary *")).toHaveLength(1);
    expect(within(dialog).getByText("PLAT-412")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "More fields (1)" })).toBeInTheDocument();
  });

  it("keeps optional fields behind More fields and sends what was filled there", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetCreateFields).mockResolvedValue([
      { id: "customfield_10050", name: "Severity", type: "option", required: true, allowedValues: [{ id: "3", value: "Critical" }] },
      { id: "customfield_10300", name: "Acceptance criteria", type: "textarea", required: false, allowedValues: [] },
      { id: "duedate", name: "Due date", type: "date", required: false, allowedValues: [] },
    ]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New task" });
    const more = await within(dialog).findByRole("button", { name: "More fields (2)" });
    expect(more).toHaveAttribute("aria-expanded", "false");
    expect(within(dialog).queryByLabelText("Acceptance criteria")).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText("Severity *")).toBeInTheDocument();
    await user.click(more);
    expect(more).toHaveAttribute("aria-expanded", "true");
    await user.type(within(dialog).getByLabelText("Acceptance criteria"), "Given a promo code");
    await user.selectOptions(within(dialog).getByLabelText("Severity *"), "3");
    await user.type(within(dialog).getByLabelText("Summary *"), "Promo input");
    await user.click(await submitButton(dialog));
    await waitFor(() => expect(api.CreateIssue).toHaveBeenCalled());
    const draft = vi.mocked(api.CreateIssue).mock.calls[0][1];
    expect(draft.extra).toEqual({ customfield_10050: "3", customfield_10300: "Given a promo code" });
    expect(draft.screenFields).toEqual(["customfield_10050", "customfield_10300", "duedate"]);
  });

  it("opens More fields when an optional number there is not a number", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetCreateFields).mockResolvedValue([
      { id: "customfield_10099", name: "Effort", type: "number", required: false, allowedValues: [] },
    ]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New task" });
    await user.click(await within(dialog).findByRole("button", { name: "More fields (1)" }));
    await user.type(within(dialog).getByLabelText("Effort"), "soon");
    await user.click(within(dialog).getByRole("button", { name: "More fields (1)" }));
    await user.type(within(dialog).getByLabelText("Summary *"), "Rework");
    await user.click(await submitButton(dialog));
    expect(await within(dialog).findByText("Effort must be a number.")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "More fields (1)" })).toHaveAttribute("aria-expanded", "true");
    expect(api.CreateIssue).not.toHaveBeenCalled();
  });
```

In `tam/frontend/src/components/IssueDetailPanel.test.tsx`, add inside the top-level `describe`:

```tsx
  // A story still drafted as TAM-NEW-2 can hold a technical task: Commit
  // creates the story first and the sub-task after it has a real key.
  it("drafts a sub-task under a draft", async () => {
    renderPanel(vi.fn(), undefined, { ...story, key: "TAM-NEW-2", status: "Draft", draft: true, pending: true });
    expect(await screen.findByRole("button", { name: "+ Technical task" })).toBeInTheDocument();
  });
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam/frontend && npx vitest run src/components/MetaField.test.tsx src/components/NewIssueModal.test.tsx src/components/IssueDetailPanel.test.tsx`
Expected: FAIL: `./MetaField` does not resolve, the `More fields` buttons are missing, `screenFields` is undefined, and the draft panel offers no sub-task button.

- [ ] **Step 3: Write `MetaField.tsx`**

`tam/frontend/src/components/MetaField.tsx`:

```tsx
import type { ReactElement } from "react";
import type { FieldSpec } from "../api";

// FORM_OWNED_FIELDS are the create-meta ids the New issue dialog carries
// itself. The Go side never offers them; this is the second fence, so a
// backend that did would still not draw a second Parent or Summary input
// under the one the dialog already has.
export const FORM_OWNED_FIELDS = new Set([
  "project", "issuetype", "summary", "description", "priority", "labels", "assignee", "reporter", "parent",
]);

// splitMetaFields drops the form's own fields and splits the rest into what
// Jira requires, shown at once, and what it merely allows, kept behind More
// fields.
export function splitMetaFields(specs: FieldSpec[]): { required: FieldSpec[]; optional: FieldSpec[] } {
  const own = specs.filter((s) => !FORM_OWNED_FIELDS.has(s.id));
  return { required: own.filter((s) => s.required), optional: own.filter((s) => !s.required) };
}

export interface MetaInputShared {
  id: string;
  className: string;
  "aria-required"?: boolean;
  "aria-invalid"?: boolean;
}

export interface MetaInputProps {
  spec: FieldSpec;
  value: string;
  onChange: (v: string) => void;
  shared: MetaInputShared;
}

// OptionInput is any field whose create-meta listed allowed values. An array
// field gets a multi-select, because a Jira array takes more than one value;
// the chosen ids are joined with a comma, which the Go side splits.
function OptionInput({ spec, value, onChange, shared }: MetaInputProps) {
  const isList = spec.type === "array";
  const selected = value === "" ? [] : value.split(",");
  return (
    <span className="edit-cell">
      <select
        {...shared}
        multiple={isList}
        size={isList ? Math.min(spec.allowedValues.length, 4) : undefined}
        value={isList ? selected : value}
        onChange={(e) =>
          onChange(isList ? Array.from(e.target.selectedOptions, (o) => o.value).join(",") : e.target.value)
        }
      >
        {!isList && <option value="">Select a {spec.name.toLowerCase()}</option>}
        {spec.allowedValues.map((o) => (
          <option key={o.id} value={o.id}>{o.value}</option>
        ))}
      </select>
      {isList && <span className="muted small">Pick one or more.</span>}
    </span>
  );
}

// TextInput is everything without options: text, a number, a date, a
// username, a comma list. The Go side shapes each from the field's schema.
function TextInput({ spec, value, onChange, shared }: MetaInputProps) {
  const dated = spec.type === "date" || spec.type === "datetime";
  return (
    <span className="edit-cell">
      <input
        {...shared}
        type={dated ? "date" : "text"}
        inputMode={spec.type === "number" ? "decimal" : undefined}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      {spec.type === "array" && <span className="muted small">A comma list.</span>}
      {spec.type === "user" && <span className="muted small">A Jira username.</span>}
    </span>
  );
}

function TextareaInput({ value, onChange, shared }: MetaInputProps) {
  return (
    <span className="edit-cell">
      <textarea {...shared} rows={3} value={value} onChange={(e) => onChange(e.target.value)} />
    </span>
  );
}

// META_INPUTS picks the input for a field type that has no options. It is a
// table rather than a branch so a type gets a richer input by replacing its
// one entry: the long-text Write and Preview input takes over "textarea"
// here and touches nothing else.
export const META_INPUTS: Record<string, (p: MetaInputProps) => ReactElement> = {
  textarea: TextareaInput,
};

// MetaField renders one create-meta field: its label, a required mark, and
// the input its type and options call for.
export function MetaField({
  spec,
  value,
  invalid,
  onChange,
}: {
  spec: FieldSpec;
  value: string;
  invalid: boolean;
  onChange: (v: string) => void;
}) {
  const id = `meta-${spec.id}`;
  const shared: MetaInputShared = {
    id,
    className: "detail-input",
    "aria-required": spec.required || undefined,
    "aria-invalid": invalid || undefined,
  };
  const Input = spec.allowedValues.length > 0 ? OptionInput : META_INPUTS[spec.type] ?? TextInput;
  return (
    <div className="edit-row">
      <label className="muted small" htmlFor={id}>
        {spec.name}
        {spec.required && <span className="field-required" aria-hidden="true"> *</span>}
      </label>
      <Input spec={spec} value={value} onChange={onChange} shared={shared} />
    </div>
  );
}
```

- [ ] **Step 4: Use it in `NewIssueModal.tsx`**

1. Delete the local `MetaField` function (lines 84-161), add `import { MetaField, splitMetaFields } from "./MetaField";`, and drop `FieldSpec` from the `import type { ... } from "../api"` line (nothing in the file names it any more).
2. Replace `const specs = meta.data ?? [];` with:

```tsx
  const { required: requiredSpecs, optional: optionalSpecs } = splitMetaFields(meta.data ?? []);
  const specs = [...requiredSpecs, ...optionalSpecs];
  // More fields opens only on request, or on a validation failure inside it,
  // so the dialog stays as short as the fields Jira insists on.
  const [moreOpen, setMoreOpen] = useState(false);
```

3. In `changeType`, add `setMoreOpen(false);` after `setExtra({});`.
4. In `onSubmit`, change the validation loop's two `fail(...)` calls to open the section for an optional field first:

```tsx
    for (const s of specs) {
      const v = (extra[s.id] ?? "").trim();
      if (s.required && !v) {
        fail(`${s.name} is required.`, `meta-${s.id}`);
        return;
      }
      // Jira's own number fields get the same check the built-in story points
      // field gets; inputMode is a keyboard hint, not validation.
      if (v !== "" && s.type === "number" && Number.isNaN(Number(v))) {
        if (!s.required) setMoreOpen(true);
        fail(`${s.name} must be a number.`, `meta-${s.id}`);
        return;
      }
    }
```

5. In the `draft` literal, add after `extra: ...`:

```tsx
      // The ids this dialog offered, so Commit can leave out any extra the
      // create screen did not carry without asking Jira again. A failed read
      // offered nothing, which is what an empty list says.
      screenFields: meta.isSuccess ? specs.map((s) => s.id) : [],
```

6. Replace the `<>...</>` fragment inside the `meta-fields` block (the branch after `checking ?`) with:

```tsx
              <>
                {requiredSpecs.length > 0 && (
                  <>
                    <p className="muted small">Jira requires these for a {typeLabel(type).toLowerCase()}:</p>
                    {requiredSpecs.map((s) => (
                      <MetaField
                        key={s.id}
                        spec={s}
                        value={extra[s.id] ?? ""}
                        invalid={invalidField === `meta-${s.id}`}
                        onChange={(v) => setExtra((cur) => ({ ...cur, [s.id]: v }))}
                      />
                    ))}
                  </>
                )}
                {optionalSpecs.length > 0 && (
                  <div className="meta-more">
                    <button
                      type="button"
                      className="btn btn-ghost meta-more-toggle"
                      aria-expanded={moreOpen}
                      aria-controls="new-issue-more-fields"
                      onClick={() => setMoreOpen((open) => !open)}
                    >
                      <span aria-hidden="true">{moreOpen ? "▾" : "▸"}</span> More fields ({optionalSpecs.length})
                    </button>
                    {moreOpen && (
                      <div id="new-issue-more-fields" className="meta-fields-optional">
                        {optionalSpecs.map((s) => (
                          <MetaField
                            key={s.id}
                            spec={s}
                            value={extra[s.id] ?? ""}
                            invalid={invalidField === `meta-${s.id}`}
                            onChange={(v) => setExtra((cur) => ({ ...cur, [s.id]: v }))}
                          />
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </>
```

7. In `tam/frontend/src/api.ts`, inside `IssueDraft`, add after `extra`:

```ts
  // The extra field ids the dialog offered for this type, read off the
  // create screen. The create sends no extra outside them. Absent on a draft
  // written before the set existed and on the importer's drafts.
  screenFields?: string[];
```

8. In `tam/frontend/src/App.css`, after the `.meta-fields` rule, add:

```css
.meta-more { display: flex; flex-direction: column; gap: 10px; }
.meta-more-toggle { align-self: flex-start; padding-left: 0; }
.meta-fields-optional { display: flex; flex-direction: column; gap: 10px; }
```

9. In `tam/frontend/src/components/IssueDetailPanel.tsx`, replace the `canHoldSubtasks` block and its comment with:

```tsx
  // Jira allows a sub-task under any standard issue, and under neither an
  // epic nor another sub-task. A draft parent is fine: Commit creates it
  // first and the sub-task after it has a real key.
  const canHoldSubtasks =
    issue.type !== "epic" &&
    issue.type !== "subtask" &&
    (subtaskType.data ?? "") !== "";
```

- [ ] **Step 5: Run the frontend tests and the type check**

Run: `cd tam/frontend && npx vitest run src/components/MetaField.test.tsx src/components/NewIssueModal.test.tsx src/components/IssueDetailPanel.test.tsx`
Expected: PASS.

Run: `npm run typecheck --workspaces --if-present` (repo root)
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add tam/frontend/src/components/MetaField.tsx tam/frontend/src/components/MetaField.test.tsx tam/frontend/src/components/NewIssueModal.tsx tam/frontend/src/components/NewIssueModal.test.tsx tam/frontend/src/components/IssueDetailPanel.tsx tam/frontend/src/components/IssueDetailPanel.test.tsx tam/frontend/src/api.ts tam/frontend/src/App.css
git commit -m "feat(tam): keep optional create fields behind More fields and never ask for a stated parent"
```

---

## Part B: draft sprints

### Task 5: Schema version 13 and draft sprints in the board reads

**Files:**
- Modify: `tam/internal/tamstore/tamstore.go:1-18` (package doc), `:47-49` (`Version: 13`), `:172-193` (add migration 13), `:349-361` (`sprintDDL`)
- Modify: `tam/internal/boardrepo/boardrepo.go:42-51` (`Sprint.Draft`)
- Modify: `tam/internal/boardrepo/boards.go:39-42` (`listSprintsSQL`), `:132-149` (`ListSprints` scan)
- Modify: `tam/internal/boardrepo/opensprints.go` (`SprintChoice.Draft`, `openSprintsSQL`, scan)
- Modify: `tam/internal/boardrepo/sprintlist.go:22-28` (`detailSprintsSQL`), `:261-276` (scan)
- Modify: `tam/internal/boardrepo/replace.go:180-194` (`writeSprints`)
- Test: `tam/internal/tamstore/tamstore_test.go`, `tam/internal/boardrepo/replace_test.go`

**Interfaces:**
- Consumes: `newIssues()` (the `staticIssues` fake in `tam/internal/boardrepo/view_test.go`), `sampleSprints()`, `newRepo` (boardrepo tests).
- Produces:
  - `sprint.draft INTEGER NOT NULL DEFAULT 0` column; `tamstore.Schema.Version == 13`.
  - `boardrepo.Sprint.Draft bool` (`json:"draft"`), `boardrepo.SprintChoice.Draft bool` (`json:"draft"`), filled by `ListSprints`, `OpenSprints`, `BoardSprintDetails`.
  - `writeSprints` (so `ReplaceBoard` and `ReplaceSprints`) deletes only `draft = 0` rows of the board: a boards refresh never removes a draft sprint.

- [ ] **Step 1: Write the failing tests**

In `tam/internal/tamstore/tamstore_test.go`, in `TestSchemaVersionTwelveAddsTheRitualSyncColumns`, change the last assertion so it no longer pins the current version:

```go
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
```

Add:

```go
func TestSchemaVersionThirteenAddsTheSprintDraftFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE sprint DROP COLUMN draft`,
		`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 13, 1, 'Sprint 13', 'future')`,
		`UPDATE meta SET value = '12' WHERE key = 'schema_version'`,
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
	var draft int
	var name string
	if err := db.DB().QueryRow(`SELECT name, draft FROM sprint WHERE profile_id = 'p1' AND id = 13`).Scan(&name, &draft); err != nil {
		t.Fatalf("read the kept sprint: %v", err)
	}
	if name != "Sprint 13" || draft != 0 {
		t.Errorf("sprint = %q draft %d, want the row kept and not a draft", name, draft)
	}
	if v, _ := store.ReadSchemaVersion(db.DB()); v != 13 || tamstore.Schema.Version != 13 {
		t.Errorf("schema version = %d (Schema.Version %d), want 13", v, tamstore.Schema.Version)
	}
}
```

In `tam/internal/boardrepo/replace_test.go`, add:

```go
// A draft sprint is TAM's own row in Jira's table, and a boards refresh
// rewrites the board's sprints from what Jira sent, which never names a
// draft. The refresh leaves it where it is.
func TestABoardsRefreshKeepsADraftSprintJiraNeverSent(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), sampleSprints(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date, draft)
		VALUES ('p1', -1, 1, 'Sprint 15', 'future', '2026-09-30T09:00:00Z', '2026-10-14T09:00:00Z', 1)`); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), sampleSprints(), nil); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceSprints(ctx, "p1", 1, sampleSprints()); err != nil {
		t.Fatal(err)
	}

	listed, err := r.ListSprints(ctx, "p1", 1)
	if err != nil {
		t.Fatal(err)
	}
	var draft *boardrepo.Sprint
	for i := range listed {
		if listed[i].ID == -1 {
			draft = &listed[i]
		}
		if listed[i].ID == 13 && listed[i].Draft {
			t.Errorf("a sprint Jira sent is not a draft: %+v", listed[i])
		}
	}
	if draft == nil || !draft.Draft || draft.Name != "Sprint 15" {
		t.Fatalf("sprints = %+v, want the draft kept and marked", listed)
	}

	open, err := r.OpenSprints(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	last := open[len(open)-1]
	if last != (boardrepo.SprintChoice{ID: -1, Name: "Sprint 15", BoardName: "PLAT Scrum", State: "future", Draft: true}) {
		t.Errorf("open sprints = %+v, want the draft last by start date and marked", open)
	}

	details, err := r.BoardSprintDetails(ctx, newIssues(), "p1", 1)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range details {
		if d.ID == -1 {
			found = d.Draft
		}
	}
	if !found {
		t.Errorf("the Sprints view's read carries the draft flag: %+v", details)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam && go test ./internal/tamstore/ ./internal/boardrepo/ -run 'Thirteen|DraftSprintJiraNeverSent|Twelve'`
Expected: FAIL: `no such column: draft` in the tamstore test, and `unknown field Draft` compiling the boardrepo test.

- [ ] **Step 3: Add the column and the migration**

In `tam/internal/tamstore/tamstore.go`:

1. Append to the package doc, after `...compares and resolves with.`: `// Version 13 adds sprint's draft flag: a sprint drafted in TAM sits in sprint under a negative id with draft = 1 until Commit creates it in Jira.`
2. Set `Version: 13`.
3. In `sprintDDL`, add `	draft         INTEGER NOT NULL DEFAULT 0,` directly after the `complete_date` line.
4. Add the migration after version 12's entry (before the closing `}},`):

```go
	}, {
		Version: 13,
		// A sprint drafted in TAM is a row here under a negative id, flagged
		// draft, beside a sprint_create journal row, until Commit creates it
		// in Jira and rewrites the id. A column add in version 7's shape: a
		// fresh database has it from sprintDDL already, which
		// AddColumnIfMissing treats as success, and every cached sprint Jira
		// sent is not a draft, which is what the default says.
		Apply: func(db *sql.DB) error {
			return store.AddColumnIfMissing(db, "sprint", "draft INTEGER NOT NULL DEFAULT 0")
		},
```

- [ ] **Step 4: Carry the flag through the board reads and keep drafts on refresh**

In `tam/internal/boardrepo/boardrepo.go`, add to `Sprint` after `CompleteDate`:

```go
	// Draft marks a sprint drafted in TAM and not yet created in Jira. Its
	// id is negative and its state is future; Start and Complete refuse it
	// and Commit creates it before anything moves into it.
	Draft bool `json:"draft"`
```

In `tam/internal/boardrepo/boards.go`, change `listSprintsSQL`'s select list to `SELECT id, board_id, name, state, start_date, end_date, goal, complete_date, draft FROM sprint` and the scan in `ListSprints` to `rows.Scan(&s.ID, &s.BoardID, &s.Name, &s.State, &s.StartDate, &s.EndDate, &s.Goal, &s.CompleteDate, &s.Draft)`.

In `tam/internal/boardrepo/sprintlist.go`, make the same two changes to `detailSprintsSQL` and to the scan in `sprintsForDetail`.

In `tam/internal/boardrepo/opensprints.go`, add to `SprintChoice` after `State`:

```go
	// Draft marks a sprint drafted in TAM, so a picker can say so beside
	// its name: a card moved into it waits for Commit to create the sprint.
	Draft bool `json:"draft"`
```

Change `openSprintsSQL`'s select list to `SELECT sprint.id, sprint.name, board.name, sprint.state, sprint.draft` and the scan in `OpenSprints` to `rows.Scan(&s.ID, &s.Name, &s.BoardName, &s.State, &s.Draft)`.

In `tam/internal/boardrepo/replace.go`, in `writeSprints`, change the delete to:

```go
	// Only the rows Jira reported are replaced. A draft sprint is TAM's own
	// row, never in what Jira sent, and issuerepo owns it until Commit makes
	// it real; deleting it here would strand its journal row and every card
	// moved into it.
	if _, err := tx.ExecContext(ctx, `DELETE FROM sprint WHERE profile_id = ? AND board_id = ? AND draft = 0`, profileID, boardID); err != nil {
```

- [ ] **Step 5: Run the tests to see them pass**

Run: `cd tam && go test ./internal/tamstore/ ./internal/boardrepo/`
Expected: PASS.

Run: `cd tam && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add tam/internal/tamstore/tamstore.go tam/internal/tamstore/tamstore_test.go tam/internal/boardrepo/boardrepo.go tam/internal/boardrepo/boards.go tam/internal/boardrepo/opensprints.go tam/internal/boardrepo/sprintlist.go tam/internal/boardrepo/replace.go tam/internal/boardrepo/replace_test.go
git commit -m "feat(tam): schema version 13, a sprint row can be a draft"
```

---

### Task 6: Draft sprints in the journal

**Files:**
- Create: `tam/internal/issuerepo/draftsprints.go`
- Modify: `tam/internal/issuerepo/discard.go:27-46` (`DiscardAllPendingChanges`), `:71-105` (`discardOne`)
- Test: `tam/internal/issuerepo/draftsprints_test.go`

**Interfaces:**
- Consumes: `journal.Put`, `journal.Audit`, `journal.List`, `journal.ListForKey`, `journal.Get`, `journal.Delete`, `journal.ErrNotFound` (`core/journal`); `editDraft`, `revertMove`, `MoveID`, `MoveValue`, `EntitySprintMove`, `EntityIssueCreate`, `FieldCreate`, `(*Repository).inTx` (issuerepo); the `sprint.draft` column (Task 5); test helpers `newRepo`, `newRepoWithDB`, `sample()` (`issues_test.go`).
- Produces:
  - `const EntitySprintCreate = "sprint_create"`: journal entity; `entity_key` is the negative id as text, `field` is `FieldCreate`, `after_val` is `DraftSprint` JSON.
  - `var ErrDraftSprintGone = errors.New("issuerepo: the draft sprint is no longer in the journal")`
  - `type DraftSprint struct { BoardID int; BoardName, Name, Goal, StartDate, EndDate string }` (JSON `boardId`, `boardName`, `name`, `goal`, `startDate`, `endDate`) with `func (d DraftSprint) SprintDraft() backend.SprintDraft`
  - `func IsDraftSprintID(id string) bool`
  - `func (r *Repository) CreateDraftSprint(ctx context.Context, profileID string, d DraftSprint) (backend.Sprint, error)`
  - `func (r *Repository) EditDraftSprint(ctx context.Context, profileID string, draftID int, d DraftSprint) (backend.Sprint, error)`
  - `func (r *Repository) DiscardDraftSprint(ctx context.Context, profileID string, draftID int) error`
  - `func rewriteSprintID(ctx context.Context, tx *sql.Tx, profileID, from, to, name string) error` (Task 9's `RekeySprint` uses it)
  - `func draftsWhere(ctx context.Context, tx *sql.Tx, profileID string, match func(backend.IssueDraft) bool) ([]string, error)` (Task 9's `rewriteParentKey` uses it)
  - `discardOne` handles `EntitySprintCreate`; `DiscardAllPendingChanges` skips a row an earlier cascade already removed.

- [ ] **Step 1: Write the failing tests**

`tam/internal/issuerepo/draftsprints_test.go`:

```go
package issuerepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func sprint15() issuerepo.DraftSprint {
	return issuerepo.DraftSprint{
		BoardID: 1, BoardName: "PLAT Scrum", Name: "Sprint 15", Goal: "Ship promos",
		StartDate: "2026-09-16T09:00:00.000+0000", EndDate: "2026-09-30T09:00:00.000+0000",
	}
}

// rowOf finds the one pending row of an entity type under a key.
func rowOf(t *testing.T, repo *issuerepo.Repository, key, entityType string) (journal.PendingChange, bool) {
	t.Helper()
	rows, err := repo.PendingForKey(context.Background(), "p1", key)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rows {
		if p.EntityType == entityType {
			return p, true
		}
	}
	return journal.PendingChange{}, false
}

func TestADraftSprintTakesANegativeIdThatIsNeverHandedOutTwice(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	first, err := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{BoardID: 1, Name: "Sprint 16"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != -1 || second.ID != -2 || first.State != "future" || first.BoardID != 1 || first.Goal != "Ship promos" {
		t.Fatalf("first = %+v, second = %+v", first, second)
	}
	if err := repo.DiscardDraftSprint(ctx, "p1", second.ID); err != nil {
		t.Fatal(err)
	}
	third, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{BoardID: 1, Name: "Sprint 17"})
	if err != nil || third.ID != -3 {
		t.Fatalf("third = %+v, %v; a discarded id is never handed out again, so nothing stale can attach to a new draft", third, err)
	}

	var draft int
	var name string
	if err := db.QueryRow(`SELECT draft, name FROM sprint WHERE profile_id = 'p1' AND id = -1`).Scan(&draft, &name); err != nil || draft != 1 || name != "Sprint 15" {
		t.Errorf("sprint row = draft %d %q, %v", draft, name, err)
	}
	var gone int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND id = -2`).Scan(&gone); err != nil || gone != 0 {
		t.Errorf("the discarded draft's row is gone: %d, %v", gone, err)
	}

	rows, err := repo.ListPendingChanges(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, p := range rows {
		if p.EntityType == issuerepo.EntitySprintCreate {
			keys = append(keys, p.EntityKey)
		}
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "-1" || keys[1] != "-3" {
		t.Errorf("sprint_create keys = %v", keys)
	}
	p, _ := rowOf(t, repo, "-1", issuerepo.EntitySprintCreate)
	var carried issuerepo.DraftSprint
	if err := json.Unmarshal([]byte(p.AfterVal), &carried); err != nil || carried != sprint15() {
		t.Errorf("create row carries the draft: %+v, %v", carried, err)
	}

	if other, err := repo.CreateDraftSprint(ctx, "p2", sprint15()); err != nil || other.ID != -1 {
		t.Errorf("another profile counts from its own start: %+v, %v", other, err)
	}
	if _, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{BoardID: 1, Name: "  "}); err == nil {
		t.Error("a sprint with no name is refused")
	}
	if _, err := repo.CreateDraftSprint(ctx, "p1", issuerepo.DraftSprint{Name: "No board"}); err == nil {
		t.Error("a sprint with no board is refused")
	}
}

func TestEditingADraftSprintRenamesItEverywhereItIsNamed(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	s, err := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(s.ID)
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", id, "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	temp, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "In the draft sprint", SprintID: id, SprintName: "Sprint 15"})
	if err != nil {
		t.Fatal(err)
	}

	edit := sprint15()
	edit.Name, edit.Goal, edit.BoardID = "Sprint 15 promos", "", 99
	got, err := repo.EditDraftSprint(ctx, "p1", s.ID, edit)
	if err != nil || got.Name != "Sprint 15 promos" || got.BoardID != 1 || got.ID != s.ID {
		t.Fatalf("edit = %+v, %v; a draft keeps its id and its board", got, err)
	}

	iss, err := repo.GetIssue(ctx, "p1", "PLAT-409")
	if err != nil || iss.SprintID != id || iss.SprintName != "Sprint 15 promos" {
		t.Errorf("the card's sprint name follows: %+v, %v", iss, err)
	}
	move, ok := rowOf(t, repo, "PLAT-409", issuerepo.EntitySprintMove)
	if !ok || move.AfterVal != id+"|Sprint 15 promos" || move.BeforeVal != "12|Sprint 12" {
		t.Errorf("the journaled move names the new sprint name and keeps where it came from: %+v", move)
	}
	create, _ := rowOf(t, repo, temp, issuerepo.EntityIssueCreate)
	var d backend.IssueDraft
	if err := json.Unmarshal([]byte(create.AfterVal), &d); err != nil || d.SprintID != id || d.SprintName != "Sprint 15 promos" {
		t.Errorf("the draft issue's sprint name follows: %+v, %v", d, err)
	}
	var name, goal string
	if err := db.QueryRow(`SELECT name, goal FROM sprint WHERE profile_id = 'p1' AND id = ?`, s.ID).Scan(&name, &goal); err != nil || name != "Sprint 15 promos" || goal != "" {
		t.Errorf("sprint row = %q %q, %v", name, goal, err)
	}
	if _, err := repo.EditDraftSprint(ctx, "p1", -9, edit); !errors.Is(err, issuerepo.ErrDraftSprintGone) {
		t.Errorf("editing a draft that is not there = %v", err)
	}
}

func TestDiscardingADraftSprintPutsEveryCardItHeldBack(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	s, _ := repo.CreateDraftSprint(ctx, "p1", sprint15())
	id := strconv.Itoa(s.ID)
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", id, "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	temp, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "In the draft sprint", SprintID: id, SprintName: "Sprint 15"})

	create, ok := rowOf(t, repo, id, issuerepo.EntitySprintCreate)
	if !ok {
		t.Fatal("no sprint_create row")
	}
	if err := repo.DiscardPendingChange(ctx, "p1", create.ID); err != nil {
		t.Fatal(err)
	}

	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-409"); iss.SprintID != "12" || iss.SprintName != "Sprint 12" {
		t.Errorf("the card is back in the sprint it came from: %+v", iss)
	}
	if _, ok := rowOf(t, repo, "PLAT-409", issuerepo.EntitySprintMove); ok {
		t.Error("the move into the discarded sprint is gone from the journal")
	}
	if iss, _ := repo.GetIssue(ctx, "p1", temp); iss.SprintID != "" || iss.SprintName != "" {
		t.Errorf("the draft issue left the sprint: %+v", iss)
	}
	draftRow, _ := rowOf(t, repo, temp, issuerepo.EntityIssueCreate)
	var d backend.IssueDraft
	_ = json.Unmarshal([]byte(draftRow.AfterVal), &d)
	if d.SprintID != "" || d.SprintName != "" {
		t.Errorf("the draft issue's JSON left the sprint: %+v", d)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND id = ?`, s.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("the draft sprint row is gone: %d, %v", n, err)
	}
	if err := repo.DiscardDraftSprint(ctx, "p1", s.ID); !errors.Is(err, issuerepo.ErrDraftSprintGone) {
		t.Errorf("a second discard = %v", err)
	}
}

// journal.List is newest first by created_at and then by id. A card moved to
// Sprint 13 first and into the draft afterwards keeps its older row id with
// the same-second created_at, so the sprint_create row, newer by id, comes
// first; its discard takes that move with it, and Discard all must not trip
// over the row it already removed.
func TestDiscardAllWithADraftSprintRevertsEachChangeOnce(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", "13", "Sprint 13"); err != nil {
		t.Fatal(err)
	}
	s, _ := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", strconv.Itoa(s.ID), "Sprint 15"); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.DiscardAllPendingChanges(ctx, "p1"); err != nil {
		t.Fatalf("discard all: %v", err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-409"); iss.SprintID != "12" {
		t.Errorf("the card is back where it started: %+v", iss)
	}
	if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
		t.Errorf("nothing pending: %+v", rows)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-409", 0)
	discards := 0
	for _, a := range act {
		if a.Action == "discard" {
			discards++
		}
	}
	if discards != 1 {
		t.Errorf("the move was discarded once, not twice: %+v", act)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam && go test ./internal/issuerepo/ -run 'DraftSprint|DiscardAllWithADraftSprint'`
Expected: FAIL, `undefined: issuerepo.DraftSprint`.

- [ ] **Step 3: Write `draftsprints.go`**

`tam/internal/issuerepo/draftsprints.go`:

```go
package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// A sprint drafted in TAM. Creating a sprint used to reach Jira the moment
// the dialog was confirmed, which made a plan that starts with a new sprint
// impossible to draft offline. It is journaled now, the way a TAM-NEW issue
// is: a sprint_create row carrying the draft, and a row in sprint under a
// negative id with draft = 1, so every picker that reads the sprint table
// offers it and every card moved into it journals an ordinary issue_sprint
// row naming that negative id. Commit's first phase creates it in Jira and
// RekeySprint rewrites the id everywhere before any move naming it is sent.
//
// This package writes those sprint rows although the sprint table is
// boardrepo's. It writes only draft = 1 rows and the one statement that
// turns one real, because the row has to land and go in the same
// transaction as its journal row, and a discard has to revert the moves into
// it in that transaction too; two repositories would mean two transactions
// and a crash window leaving a sprint nobody can discard.
//
// Only the creation moves into the journal. Starting, completing, editing
// and deleting a sprint Jira already holds stay immediate writes in
// internal/sprints.

// EntitySprintCreate is the journal entity type of a drafted sprint. Its key
// is the negative id as text, its field FieldCreate, and its after_val the
// DraftSprint as JSON.
const EntitySprintCreate = "sprint_create"

// draftSprintSeq is the profile setting holding the last negative id handed
// out, so an id is never handed out twice even after its draft is discarded
// or committed: a stale reference to an old draft can then never attach to
// a new one.
const draftSprintSeq = "draft_sprint_seq"

// ErrDraftSprintGone is what a write aimed at a draft sprint answers when its
// sprint_create row is no longer in the journal: discarded, or committed.
var ErrDraftSprintGone = errors.New("issuerepo: the draft sprint is no longer in the journal")

// DraftSprint is what a sprint_create row carries. BoardName is kept for the
// Pending changes dialog, which has no board list of its own to name it from.
type DraftSprint struct {
	BoardID   int    `json:"boardId"`
	BoardName string `json:"boardName"`
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// SprintDraft is the part of the draft the Agile create takes.
func (d DraftSprint) SprintDraft() backend.SprintDraft {
	return backend.SprintDraft{Name: d.Name, Goal: d.Goal, StartDate: d.StartDate, EndDate: d.EndDate}
}

// IsDraftSprintID says a sprint id is a draft's: a negative whole number.
func IsDraftSprintID(id string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(id))
	return err == nil && n < 0
}

// CreateDraftSprint journals a new sprint on a board and writes its draft
// row, and answers with the sprint as the pickers will read it. The dates
// arrive already in the Agile API's own format: converting a date input's
// bare day is internal/sprints' job, the same as for a sprint Jira holds.
func (r *Repository) CreateDraftSprint(ctx context.Context, profileID string, d DraftSprint) (backend.Sprint, error) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return backend.Sprint{}, errors.New("a sprint needs a name")
	}
	if d.BoardID <= 0 {
		return backend.Sprint{}, errors.New("a sprint needs the board it belongs to")
	}
	var made backend.Sprint
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		id, err := nextDraftSprintID(ctx, tx, profileID)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("encode draft sprint: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date, goal, draft)
			 VALUES (?, ?, ?, ?, 'future', ?, ?, ?, 1)`,
			profileID, id, d.BoardID, d.Name, d.StartDate, d.EndDate, d.Goal); err != nil {
			return fmt.Errorf("insert draft sprint: %w", err)
		}
		key := strconv.Itoa(id)
		if err := journal.Put(tx, profileID, EntitySprintCreate, key, FieldCreate, "", string(encoded), ""); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, EntitySprintCreate, key, "create", "", "", d.Name, "drafted on board "+strconv.Itoa(d.BoardID)); err != nil {
			return err
		}
		made = backend.Sprint{ID: id, BoardID: d.BoardID, Name: d.Name, State: "future", StartDate: d.StartDate, EndDate: d.EndDate, Goal: d.Goal}
		return nil
	})
	if err != nil {
		return backend.Sprint{}, err
	}
	return made, nil
}

// nextDraftSprintID is one below the lowest of the stored sequence and every
// negative id already in the sprint table, and records itself as the new
// sequence inside the caller's transaction, so two creates cannot share it.
func nextDraftSprintID(ctx context.Context, tx *sql.Tx, profileID string) (int, error) {
	lowest := 0
	var stored string
	err := tx.QueryRowContext(ctx, `SELECT value FROM profile_setting WHERE profile_id = ? AND key = ?`, profileID, draftSprintSeq).Scan(&stored)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("draft sprint sequence: %w", err)
	}
	if n, perr := strconv.Atoi(stored); perr == nil && n < lowest {
		lowest = n
	}
	var inTable sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MIN(id) FROM sprint WHERE profile_id = ? AND id < 0`, profileID).Scan(&inTable); err != nil {
		return 0, fmt.Errorf("lowest draft sprint id: %w", err)
	}
	if inTable.Valid && int(inTable.Int64) < lowest {
		lowest = int(inTable.Int64)
	}
	next := lowest - 1
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO profile_setting (profile_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(profile_id, key) DO UPDATE SET value = excluded.value`,
		profileID, draftSprintSeq, strconv.Itoa(next)); err != nil {
		return 0, fmt.Errorf("record draft sprint sequence: %w", err)
	}
	return next, nil
}

// EditDraftSprint rewrites a draft sprint's name, goal and dates, locally,
// and renames it everywhere its name is carried: the sprint row, the cached
// cards in it, the journaled moves into it, and the draft issues in it. The
// board is the draft's own and does not change.
func (r *Repository) EditDraftSprint(ctx context.Context, profileID string, draftID int, d DraftSprint) (backend.Sprint, error) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return backend.Sprint{}, errors.New("a sprint needs a name")
	}
	key := strconv.Itoa(draftID)
	var made backend.Sprint
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		was, err := readDraftSprint(ctx, tx, profileID, key)
		if err != nil {
			return err
		}
		d.BoardID, d.BoardName = was.BoardID, was.BoardName
		encoded, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("encode draft sprint: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			string(encoded), profileID, EntitySprintCreate, key); err != nil {
			return fmt.Errorf("rewrite draft sprint %s: %w", key, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE sprint SET name = ?, goal = ?, start_date = ?, end_date = ? WHERE profile_id = ? AND id = ? AND draft = 1`,
			d.Name, d.Goal, d.StartDate, d.EndDate, profileID, draftID); err != nil {
			return fmt.Errorf("rewrite draft sprint row %s: %w", key, err)
		}
		if err := rewriteSprintID(ctx, tx, profileID, key, key, d.Name); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, EntitySprintCreate, key, "edit", "", was.Name, d.Name, ""); err != nil {
			return err
		}
		made = backend.Sprint{ID: draftID, BoardID: d.BoardID, Name: d.Name, State: "future", StartDate: d.StartDate, EndDate: d.EndDate, Goal: d.Goal}
		return nil
	})
	if err != nil {
		return backend.Sprint{}, err
	}
	return made, nil
}

// DiscardDraftSprint is the Sprints view's Delete on a draft: exactly what
// discarding its sprint_create row from Pending changes does.
func (r *Repository) DiscardDraftSprint(ctx context.Context, profileID string, draftID int) error {
	key := strconv.Itoa(draftID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := journal.ListForKey(tx, profileID, key)
		if err != nil {
			return err
		}
		for _, p := range rows {
			if p.EntityType == EntitySprintCreate {
				return discardOne(ctx, tx, profileID, p)
			}
		}
		return ErrDraftSprintGone
	})
}

// readDraftSprint decodes the draft a sprint_create row carries.
func readDraftSprint(ctx context.Context, tx *sql.Tx, profileID, key string) (DraftSprint, error) {
	var raw string
	err := tx.QueryRowContext(ctx,
		`SELECT after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		profileID, EntitySprintCreate, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DraftSprint{}, ErrDraftSprintGone
	}
	if err != nil {
		return DraftSprint{}, fmt.Errorf("read draft sprint %s: %w", key, err)
	}
	var d DraftSprint
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return DraftSprint{}, fmt.Errorf("decode draft sprint %s: %w", key, err)
	}
	return d, nil
}

// discardDraftSprint is what discarding a sprint_create row takes with it:
// every journaled move into the sprint is reverted and dropped, every draft
// issue in it leaves it, and its draft row goes. The create row itself is
// deleted and audited by discardOne, like every other row.
func discardDraftSprint(ctx context.Context, tx *sql.Tx, profileID, key string) error {
	all, err := journal.List(tx, profileID)
	if err != nil {
		return err
	}
	for _, p := range all {
		if p.EntityType != EntitySprintMove || MoveID(p.AfterVal) != key {
			continue
		}
		if err := revertMove(ctx, tx, profileID, p); err != nil {
			return err
		}
		if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "discard", p.Field, p.AfterVal, p.BeforeVal, "the draft sprint it was moving into was discarded"); err != nil {
			return err
		}
	}
	if err := rewriteSprintID(ctx, tx, profileID, key, "", ""); err != nil {
		return err
	}
	draftID, _ := strconv.Atoi(key)
	if _, err := tx.ExecContext(ctx, `DELETE FROM sprint WHERE profile_id = ? AND id = ? AND draft = 1`, profileID, draftID); err != nil {
		return fmt.Errorf("drop draft sprint %s: %w", key, err)
	}
	return nil
}

// rewriteSprintID repoints every place a sprint id is carried from one id to
// another, under the name given: the cached cards' sprint columns, both
// halves of every journaled sprint move, and every draft issue's JSON. It is
// how an edit renames a draft (from == to), how a discard empties it (to ==
// ""), and how RekeySprint makes it real. A move whose value becomes the
// backlog is packed empty, the way MoveValue packs every backlog move.
func rewriteSprintID(ctx context.Context, tx *sql.Tx, profileID, from, to, name string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE issue SET sprint_id = ?, sprint_name = ? WHERE profile_id = ? AND sprint_id = ?`,
		to, name, profileID, from); err != nil {
		return fmt.Errorf("repoint cards from sprint %s: %w", from, err)
	}
	moves, err := tx.QueryContext(ctx,
		`SELECT id, before_val, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ?`,
		profileID, EntitySprintMove)
	if err != nil {
		return fmt.Errorf("sprint moves naming %s: %w", from, err)
	}
	type repoint struct {
		id            int64
		before, after string
	}
	var todo []repoint
	for moves.Next() {
		var rp repoint
		if err := moves.Scan(&rp.id, &rp.before, &rp.after); err != nil {
			moves.Close()
			return err
		}
		changed := false
		if MoveID(rp.before) == from {
			rp.before, changed = MoveValue(to, name), true
		}
		if MoveID(rp.after) == from {
			rp.after, changed = MoveValue(to, name), true
		}
		if changed {
			todo = append(todo, rp)
		}
	}
	moves.Close()
	if err := moves.Err(); err != nil {
		return err
	}
	for _, rp := range todo {
		if _, err := tx.ExecContext(ctx,
			`UPDATE pending_change SET before_val = ?, after_val = ? WHERE profile_id = ? AND id = ?`,
			rp.before, rp.after, profileID, rp.id); err != nil {
			return fmt.Errorf("repoint sprint move %d: %w", rp.id, err)
		}
	}
	drafts, err := draftsWhere(ctx, tx, profileID, func(d backend.IssueDraft) bool { return d.SprintID == from })
	if err != nil {
		return err
	}
	for _, key := range drafts {
		if err := editDraft(ctx, tx, profileID, key, func(d *backend.IssueDraft) {
			d.SprintID, d.SprintName = to, name
		}); err != nil {
			return err
		}
	}
	return nil
}

// draftsWhere lists the keys of the drafts whose create JSON matches. A row
// that will not decode is skipped: it cannot name anything, and Commit
// reports it on its own.
func draftsWhere(ctx context.Context, tx *sql.Tx, profileID string, match func(backend.IssueDraft) bool) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT entity_key, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ?`,
		profileID, EntityIssueCreate)
	if err != nil {
		return nil, fmt.Errorf("draft issues: %w", err)
	}
	var keys []string
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		var d backend.IssueDraft
		if json.Unmarshal([]byte(raw), &d) == nil && match(d) {
			keys = append(keys, key)
		}
	}
	rows.Close()
	return keys, rows.Err()
}
```

- [ ] **Step 4: Wire the discard**

In `tam/internal/issuerepo/discard.go`, in `discardOne`, add a case directly after the `EntityIssueCreate` case:

```go
	case p.EntityType == EntitySprintCreate:
		if err := discardDraftSprint(ctx, tx, profileID, p.EntityKey); err != nil {
			return err
		}
```

In `DiscardAllPendingChanges`, replace the `for _, p := range all { ... }` loop with:

```go
		for _, p := range all {
			// Discarding a draft sprint takes the moves into it along with
			// it, so a row read at the start may already be gone.
			if _, err := journal.Get(tx, profileID, p.ID); errors.Is(err, journal.ErrNotFound) {
				continue
			} else if err != nil {
				return err
			}
			if err := discardOne(ctx, tx, profileID, p); err != nil {
				return err
			}
		}
```

- [ ] **Step 5: Run the tests to see them pass**

Run: `cd tam && go test ./internal/issuerepo/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add tam/internal/issuerepo/draftsprints.go tam/internal/issuerepo/draftsprints_test.go tam/internal/issuerepo/discard.go
git commit -m "feat(tam): journal a new sprint as a draft with a negative id"
```

---

### Task 7: Sprint creation leaves the immediate writes

**Files:**
- Create: `tam/internal/sprints/draft.go`, `tam/internal/sprints/draft_test.go`, `tam/internal/ritualsync/drafts_test.go`, `tam/internal/importer/draftsprint_test.go`
- Modify: `tam/internal/sprints/manage.go:31-62` (delete `Create`), `:76` (`Edit`), `:169` (`Delete`), `:297-305` (delete `createdName`)
- Modify: `tam/internal/sprints/sprints.go:1-30` (package doc), `:80-88` (`lifecycle` loses `CreateSprint`), `Start`, `Complete`
- Modify: `tam/internal/sprints/guards.go:29-42` (`destinationID`)
- Modify: `tam/internal/sprints/exceptions_test.go`, `tam/internal/sprints/manage_test.go`
- Modify: `tam/app_sprintmanage.go` (the three bindings), `tam/app_rituals.go:131-142` (`ritualSprint`), `tam/internal/ritualsync/ensure.go:42-54` (`Sprints`)
- Test: `tam/app_sprintmanage_test.go`, `tam/app_rituals_test.go`
- Regenerate: `tam/frontend/wailsjs/**`

**Interfaces:**
- Consumes: `issuerepo.CreateDraftSprint`, `EditDraftSprint`, `DiscardDraftSprint`, `DraftSprint`, `EntitySprintCreate` (Task 6); `boardrepo.Sprint.Draft`, `SprintChoice.Draft` (Task 5); the existing `dates` helper in `guards.go`.
- Produces:
  - `func sprints.DraftSprint(d backend.SprintDraft) (backend.SprintDraft, error)`: a package function, which the fence does not see.
  - `errDraftSprint` (`this sprint is a draft in TAM; Commit creates it in Jira first`), returned first by `Start`, `Complete`, `Edit`, `Delete` for a negative id.
  - `sprints.Service`'s exported methods are exactly `Complete, Delete, Edit, Start`.
  - `App.CreateSprint` journals a draft and answers `SprintCreated{Sprint: <draft>, Note: ""}`; `App.EditSprint`/`App.DeleteSprint` handle a negative id locally; all three still take `a.acquire(p.ID, "sprint")`.
  - `func (a *App) cachedBoardName(profileID string, boardID int) (string, error)`
  - `ritualsync.Sprints` skips a draft; `App.ritualSprint` refuses one.

- [ ] **Step 1: Write the failing tests**

`tam/internal/sprints/draft_test.go`:

```go
package sprints_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprints"
)

func TestDraftSprintConvertsTheDatesAndRefusesWhatCannotBeASprint(t *testing.T) {
	d, err := sprints.DraftSprint(backend.SprintDraft{Name: "  Sprint 15 ", Goal: "Ship promos", StartDate: "2026-09-16", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatalf("DraftSprint: %v", err)
	}
	if d.Name != "Sprint 15" || !strings.HasPrefix(d.StartDate, "2026-09-16T") || !strings.HasPrefix(d.EndDate, "2026-09-30T") {
		t.Errorf("draft = %+v, want a trimmed name and the Agile API's own datetimes", d)
	}
	if _, err := sprints.DraftSprint(backend.SprintDraft{Name: " ", StartDate: "2026-09-16", EndDate: "2026-09-30"}); err == nil {
		t.Error("a sprint with no name is refused")
	}
	if _, err := sprints.DraftSprint(backend.SprintDraft{Name: "Sprint 15", StartDate: "2026-09-30", EndDate: "2026-09-16"}); err == nil {
		t.Error("a sprint that ends before it starts is refused")
	}
}

// A draft sprint's id is negative and means nothing to Jira. None of the four
// immediate writes may send it, and a completion may not push cards into
// one, whichever path reached the service.
func TestNoImmediateWriteReachesJiraForADraftSprint(t *testing.T) {
	b := &fakeBackend{sprints: []backend.Sprint{{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"}}}
	store := newStore()
	s := manageService(b, store, newIssues(store))
	ctx := context.Background()

	if _, err := s.Start(ctx, "p1", 1, -1, draft("Sprint 15", "")); err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("start = %v", err)
	}
	if _, err := s.Edit(ctx, "p1", 1, -1, draft("Sprint 15", ""), false); err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("edit = %v", err)
	}
	if _, err := s.Delete(ctx, "p1", 1, -1); err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("delete = %v", err)
	}
	if _, err := s.Complete(ctx, "p1", 1, -1, ""); err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("complete = %v", err)
	}
	if _, err := s.Complete(ctx, "p1", 1, 12, "-1"); err == nil || !strings.Contains(err.Error(), "draft sprint") {
		t.Errorf("complete into a draft = %v", err)
	}
	if b.sprintReads != 0 || len(b.starts) != 0 || len(b.edited) != 0 || len(b.deleted) != 0 || len(b.completed) != 0 || len(b.moves) != 0 {
		t.Errorf("Jira was asked something: reads %d, starts %v, edits %v, deletes %v, completes %v, moves %v",
			b.sprintReads, b.starts, b.edited, b.deleted, b.completed, b.moves)
	}
}
```

In `tam/internal/sprints/manage_test.go`, delete `TestCreateSendsTheDraftToJiraAndCachesTheSprintWithItsGoal`, `TestACreateWhoseRefreshComesBackEmptyStillReportsTheSprintAndWhatToDo`, `TestCreateRefusesAnEndBeforeItsStartBeforeJiraHearsAboutIt`, and `TestAManagementWriteWithNoIssueCacheWiredStillLandsInJira`. The fakes' `CreateSprint` methods in `sprints_test.go` and `cache_internal_test.go` stay: harmless extra methods on test doubles.

In `tam/internal/sprints/exceptions_test.go`, replace the `immediateWrites` comment and value with:

```go
// immediateWrites is the fence. Every method named here reaches Jira the
// moment it is called, outside the journal and outside Commit, and they are
// the only writes in TAM that do.
//
// Create was a fifth until the create and commit correctness bundle
// (docs/superpowers/specs/2026-09-15-tam-01-create-commit-correctness-design.md).
// A plan that starts with a new sprint could not be drafted offline while
// creating one reached Jira at once, so creation moved into the journal: a
// sprint_create row and a draft sprint under a negative id, created in
// Commit's first phase. Editing, deleting, starting and completing a sprint
// Jira already holds stay here, for the reasons the sprints design gives.
//
// It is the Service's own exported method set and deliberately not the
// lifecycle interface, which is a different list for a different purpose:
// that one carries BoardSprints, a read, and MoveIssuesToSprint, which its
// other caller journals like every other membership change. A test claiming
// those were immediate writes would have been false the day it was written,
// and a fence nobody believes is worse than no fence.
var immediateWrites = []string{"Complete", "Delete", "Edit", "Start"}
```

Rename the test to `TestTheImmediateWritesAreExactlyTheFourThatWereArguedFor`, and in its message replace `here adds a sixth, so amend section 3 of the sprints design ` with `here adds a fifth, so amend section 3 of the sprints design `, and `exactly that reason. This fence only ever sees methods, and cannot: a sixth immediate ` with `exactly that reason, and so is DraftSprint. This fence only ever sees methods, and cannot: a fifth immediate `.

In `tam/app_sprintmanage_test.go`:

1. In `manageLifecycleBackend`, add the field `createCalls int` and change its `CreateSprint` to:

```go
func (b *manageLifecycleBackend) CreateSprint(context.Context, int, backend.SprintDraft) (backend.Sprint, error) {
	b.createCalls++
	return b.made, nil
}
```

2. Replace `TestCreateSprintReachesTheServiceAndReturnsTheSprintJiraMade` with:

```go
// Creating a sprint is journaled now: the binding answers with a draft under
// a negative id, every picker offers it at once, and Jira hears nothing
// until Commit.
func TestCreateSprintDraftsTheSprintAndSendsNothingToJira(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	fake := &manageLifecycleBackend{simpleBoardBackend: *twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")}
	a.backends[p.ID] = fake
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	got, err := a.CreateSprint(p.ID, 1, "Sprint 15", "Ship promos", "2026-09-16", "2026-09-30")
	if err != nil {
		t.Fatalf("CreateSprint: %v", err)
	}
	if got.Sprint.ID != -1 || got.Sprint.State != "future" || got.Sprint.Goal != "Ship promos" || got.Note != "" {
		t.Errorf("CreateSprint = %+v", got)
	}
	if fake.createCalls != 0 {
		t.Errorf("Jira was asked to create the sprint %d times, want none before Commit", fake.createCalls)
	}
	open, err := a.ListOpenSprints(p.ID)
	if err != nil || len(open) != 1 || open[0] != (boardrepo.SprintChoice{ID: -1, Name: "Sprint 15", BoardName: "PLAT Scrum", State: "future", Draft: true}) {
		t.Errorf("open sprints = %+v, %v", open, err)
	}
	listed, _ := a.ListBoardSprints(p.ID, 1)
	if len(listed) != 1 || !strings.HasPrefix(listed[0].StartDate, "2026-09-16") || !listed[0].Draft {
		t.Errorf("board sprints = %+v, want the draft with converted dates", listed)
	}
	pending, _ := a.ListPendingChanges(p.ID)
	if len(pending) != 1 || pending[0].EntityType != issuerepo.EntitySprintCreate || pending[0].EntityKey != "-1" {
		t.Errorf("pending = %+v", pending)
	}
	if _, err := a.CreateSprint(p.ID, 7, "Sprint 16", "", "2026-09-16", "2026-09-30"); err == nil || !strings.Contains(err.Error(), "not in the cache") {
		t.Errorf("a board the cache does not hold = %v", err)
	}
}

// A draft sprint is edited and deleted locally, cards and all.
func TestEditAndDeleteOfADraftSprintStayLocal(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	fake := &manageLifecycleBackend{simpleBoardBackend: *twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")}
	a.backends[p.ID] = fake
	if err := a.repo.UpsertPage(a.ctx, p.ID, []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", StatusID: "1", Updated: "2026-09-01T00:00:00Z"},
	}, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum},
		[]backend.BoardColumn{{Name: "To Do", StatusIDs: []string{"1"}}}, nil, map[string][]string{"": {"PLAT-1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateSprint(p.ID, 1, "Sprint 15", "", "2026-09-16", "2026-09-30"); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveIssueToSprint(p.ID, "PLAT-1", "-1"); err != nil {
		t.Fatal(err)
	}

	details, err := a.ListBoardSprintDetails(p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) < 1 || details[0].ID != -1 || !details[0].Draft || details[0].Total != 1 || details[0].Issues[0].Key != "PLAT-1" {
		t.Fatalf("details = %+v, want the draft sprint holding the moved card", details)
	}

	if note, err := a.EditSprint(p.ID, 1, -1, "Sprint 15 promos", "", "2026-09-16", "2026-10-01", false); err != nil || note != "" {
		t.Fatalf("EditSprint = %q, %v", note, err)
	}
	if fake.editedID != 0 {
		t.Errorf("the edit reached Jira for sprint %d", fake.editedID)
	}
	if listed, _ := a.ListBoardSprints(p.ID, 1); len(listed) != 1 || listed[0].Name != "Sprint 15 promos" {
		t.Errorf("board sprints = %+v", listed)
	}

	if note, err := a.DeleteSprint(p.ID, 1, -1); err != nil || note != "" {
		t.Fatalf("DeleteSprint = %q, %v", note, err)
	}
	if listed, _ := a.ListBoardSprints(p.ID, 1); len(listed) != 0 {
		t.Errorf("board sprints after delete = %+v", listed)
	}
	if pending, _ := a.ListPendingChanges(p.ID); len(pending) != 0 {
		t.Errorf("pending after delete = %+v, want the create and the move both gone", pending)
	}
}
```

Add `"time"`, `"agile-suite/tam/internal/boardrepo"` and `"agile-suite/tam/internal/issuerepo"` to that file's imports.

In `tam/app_rituals_test.go`, add `"strings"` to its imports (`backend` is already imported) and add:

```go
// A draft sprint gets no ritual pages: Commit may never create it, and a
// Sync would otherwise write Confluence pages for a sprint Jira does not
// have.
func TestEnsureSprintRitualsRefusesADraftSprint(t *testing.T) {
	a, p := newTestAppWithRituals(t)
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateSprint(p.ID, 1, "Sprint 15", "", "2026-09-16", "2026-09-30"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.EnsureSprintRituals(p.ID, 1, -1); err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("EnsureSprintRituals on a draft = %v", err)
	}
}
```

`tam/internal/importer/draftsprint_test.go` (a guard rather than a red test: the importer needs no change, since it matches every sprint `OpenSprints` offers, and this pins that a draft is among them):

```go
package importer_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/importer"
)

// A draft sprint is an open sprint like any other to the Sprint column: the
// imported draft carries its negative id, and Commit creates the sprint
// before the draft is moved into it.
func TestTheSprintColumnMatchesADraftSprint(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	open := append(openSprints(), boardrepo.SprintChoice{ID: -1, Name: "Sprint 15", BoardName: "PLAT Scrum", State: "future", Draft: true})
	records := [][]string{{"Type", "Summary", "Sprint"}, {"Task", "Into the draft sprint", "sprint 15"}}
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", open, records, importer.AutoMap(records[0]), "plan.csv", false)
	if err != nil || len(res.Created) != 1 || len(res.Errors) != 0 {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	iss, err := repo.GetIssue(ctx, "p1", res.Created[0])
	if err != nil || iss.SprintID != "-1" || iss.SprintName != "Sprint 15" {
		t.Errorf("the imported draft is in the draft sprint: %+v, %v", iss, err)
	}
}
```

`tam/internal/ritualsync/drafts_test.go`:

```go
package ritualsync

import (
	"testing"

	"agile-suite/tam/internal/boardrepo"
)

func TestSprintsLeavesOutADraftSprint(t *testing.T) {
	got := Sprints([]boardrepo.Sprint{
		{ID: -1, BoardID: 1, Name: "Sprint 15", State: "future", Draft: true},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"},
	}, "PLAT Scrum", nil)
	if len(got) != 1 || got[0].Info.ID != 13 {
		t.Errorf("sprints = %+v, want only the one Jira holds", got)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam && go test ./internal/sprints/ ./internal/ritualsync/`
Expected: FAIL: `undefined: sprints.DraftSprint`, the fence lists `Create`, and `Sprints` keeps the draft.

Run: `cd tam && go test . -run 'CreateSprintDrafts|EditAndDeleteOfADraftSprint|EnsureSprintRitualsRefuses'`
Expected: FAIL: `CreateSprint` answers the sprint the fake made rather than a draft.

- [ ] **Step 3: The package function and the draft refusals**

`tam/internal/sprints/draft.go`:

```go
package sprints

import (
	"errors"
	"strings"

	"agile-suite/tam/internal/backend"
)

// errDraftSprint is what every immediate write answers for a draft sprint's
// negative id. The id means nothing to Jira, and the bound methods are
// reachable without the menus that disable these actions for a draft.
var errDraftSprint = errors.New("this sprint is a draft in TAM; Commit creates it in Jira first")

// DraftSprint checks a sprint about to be drafted and converts its dates the
// way a start or an edit converts them: a name is required, and a date that
// is not a date or an end before its start is refused here, where the
// message can name it. It is a package function and not a Service method
// because it writes nothing and reaches nothing: the draft is journaled by
// issuerepo, and Commit creates it.
func DraftSprint(d backend.SprintDraft) (backend.SprintDraft, error) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return d, errors.New("a sprint needs a name")
	}
	var err error
	if d.StartDate, d.EndDate, err = dates(d.StartDate, d.EndDate); err != nil {
		return d, err
	}
	return d, nil
}
```

In `tam/internal/sprints/manage.go`, delete `Create` and its doc comment, and delete `createdName`. Add as the first statement of both `Edit` and `Delete` (before `Delete`'s `s.Issues == nil` check):

```go
	if sprintID < 0 {
		return "", errDraftSprint
	}
```

In `tam/internal/sprints/sprints.go`, add the same three lines as the first statement of `Start`, and as the first statement of `Complete`:

```go
	if sprintID < 0 {
		return Completion{Failed: []string{}}, errDraftSprint
	}
```

Remove the `CreateSprint(ctx context.Context, boardID int, d backend.SprintDraft) (backend.Sprint, error)` line from the `lifecycle` interface.

Replace the package doc's first two paragraphs (from `// Package sprints owns the five writes` through `// what it is.`) with:

```go
// Package sprints owns the four writes that reach Jira the moment they are
// made: starting a sprint, completing one, and editing and deleting one. They
// are the only writes in TAM that do not go through the journal, and the
// reason is a cost rather than a principle.
//
// Creating a sprint was the fifth, on the argument that a sprint's id has to
// be real before anything can point at it. The create and commit
// correctness bundle paid the cost that argument named: a sprint created in
// TAM is a sprint_create journal row and a draft row under a negative id,
// every card moved into it journals that id, and Commit's first phase
// creates it in Jira and rewrites the id everywhere before anything naming
// it is sent. DraftSprint is the check a draft gets; issuerepo holds the
// rest.
```

In the paragraph beginning `// The exception has one home here`, replace `a sixth` with `a fifth`.

In `tam/internal/sprints/guards.go`, in `destinationID`, replace:

```go
	n, err := strconv.Atoi(moveTo)
	if err != nil || n <= 0 || strconv.Itoa(n) != moveTo {
```

with:

```go
	n, err := strconv.Atoi(moveTo)
	if err == nil && n < 0 {
		return "", errors.New("a completion cannot move cards into a draft sprint; Commit the draft sprint first")
	}
	if err != nil || n <= 0 || strconv.Itoa(n) != moveTo {
```

- [ ] **Step 4: The bindings**

In `tam/app_sprintmanage.go`, replace everything from `// SprintCreated is what CreateSprint answers with.` down to the end of `DeleteSprint` with:

```go
// SprintCreated is what CreateSprint answers with: the draft sprint, whose
// negative id is what the dialog switches a picker to. Note stays in the
// shape, always empty now that nothing is re-read from Jira, so the
// frontend's one path reads the same field it always has.
type SprintCreated struct {
	Sprint backend.Sprint `json:"sprint"`
	Note   string         `json:"note"`
}

// CreateSprint drafts a new sprint on the board with the dialog's four
// fields. Nothing reaches Jira: the draft is journaled, every picker offers
// it at once, and Commit creates it before any card moved into it is sent.
// The "sprint" lock is still taken, which serialises a draft against a
// Commit rewriting the same rows and keeps the frontend's runQuietLock
// honest.
func (a *App) CreateSprint(profileID string, boardID int, name, goal, start, end string) (SprintCreated, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return SprintCreated{}, err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return SprintCreated{}, err
	}
	defer a.release(p.ID)

	d, err := sprints.DraftSprint(backend.SprintDraft{Name: name, Goal: goal, StartDate: start, EndDate: end})
	if err != nil {
		return SprintCreated{}, err
	}
	boardName, err := a.cachedBoardName(p.ID, boardID)
	if err != nil {
		return SprintCreated{}, err
	}
	made, err := a.repo.CreateDraftSprint(a.ctx, p.ID, issuerepo.DraftSprint{
		BoardID: boardID, BoardName: boardName, Name: d.Name, Goal: d.Goal, StartDate: d.StartDate, EndDate: d.EndDate,
	})
	if err != nil {
		return SprintCreated{}, err
	}
	log.Printf("tam: drafted sprint %d %q on board %d for %s (%s)", made.ID, made.Name, boardID, p.Name, p.ProjectKey)
	return SprintCreated{Sprint: made}, nil
}

// cachedBoardName is the name of a board the cache holds, and a refusal for
// one it does not: a draft sprint on a board nobody synced would be offered
// by no picker, since every one of them joins to the board row.
func (a *App) cachedBoardName(profileID string, boardID int) (string, error) {
	boards, err := a.boards.ListBoards(a.ctx, profileID)
	if err != nil {
		return "", err
	}
	for _, b := range boards {
		if b.ID == boardID {
			return b.Name, nil
		}
	}
	return "", fmt.Errorf("board %d is not in the cache; refresh the boards first", boardID)
}

// EditSprint rewrites a sprint's name, goal and dates. A draft sprint, a
// negative id, is rewritten locally; a sprint Jira holds is edited in Jira at
// once. clearGoal is what tells an empty goal box left that way from one
// asking to remove a goal that was there, the same distinction
// internal/sprints.Service.Edit's own doc explains; a draft needs no such
// flag, since its goal is simply what the dialog sent.
func (a *App) EditSprint(profileID string, boardID, sprintID int, name, goal, start, end string, clearGoal bool) (string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return "", err
	}
	defer a.release(p.ID)

	draft := backend.SprintDraft{Name: name, Goal: goal, StartDate: start, EndDate: end}
	if sprintID < 0 {
		d, err := sprints.DraftSprint(draft)
		if err != nil {
			return "", err
		}
		if _, err := a.repo.EditDraftSprint(a.ctx, p.ID, sprintID, issuerepo.DraftSprint{Name: d.Name, Goal: d.Goal, StartDate: d.StartDate, EndDate: d.EndDate}); err != nil {
			return "", err
		}
		return "", nil
	}
	b, err := a.backendFor(p)
	if err != nil {
		return "", err
	}
	log.Printf("tam: editing sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	note, err := a.sprintService(p, b).Edit(a.ctx, p.ID, boardID, sprintID, draft, clearGoal)
	if err != nil {
		log.Printf("tam: edit sprint %d for %s failed: %v", sprintID, p.Name, err)
		return "", ceremonyError(err)
	}
	if note != "" {
		log.Printf("tam: sprint %d for %s edited, with a note: %s", sprintID, p.Name, note)
	}
	return note, nil
}

// DeleteSprint deletes a sprint. A draft sprint is discarded locally, which
// puts every card moved into it back where it was; a sprint Jira holds is
// destroyed in Jira and then removed from TAM's copies, which is why
// internal/sprints.Service.Delete refuses before Jira is asked anything when
// the issue cache is not wired.
func (a *App) DeleteSprint(profileID string, boardID, sprintID int) (string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", err
	}
	if err := a.acquire(p.ID, "sprint"); err != nil {
		return "", err
	}
	defer a.release(p.ID)

	if sprintID < 0 {
		return "", a.repo.DiscardDraftSprint(a.ctx, p.ID, sprintID)
	}
	b, err := a.backendFor(p)
	if err != nil {
		return "", err
	}
	log.Printf("tam: deleting sprint %d on board %d for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	line, err := a.sprintService(p, b).Delete(a.ctx, p.ID, boardID, sprintID)
	if err != nil {
		log.Printf("tam: delete sprint %d for %s failed: %v", sprintID, p.Name, err)
		return "", ceremonyError(err)
	}
	if line != "" {
		log.Printf("tam: sprint %d for %s deleted, with a note: %s", sprintID, p.Name, line)
	}
	return line, nil
}
```

In the file's header comment, replace `// These three bindings are the exercisable surface over internal/sprints' Create, Edit, and Delete,` with `// These three bindings are the surface over a sprint's creation, which is journaled, and internal/sprints' Edit and Delete,` and replace the paragraph starting `// Nothing here is journaled,` (three lines) with:

```go
// A new sprint is journaled as a draft under a negative id, and an edit or a
// delete of such a draft stays local; only a sprint Jira already holds is
// edited or deleted in Jira at once.
```

Add `"fmt"`, `"agile-suite/tam/internal/issuerepo"`, and `"agile-suite/tam/internal/sprints"` to its imports.

- [ ] **Step 5: Rituals ignore drafts**

In `tam/internal/ritualsync/ensure.go`, in `Sprints`, add as the first statement of the loop body:

```go
		// A draft sprint is not in Jira and may never be, so it gets no
		// pages until Commit creates it.
		if s.Draft {
			continue
		}
```

In `tam/app_rituals.go`, in `ritualSprint`, replace the `if s.ID == sprintID { ... }` block with:

```go
		if s.ID == sprintID {
			if s.Draft {
				return ritualsync.Sprint{}, fmt.Errorf("%s is a draft sprint; Commit creates it in Jira before it gets ritual pages", s.Name)
			}
			return ritualsync.Sprint{Info: ritualsync.Info(s, a.boardName(profileID, boardID)), State: s.State}, nil
		}
```

- [ ] **Step 6: Regenerate the bindings and run the gate**

Run: `cd tam && wails generate module`
Expected: `models.ts` gains `draft: boolean;` on `boardrepo.Sprint` and `boardrepo.SprintChoice`. Check with `git diff --stat tam/frontend/wailsjs`.

Run: `cd tam && go test ./...`
Expected: PASS, including `TestManagementBindingsRequireAProfile` and `TestManagementBindingsAreRefusedWhileABoardsRefreshHoldsTheLock` unchanged.

- [ ] **Step 7: Commit**

```bash
git add tam/internal/sprints tam/internal/importer/draftsprint_test.go tam/internal/ritualsync/ensure.go tam/internal/ritualsync/drafts_test.go tam/app_sprintmanage.go tam/app_sprintmanage_test.go tam/app_rituals.go tam/app_rituals_test.go tam/frontend/wailsjs
git commit -m "feat(tam): creating a sprint drafts it, and the fence drops to four immediate writes"
```

---

### Task 8: Draft sprints on screen

**Files:**
- Modify: `tam/frontend/src/api.ts:235-288` (`Sprint.draft`, `SprintChoice.draft`, `SprintOption.draft`)
- Modify: `tam/frontend/src/lib/sprintOptions.ts` (`DRAFT_SPRINT_HINT`, `sprintOptionLabel`)
- Modify: `tam/frontend/src/components/SprintRow.tsx:75-111`, `BoardsToolbar.tsx:36-39, 116-118`, `CreateSprintModal.tsx`, `EditSprintModal.tsx:~140-150`, `SprintsView.tsx:175 (askDelete), 438`, `BoardCeremonies.tsx:128`, `RitualsView.tsx:78-83`
- Modify: `tam/frontend/src/queries/sprints.ts:45-58` (`invalidateSprintWrites`)
- Create: `tam/frontend/src/components/BoardsToolbar.test.tsx`
- Test: `SprintRow.test.tsx`, `SprintField.test.tsx`, `EditSprintModal.test.tsx`, `SprintsView.test.tsx`, `BoardsView.test.tsx:1564`

**Interfaces:**
- Consumes: `draft` on `Sprint`, `SprintChoice`, `SprintDetail` from Go (Tasks 5 and 7); `CreateSprint` answering a negative id; the core `Menu`, which renders `MenuItem.title` on the `role="menuitem"` button.
- Produces:
  - `export const DRAFT_SPRINT_HINT = "Commit this sprint first"`
  - `sprintOptionLabel` appends ` (draft)` for a draft.
  - `export function sprintOption(s: Sprint): string` from `BoardsToolbar.tsx`, `Sprint 15 (Draft)` for a draft.
  - Start and Complete disabled with `title={DRAFT_SPRINT_HINT}` for a draft in the Sprints view's row menu; Start disabled with the same title in the Boards toolbar.

- [ ] **Step 1: Write the failing tests**

In `tam/frontend/src/components/SprintRow.test.tsx`, add inside `describe("SprintRow", ...)`:

```tsx
  it("marks a draft sprint and holds back Start and Complete until Commit", async () => {
    const user = userEvent.setup();
    renderRow({ id: -1, name: "Sprint 15", state: "future", draft: true, total: 0, done: 0, points: 0, donePoints: 0 });
    expect(screen.getByRole("treeitem", { name: "Sprint 15, Draft" })).toBeInTheDocument();
    expect(screen.getByText("Draft")).toHaveClass("chip-draft");
    await user.click(screen.getByRole("button", { name: "Actions on Sprint 15" }));
    const menu = await screen.findByRole("menu");
    for (const name of ["Start sprint…", "Complete sprint…"]) {
      const item = within(menu).getByRole("menuitem", { name });
      expect(item).toBeDisabled();
      expect(item).toHaveAttribute("title", "Commit this sprint first");
    }
    expect(within(menu).getByRole("menuitem", { name: "Edit sprint…" })).toBeEnabled();
    expect(within(menu).getByRole("menuitem", { name: "Delete sprint…" })).toBeEnabled();
  });
```

In `tam/frontend/src/components/SprintField.test.tsx`, add inside `describe("SprintField", ...)`:

```tsx
  it("says which sprint is still a draft", () => {
    renderField([
      { id: -1, name: "Sprint 15", boardName: "PLAT Scrum", draft: true },
      { id: 12, name: "Sprint 12" },
    ]);
    expect(screen.getByRole("option", { name: "Sprint 15 (draft)" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Sprint 12" })).toBeInTheDocument();
  });
```

`tam/frontend/src/components/BoardsToolbar.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { Board, Sprint } from "../api";
import { BoardsToolbar, sprintOption } from "./BoardsToolbar";

const board: Board = { id: 1, name: "PLAT Scrum", type: "scrum" };
const draft: Sprint = { id: -1, boardId: 1, name: "Sprint 15", state: "future", startDate: "", endDate: "", goal: "", draft: true };

function renderToolbar(sprint: Sprint) {
  render(
    <BoardsToolbar
      boards={[board]} board={board} onBoard={vi.fn()}
      sprints={[sprint]} sprint={sprint} onSprint={vi.fn()}
      swimlane="none" onSwimlane={vi.fn()}
      refreshing={false} canRefresh onRefresh={vi.fn()}
      filter="" onFilter={vi.fn()}
      onStart={vi.fn()} onComplete={vi.fn()} onCreate={vi.fn()}
    />,
  );
}

describe("BoardsToolbar", () => {
  it("labels a draft sprint in the picker and holds Start back", () => {
    renderToolbar(draft);
    expect(sprintOption(draft)).toBe("Sprint 15 (Draft)");
    const start = screen.getByRole("button", { name: "Start sprint" });
    expect(start).toBeDisabled();
    expect(start).toHaveAttribute("title", "Commit this sprint first");
  });

  it("starts a future sprint Jira holds", () => {
    renderToolbar({ ...draft, id: 14, name: "Sprint 14", draft: false });
    expect(screen.getByRole("button", { name: "Start sprint" })).toBeEnabled();
    expect(sprintOption({ ...draft, id: 14, name: "Sprint 14", draft: false })).toBe("Sprint 14 (Future)");
  });
});
```

In `tam/frontend/src/components/SprintsView.test.tsx`:

1. In `"creates a sprint, announces it, and points at the row it landed on"`, replace `"Sprint 14 was created, 2026-09-14 to 2026-09-28."` with `"Sprint 14 was drafted, 2026-09-14 to 2026-09-28. Commit creates it in Jira."` and `"Sprint 14 was created"` with `"Sprint 14 was drafted"`.
2. Replace the test `"marks the create dialog as sending to Jira now, and still names Commit under it"` with:

```tsx
  it("says the new sprint is drafted locally and created on Commit", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "New sprint" }));
    const dialog = await screen.findByRole("dialog", { name: "New sprint" });
    expect(within(dialog).getByText("Drafted locally. Commit creates it in Jira.")).toBeInTheDocument();
    expect(within(dialog).queryByText("Sends to Jira now")).not.toBeInTheDocument();
  });
```

3. Add:

```tsx
  it("deletes a draft sprint without claiming Jira deletes anything", async () => {
    const user = userEvent.setup();
    const DRAFT = detail({ id: -1, name: "Sprint 15", state: "future", draft: true, goal: "", startDate: "", endDate: "", issues: [] });
    vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([ACTIVE, DRAFT, FUTURE, CLOSED, BACKLOG]);
    vi.mocked(api.DeleteSprint).mockResolvedValue("");
    renderView();
    const menu = await openMenu(user, "Sprint 15");
    await user.click(within(menu).getByRole("menuitem", { name: "Delete sprint…" }));
    const ask = await screen.findByRole("alertdialog");
    expect(within(ask).getByText("Delete the draft Sprint 15?")).toBeInTheDocument();
    expect(within(ask).getByText("Nothing reaches Jira. Cards moved into it go back to where they were.")).toBeInTheDocument();
    expect(within(ask).queryByText("Sends to Jira now")).not.toBeInTheDocument();
    await user.click(within(ask).getByRole("button", { name: "Delete draft" }));
    await waitFor(() => expect(api.DeleteSprint).toHaveBeenCalledWith("p1", 1, -1));
    await waitFor(() => expect(banner()).toHaveTextContent("The draft Sprint 15 was deleted."));
  });
```

In `tam/frontend/src/components/BoardsView.test.tsx:1564`, replace `"Sprint 14 was created, 2026-09-14 to 2026-09-28."` with `"Sprint 14 was drafted, 2026-09-14 to 2026-09-28. Commit creates it in Jira."`.

In `tam/frontend/src/components/EditSprintModal.test.tsx`, add inside `describe("EditSprintModal", ...)`:

```tsx
  it("edits a draft sprint locally and says so instead of the Jira chip", () => {
    renderModal({ id: -1, name: "Sprint 15", draft: true, goal: "" });
    expect(screen.getByText("A draft sprint. Changes stay local until Commit.")).toBeInTheDocument();
    expect(screen.queryByText("Sends to Jira now")).not.toBeInTheDocument();
  });
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam/frontend && npx vitest run src/components/SprintRow.test.tsx src/components/SprintField.test.tsx src/components/BoardsToolbar.test.tsx src/components/SprintsView.test.tsx src/components/BoardsView.test.tsx src/components/EditSprintModal.test.tsx`
Expected: FAIL on every new or changed assertion (and a type error on `draft` in the fixtures until Step 3).

- [ ] **Step 3: Types and labels**

In `tam/frontend/src/api.ts`, add to `Sprint` after `goal`:

```ts
  // A sprint drafted in TAM and not yet created in Jira: its id is negative,
  // its state is future, and Commit creates it before any card moved into it
  // is sent. Absent on fixtures written before drafts existed.
  draft?: boolean;
```

Add to both `SprintChoice` declarations and both `SprintOption` declarations:

```ts
  // A sprint drafted in TAM, named so beside its name.
  draft?: boolean;
```

In `tam/frontend/src/lib/sprintOptions.ts`, add after the import:

```ts
// DRAFT_SPRINT_HINT is the tooltip on every action a draft sprint cannot
// take yet: Start and Complete need a sprint Jira holds.
export const DRAFT_SPRINT_HINT = "Commit this sprint first";
```

and replace `sprintOptionLabel` and its comment with:

```ts
// sprintOptionLabel is the text an option shows: the sprint's own name,
// with its board name beside it only when another sprint in the same list
// answers to the same name, and "(draft)" after a sprint Commit has not
// created yet.
export function sprintOptionLabel(s: SprintOption, dupIds: Set<number>): string {
  const base = dupIds.has(s.id) && s.boardName ? `${s.name} (${s.boardName})` : s.name;
  return s.draft ? `${base} (draft)` : base;
}
```

- [ ] **Step 4: The row, the toolbar, the dialogs, and the lists**

In `tam/frontend/src/components/SprintRow.tsx`:

1. Add `import { DRAFT_SPRINT_HINT } from "../lib/sprintOptions";`.
2. Replace the `const items: MenuItem[] = [];` block through the `delete` push with:

```tsx
  const items: MenuItem[] = [];
  if (detail.draft) {
    // A draft is not in Jira, so neither ceremony can act on it; both are
    // shown and held back, so the menu says what Commit unlocks.
    items.push({ key: "start", label: "Start sprint…", disabled: true, title: DRAFT_SPRINT_HINT });
    items.push({ key: "complete", label: "Complete sprint…", disabled: true, title: DRAFT_SPRINT_HINT });
  } else if (detail.state === "future") {
    items.push({ key: "start", label: "Start sprint…", disabled: busy, onClick: actions.onStart });
  } else if (detail.state === "active") {
    items.push({ key: "complete", label: "Complete sprint…", disabled: busy, onClick: actions.onComplete });
  }
  if (items.length > 0) items.push({ key: "manage", divider: true });
  items.push({ key: "edit", label: "Edit sprint…", disabled: busy, onClick: actions.onEdit });
  items.push({ key: "delete", label: "Delete sprint…", danger: true, disabled: busy, onClick: actions.onDelete });
  // A draft reads "Draft" in the chip and the tree's own label, painted in
  // the amber every other thing waiting for Commit wears.
  const label = detail.draft ? "Draft" : stateLabel(detail.state);
```

3. Change the treeitem's `aria-label` to `aria-label={isSprint ? `${detail.name}, ${label}` : detail.name}` and the state cell's chip to:

```tsx
        {isSprint && (
          <span className={detail.draft ? "chip chip-draft" : `chip chip-status chip-status-${stateClass(detail.state)}`}>
            {label}
          </span>
        )}
```

In `tam/frontend/src/components/BoardsToolbar.tsx`:

1. Add `import { DRAFT_SPRINT_HINT } from "../lib/sprintOptions";`.
2. Replace `sprintOption` and its comment with:

```tsx
// sprintOption labels a sprint "Name (State)", or "Name (Draft)" for one
// Commit has not created yet. Jira sends the state lowercase, so it is
// capitalised here for display only.
export function sprintOption(s: Sprint): string {
  if (s.draft) return `${s.name} (Draft)`;
  return `${s.name} (${s.state.charAt(0).toUpperCase()}${s.state.slice(1)})`;
}
```

3. Replace the Start button with:

```tsx
        {canStart && (
          <button
            type="button"
            className="btn"
            onClick={onStart}
            disabled={!!sprint?.draft}
            title={sprint?.draft ? DRAFT_SPRINT_HINT : undefined}
          >
            Start sprint
          </button>
        )}
```

In `tam/frontend/src/components/CreateSprintModal.tsx`:

1. Remove the `ImmediateWriteChip` import.
2. Replace the header's `<span className="immediate-write">...</span>` element (the chip and the "This does not wait for Commit." sentence) with:

```tsx
        <p className="muted small">Drafted locally. Commit creates it in Jira.</p>
```

3. In `onSuccess`, replace the `made` line with:

```tsx
          const made = `${created.sprint.name || values.name} was drafted, ${values.from} to ${values.to}. Commit creates it in Jira.`;
```

4. Replace the component's doc comment with:

```tsx
// CreateSprintModal drafts a new sprint. Like a new issue it waits for
// Commit: the sprint is journaled under a negative id, every picker offers
// it at once, and Commit creates it in Jira before any card moved into it is
// sent. It still takes the app's lock quietly, through runQuietLock, because
// the Go binding serialises a draft against a Commit rewriting the same rows.
```

In `tam/frontend/src/components/EditSprintModal.tsx`, replace the header block

```tsx
        <div className="edit-sprint-title">
          <h2 id="edit-sprint-title">{`Edit ${sprint.name}`}</h2>
          <p>Changes save to Jira immediately.</p>
        </div>
        <ImmediateWriteChip />
```

with:

```tsx
        <div className="edit-sprint-title">
          <h2 id="edit-sprint-title">{`Edit ${sprint.name}`}</h2>
          <p>{sprint.draft ? "A draft sprint. Changes stay local until Commit." : "Changes save to Jira immediately."}</p>
        </div>
        {!sprint.draft && <ImmediateWriteChip />}
```

In `tam/frontend/src/components/SprintsView.tsx`, add as the first statement of `askDelete`:

```tsx
    if (detail.draft) {
      // A draft is not in Jira: deleting it is a discard, and the only thing
      // to warn about is the cards moved into it, which go back.
      const ok = await confirm({
        title: `Delete the draft ${detail.name}?`,
        message: <p>Nothing reaches Jira. Cards moved into it go back to where they were.</p>,
        confirmLabel: "Delete draft",
        cancelLabel: "Keep it",
        danger: true,
      });
      if (!ok) return;
      setBusyRowId(rowIdOf(detail));
      del.mutate(
        { boardId: board?.id ?? 0, sprintId: detail.id },
        {
          onSuccess: () => {
            const sentence = `The draft ${detail.name} was deleted.`;
            announce(sentence);
            afterWrite("", sentence);
          },
          onError: (e) => void notice({ title: "The draft sprint was not deleted", message: errMsg(e), tone: "error" }),
          onSettled: () => setBusyRowId(""),
        },
      );
      return;
    }
```

In the same file change `futures={sprints.filter((s) => s.state === "future")}` to `futures={sprints.filter((s) => s.state === "future" && !s.draft)}`, and make the same change at `tam/frontend/src/components/BoardCeremonies.tsx:128`: a completion cannot move cards into a draft.

In `tam/frontend/src/components/RitualsView.tsx`, replace the `ListBoardSprints(activeId, boardId).then((list) => { ... })` callback body with:

```tsx
      if (!live) return;
      // A draft sprint gets no ritual pages until Commit creates it.
      const held = list.filter((s) => !s.draft);
      setSprints(held);
      setSprintId(held.find((s) => s.state === "active")?.id ?? held[0]?.id ?? 0);
```

In `tam/frontend/src/queries/sprints.ts`, add `keys.pending(profileId),` to the list in `invalidateSprintWrites`, and add to its comment: `// The pending list is here because creating, editing or deleting a draft sprint is a journal write.`

- [ ] **Step 5: Run the tests and the type check**

Run: `cd tam/frontend && npx vitest run`
Expected: PASS.

Run: `npm run typecheck --workspaces --if-present`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add tam/frontend/src
git commit -m "feat(tam): show draft sprints wherever a sprint is picked, and hold their ceremonies"
```

---

## Part C: the phased Commit

### Task 9: Rewriting a placeholder once Jira has answered

**Files:**
- Create: `tam/internal/issuerepo/rekeysprint.go`, `tam/internal/issuerepo/rekeysprint_test.go`
- Modify: `tam/internal/issuerepo/rekey.go:46-79` (`Rekey` calls `rewriteParentKey`)

**Interfaces:**
- Consumes: `rewriteSprintID`, `draftsWhere`, `EntitySprintCreate`, `ErrDraftSprintGone` (Task 6); `editDraft`, `EntitySprint`, `journal.Audit`, `(*Repository).inTx` (existing).
- Produces:
  - `func rewriteParentKey(ctx context.Context, tx *sql.Tx, profileID, from, to string) error`: every other draft whose JSON `parentKey` is `from` now names `to`. `Rekey` calls it, so a phase-2 epic create repoints its stories before phase 3 reads them.
  - `func (r *Repository) RekeySprint(ctx context.Context, profileID string, draftID int, made backend.Sprint) error`: in one transaction, turns the draft row real (`id = made.ID`, `draft = 0`, Jira's name, state, dates, goal), calls `rewriteSprintID(from, to, made.Name)`, deletes the `sprint_create` row, audits the commit under the draft key and the creation under the real id. Answers `ErrDraftSprintGone` when the create row is gone.
  - `func (r *Repository) MarkSprintCreatedWithoutRekey(ctx context.Context, profileID string, draftID, realID int) error`: Jira has the sprint and the local rename failed; deletes the create row and the draft row so a retry does not create a second sprint, and audits why.

- [ ] **Step 1: Write the failing tests**

`tam/internal/issuerepo/rekeysprint_test.go`:

```go
package issuerepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// A story drafted under a draft epic, and a sub-task under the story. The
// epic's create rekeys it, and the story's JSON has to name the real key
// before the next phase reads it, or Jira gets a TAM-NEW key as an Epic Link.
func TestRekeyRepointsThePlaceholderParentOfEveryDraftUnderIt(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	epic, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeEpic, Summary: "Promotions"})
	if err != nil {
		t.Fatal(err)
	}
	story, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Promo input", ParentKey: epic})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeSubtask, Summary: "Wire it", ParentKey: story})
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Rekey(ctx, "p1", epic, "PLAT-900"); err != nil {
		t.Fatal(err)
	}

	decode := func(key string) backend.IssueDraft {
		row, ok := rowOf(t, repo, key, issuerepo.EntityIssueCreate)
		if !ok {
			t.Fatalf("no create row for %s", key)
		}
		var d backend.IssueDraft
		if err := json.Unmarshal([]byte(row.AfterVal), &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	if d := decode(story); d.ParentKey != "PLAT-900" {
		t.Errorf("the story's draft names %q, want the epic's real key", d.ParentKey)
	}
	if d := decode(sub); d.ParentKey != story {
		t.Errorf("the sub-task's parent is the story, still a draft: %q", d.ParentKey)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", story); iss.ParentKey != "PLAT-900" {
		t.Errorf("the story's row names the real epic: %+v", iss)
	}
}

func TestRekeySprintMakesADraftSprintRealEverywhereItIsNamed(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	s, err := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(s.ID)
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", id, "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	temp, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "In the sprint", SprintID: id, SprintName: "Sprint 15"})
	if err != nil {
		t.Fatal(err)
	}

	made := backend.Sprint{ID: 88, BoardID: 1, Name: "Sprint 15", State: "future", StartDate: s.StartDate, EndDate: s.EndDate, Goal: s.Goal}
	if err := repo.RekeySprint(ctx, "p1", s.ID, made); err != nil {
		t.Fatal(err)
	}

	var draft int
	if err := db.QueryRow(`SELECT draft FROM sprint WHERE profile_id = 'p1' AND id = 88`).Scan(&draft); err != nil || draft != 0 {
		t.Errorf("the sprint row is real: draft %d, %v", draft, err)
	}
	var left int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND id = ?`, s.ID).Scan(&left); err != nil || left != 0 {
		t.Errorf("no row keeps the draft id: %d, %v", left, err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-409"); iss.SprintID != "88" || iss.SprintName != "Sprint 15" {
		t.Errorf("the moved card names the real sprint: %+v", iss)
	}
	move, ok := rowOf(t, repo, "PLAT-409", issuerepo.EntitySprintMove)
	if !ok || move.AfterVal != "88|Sprint 15" || move.BeforeVal != "12|Sprint 12" {
		t.Errorf("the journaled move targets the real sprint: %+v", move)
	}
	create, _ := rowOf(t, repo, temp, issuerepo.EntityIssueCreate)
	if !strings.Contains(create.AfterVal, `"sprintId":"88"`) {
		t.Errorf("the draft issue's JSON names the real sprint: %s", create.AfterVal)
	}
	if _, ok := rowOf(t, repo, id, issuerepo.EntitySprintCreate); ok {
		t.Error("the sprint_create row is gone")
	}
	act, _ := repo.ListActivity(ctx, "p1", "88", 0)
	if len(act) == 0 || act[0].EntityType != issuerepo.EntitySprint || act[0].Action != "create" {
		t.Errorf("the real sprint's creation is audited: %+v", act)
	}
	if err := repo.RekeySprint(ctx, "p1", s.ID, made); !errors.Is(err, issuerepo.ErrDraftSprintGone) {
		t.Errorf("a second rekey = %v", err)
	}
}

func TestMarkSprintCreatedWithoutRekeyLeavesNothingToCreateTwice(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	s, err := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkSprintCreatedWithoutRekey(ctx, "p1", s.ID, 88); err != nil {
		t.Fatal(err)
	}
	if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
		t.Errorf("nothing left for a retry to create: %+v", rows)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND id = ?`, s.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("the draft row is gone: %d, %v", n, err)
	}
	act, _ := repo.ListActivity(ctx, "p1", strconv.Itoa(s.ID), 0)
	if len(act) == 0 || !strings.Contains(act[0].Note, "88") {
		t.Errorf("the trail says where the sprint went: %+v", act)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam && go test ./internal/issuerepo/ -run 'RepointsThePlaceholderParent|RekeySprint|MarkSprintCreated'`
Expected: FAIL: `undefined: issuerepo.RekeySprint`, and the story's draft still names the `TAM-NEW` key.

- [ ] **Step 3: Write `rekeysprint.go` and call `rewriteParentKey` from `Rekey`**

`tam/internal/issuerepo/rekeysprint.go`:

```go
package issuerepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// rewriteParentKey repoints every other draft whose JSON names from as its
// parent. Rekey already repoints the cached rows and every pending parentKey
// edit; a draft's own parent lives in its create JSON, which no UPDATE over
// a column can reach, and the next phase of the same Commit posts that JSON.
func rewriteParentKey(ctx context.Context, tx *sql.Tx, profileID, from, to string) error {
	drafts, err := draftsWhere(ctx, tx, profileID, func(d backend.IssueDraft) bool { return d.ParentKey == from })
	if err != nil {
		return err
	}
	for _, key := range drafts {
		if err := editDraft(ctx, tx, profileID, key, func(d *backend.IssueDraft) { d.ParentKey = to }); err != nil {
			return err
		}
	}
	return nil
}

// RekeySprint turns a draft sprint into the sprint Jira just created, in one
// transaction: the draft row takes Jira's id and values and stops being a
// draft, every card, journaled move and draft issue naming the negative id
// names the real one, and the sprint_create row goes. The board pass of the
// same Commit then pushes the moves into it.
//
// A row Jira's id already has on the board, which a boards refresh between
// the create and this call would leave, is replaced rather than collided
// with: the primary key is (profile, board, id).
func (r *Repository) RekeySprint(ctx context.Context, profileID string, draftID int, made backend.Sprint) error {
	from, to := strconv.Itoa(draftID), strconv.Itoa(made.ID)
	state := made.State
	if state == "" {
		state = "future"
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		var raw string
		err := tx.QueryRowContext(ctx,
			`SELECT after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			profileID, EntitySprintCreate, from).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDraftSprintGone
		}
		if err != nil {
			return fmt.Errorf("read draft sprint %s: %w", from, err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM sprint WHERE profile_id = ? AND id = ? AND draft = 0`, profileID, made.ID); err != nil {
			return fmt.Errorf("clear sprint %s before the rekey: %w", to, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE sprint SET id = ?, draft = 0, name = ?, state = ?, start_date = ?, end_date = ?, goal = ?
			 WHERE profile_id = ? AND id = ? AND draft = 1`,
			made.ID, made.Name, state, made.StartDate, made.EndDate, made.Goal, profileID, draftID); err != nil {
			return fmt.Errorf("rekey sprint %s to %s: %w", from, to, err)
		}
		if err := rewriteSprintID(ctx, tx, profileID, from, to, made.Name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			profileID, EntitySprintCreate, from); err != nil {
			return fmt.Errorf("clear draft sprint %s: %w", from, err)
		}
		if err := journal.Audit(tx, profileID, EntitySprintCreate, from, "commit", FieldCreate, "", raw, "created in Jira as sprint "+to); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntitySprint, to, "create", "", "", made.Name, "created in Jira from draft sprint "+from)
	})
}

// MarkSprintCreatedWithoutRekey clears a draft sprint Jira accepted when the
// local rename failed: the create row and the draft row go, so a retry does
// not create a second sprint, and the trail says which sprint Jira made. The
// cards moved into the draft keep naming its negative id; the Commit that
// hit this reports them held, and the user moves them again after a boards
// refresh brings the real sprint in.
func (r *Repository) MarkSprintCreatedWithoutRekey(ctx context.Context, profileID string, draftID, realID int) error {
	from := strconv.Itoa(draftID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			profileID, EntitySprintCreate, from); err != nil {
			return fmt.Errorf("clear draft sprint %s: %w", from, err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM sprint WHERE profile_id = ? AND id = ? AND draft = 1`, profileID, draftID); err != nil {
			return fmt.Errorf("drop draft sprint %s: %w", from, err)
		}
		note := fmt.Sprintf("created in Jira as sprint %d but the local rename failed; refresh the board to see it, and move its cards again", realID)
		return journal.Audit(tx, profileID, EntitySprintCreate, from, "created", "", from, strconv.Itoa(realID), note)
	})
}
```

In `tam/internal/issuerepo/rekey.go`, in `Rekey`, directly after the `for _, stmt := range []string{...}` loop, add:

```go
	// A draft's parent is in its create JSON, which none of the UPDATEs
	// above reach, and the next phase of the same Commit posts that JSON.
	if err := rewriteParentKey(ctx, tx, profileID, tempKey, realKey); err != nil {
		return err
	}
```

and add `and repoints every other draft whose own JSON names the temporary key as its parent,` to `Rekey`'s doc comment after `the parent of an edit and the neighbour of a rank alike,`.

- [ ] **Step 4: Run the tests to see them pass**

Run: `cd tam && go test ./internal/issuerepo/ ./internal/committer/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/issuerepo/rekeysprint.go tam/internal/issuerepo/rekeysprint_test.go tam/internal/issuerepo/rekey.go
git commit -m "feat(tam): rewrite a draft sprint id and a draft parent once Jira has answered"
```

---

### Task 10: Commit in phases

**Files:**
- Create: `tam/internal/committer/phases.go`, `tam/internal/committer/held.go`, `tam/internal/committer/phases_test.go`
- Modify: `tam/internal/committer/committer.go:1-7` (package doc), `:80-89` (`Result`), `:106-158` (`Commit`), `:185-237` (`commitCreate`), `:324-353` (delete `regroupEdits`)
- Modify: `tam/internal/committer/boards.go:105-157` (`commitBoardMoves`, `boardRows` take `deps`)
- Modify: `tam/internal/committer/committer_test.go:16-47` (fake fields), `:107-116` (`CreateIssue`)

**Interfaces:**
- Consumes: `issuerepo.EntitySprintCreate`, `DraftSprint`, `RekeySprint`, `MarkSprintCreatedWithoutRekey`, `ErrDraftSprintGone` (Tasks 6 and 9); `Rekey` repointing draft parents (Task 9); `backend.BoardBackend.CreateSprint`.
- Produces:
  - `type CreatedSprint struct { DraftID int; ID int; Name string }` (JSON `draftId`, `id`, `name`) and `Result.CreatedSprints []CreatedSprint`.
  - `type Held struct { Key, EntityType string; RowID int64; WaitsFor, Reason string }` (JSON `key`, `entityType`, `rowId`, `waitsFor`, `reason`) and `Result.Held []Held`.
  - `type phase struct { name string; run func(ctx context.Context, r *commitRun) }` and `func phases() []phase` in the order sprints, epics, issues, sub-tasks, edits, board moves, links.
  - `type commitRun struct { e *Engine; profileID, projectKey string; res *Result; deps *dependencies; rows []journal.PendingChange }` with `reload(ctx) error`, `createSprints(ctx)`, `createDrafts(ctx, level draftLevel)`, `pushEdits(ctx)`.
  - `func draftOrdinal(key string) int`
  - `type dependencies` with `block(placeholder, label, why string)`, `blockedBy(values ...string) (string, bool)`, `reason(placeholder string) string`, `hold(res *Result, key, entityType string, rowID int64, waits string)`.
  - `func (e *Engine) commitBoardMoves(ctx context.Context, profileID string, res *Result, deps *dependencies)`

- [ ] **Step 1: Give the fake Jira a sprint create and per-draft refusals**

In `tam/internal/committer/committer_test.go`, add to the `fake` struct:

```go
	// createErrFor refuses the create of a draft by summary, so one draft
	// of a plan can fail while the rest go through.
	createErrFor map[string]error
	// sprintsMade records "board name" for every sprint created, and
	// sprintCreateErr refuses them all. Sprint ids count up from 100.
	sprintsMade     []string
	sprintCreateErr error
	nextSprint      int
```

In `newFake`, add `createErrFor: map[string]error{},` to the literal. In `(*fake).CreateIssue`, directly after the `if f.createErr != nil { ... }` block, add:

```go
	if err := f.createErrFor[d.Summary]; err != nil {
		return "", err
	}
```

- [ ] **Step 2: Write the failing tests**

`tam/internal/committer/phases_test.go`:

```go
package committer_test

import (
	"context"
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
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `cd tam && go test ./internal/committer/ -run 'NumericOrder|OnePress|ExactlyItsDescendants|SprintJiraRefuses'`
Expected: FAIL: `res.CreatedSprints undefined`, `res.Held undefined` (compile), then `TAM-NEW-10` before `TAM-NEW-2`.

- [ ] **Step 4: Write `held.go`**

`tam/internal/committer/held.go`:

```go
package committer

import (
	"fmt"

	"agile-suite/tam/internal/issuerepo"
)

// Held is a pending row this Commit did not send because something it names
// was not created in Jira: a draft issue or a draft sprint whose create was
// refused, or which is itself waiting for one. It stays in the journal and
// the next Commit tries again. It is not a failure: nothing was sent, so
// there is nothing Jira refused about this row itself, and Reason says what
// it waits for.
type Held struct {
	Key        string `json:"key"`
	EntityType string `json:"entityType"`
	RowID      int64  `json:"rowId"`
	WaitsFor   string `json:"waitsFor"`
	Reason     string `json:"reason"`
}

// dependencies is what one Commit knows about placeholders it could not make
// real: a TAM-NEW key or a draft sprint id, with the label a sentence names
// it by and why it is still a placeholder. Every phase asks it before it
// sends a row naming one.
type dependencies struct {
	blocked map[string]blocker
}

type blocker struct {
	label string
	why   string
}

func newDependencies() *dependencies {
	return &dependencies{blocked: map[string]blocker{}}
}

// block records a placeholder this Commit will not make real.
func (d *dependencies) block(placeholder, label, why string) {
	d.blocked[placeholder] = blocker{label: label, why: why}
}

// blockedBy answers the first of values that names a blocked placeholder:
// an issue key as it stands, or a sprint id either bare or as the id half of
// a journaled move value.
func (d *dependencies) blockedBy(values ...string) (string, bool) {
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := d.blocked[v]; ok {
			return v, true
		}
		if id := issuerepo.MoveID(v); id != v {
			if _, ok := d.blocked[id]; ok {
				return id, true
			}
		}
	}
	return "", false
}

// reason is the sentence a held row carries: "waits for TAM-NEW-2, which
// Jira refused".
func (d *dependencies) reason(placeholder string) string {
	b := d.blocked[placeholder]
	return fmt.Sprintf("waits for %s, %s", b.label, b.why)
}

// hold reports a row held back for waits, and blocks the row's own key when
// that key is a draft not already blocked, so whatever waits for it is held
// in turn. A link drafted from a refused epic is a row under the epic's own
// key; it must not overwrite why the epic itself is blocked.
func (d *dependencies) hold(res *Result, key, entityType string, rowID int64, waits string) {
	res.Held = append(res.Held, Held{Key: key, EntityType: entityType, RowID: rowID, WaitsFor: waits, Reason: d.reason(waits)})
	if _, already := d.blocked[key]; isDraftKey(key) && !already {
		d.block(key, key, "which is waiting for "+d.blocked[waits].label)
	}
}
```

- [ ] **Step 5: Write `phases.go`**

`tam/internal/committer/phases.go`:

```go
package committer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	corejira "agile-suite/core/jira"
	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// A Commit runs in phases, in dependency order, the way XTM's commit creates
// preconditions before containers before tests. Each phase reads the journal
// the one before it left, because a create rewrites every row that named its
// placeholder: a sprint's id, an epic's key under its stories, a story's key
// under its sub-tasks. A later phase then sends real ids only, and whatever
// still names a placeholder that could not be made real is held, not sent.

// phase is one step of a Commit. A later bundle adds a step by adding one
// entry to phases(); a step reads r.rows, reports into r.res, and holds what
// waits on a placeholder through r.deps.
type phase struct {
	name string
	run  func(ctx context.Context, r *commitRun)
}

// phases is the order a Commit runs in:
//
//  1. sprints, so a card moved into a draft sprint names a real one;
//  2. epics, so a story's Epic Link names a real key;
//  3. every other creatable type, so a sub-task's parent does;
//  4. sub-tasks;
//  5. edits, then the board moves (sprint moves, transitions, ranks), then
//     links, exactly as before phases existed.
func phases() []phase {
	return []phase{
		{name: "sprints", run: func(ctx context.Context, r *commitRun) { r.createSprints(ctx) }},
		{name: "epics", run: func(ctx context.Context, r *commitRun) { r.createDrafts(ctx, levelEpic) }},
		{name: "issues", run: func(ctx context.Context, r *commitRun) { r.createDrafts(ctx, levelIssue) }},
		{name: "sub-tasks", run: func(ctx context.Context, r *commitRun) { r.createDrafts(ctx, levelSubtask) }},
		{name: "edits", run: func(ctx context.Context, r *commitRun) { r.pushEdits(ctx) }},
		{name: "board moves", run: func(ctx context.Context, r *commitRun) { r.e.commitBoardMoves(ctx, r.profileID, r.res, r.deps) }},
		{name: "links", run: func(ctx context.Context, r *commitRun) { r.e.commitLinks(ctx, r.profileID, r.res) }},
	}
}

// commitRun is one Commit's state across its phases.
type commitRun struct {
	e          *Engine
	profileID  string
	projectKey string
	res        *Result
	deps       *dependencies
	// rows is the journal as the last phase left it, oldest first.
	rows []journal.PendingChange
}

// reload reads the journal again, oldest first.
func (r *commitRun) reload(ctx context.Context) error {
	all, err := r.e.repo.ListPendingChanges(ctx, r.profileID)
	if err != nil {
		return err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	r.rows = all
	return nil
}

// draftLevel is which create phase a draft belongs to.
type draftLevel int

const (
	levelEpic draftLevel = iota
	levelIssue
	levelSubtask
)

func levelOf(logicalType string) draftLevel {
	switch logicalType {
	case backend.TypeEpic:
		return levelEpic
	case backend.TypeSubtask:
		return levelSubtask
	}
	return levelIssue
}

// draftOrdinal is the n of TAM-NEW-n, which is the order drafts of one level
// are created in: the order they were drafted. A key that is not a draft key
// sorts last.
func draftOrdinal(key string) int {
	if !strings.HasPrefix(key, issuerepo.DraftPrefix) {
		return math.MaxInt
	}
	n, err := strconv.Atoi(strings.TrimPrefix(key, issuerepo.DraftPrefix))
	if err != nil {
		return math.MaxInt
	}
	return n
}

// sprintCreator is the one Agile write the sprints phase makes. Both shipped
// backends answer it; a backend that does not fails each draft sprint with a
// reason and holds what waits on it.
type sprintCreator interface {
	CreateSprint(ctx context.Context, boardID int, d backend.SprintDraft) (backend.Sprint, error)
}

// createSprints creates every draft sprint, oldest first, and rewrites its
// negative id everywhere it is named. A sprint created here stays created
// whatever the later phases do.
func (r *commitRun) createSprints(ctx context.Context) {
	w, canCreate := r.e.b.(sprintCreator)
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntitySprintCreate {
			continue
		}
		var d issuerepo.DraftSprint
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			r.res.Failures = append(r.res.Failures, sprintFailure(p, "draft sprint "+p.EntityKey, "the draft sprint could not be decoded: "+err.Error(), false))
			r.deps.block(p.EntityKey, "draft sprint "+p.EntityKey, "which could not be read")
			continue
		}
		label := fmt.Sprintf("sprint %q", d.Name)
		draftID, err := strconv.Atoi(p.EntityKey)
		if err != nil || draftID >= 0 {
			r.res.Failures = append(r.res.Failures, sprintFailure(p, d.Name, fmt.Sprintf("the draft sprint's id %q is not a draft id", p.EntityKey), false))
			r.deps.block(p.EntityKey, label, "which could not be sent")
			continue
		}
		if !canCreate {
			r.res.Failures = append(r.res.Failures, sprintFailure(p, d.Name, errNoBoardWrites.Error(), false))
			r.deps.block(p.EntityKey, label, "which this connection cannot create")
			continue
		}
		made, err := w.CreateSprint(ctx, d.BoardID, d.SprintDraft())
		if err != nil {
			r.res.Failures = append(r.res.Failures, sprintFailure(p, d.Name, err.Error(), !errors.Is(err, corejira.ErrNoAgile)))
			r.deps.block(p.EntityKey, label, "which Jira refused")
			continue
		}
		if made.ID <= 0 {
			r.forgetSprint(ctx, p, draftID, 0, d.Name, label, "Jira created the sprint but answered with no id; refresh the board to see it, then move its cards again")
			continue
		}
		fillSprint(&made, d)
		if err := r.e.repo.RekeySprint(ctx, r.profileID, draftID, made); err != nil {
			if errors.Is(err, issuerepo.ErrDraftSprintGone) {
				r.res.Failures = append(r.res.Failures, sprintFailure(p, d.Name, fmt.Sprintf("created in Jira as sprint %d, but the draft was discarded while Commit ran; refresh the board to see it", made.ID), false))
				continue
			}
			r.forgetSprint(ctx, p, draftID, made.ID, d.Name, label, fmt.Sprintf("created in Jira as sprint %d but the local rename failed: %v", made.ID, err))
			continue
		}
		r.res.CreatedSprints = append(r.res.CreatedSprints, CreatedSprint{DraftID: draftID, ID: made.ID, Name: made.Name})
	}
}

// fillSprint keeps the draft's own values where Jira's answer left a field
// empty, which a create that answers without a body does.
func fillSprint(made *backend.Sprint, d issuerepo.DraftSprint) {
	made.BoardID = d.BoardID
	if made.Name == "" {
		made.Name = d.Name
	}
	if made.StartDate == "" {
		made.StartDate = d.StartDate
	}
	if made.EndDate == "" {
		made.EndDate = d.EndDate
	}
	if made.Goal == "" {
		made.Goal = d.Goal
	}
}

// forgetSprint is a sprint Jira created that TAM could not rename locally:
// its draft is cleared so a retry does not create a second sprint, and what
// waited on it is held.
func (r *commitRun) forgetSprint(ctx context.Context, p journal.PendingChange, draftID, realID int, name, label, message string) {
	if err := r.e.repo.MarkSprintCreatedWithoutRekey(ctx, r.profileID, draftID, realID); err != nil {
		message += "; the draft could not be cleared either, so discard it before the next Commit or Jira gets a second sprint: " + err.Error()
	}
	r.res.Failures = append(r.res.Failures, sprintFailure(p, name, message, false))
	r.deps.block(p.EntityKey, label, "which Jira created but TAM could not rename; refresh the board, then move its cards again")
}

func sprintFailure(p journal.PendingChange, name, message string, retryable bool) Failure {
	return Failure{Key: name, EntityType: issuerepo.EntitySprintCreate, RowID: p.ID, Error: message, Retryable: retryable, Reachable: []string{}}
}

// createDrafts creates the drafts of one level in the order they were
// drafted. A draft whose parent or sprint is a placeholder this Commit could
// not make real is held; one whose parent is a placeholder nothing in this
// Commit knows is a failure, since no retry will create that parent.
func (r *commitRun) createDrafts(ctx context.Context, level draftLevel) {
	type pending struct {
		row   journal.PendingChange
		draft backend.IssueDraft
		bad   error
	}
	var todo []pending
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntityIssueCreate {
			continue
		}
		var d backend.IssueDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			// A draft that will not decode has no type to phase it by, so it
			// is reported once, with the ordinary issues.
			if level == levelIssue {
				todo = append(todo, pending{row: p, bad: err})
			}
			continue
		}
		if levelOf(d.Type) == level {
			todo = append(todo, pending{row: p, draft: d})
		}
	}
	sort.SliceStable(todo, func(i, j int) bool {
		return draftOrdinal(todo[i].row.EntityKey) < draftOrdinal(todo[j].row.EntityKey)
	})
	for _, t := range todo {
		key := t.row.EntityKey
		if t.bad != nil {
			// A draft that will not decode will not decode on the next Commit
			// either; the row has to be discarded, not retried.
			r.res.Failures = append(r.res.Failures, failure(key, issuerepo.EntityIssueCreate, "the draft could not be decoded: "+t.bad.Error(), false))
			r.deps.block(key, key, "which could not be read")
			continue
		}
		if waits, held := r.deps.blockedBy(t.draft.ParentKey, t.draft.SprintID); held {
			r.deps.hold(r.res, key, issuerepo.EntityIssueCreate, t.row.ID, waits)
			continue
		}
		if isDraftKey(t.draft.ParentKey) {
			r.res.Failures = append(r.res.Failures, failure(key, issuerepo.EntityIssueCreate, fmt.Sprintf("its parent %s is not a draft this Commit could create; set the parent again", t.draft.ParentKey), false))
			r.deps.block(key, key, "which could not be sent")
			continue
		}
		r.e.commitCreate(ctx, r, t.row, t.draft)
	}
}

// pushEdits pushes every edited issue's fields, keys in order.
func (r *commitRun) pushEdits(ctx context.Context) {
	byKey := map[string][]journal.PendingChange{}
	var keys []string
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntityIssue || isDraftKey(p.EntityKey) {
			continue
		}
		if _, seen := byKey[p.EntityKey]; !seen {
			keys = append(keys, p.EntityKey)
		}
		byKey[p.EntityKey] = append(byKey[p.EntityKey], p)
	}
	sort.Strings(keys)
	for _, key := range keys {
		r.e.commitEdit(ctx, r.profileID, key, byKey[key], r.res)
	}
}
```

- [ ] **Step 6: Rewire `committer.go` and the board pass**

In `tam/internal/committer/committer.go`:

1. Replace the package doc with:

```go
// Package committer pushes TAM's journal to Jira in phases (phases.go):
// draft sprints, then draft epics, then the other drafts, then sub-tasks,
// each followed by a re-read of the journal so the next phase sees the ids
// the last one rewrote; then each edited issue is version-checked, pushed,
// and refreshed, then the board moves in boards.go, then the links. A row
// naming a placeholder this Commit could not make real is held, not sent.
// An issue whose remote version moved is held back as a conflict carrying
// base, mine, and remote for every pending field; the two resolutions rebase
// the edits or drop them.
```

2. Add below `Created`:

```go
// CreatedSprint pairs a draft sprint's negative id with the id Jira gave it.
type CreatedSprint struct {
	DraftID int    `json:"draftId"`
	ID      int    `json:"id"`
	Name    string `json:"name"`
}
```

3. Replace `Result` with:

```go
// Result is what one Commit did. Held are the rows it did not send because
// something they name was not created; Remaining counts the journal rows
// left, held ones included.
type Result struct {
	Committed      []string        `json:"committed"`
	Created        []Created       `json:"created"`
	CreatedSprints []CreatedSprint `json:"createdSprints"`
	Linked         []Linked        `json:"linked"`
	Moved          []Moved         `json:"moved"`
	Conflicts      []Conflict      `json:"conflicts"`
	Failures       []Failure       `json:"failures"`
	Held           []Held          `json:"held"`
	Remaining      int             `json:"remaining"`
}
```

4. Replace `Commit` with:

```go
// Commit pushes every pending change of the profile, phase by phase. Only a
// store failure before the first phase returns an error; per-row outcomes
// land in the Result, and a journal that cannot be re-read between two
// phases stops the Commit with a failure saying so, keeping what the earlier
// phases did.
func (e *Engine) Commit(ctx context.Context, profileID, projectKey string) (Result, error) {
	res := Result{
		Committed: []string{}, Created: []Created{}, CreatedSprints: []CreatedSprint{}, Linked: []Linked{},
		Moved: []Moved{}, Conflicts: []Conflict{}, Failures: []Failure{}, Held: []Held{},
	}
	run := &commitRun{e: e, profileID: profileID, projectKey: projectKey, res: &res, deps: newDependencies()}
	if err := run.reload(ctx); err != nil {
		return res, err
	}
	for _, p := range phases() {
		p.run(ctx, run)
		if err := run.reload(ctx); err != nil {
			res.Failures = append(res.Failures, failure(p.name, "", fmt.Sprintf("the journal could not be reread after the %s phase, so the rest of this Commit did not run: %v", p.name, err), true))
			break
		}
	}
	left, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return res, err
	}
	res.Remaining = len(left)
	return res, nil
}
```

5. Replace `commitCreate` with:

```go
// commitCreate posts one draft and rekeys it. A refused create blocks its
// key, so every draft, edit, move and link naming it is held this Commit.
func (e *Engine) commitCreate(ctx context.Context, r *commitRun, createRow journal.PendingChange, d backend.IssueDraft) {
	tempKey := createRow.EntityKey
	realKey, err := e.b.CreateIssue(ctx, r.projectKey, d)
	if err != nil {
		r.res.Failures = append(r.res.Failures, failure(tempKey, issuerepo.EntityIssueCreate, err.Error(), true))
		r.deps.block(tempKey, tempKey, "which Jira refused")
		return
	}
	rows := []journal.PendingChange{createRow}
	if err := e.repo.Rekey(ctx, r.profileID, tempKey, realKey); err != nil {
		// Jira has the issue even though the local rename failed. Clear the
		// journal under the temp key and audit the creation there so a retry
		// reconciles instead of posting a duplicate; report the real key so
		// the user can find it. The next full sync brings its row in.
		r.deps.block(tempKey, tempKey, fmt.Sprintf("which Jira created as %s but TAM could not rename; sync, then set it again", realKey))
		if merr := e.repo.MarkCreatedWithoutRekey(ctx, r.profileID, tempKey, realKey, rows); merr != nil {
			// Jira holds the issue and the create row is still pending, so a
			// second Commit would post a duplicate rather than recover.
			r.res.Failures = append(r.res.Failures, failure(realKey, issuerepo.EntityIssueCreate, fmt.Sprintf("created in Jira as %s but the local row could not be renamed, and the journal could not be cleared: %v", realKey, merr), false))
			return
		}
		r.res.Failures = append(r.res.Failures, failure(realKey, issuerepo.EntityIssueCreate, fmt.Sprintf("created in Jira as %s but the local row could not be renamed: %v", realKey, err), false))
		return
	}
	// Rekey already moved these rows' audit trail to realKey; follow suit so
	// the commit entries land there too instead of under the old temp key.
	rows[0].EntityKey = realKey
	if err := e.repo.MarkCommitted(ctx, r.profileID, rows); err != nil {
		// Same duplicate risk as the rekey failure above: Jira has the issue
		// and the create row survived, so this is not for the user to retry.
		r.res.Failures = append(r.res.Failures, failure(realKey, issuerepo.EntityIssueCreate, "created in Jira but the journal could not be cleared: "+err.Error(), false))
		return
	}
	e.refresh(ctx, r.profileID, realKey)
	r.res.Created = append(r.res.Created, Created{TempKey: tempKey, Key: realKey})
}
```

6. Delete `regroupEdits`. Keep the `"sort"` import: `commitLinks` still sorts its rows.

In `tam/internal/committer/boards.go`:

1. Change `commitBoardMoves`'s signature to `func (e *Engine) commitBoardMoves(ctx context.Context, profileID string, res *Result, deps *dependencies)` and its first line to `rows, err := e.boardRows(ctx, profileID, res, deps)`.
2. Replace `boardRows` with:

```go
// boardRows reads the journal again, after the creates and the edits have
// run, and returns the board rows oldest first. Rereading is what lets a
// row that was journaled against a draft be pushed under the key the create
// pass gave it. A row naming a placeholder this Commit could not make real,
// a draft issue as its key or a draft sprint as its target, is held; any
// other row under a key that is still a draft is left out, as before.
func (e *Engine) boardRows(ctx context.Context, profileID string, res *Result, deps *dependencies) ([]journal.PendingChange, error) {
	all, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	rows := make([]journal.PendingChange, 0, len(all))
	for _, p := range all {
		if !boardRow(p.EntityType) {
			continue
		}
		if waits, held := deps.blockedBy(p.EntityKey, p.AfterVal); held {
			deps.hold(res, p.EntityKey, p.EntityType, p.ID, waits)
			continue
		}
		if isDraftKey(p.EntityKey) {
			continue
		}
		rows = append(rows, p)
	}
	return rows, nil
}
```

- [ ] **Step 7: Run the committer tests**

Run: `cd tam && go test ./internal/committer/`
Expected: PASS, every existing test included. `TestAnEditNamingAnUncreatedDraftWaits` still passes here (its edit is not held until Task 11).

- [ ] **Step 8: Commit**

```bash
git add tam/internal/committer
git commit -m "feat(tam): commit in phases, sprints then epics then issues then sub-tasks, holding what waits"
```

---

### Task 11: The placeholder firewall, and holding edits and links

**Files:**
- Create: `tam/internal/committer/firewall.go`, `tam/internal/committer/firewall_internal_test.go`, `tam/internal/committer/held_test.go`
- Modify: `tam/internal/committer/phases.go` (`createDrafts`, `createSprints`, `pushEdits`, the links entry of `phases()`)
- Modify: `tam/internal/committer/committer.go` (`commitEdit`, `commitLinks`)
- Modify: `tam/internal/committer/boards.go` (`pushSprints`, `pushTransitions`), `tam/internal/committer/ranks.go` (`pushBoardRanks`)
- Modify: `tam/internal/committer/committer_test.go` (`TestAnEditNamingAnUncreatedDraftWaits`)

**Interfaces:**
- Consumes: `dependencies`, `Held`, `commitRun` (Task 10).
- Produces:
  - `func assertNoPlaceholders(payload any) error`: marshals the payload and walks it; any string starting `TAM-NEW-`, and any negative whole number (string or number) under a JSON key containing `sprint`, fails with the firewall sentence. Called immediately before every backend write the committer makes.
  - `func (e *Engine) commitLinks(ctx context.Context, profileID string, res *Result, deps *dependencies)`
  - Edits naming a blocked placeholder are held (RowID 0: the whole issue's edits); links whose source or target is blocked are held.

- [ ] **Step 1: Write the failing tests**

`tam/internal/committer/firewall_internal_test.go`:

```go
package committer

import (
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestAssertNoPlaceholdersTripsOnWhatRewritingMissed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload any
		trip    bool
	}{
		{"a draft with real ids", backend.IssueDraft{Type: "story", Summary: "x", ParentKey: "PLAT-9", SprintID: "100", Labels: []string{"a"}}, false},
		{"a parent still a placeholder", backend.IssueDraft{Summary: "x", ParentKey: "TAM-NEW-2"}, true},
		{"a placeholder deep in an extra", backend.IssueDraft{Summary: "x", Extra: map[string]string{"customfield_1": "TAM-NEW-9"}}, true},
		{"a draft sprint id on a draft", backend.IssueDraft{Summary: "x", SprintID: "-1"}, true},
		{"a draft sprint id as a move target", map[string]any{"sprintId": "-3", "issues": []string{"PLAT-1"}}, true},
		{"a numeric draft sprint id", map[string]any{"sprintId": -3}, true},
		{"negative points are not a sprint", map[string]any{"storyPoints": -1}, false},
		{"a placeholder in a list of keys", map[string]any{"sprintId": "100", "issues": []string{"PLAT-1", "TAM-NEW-4"}}, true},
		{"a link to a placeholder", map[string]any{"from": "PLAT-1", "link": backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "TAM-NEW-3"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := assertNoPlaceholders(tc.payload)
			if tc.trip != (err != nil) {
				t.Fatalf("err = %v, want trip %v", err, tc.trip)
			}
			if err != nil && (!strings.HasPrefix(err.Error(), "internal error: ") || !strings.Contains(err.Error(), "still names a local placeholder")) {
				t.Errorf("message = %q", err)
			}
		})
	}
}
```

`tam/internal/committer/held_test.go`:

```go
package committer_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// An epic Jira refuses holds the edit parenting a real story under it, the
// link drafted from it, and the link drafted to it. None of them is a
// failure: nothing about those rows was sent, and the next Commit sends them
// once the epic exists.
func TestAnEditAndTheLinksWaitingOnARefusedEpicAreHeldNotSent(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	epic, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeEpic, Summary: "Epic"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.EditField(ctx, "p1", "PLAT-2", "parentKey", epic); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.AddLink(ctx, "p1", epic, backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: "XT-9"}); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.AddLink(ctx, "p1", "PLAT-1", backend.LinkDraft{Type: "Relates", Direction: "outward", ToKey: epic}); err != nil {
		t.Fatal(err)
	}
	h.jira.createErrFor["Epic"] = errors.New("POST failed: 400 Epic Name is required")

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != epic {
		t.Errorf("failures = %+v, want only the epic's", res.Failures)
	}
	if len(h.jira.updates) != 0 || len(h.jira.links) != 0 {
		t.Errorf("nothing naming the epic was sent: updates %v, links %v", h.jira.updates, h.jira.links)
	}
	want := "waits for " + epic + ", which Jira refused"
	var sawEdit, sawFrom, sawTo bool
	for _, held := range res.Held {
		if held.Reason != want {
			t.Errorf("held %+v, want reason %q", held, want)
		}
		switch {
		case held.Key == "PLAT-2" && held.EntityType == issuerepo.EntityIssue && held.RowID == 0:
			sawEdit = true
		case held.Key == epic && held.EntityType == issuerepo.EntityLink:
			sawFrom = true
		case held.Key == "PLAT-1" && held.EntityType == issuerepo.EntityLink && held.RowID != 0:
			sawTo = true
		}
	}
	if !sawEdit || !sawFrom || !sawTo {
		t.Errorf("held = %+v, want the edit and both links", res.Held)
	}
}

// A rewriting bug that leaves a placeholder in a payload must end as a
// failure TAM owns, not as a 400 Jira answers.
func TestTheFirewallStopsAPlaceholderRewritingMissed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	temp, err := h.repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Leaky"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Exec(`UPDATE pending_change SET after_val = ? WHERE profile_id = 'p1' AND entity_key = ?`,
		`{"type":"task","summary":"Leaky","labels":[],"extra":{"customfield_10001":"TAM-NEW-77"}}`, temp); err != nil {
		t.Fatal(err)
	}

	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.creates) != 0 {
		t.Fatalf("the payload reached Jira: %+v", h.jira.creates)
	}
	if len(res.Failures) != 1 || res.Failures[0].Retryable || !strings.HasPrefix(res.Failures[0].Error, "internal error: ") {
		t.Errorf("failures = %+v", res.Failures)
	}
}
```

In `tam/internal/committer/committer_test.go`, replace the assertions of `TestAnEditNamingAnUncreatedDraftWaits` from `if len(res.Failures) != 2 {` down to the closing of the `keys["PLAT-2"]` check with:

```go
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Error, "Severity") || res.Failures[0].Key != temp {
		t.Fatalf("one failure, the create's: %+v", res.Failures)
	}
	if len(res.Held) != 1 || res.Held[0].Key != "PLAT-2" || res.Held[0].Reason != "waits for "+temp+", which Jira refused" {
		t.Errorf("the story's edit is held with the reason: %+v", res.Held)
	}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam && go test ./internal/committer/ -run 'AssertNoPlaceholders|HeldNotSent|Firewall|AnEditNamingAnUncreatedDraftWaits'`
Expected: FAIL: `undefined: assertNoPlaceholders`, the edit is reported as a second failure, the links are not held.

- [ ] **Step 3: Write `firewall.go`**

`tam/internal/committer/firewall.go`:

```go
package committer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"agile-suite/tam/internal/issuerepo"
)

// assertNoPlaceholders is the last check before a write leaves the machine.
// Every phase rewrites placeholders before a later phase reads them, and
// holds what it could not make real; if a bug in that rewriting ever lets a
// TAM-NEW key or a draft sprint's negative id through, it ends here as an
// internal error on that one write, and never as a 400 from Jira.
//
// A label that happens to read TAM-NEW-9 trips it too. That is the price of
// a check that does not need to know which fields carry keys.
func assertNoPlaceholders(payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("internal error: the payload could not be checked for placeholders, so it was not sent to Jira: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return fmt.Errorf("internal error: the payload could not be checked for placeholders, so it was not sent to Jira: %w", err)
	}
	if where, found := findPlaceholder(tree, "the payload", false); found {
		return fmt.Errorf("internal error: %s still names a local placeholder, so it was not sent to Jira; this is a TAM bug, discard the change and make it again", where)
	}
	return nil
}

// findPlaceholder walks a decoded payload. sprint says the value sits under a
// key naming a sprint, where a negative whole number is a draft sprint's id.
func findPlaceholder(v any, path string, sprint bool) (string, bool) {
	switch t := v.(type) {
	case string:
		if strings.HasPrefix(t, issuerepo.DraftPrefix) || (sprint && negativeWhole(t)) {
			return fmt.Sprintf("%s (%s)", path, t), true
		}
	case json.Number:
		if sprint && negativeWhole(t.String()) {
			return fmt.Sprintf("%s (%s)", path, t), true
		}
	case []any:
		for i, e := range t {
			if where, found := findPlaceholder(e, fmt.Sprintf("%s[%d]", path, i), sprint); found {
				return where, true
			}
		}
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if where, found := findPlaceholder(t[k], path+"."+k, strings.Contains(strings.ToLower(k), "sprint")); found {
				return where, true
			}
		}
	}
	return "", false
}

func negativeWhole(s string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil && n < 0
}
```

- [ ] **Step 4: Call it before every write, and hold edits and links**

In `tam/internal/committer/phases.go`:

1. In `createDrafts`, directly before `r.e.commitCreate(ctx, r, t.row, t.draft)`, add:

```go
		if err := assertNoPlaceholders(t.draft); err != nil {
			r.res.Failures = append(r.res.Failures, failure(key, issuerepo.EntityIssueCreate, err.Error(), false))
			r.deps.block(key, key, "which could not be sent")
			continue
		}
```

2. In `createSprints`, directly before `made, err := w.CreateSprint(ctx, d.BoardID, d.SprintDraft())`, add:

```go
		if err := assertNoPlaceholders(map[string]any{"boardId": d.BoardID, "draft": d.SprintDraft()}); err != nil {
			r.res.Failures = append(r.res.Failures, sprintFailure(p, d.Name, err.Error(), false))
			r.deps.block(p.EntityKey, label, "which could not be sent")
			continue
		}
```

3. In `pushEdits`, replace the final loop with:

```go
	for _, key := range keys {
		rows := byKey[key]
		values := make([]string, 0, len(rows))
		for _, p := range rows {
			values = append(values, p.AfterVal)
		}
		// Every edit of the issue waits together, the way a conflict holds
		// every edit of an issue together: half an issue's edits pushed is
		// an intent Jira never saw whole.
		if waits, held := r.deps.blockedBy(values...); held {
			r.deps.hold(r.res, key, issuerepo.EntityIssue, 0, waits)
			continue
		}
		r.e.commitEdit(ctx, r.profileID, key, rows, r.res)
	}
```

4. Change the links entry of `phases()` to `{name: "links", run: func(ctx context.Context, r *commitRun) { r.e.commitLinks(ctx, r.profileID, r.res, r.deps) }},`.

In `tam/internal/committer/committer.go`:

1. In `commitEdit`, directly before `if err := e.b.UpdateIssue(ctx, key, fields); err != nil {`, add:

```go
	if err := assertNoPlaceholders(map[string]any{"issue": key, "fields": fields}); err != nil {
		res.Failures = append(res.Failures, failure(key, issuerepo.EntityIssue, err.Error(), false))
		return
	}
```

2. Change `commitLinks`'s signature to `func (e *Engine) commitLinks(ctx context.Context, profileID string, res *Result, deps *dependencies)` and replace its loop body from `if strings.HasPrefix(p.EntityKey, issuerepo.DraftPrefix) {` down to `if err := e.b.CreateLink(ctx, p.EntityKey, d); err != nil {` (exclusive) with:

```go
		if waits, held := deps.blockedBy(p.EntityKey); held {
			deps.hold(res, p.EntityKey, issuerepo.EntityLink, p.ID, waits)
			continue
		}
		if isDraftKey(p.EntityKey) {
			continue
		}
		var d backend.LinkDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			// A link row nobody can decode is not going to decode next time.
			res.Failures = append(res.Failures, linkFailure(p, "the link could not be decoded: "+err.Error(), false))
			continue
		}
		if waits, held := deps.blockedBy(d.ToKey); held {
			deps.hold(res, p.EntityKey, issuerepo.EntityLink, p.ID, waits)
			continue
		}
		if err := assertNoPlaceholders(map[string]any{"from": p.EntityKey, "link": d}); err != nil {
			res.Failures = append(res.Failures, linkFailure(p, err.Error(), false))
			continue
		}
```

In `tam/internal/committer/boards.go`:

1. In `pushSprints`, directly before `if err := w.MoveIssuesToSprint(ctx, target, keys); err != nil {`, add:

```go
			if err := assertNoPlaceholders(map[string]any{"sprintId": target, "issues": keys}); err != nil {
				for _, p := range batch {
					res.Failures = append(res.Failures, boardFailure(p, err, false))
				}
				continue
			}
```

2. In `pushTransitions`, directly before `if err := e.b.Transition(ctx, key, targets); err != nil {`, add:

```go
		if err := assertNoPlaceholders(map[string]any{"issue": key}); err != nil {
			res.Failures = append(res.Failures, boardFailure(p, err, false))
			continue
		}
```

In `tam/internal/committer/ranks.go`, in `pushBoardRanks`, directly before `if err := w.RankIssue(ctx, key, anchor, before); err != nil {`, add:

```go
		if err := assertNoPlaceholders(map[string]any{"issue": key, "neighbour": anchor}); err != nil {
			res.Failures = append(res.Failures, boardFailure(p, err, false))
			continue
		}
```

The `continue` leaves `prev` where it was, exactly as the `RankIssue` error branch below it does: the next card must not anchor against a card Jira never moved.

- [ ] **Step 5: Run the committer tests and the whole Go gate**

Run: `cd tam && go test ./internal/committer/`
Expected: PASS.

Run: `cd tam && go test ./... && cd ../core && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add tam/internal/committer
git commit -m "feat(tam): never send a placeholder to Jira, and hold every edit and link waiting on one"
```

---

### Task 12: Held rows and draft sprints in Pending changes

**Files:**
- Modify: `tam/frontend/src/api.ts:607-617` (after `PendingChange`: `DraftSprint`, `ENTITY_SPRINT_CREATE`), `:736-745` (`CommitResult`, `CommitHeld`)
- Modify: `tam/frontend/src/queries/pending.ts:106-151` (`PendingGroup`, `groupPending`), `:63-69` (`useDiscardChange`)
- Modify: `tam/frontend/src/components/PendingChangesModal.tsx`
- Modify: `tam/frontend/src/components/CommitBanner.tsx`
- Modify: `tam/frontend/src/App.css` (after `.chip-draft`, line 381)
- Test: `tam/frontend/src/components/PendingChangesModal.test.tsx`

**Interfaces:**
- Consumes: `Result.Held`, `Result.CreatedSprints` (Tasks 10 and 11); `sprint_create` journal rows (Task 6); `invalidateSprintWrites` (`queries/sprints.ts`); `dayInput` (`lib/format.ts`).
- Produces:
  - `export interface CommitHeld { key: string; entityType: string; rowId: number; waitsFor: string; reason: string }`
  - `CommitResult.createdSprints?: { draftId: number; id: number; name: string }[]`, `CommitResult.held?: CommitHeld[]`
  - `export interface DraftSprint { boardId: number; boardName: string; name: string; goal: string; startDate: string; endDate: string }`
  - `export const ENTITY_SPRINT_CREATE = "sprint_create"`
  - `PendingGroup.sprint: DraftSprint | null`, `PendingGroup.sprintRow: PendingChange | null`
  - `export function sprintDraftLine(s: DraftSprint | null): string` from `PendingChangesModal.tsx`

- [ ] **Step 1: Write the failing tests**

In `tam/frontend/src/components/PendingChangesModal.test.tsx`, add inside `describe("PendingChangesModal", ...)`:

```tsx
  it("shows a draft sprint as its own card, first, and discards it", async () => {
    const user = userEvent.setup();
    const sprintRow: PendingChange = {
      id: 9, entityType: "sprint_create", entityKey: "-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "2026-09-15T10:00:00Z",
      afterVal: JSON.stringify({ boardId: 1, boardName: "PLAT Scrum", name: "Sprint 15", goal: "", startDate: "2026-09-16T09:00:00.000+0000", endDate: "2026-09-30T09:00:00.000+0000" }),
    };
    vi.mocked(api.ListPendingChanges).mockResolvedValue([sprintRow, ...rows]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    expect(await within(dialog).findByText("4 changes on 2 issues, 1 of them new, and 1 new sprint")).toBeInTheDocument();
    const cards = within(dialog).getAllByRole("group");
    expect(cards.map((c) => c.getAttribute("aria-label"))).toEqual(["Sprint 15", "TAM-NEW-1", "PLAT-409"]);
    expect(within(cards[0]).getByText("Draft sprint")).toBeInTheDocument();
    expect(within(cards[0]).getByText("New sprint on PLAT Scrum, 2026-09-16 to 2026-09-30")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Commit (3)" })).toBeEnabled();
    await user.click(within(cards[0]).getByRole("button", { name: "Discard Sprint 15" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 9));
  });

  it("says what the last Commit held and what each held row waits for", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], createdSprints: [{ draftId: -1, id: 100, name: "Sprint 15" }], linked: [], conflicts: [], remaining: 3,
      failures: [{ key: "TAM-NEW-1", error: "POST failed: 400 Epic Name is required", entityType: "issue_create", rowId: 3, retryable: true }],
      held: [{ key: "PLAT-409", entityType: "issue", rowId: 0, waitsFor: "TAM-NEW-1", reason: "waits for TAM-NEW-1, which Jira refused" }],
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    expect(await within(dialog).findByText("Last commit: 1 sprint created (Sprint 15 is sprint 100), 1 failed, 1 waiting.")).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-409 waits for TAM-NEW-1, which Jira refused.")).toBeInTheDocument();
    const card = within(dialog).getByRole("group", { name: "PLAT-409" });
    expect(within(card).getByText("Waiting")).toBeInTheDocument();
    expect(within(card).getByText("Waits for TAM-NEW-1, which Jira refused.")).toBeInTheDocument();
    expect(within(dialog).getByText("Commit again to retry the failures.")).toBeInTheDocument();
  });

  it("offers Commit again for held rows even when nothing failed for good", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], linked: [], conflicts: [], remaining: 2,
      failures: [{ key: "Sprint 15", error: "the draft sprint could not be decoded", entityType: "sprint_create", rowId: 9, retryable: false }],
      held: [{ key: "TAM-NEW-1", entityType: "issue_create", rowId: 3, waitsFor: "-1", reason: "waits for sprint \"Sprint 15\", which could not be read" }],
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    expect(await within(dialog).findByText("Commit again once what they wait for is in Jira.")).toBeInTheDocument();
    expect(within(dialog).queryByText("Commit again to retry the failures.")).not.toBeInTheDocument();
  });
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam/frontend && npx vitest run src/components/PendingChangesModal.test.tsx`
Expected: FAIL: the sprint row renders as an edit row, `held` and `createdSprints` are ignored.

- [ ] **Step 3: Types**

In `tam/frontend/src/api.ts`, directly after `PendingChange`, add:

```ts
// ENTITY_SPRINT_CREATE is the journal entity of a sprint drafted in TAM. Its
// entityKey is the draft's negative id and its afterVal a DraftSprint.
export const ENTITY_SPRINT_CREATE = "sprint_create";

// DraftSprint mirrors issuerepo.DraftSprint: what a sprint_create row carries.
export interface DraftSprint {
  boardId: number;
  boardName: string;
  name: string;
  goal: string;
  startDate: string;
  endDate: string;
}
```

Replace `CommitResult` with:

```ts
// CommitHeld mirrors committer.Held: a row the Commit did not send because
// something it names was not created in Jira. It stays pending; reason is
// the sentence that says what it waits for.
export interface CommitHeld {
  key: string;
  entityType: string;
  rowId: number;
  waitsFor: string;
  reason: string;
}

export interface CommitResult {
  committed: string[];
  created: { tempKey: string; key: string }[];
  // createdSprints and held are optional for the same reason CommitFailure's
  // fields are: fixtures written before phased Commit do not spell them out.
  createdSprints?: { draftId: number; id: number; name: string }[];
  linked: { key: string; toKey: string; type: string }[];
  // moved is optional for the same reason CommitFailure's fields are.
  moved?: CommitMove[];
  conflicts: Conflict[];
  failures: CommitFailure[];
  held?: CommitHeld[];
  remaining: number;
}
```

- [ ] **Step 4: Grouping and discard**

In `tam/frontend/src/queries/pending.ts`:

1. Add `ENTITY_SPRINT_CREATE` to the value import from `"../api"`, `DraftSprint` to the type import, and `import { invalidateSprintWrites } from "./sprints";`.
2. Replace `PendingGroup` and `groupPending` with:

```ts
// A PendingGroup is one issue's rows, or one draft sprint. A draft group
// carries its decoded draft; a sprint group its decoded DraftSprint; an edit
// group carries one row per field; a link group one row per journaled link;
// a move group the board rows, which are their own kind because they are
// pushed their own way and read as places rather than as field values.
export interface PendingGroup {
  key: string;
  draft: IssueDraft | null;
  createRow: PendingChange | null;
  sprint: DraftSprint | null;
  sprintRow: PendingChange | null;
  edits: PendingChange[];
  links: { row: PendingChange; link: LinkDraft }[];
  moves: PendingChange[];
}

// groupPending folds the journal (newest first) into one group per key:
// draft sprints first, since Commit creates them first, then drafts, then
// keys in the order they first appear.
export function groupPending(rows: PendingChange[]): PendingGroup[] {
  const byKey = new Map<string, PendingGroup>();
  for (const row of rows) {
    let g = byKey.get(row.entityKey);
    if (!g) {
      g = { key: row.entityKey, draft: null, createRow: null, sprint: null, sprintRow: null, edits: [], links: [], moves: [] };
      byKey.set(row.entityKey, g);
    }
    if (row.entityType === "issue_create") {
      g.createRow = row;
      try {
        g.draft = JSON.parse(row.afterVal) as IssueDraft;
      } catch {
        g.draft = null;
      }
    } else if (row.entityType === ENTITY_SPRINT_CREATE) {
      g.sprintRow = row;
      try {
        g.sprint = JSON.parse(row.afterVal) as DraftSprint;
      } catch {
        g.sprint = null;
      }
    } else if (isMoveEntity(row.entityType)) {
      g.moves.push(row);
    } else if (row.entityType === "link") {
      try {
        g.links.push({ row, link: JSON.parse(row.afterVal) as LinkDraft });
      } catch {
        g.edits.push(row);
      }
    } else {
      g.edits.push(row);
    }
  }
  const groups = [...byKey.values()];
  return [
    ...groups.filter((g) => g.sprintRow),
    ...groups.filter((g) => !g.sprintRow && g.createRow),
    ...groups.filter((g) => !g.sprintRow && !g.createRow),
  ];
}
```

3. In `useDiscardChange`, replace `onSuccess` with:

```ts
    onSuccess: (_, change) => {
      invalidateWrites(qc, profileId, change.entityKey);
      // Discarding a draft sprint removes it from every picker and puts the
      // cards moved into it back, so the sprint lists refresh too.
      if (change.entityType === ENTITY_SPRINT_CREATE) invalidateSprintWrites(qc, profileId);
    },
```

(`queries/sprints.ts` imports nothing from `queries/pending.ts`, so this adds no cycle.)

- [ ] **Step 5: The dialog and the banner**

In `tam/frontend/src/components/CommitBanner.tsx`:

1. Import `CommitHeld` beside the other types.
2. In `bannerLine`, add as the first `parts.push` (before `committed`):

```tsx
  const sprints = r.createdSprints ?? [];
  if (sprints.length) {
    parts.push(`${plural(sprints.length, "sprint created", "sprints created")} (${sprints.map((s) => `${s.name} is sprint ${s.id}`).join(", ")})`);
  }
```

   after `if (r.failures.length) ...` add:

```tsx
  const held = r.held ?? [];
  if (held.length) parts.push(`${held.length} waiting`);
```

   and change the nothing-pushed condition to `if (!sprints.length && !r.committed.length && !r.created.length && !r.linked.length && !pushed.length) {`.

3. In `CommitBanner`, after `const failures = ...`, add `const held: CommitHeld[] = result.held ?? [];`, change `warn` to `result.conflicts.length > 0 || failures.length > 0 || held.length > 0`, add after the failures map:

```tsx
      {held.map((h) => (
        <p key={`held-${h.key}-${h.entityType}-${h.rowId}`} className="small">{`${h.key} ${h.reason}.`}</p>
      ))}
```

   and replace the retry line with:

```tsx
      {retryWorthOffering(failures) ? (
        <p className="muted small">Commit again to retry the failures.</p>
      ) : (
        held.length > 0 && <p className="muted small">Commit again once what they wait for is in Jira.</p>
      )}
```

In `tam/frontend/src/components/PendingChangesModal.tsx`:

1. Add `DraftSprint` to the type import from `"../api"` and `import { dayInput } from "../lib/format";`.
2. Replace `summaryLine` with:

```tsx
// summaryLine is the dialog's subtitle: "3 changes on 2 issues, 1 of them
// new", with the draft sprints counted apart, since a sprint is not an issue.
export function summaryLine(groups: PendingGroup[], rowCount: number): string {
  const issues = groups.filter((g) => !g.sprintRow);
  const sprints = groups.length - issues.length;
  const changes = plural(rowCount, "change", "changes");
  const newSprints = plural(sprints, "new sprint", "new sprints");
  if (issues.length === 0) return `${changes}: ${newSprints}`;
  const drafts = issues.filter((g) => g.createRow).length;
  let line = `${changes} on ${plural(issues.length, "issue", "issues")}`;
  if (drafts > 0) line += `, ${drafts} of them new`;
  if (sprints > 0) line += `, and ${newSprints}`;
  return line;
}

// sprintDraftLine says where and when a draft sprint runs.
export function sprintDraftLine(s: DraftSprint | null): string {
  if (!s) return "A draft sprint that could not be read. Discard it and draft it again.";
  const board = s.boardName || `board ${s.boardId}`;
  return `New sprint on ${board}, ${dayInput(s.startDate)} to ${dayInput(s.endDate)}`;
}
```

3. After `const conflictKeys = ...`, add:

```tsx
  // What the last Commit held, by key: a row is held when something it names
  // was not created, and the card says what that is.
  const heldByKey = useMemo(() => {
    const byKey = new Map<string, string[]>();
    for (const h of lastCommit?.held ?? []) {
      const reasons = byKey.get(h.key) ?? [];
      if (!reasons.includes(h.reason)) reasons.push(h.reason);
      byKey.set(h.key, reasons);
    }
    return byKey;
  }, [lastCommit]);
```

4. Replace `orderedGroups` with:

```tsx
  // The held-back issue sits above everything: it is what blocks a clean
  // commit, so it belongs where the eye lands first. Draft sprints follow,
  // since Commit creates them before anything that moves into them.
  const orderedGroups = useMemo(
    () => [
      ...groups.filter((g) => conflictKeys.has(g.key)),
      ...groups.filter((g) => !conflictKeys.has(g.key) && g.sprintRow),
      ...groups.filter((g) => !conflictKeys.has(g.key) && !g.sprintRow && g.createRow),
      ...groups.filter((g) => !conflictKeys.has(g.key) && !g.sprintRow && !g.createRow),
    ],
    [groups, conflictKeys],
  );
```

5. Inside `orderedGroups.map`, directly after the `if (conflict) { return <ConflictCard ... />; }` block, add:

```tsx
              if (g.sprintRow) {
                const name = g.sprint?.name ?? "Draft sprint";
                return (
                  <section key={g.key} className="pending-card" role="group" aria-label={name}>
                    <div className="pending-card-head">
                      <span className="b">{name}</span>
                      <span className="chip chip-draft">Draft sprint</span>
                      <button type="button" className="btn btn-discard pending-discard" disabled={busy} aria-label={`Discard ${name}`} onClick={() => discardOne.mutate(g.sprintRow!, { onError: onDiscardError })}><span className="discard-mark" aria-hidden="true">✕</span>Discard
                      </button>
                    </div>
                    <p className="muted small">{sprintDraftLine(g.sprint)}</p>
                    <p className="muted small">Commit creates it in Jira first, then moves its cards into it. Discarding it puts those cards back.</p>
                  </section>
                );
              }
              const heldReasons = heldByKey.get(g.key) ?? [];
```

6. In the ordinary card's head, directly after `{g.createRow && <span className="chip chip-draft">Draft</span>}`, add `{heldReasons.length > 0 && <span className="chip chip-held">Waiting</span>}`, and directly after the closing `</div>` of `pending-card-head`, add:

```tsx
                  {heldReasons.map((reason) => (
                    <p key={reason} className="small pending-held">{`${reason.charAt(0).toUpperCase()}${reason.slice(1)}.`}</p>
                  ))}
```

In `tam/frontend/src/App.css`, after `.chip-draft`, add:

```css
.chip-held { background: var(--warn-bg, #fef3c7); color: var(--warn-fg, #92400e); border: 1px solid var(--warn-border, #f59e0b); }
.pending-held { margin: 4px 0 0; color: var(--warn-fg, #92400e); }
```

- [ ] **Step 6: Run the tests and the type check**

Run: `cd tam/frontend && npx vitest run`
Expected: PASS.

Run: `npm run typecheck --workspaces --if-present`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add tam/frontend/src/api.ts tam/frontend/src/queries/pending.ts tam/frontend/src/components/PendingChangesModal.tsx tam/frontend/src/components/PendingChangesModal.test.tsx tam/frontend/src/components/CommitBanner.tsx tam/frontend/src/App.css
git commit -m "feat(tam): show draft sprints and held rows in Pending changes"
```

---

### Task 13: The demo backend plays along

**Files:**
- Modify: `tam/internal/backend/demo/demo.go:24-55` (`Backend`), `:58-79` (`New`), `:277-302` (`CreateIssue`), `:304-318` (`CreateFields`)
- Test: `tam/internal/backend/demo/demo_test.go`, `tam/app_commitplan_test.go` (create)

**Interfaces:**
- Consumes: the whole of Parts A to C.
- Produces:
  - `const demobackend.RefusedEpicMarker = "refused"`: the first create of an epic whose summary contains it (case-insensitive) is refused once per run, so held dependents can be seen on a demo profile without touching the ordinary plan.
  - Demo `CreateIssue` refuses a `ParentKey` that is still a `TAM-NEW-` key, the way Jira answers 400.
  - Demo `CreateFields`: Bug gains optional `environment` (`textarea`); Story gains optional `duedate` (`date`) and `customfield_10300` Acceptance criteria (`textarea`).

- [ ] **Step 1: Write the failing tests**

In `tam/internal/backend/demo/demo_test.go`, in `TestDemoBackendWritesInMemoryAndStagesOneConflict`, replace the bug create-fields assertion with:

```go
	specs, err := b.CreateFields(ctx, "ACME", backend.TypeBug)
	if err != nil || len(specs) != 2 || specs[0].Type != "option" || !specs[0].Required || len(specs[0].AllowedValues) != 3 ||
		specs[1].ID != "environment" || specs[1].Required || specs[1].Type != "textarea" {
		t.Errorf("bug create fields: %+v %v", specs, err)
	}
```

and add at the end of the file:

```go
func TestDemoRefusesAMarkedEpicOnceAndAPlaceholderParentAlways(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	if _, err := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeEpic, Summary: "Plain epic"}); err != nil {
		t.Fatalf("an ordinary epic is created: %v", err)
	}
	marked := backend.IssueDraft{Type: backend.TypeEpic, Summary: "Refused promotions epic"}
	if _, err := b.CreateIssue(ctx, "ACME", marked); err == nil || !strings.Contains(err.Error(), "Commit again") {
		t.Fatalf("the marked epic is refused the first time: %v", err)
	}
	if key, err := b.CreateIssue(ctx, "ACME", marked); err != nil || key == "" {
		t.Fatalf("and created the second time: %q %v", key, err)
	}
	if _, err := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeStory, Summary: "Leaky", ParentKey: "TAM-NEW-2"}); err == nil {
		t.Error("a placeholder parent is refused the way Jira refuses it")
	}
	story, _ := b.CreateFields(ctx, "ACME", backend.TypeStory)
	if len(story) != 2 || story[0].Required || story[1].Required {
		t.Errorf("story offers two optional fields: %+v", story)
	}
}
```

Create `tam/app_commitplan_test.go`:

```go
package main

import (
	"strconv"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

// The ticket's plan, end to end on the demo backend: a new sprint, an epic, a
// story under the epic in the sprint, and a technical task under the story,
// drafted offline and committed with one press.
func TestTheTicketsPlanCommitsInOnePressOnTheDemoBackend(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	demo := demobackend.New(p.ProjectKey)
	a.backends[p.ID] = demo
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	sprint, err := a.CreateSprint(p.ID, 1, "Sprint 15", "Ship promos", "2026-09-16", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	sid := strconv.Itoa(sprint.Sprint.ID)
	epic, err := a.CreateIssue(p.ID, backend.IssueDraft{Type: backend.TypeEpic, Summary: "Promotions"})
	if err != nil {
		t.Fatal(err)
	}
	story, err := a.CreateIssue(p.ID, backend.IssueDraft{Type: backend.TypeStory, Summary: "Promo input", ParentKey: epic, SprintID: sid, SprintName: "Sprint 15"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateIssue(p.ID, backend.IssueDraft{Type: backend.TypeSubtask, Summary: "Wire the input", ParentKey: story}); err != nil {
		t.Fatal(err)
	}

	res, err := a.CommitPendingChanges(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 0 || len(res.Held) != 0 || res.Remaining != 0 || len(res.Created) != 3 || len(res.CreatedSprints) != 1 {
		t.Fatalf("result = %+v", res)
	}
	real := map[string]string{}
	for _, c := range res.Created {
		real[c.TempKey] = c.Key
	}
	sprintID := strconv.Itoa(res.CreatedSprints[0].ID)
	gotStory, err := demo.GetIssue(a.ctx, real[story])
	if err != nil || gotStory.ParentKey != real[epic] || gotStory.SprintID != sprintID {
		t.Errorf("story in Jira = %+v, %v; want under %s in sprint %s", gotStory, err, real[epic], sprintID)
	}
	for temp, key := range real {
		if temp == story || temp == epic {
			continue
		}
		sub, err := demo.GetIssue(a.ctx, key)
		if err != nil || sub.ParentKey != real[story] {
			t.Errorf("technical task in Jira = %+v, %v; want under %s", sub, err, real[story])
		}
	}
	if open, _ := a.ListOpenSprints(p.ID); len(open) != 1 || open[0].Draft || open[0].ID != res.CreatedSprints[0].ID {
		t.Errorf("the picker offers the real sprint now: %+v", open)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd tam && go test ./internal/backend/demo/ -run 'StagesOneConflict|RefusesAMarkedEpic' && go test . -run TheTicketsPlan`
Expected: FAIL: bug fields are one, the marked epic is created at once, `RefusedEpicMarker` behaviour missing. The app test may already pass; if it does, it stands as the regression guard for the whole bundle.

- [ ] **Step 3: Implement**

In `tam/internal/backend/demo/demo.go`:

1. Add to the `Backend` struct:

```go
	// refusedEpic is whether an epic carrying RefusedEpicMarker has already
	// been refused this run. The refusal is staged once, the way the
	// conflict on the curated story is, so a demo profile can show a Commit
	// holding the drafts that wait on a refused create.
	refusedEpic bool
```

2. Add below `New`:

```go
// RefusedEpicMarker is the word that stages a refusal: the first create of an
// epic whose summary contains it, in any case, is refused, and the next one
// goes through. An ordinary plan never trips it.
const RefusedEpicMarker = "refused"
```

3. At the top of `CreateIssue`'s body, after `defer b.mu.Unlock()`, add:

```go
	// Jira answers a parent it does not know with a 400. Commit should never
	// send one; if it does, the demo says so the way the real thing would.
	if strings.HasPrefix(d.ParentKey, "TAM-NEW-") {
		return "", fmt.Errorf("demo: %s is not an issue, so it cannot be a parent", d.ParentKey)
	}
	if d.Type == backend.TypeEpic && !b.refusedEpic && strings.Contains(strings.ToLower(d.Summary), RefusedEpicMarker) {
		b.refusedEpic = true
		return "", errors.New("demo: Jira refused this epic once, so the drafts waiting for it are held; Commit again to create it")
	}
```

4. Replace `CreateFields` with:

```go
// CreateFields offers a required and an optional field on bugs, one required
// field on requirements, and two optional fields on stories, so the New issue
// dialog's required section and its More fields section can both be seen
// offline, for an option, a text, a long text and a date field.
func (b *Backend) CreateFields(_ context.Context, _, logicalType string) ([]backend.FieldSpec, error) {
	switch logicalType {
	case backend.TypeBug:
		return []backend.FieldSpec{{
			ID: "customfield_10050", Name: "Severity", Type: "option", Required: true,
			AllowedValues: []backend.FieldOption{{ID: "1", Value: "Minor"}, {ID: "2", Value: "Major"}, {ID: "3", Value: "Critical"}},
		}, {
			ID: "environment", Name: "Environment", Type: "textarea", Required: false, AllowedValues: []backend.FieldOption{},
		}}, nil
	case backend.TypeRequirement:
		return []backend.FieldSpec{{ID: "customfield_10060", Name: "Source", Type: "string", Required: true, AllowedValues: []backend.FieldOption{}}}, nil
	case backend.TypeStory:
		return []backend.FieldSpec{
			{ID: "customfield_10300", Name: "Acceptance criteria", Type: "textarea", Required: false, AllowedValues: []backend.FieldOption{}},
			{ID: "duedate", Name: "Due date", Type: "date", Required: false, AllowedValues: []backend.FieldOption{}},
		}, nil
	}
	return []backend.FieldSpec{}, nil
}
```

Add `"errors"` to the imports (`strings` is already there).

- [ ] **Step 4: Run the tests to see them pass**

Run: `cd tam && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/backend/demo/demo.go tam/internal/backend/demo/demo_test.go tam/app_commitplan_test.go
git commit -m "test(tam): the demo backend stages a refused epic and offers optional create fields"
```

---

### Task 14: Documentation, the full gate, and the build

**Files:**
- Modify: `tam/CLAUDE.md` (Status paragraph lines 25-31; "The Sprints view" lines 677-700; "The create dialog" lines 942-946; Layout lines 1221-1300 and the `src/components/` list)

- [ ] **Step 1: Update the Status paragraph**

In `tam/CLAUDE.md`, replace:

```
board's unassigned work folded in, detail panel beside it. Create, edit,
delete sprint live here, and reach Jira moment pressed, not wait for
Commit, same exception Phase 3c carve out for start + complete. Fill
```

with:

```
board's unassigned work folded in, detail panel beside it. Create, edit,
delete sprint live here; edit + delete of sprint Jira hold reach Jira moment
pressed, same exception Phase 3c carve out for start + complete, while
create journal draft sprint (bundle 01, below). Fill
```

- [ ] **Step 2: Rewrite the Sprints view's immediate-write paragraphs**

Replace the paragraph starting `**Create, edit, delete reach Jira immediately, and honest reason is cost not` (through `exception should start reading rather than re-derive point from scratch.`) with:

```
**Edit and delete of sprint Jira hold reach Jira immediately; create does
not any more.** Reason for immediate edit + delete = cost not principle: TAM
journal is issue machinery (pending change keyed by issue key, conflict by
`updated` stamp), and sprint have no cached version to rebase edit on and no
conflict card. Create was same exception until bundle 01 paid its cost: plan
starting with new sprint could not be drafted offline. Now sprint create =
`sprint_create` journal row + draft row in `sprint` under negative id, and
Commit phase 1 create it. Full argument for edit + delete still on
`UpdateSprint` in `core/jira/sprintwrite.go`.
```

In the next paragraph replace ``exported method set is exactly `Complete`, `Create`, `Delete`, `Edit`,`` and the following ``` `Start`; ``` with ``exported method set is exactly `Complete`, `Delete`, `Edit`, `Start`;``.

- [ ] **Step 3: Rewrite the create-meta paragraph and add the bundle's sections**

Replace the paragraph starting ``Create-meta value JSON shape come from its own create-meta, in `shapeExtra`:`` (five lines) with:

```
Create fields come from `core/jira/createmeta.go` `CreateMeta`: per-type
endpoint `GET /rest/api/2/issue/createmeta/{project}/issuetypes/{typeId}`,
paged, classic expand call only on 404 (or when type has no id in project
list). Field absent from per-type answer = not on screen. `CreateFields`
return required and optional, never base field (`isBaseField`: summary,
description, priority, labels, assignee, reporter, parent, project,
issuetype, discovered Story Points / Epic Link / Epic Name / Sprint / Rank,
and greenhopper custom types by suffix), never optional field no text form
fill (`KindOther`). Value shape = `ShapeValue`: option id when Jira listed
values, `{"value"}` when not, comma list to array (`{"id"}`, `{"value"}`,
`{"name"}` by items), `{"name"}` for user, ISO day for date, midnight for
datetime. Dialog show required at once, optional behind **More fields (n)**;
`MetaField.tsx` `META_INPUTS` = per-type input table (bundle 06 replace
`textarea` entry). Draft carry `screenFields`, ids dialog offered;
`CreateIssue` `applyExtras` never let extra overwrite key already in payload
or base field, drop extra outside `screenFields` (nil = legacy draft, no
check), drop extra per-type metadata no longer list, log each drop. That =
fix for `parent: data was not an object` (duplicate Parent input) and
`customfield_10253 ... not on the appropriate screen` (classic answer
listing off-screen field). Sub-task drafted from draft parent allowed:
Commit create parent first.

## Draft sprints

Sprint created in TAM = draft. `issuerepo.CreateDraftSprint` write, in one
transaction, `sprint_create` journal row (key = negative id text, after_val
= `DraftSprint` JSON with board name) and `sprint` row with `draft = 1`,
state future (schema version 13 add `draft`). Id from profile setting
`draft_sprint_seq`, lowest of it and every negative id minus one: never
reused, so stale reference to discarded draft never attach to new one.
issuerepo write `sprint` table only for draft rows and `RekeySprint`, because
row must land and go with its journal row; every row Jira sent stay
boardrepo's, and `writeSprints` delete only `draft = 0`, so boards refresh
keep drafts. Every picker (`OpenSprints`, board sprint list, Sprints view,
detail panel Sprint field, New issue dialog, importer Sprint column) offer
draft, labelled "(draft)" / Draft chip. Card moved into draft = ordinary
`issue_sprint` row naming negative id. Start + Complete disabled with
"Commit this sprint first", and refused in Go (`errDraftSprint`) for negative
id; completion cannot move cards into draft. Edit + delete of draft local
(`EditDraftSprint` rename everywhere through `rewriteSprintID`;
`DiscardDraftSprint` = discard of `sprint_create` row: revert every move into
it, clear it off draft issues, drop row). Bindings keep `"sprint"` lock,
frontend keep `runQuietLock`. Rituals skip drafts. `RemoveBoards` unchanged:
draft on board that leave cache go with it, journal row stay in Pending
changes.

## Phased Commit

`committer.Commit` = `phases()` in order: sprints, epics, issues (task,
story, bug, requirement), sub-tasks, edits, board moves, links; journal
re-read (`commitRun.reload`) after each, so next phase see ids last one
rewrote. Inside create phase, drafts by `draftOrdinal` (n of TAM-NEW-n),
never string order (`TAM-NEW-10` used to go before `TAM-NEW-2`). Phase 1
`CreateSprint` then `RekeySprint`: draft row real, `rewriteSprintID` across
issue columns, both halves of `issue_sprint` rows, draft JSON; create row
gone. Epic/issue create `Rekey` now also `rewriteParentKey` in every other
draft JSON. Created sprint stay created if later phase fail
(`Result.CreatedSprints`). Refused create block its placeholder
(`dependencies.block`); any draft (parent or sprint), edit (after_val), board
row (key or move target), link (source or target) naming blocked placeholder
held (`Result.Held`, reason `waits for TAM-NEW-2, which Jira refused`, chain
`which is waiting for ...`), stay in journal, retried next Commit; held draft
block own key. Held != failure: nothing sent. `assertNoPlaceholders`
(`firewall.go`) walk every payload right before backend call: `TAM-NEW-`
string anywhere, or negative whole number under key naming sprint, fail that
one write with internal error, never 400 from Jira (label reading
`TAM-NEW-9` trip it too, accepted). Pending changes dialog show draft sprint
as own card first, held rows with Waiting chip + reason; banner count
"n waiting". Later bundle add commit step = one entry in `phases()`.
Demo: epic whose summary contain "refused" refused once per run; demo refuse
placeholder parent.
```

Insert these three sections (the rewritten paragraph stays in "The create dialog"; `## Draft sprints` and `## Phased Commit` go directly after "The create dialog" section, before `## The grids' columns`).

- [ ] **Step 4: Update the Layout section**

Apply these replacements in the Layout block:

1. `    app_sprintmanage.go  Create, Edit and Delete sprint, and ListBoardSprintDetails for the` becomes `    app_sprintmanage.go  Create (a draft), Edit and Delete sprint (local for a draft), and ListBoardSprintDetails for the`.
2. `    internal/tamstore/   TAM's own SQLite file (schema version 12: issue (with status_id), issue_link,` becomes `    internal/tamstore/   TAM's own SQLite file (schema version 13: issue (with status_id), issue_link,`, and `                          sprint (with goal, added at version 7, and complete_date, added at` / `                          version 8), sprint_report ...` gains `, and draft, added at version 13` after `complete_date, added at version 8`.
3. After the `internal/issuerepo/` entry's last line (`before_val/after_val packing, and what Override does to a held one`), append: `; draftsprints.go is the draft sprint's create, edit, discard and rewriteSprintID, and rekeysprint.go RekeySprint, MarkSprintCreatedWithoutRekey and rewriteParentKey`.
4. Replace the `internal/sprints/` entry's first four lines with:

```
    internal/sprints/    the sprint writes that reach Jira outside a Commit: Start and Complete,
                          the ceremonies, and Edit and Delete of a sprint Jira holds;
                          exceptions_test.go fences the package's exported method set to
                          exactly those four; draft.go is DraftSprint, the check a drafted
                          sprint gets, and errDraftSprint; guards.go is what a write refuses before it reaches
```

   and change `manage.go` `Create, Edit and Delete themselves` to `manage.go Edit and Delete themselves`.
5. Replace the `internal/committer/` entry with:

```
    internal/committer/  pushes the journal to Jira in phases and resolves conflicts; phases.go is
                          the phase list, the draft sprint and draft issue creates, and the edits;
                          held.go is what a refused create blocks and the rows held for it;
                          firewall.go is assertNoPlaceholders; boards.go, ranks.go, and
                          boardvalues.go are the board pass, after the edits and before the links
```

6. In the `src/components/` list, after `NewIssueModal,` add `MetaField (a create-meta field's input, by type, through META_INPUTS),`.
7. Add a line under `internal/backend/` saying: `                          core/jira/createmeta.go is the createmeta reader and value shaper backend/jira builds the create dialog's fields from`.

- [ ] **Step 5: Run the full gate**

Run each and expect PASS / no errors:

```bash
cd core && go test ./...
cd tam && go test ./...
cd tam/frontend && npx vitest run
cd frontend/core && npx vitest run
npm run typecheck --workspaces --if-present
cd tam/frontend && npm run build
```

Search the diff for em dashes in UI text: `git diff main -- tam/frontend/src | grep -n "—"` must print nothing.

- [ ] **Step 6: Build the app**

Run: `cd tam && wails build`
Expected: `build/bin/task-activity-manager.exe` produced with no errors.

- [ ] **Step 7: Manual check on the demo profile**

Launch `cd tam && wails dev`, open the demo profile, then:

1. Boards view, scrum board, **New sprint**: "Sprint 15", dates. The dialog says `Drafted locally. Commit creates it in Jira.`; the picker shows `Sprint 15 (Draft)` and Start is disabled with `Commit this sprint first`.
2. Epics view, **+ New epic** "Promotions". Backlog **+ New** Story "Promo input", Epic = the draft epic, Sprint = `Sprint 15 (draft)`. Open More fields: Acceptance criteria and Due date appear.
3. Select the story, **+ Technical task**: the Parent row states `TAM-NEW-n` and there is no Parent input.
4. Pending changes shows the draft sprint card first. **Commit**. The banner names the sprint and three creates, nothing waiting; the story sits in the new sprint under the epic and the task under the story.
5. Draft an epic "Refused epic" and a story under it; Commit: one failure, the story shows **Waiting** with `Waits for TAM-NEW-n, which Jira refused.`; Commit again: both created.

- [ ] **Step 8: Commit**

```bash
git add tam/CLAUDE.md
git commit -m "docs(tam): draft sprints, the screen-scoped create dialog, and phased Commit"
```

After merge, update the Outline page "Task Activity Manager › User Guide" by hand with the same three topics (draft sprints, More fields, what Waiting means in Pending changes); there is no TAM user guide in this repository.

---

## Part D: the probe

### Task 15: Run the createmeta probe against the real instance (manual, non-blocking)

This task is run **by the user**. Nothing above waits for it: the code prefers the per-type endpoint, falls back only on 404, and never sends an extra outside the draft's stored field ids. The probe confirms those assumptions on the instance from the ticket.

**Files:**
- Already committed with this plan: `docs/superpowers/plans/assets/2026-09-15-createmeta-probe.md`

- [ ] **Step 1: Ask the user to run the probe**

Say: "The createmeta probe in `docs/superpowers/plans/assets/2026-09-15-createmeta-probe.md` needs your Jira instance. It is five read-mostly curl calls; please paste the answers table when you have run it."

- [ ] **Step 2: Record the answers and apply what they change**

Fill in the answers table in the probe file and commit it (`docs(tam): the createmeta probe answers`). Then:

- Probe 1 `404`: the instance is before 8.4; nothing changes, the classic fallback plus `screenFields` is what protects it.
- Probe 2 lists `customfield_10253`: add to `CreateFields` in `tam/internal/backend/jira/writes.go` a rule that skips an optional field whose `Operations` is non-empty and does not contain `"set"`, with a test in `writes_test.go` using a per-type fixture field `{"fieldId":"customfield_10253","required":false,"operations":[],"schema":{"type":"string"}}`. Commit as `fix(tam): leave out a create field the screen cannot set`.
- Probe 3 shows the parent under another id or schema: add that id to `baseFieldIDs` with a test.
- Probe 5 answers anything but a 400 naming the screen: record it; no code change.

- [ ] **Step 3: Confirm the ticket's project on a build of this branch**

With `wails dev` on the real profile, draft a Story and a Technical task under it in the ticket's project, open More fields on the Story to see what the screen offers, and Commit. Then search the log (`%APPDATA%\task-activity-manager\` log file, or the `wails dev` console) for `customfield_10253`: it may appear only in a `leaves out customfield_10253` line, never in a request. Both issues are created; the Technical task's parent is the Story's real key. Record the outcome under the answers table.
