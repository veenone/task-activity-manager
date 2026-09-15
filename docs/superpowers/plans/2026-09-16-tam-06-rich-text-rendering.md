# TAM Rich Text Rendering: Implementation Plan

**Goal:** Render Jira wiki markup and Markdown (detected per field, with a toggle) in the detail panel's description, summary heading and a new read-only Comments section, with Write and Preview wherever a long text is typed, and save every text exactly as typed.

**Spec (binding):** `docs/superpowers/specs/2026-09-15-tam-06-rich-text-rendering-design.md`, mockups `docs/superpowers/specs/assets/2026-09-15-tam-bundles/f1.png`, `f2.png`.

**Shape:** a pure parser pair (`wiki.ts` hand-written, `markdown.ts` on `marked`'s lexer) onto one small AST in `frontend/core/src/richtext/`, one React renderer over it, one Write and Preview field. Go adds comments to `backend.IssueDetail`; the detail cache stores the detail as JSON, so comments are cached, read offline and refreshed with no schema change.

## Rulings on what the spec leaves open

| Question | Ruling | Cost if wrong |
|---|---|---|
| Issue key node | No AST node. `RichText` splits text leaves on `\b<projectKey>-\d+\b` at render time, so parsers and `toPlainText` stay project-blind. | One render-time regex moves into a post-parse pass. |
| Project key for the panel | Derived from `issue.key` (text before the last `-`), not threaded from the profile. | A prop from `activeProfile.projectKey` instead. |
| Link element | `<button type="button" role="link" class="rich-link" title={url}>` with no `href`, click calls `onOpenLink` (same as `IssueKeyLink`). No `href` means no path, middle click or Ctrl click included, can navigate the WebView. A refused scheme renders its label as plain text. | Styling a button as inline text; screen readers say "link" either way. |
| Link safety | `compact` and `isAllowedLink` move from `tam/frontend/src/lib/sanitizeHtml.ts` to `frontend/core/src/lib/links.ts` (exported), and `sanitizeHtml.ts` imports them. One definition of an allowed link for the ritual editor and the renderer. | None; it is a move. |
| Detection score | Count of **distinct** signal kinds matched, not occurrences, so thirty `- ` bullets do not outvote one `{code}`. The matched labels (`h3.`, `\|\|table\|\|`, `{code}`) are returned for the hint line. | Swap to occurrence counts inside `detect.ts`; the 30-sample test says whether it helped. |
| `#` collision | A `^#{1,6} ` line counts for Markdown only when the line before and after it is not also a `#` line. Two or more stacked `#` lines are a Jira numbered list far more often than back-to-back Markdown headings. | Stacked Markdown headings read as a wiki list until toggled. |
| Where Description is edited | Stays a row of `EditableFields`: read view with toggle and **Edit**, which swaps in `RichTextField`. One Save edit, dirty tracking and journal unchanged. Edit stays open while the field is dirty; Save or Cancel (restore base, clear dirty) closes it. | Its own section with its own Save, if users want it beside Comments. |
| Toggle memory | Module-level `Map<string, RichFormat>` in `EditableFields.tsx` keyed `profileId:key:description`, shared by the read view and the editor. Lives for the app run. | None beyond a restart forgetting it, which is the spec's "session". |
| Comments section on mount | Closed, with its count in the heading, like Links: `tam/CLAUDE.md` keeps Fields the only section open on mount so the panel starts short. | Flip `open.comments` default to true. |
| Which 5 comments | The newest 5 (the list's last five), "Show all 23" above them. | Show the first 5 instead. |
| Mockup's "cached 14:02" | Not built: `IssueDetail` carries no fetch time and the spec text does not ask for it. | Add `fetchedAt` to the binding's answer. |
| Comment timestamps | Go normalises `created`/`updated` through `sprintdate.Parse` to RFC 3339 (it already reads Jira's no-colon `Agile` layout); an unreadable value is kept raw, which `formatWhen` then prints as is. "edited" = normalised `updated != created`. | None; frontend `new Date` no longer depends on the engine accepting `+0000`. |
| Comment paging | When `total` exceeds the inline count, re-read from `startAt=0` in pages of 100 up to 500 and replace the inline list (no overlap arithmetic). A failed paged read keeps the inline comments, sets `CommentsTruncated`, logs, and is **not** a Go error (the description must still show). | A retry button, if partial reads turn out common. |
| Size guard unit | 200 KB = `text.length > 200_000` UTF-16 units, cut before parsing. | Bytes would cut a CJK description sooner. |
| Inline mode | Parse with the detected format, keep the inline children of paragraph and heading blocks joined by a space, drop every other block. | Summaries are one line in Jira; nothing else uses inline. |
| Sort and search | Backlog sort and text search stay server-side on the raw summary. Plain text is for display, titles and aria labels. | A summary starting with `*` sorts under `*`. |
| Demo issue keys | `browseUrl` answers `""` on demo, so a key renders as plain accent text there, as `IssueKeyLink` already does. | QA checks key links on a real instance only. |
| `marked` version | `18.0.13` (latest on 2026-09-16), exact, in `frontend/core/package.json` `dependencies`. | Bump the pin. |
| User Guide | No TAM user guide exists in the repo; the docs task updates `tam/CLAUDE.md` and gives Outline text to paste. | Same ruling as bundle 02. |

## Constraints (every task)

- Worktree `C:\tool-projects\task-activity-manager\.claude\worktrees\tam-planning-bundles`, branch `feat/tam-bundles-01-06`. Commit only the files a task names; **never stage `tam/go.mod`** (line endings only).
- Commit messages use `feat(core):`, `feat(tam):`, `test(...)`, `docs:` and carry **no** `Co-Authored-By`, `Claude-Session` or "Generated with Claude Code" lines. Check `git log -1 --format=%B`.
- The raw string is the value. Nothing converts between syntaxes; the journal, `EditField`, `CreateDraft` and Commit see exactly what was typed.
- No `dangerouslySetInnerHTML`, no `innerHTML`, no `<img>`, no `href` anywhere under `frontend/core/src/richtext/`. Raw HTML in Markdown (`html` tokens) renders as literal text.
- Links: only `http`, `https`, `mailto` via `isAllowedLink`; a click never navigates the WebView, it calls `onOpenLink`, which TAM wires to `BrowserOpenURL`.
- Wails value-or-error: comment trouble after the issue read travels in `CommentsTruncated`, never as an error.
- Logic in `internal/` (Go) and `lib/`/`richtext/` (frontend); `app*.go` untouched. `tam/frontend/wailsjs/**` regenerated with `cd tam && wails generate module`, never hand-edited.
- No new lock: comments ride `GetIssueDetail`, which takes none.
- Core styles on tokens from `frontend/core/styles/tokens.css` only (no hex), so dark mode follows. Tables and code blocks scroll sideways inside their own `overflow-x: auto` box, never the panel.
- UI text has no em dashes. Sentences, verbatim:
  - Detected, wiki: `Detected Jira markup (<labels>). Saved exactly as typed; Jira renders it the same way.`
  - Detected, Markdown: `Detected Markdown (<labels>). Saved exactly as typed.`
  - No signal: `No markup detected, read as Jira markup. Saved exactly as typed.`
  - Picked: `Read as <Jira markup|Markdown>, your choice. Saved exactly as typed.`
  - Markdown warning: `Jira Data Center shows Markdown as plain text in its own web view. TAM renders it here; the text is sent to Jira unchanged.`
  - Size guard: `Showing the first 200 KB. Open in Jira for the rest.`
  - Comments truncated: `Showing <n> of <total> comments. Open in Jira for the rest.`
  - `No comments.` · `Show all <total>` · `edited` · toggle `Jira markup` / `Markdown` · tabs `Write` / `Preview` · `Edit` · `Cancel`
  - Image placeholder: `Image: <file>`

## Tasks

### Task 1: AST and the wiki markup parser

**Files:** create `frontend/core/src/richtext/ast.ts`, `wiki.ts`, `wiki.test.ts`.

**Produces:**
```ts
export type RichFormat = "wiki" | "markdown";
export type Mark = "bold" | "italic" | "underline" | "strike" | "sup" | "sub";
export type Inline =
  | { t: "text"; text: string } | { t: "mark"; mark: Mark; children: Inline[] }
  | { t: "code"; text: string } | { t: "link"; href: string; children: Inline[] }
  | { t: "br" } | { t: "image"; name: string };
export type Block =
  | { t: "p"; children: Inline[] } | { t: "h"; level: 1 | 2 | 3 | 4 | 5 | 6; children: Inline[] }
  | { t: "list"; ordered: boolean; items: { checked?: boolean; children: Block[] }[] }
  | { t: "table"; rows: { header: boolean; children: Inline[] }[][] }
  | { t: "codeblock"; lang: string; text: string } | { t: "quote"; children: Block[] }
  | { t: "rule" } | { t: "panel"; title: string; children: Block[] };
export function parseWiki(text: string): Block[];
```

**Tests (fixture strings of real Jira markup, `toEqual` on the AST):**
- `h1.` to `h6.`; `*bold*`, `_italic_`, `+u+`, `-strike-`, `{{mono}}`, `^sup^`, `~sub~`, nesting `*_both_*`.
- Marks need a boundary: `mid-session re-run`, `my_var_name`, `2*3*4` stay plain text; `a -gone- b` strikes.
- `[text|https://x]`, `[https://x]`, bare `https://x` become links; `[~jdoe]` and `[text|/relative]` keep their text (the renderer refuses, the parser just carries `href`).
- Lists: `*`, `#`, nested `**`, `##`, mixed `#*` (ordered list whose item holds a bullet list), a list ending at a blank line.
- Tables: `||h1||h2||` then `|a|b|`, a row mixing `||h||` and `|c|`.
- `{code:java}`...`{code}` keeps inner text verbatim (no marks, braces and `*` intact); `{code}` with no language; `{noformat}`; unclosed `{code}` runs to the end.
- `{quote}`...`{quote}` and `bq. line`; `{panel:title=Notes}`...`{panel}`; `----`; `a\\b` is a `br`.
- `{color:red}warm{color}` keeps `warm`, no colour; `!screen.png!` and `!screen.png|thumbnail!` give `image` named `screen.png`.
- Unknown macros: `{status}` alone vanishes, `{expand}inner{expand}` keeps `inner`; `{ "code": 1 }` (brace then space) is literal text.

**Done:** every case passes; `wiki.ts` has no imports but `ast.ts`.
**Gate:** `cd frontend/core; npx vitest run src/richtext; npm run typecheck`
**Commit:** `feat(core): a Jira wiki markup parser onto a small rich text tree`

### Task 2: Markdown, detection, plain text

**Files:** modify `frontend/core/package.json` (`"dependencies": { "marked": "18.0.13" }`), `package-lock.json` (root `npm install`); create `frontend/core/src/richtext/markdown.ts`, `detect.ts`, `plain.ts`, `markdown.test.ts`, `detect.test.ts`, `plain.test.ts`.

**Produces:**
```ts
export function parseMarkdown(text: string): Block[];              // marked.lexer(text, { gfm: true }) mapped
export function detectFormat(text: string): { format: RichFormat; signals: string[] };
export function parseRich(text: string, format: RichFormat | "auto"): { format: RichFormat; blocks: Block[] };
export function toPlainText(text: string): string;                  // auto-detect, parse, join text, collapse whitespace
```
`parseRich` lives in `detect.ts` (it is the one place `auto` is resolved).

**Tests:**
- Markdown mapping: headings, `**b**`/`*i*`/`~~s~~`/`` `c` ``, nested and ordered lists, GFM table (header row flagged), fenced code with language, blockquote, `---`, `- [x] done` gives `checked: true`, `![alt](http://x/p.png)` gives `image` named `p.png`, `<b>hi</b>` gives literal text `<b>hi</b>`, `` `a<b &amp;` `` keeps those characters exactly (no double escaping from the lexer).
- Collisions: `*x*` is bold in `parseWiki` and italic in `parseMarkdown`; `# one\n# two` is an ordered list in wiki and two headings in Markdown.
- Detection: 30 labelled samples in `detect.test.ts` (15 each, from Jira and GitHub style issues) all classify correctly; empty text, `plain sentence`, and one wiki plus one Markdown signal each give `wiki`; `# one\n# two\n# three` gives `wiki` with no Markdown signal; `# Steps\n\nClick it` gives `markdown`; `signals` for the f2 sample are `["h3.", "||table||", "{code}"]`.
- `toPlainText`: `Fix *login* at {{/auth}}` gives `Fix login at /auth`; `[Figma|https://f]` gives `Figma`; `**Bold** move` gives `Bold move`; `mid-session` unchanged; multi-line text collapses to one line.

**Done:** tests pass; `marked` appears exactly pinned and only in core.
**Gate:** `npm install` (root), `cd frontend/core; npx vitest run src/richtext; npm run typecheck`
**Commit:** `feat(core): Markdown on marked's lexer, per-field syntax detection, and plain text`

### Task 3: The renderer and the shared link rule

**Files:** create `frontend/core/src/lib/links.ts`, `frontend/core/src/richtext/RichText.tsx`, `RichText.test.tsx`, `noHtml.test.ts`; modify `frontend/core/src/index.ts`, `frontend/core/styles/primitives.css`, `tam/frontend/src/lib/sanitizeHtml.ts` (import `compactUrl`, `isAllowedLink` from core, re-export `isAllowedLink` so its test and `useRitualToolbar` keep their import), move the `isAllowedLink` cases from `sanitizeHtml.test.ts` to `frontend/core/src/lib/links.test.ts`.

**Produces:**
```ts
export function compactUrl(value: string): string;   // was sanitizeHtml's compact
export function isAllowedLink(value: string): boolean;
export const RICH_TEXT_LIMIT = 200_000;
export function RichText(props: {
  text: string; format?: RichFormat | "auto"; inline?: boolean;
  projectKey?: string; onOpenLink: (url: string) => void; onIssueKey?: (key: string) => void;
}): ReactElement;  // root <div class="rich-text"> (<span> when inline)
```
Index exports: `RichText`, `parseRich`, `detectFormat`, `toPlainText`, `isAllowedLink`, `compactUrl`, `RICH_TEXT_LIMIT`, types `RichFormat`, `Block`, `Inline`.

**Tests:**
- Blocks render as semantic elements (`h3`, `ul/ol/li`, `table/th/td` inside `.rich-table-scroll`, `pre > code`, `blockquote`, `hr`, `.rich-panel` with its title); a checked task item is a disabled checked checkbox.
- Security: `[x|javascript:alert(1)]`, `[x|java&#9;script:alert(1)]` (and a literal tab), `[x](javascript:alert(1))`, `[x](data:text/html,hi)`, `[x](/relative)` render `x` as text with no button; `container.querySelector("[href]")` and `container.querySelector("img")` are null for every sample; Markdown `<script>alert(1)</script>` shows as text.
- Clicking `[Figma|https://figma.com/f]` calls `onOpenLink("https://figma.com/f")` once and leaves `window.location.href` unchanged; `mailto:` is allowed.
- `projectKey="PLAT"`: `see PLAT-409.` renders `PLAT-409` as a button calling `onIssueKey("PLAT-409")`; `XT-9` and `PLAT-409` without `onIssueKey` stay text.
- `inline`: `apply at {{/payment}} step` renders a `code` inside a `span`, no `p`; `h2. Title` inline renders `Title` without a heading.
- A 250 000 character text renders the size sentence and no more than `RICH_TEXT_LIMIT` characters.
- `noHtml.test.ts`: `import.meta.glob("./**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true })` over `richtext/`, excluding test files: none contains `dangerouslySetInnerHTML`, `innerHTML` or `href=`.

**Done:** tests pass in core; `tam/frontend` `sanitizeHtml.test.ts` and ritual toolbar tests still pass; `.rich-text` styles use tokens only.
**Gate:** `cd frontend/core; npx vitest run; npm run typecheck` and `cd tam/frontend; npx vitest run src/lib src/components/ritual-editor`
**Commit:** `feat(core): render rich text as React elements, with one shared link rule`

### Task 4: Write and Preview

**Files:** create `frontend/core/src/richtext/RichTextField.tsx`, `RichTextField.test.tsx`; modify `frontend/core/src/index.ts`, `primitives.css`.

**Produces:**
```ts
export function SyntaxToggle(p: { value: RichFormat; onChange: (f: RichFormat) => void; disabled?: boolean }): ReactElement;
export function RichTextField(p: {
  value: string; onChange: (v: string) => void;
  format: RichFormat | "auto"; onFormatChange: (f: RichFormat) => void;   // "auto" until the user picks
  onOpenLink: (url: string) => void; projectKey?: string; onIssueKey?: (key: string) => void;
  textarea?: TextareaHTMLAttributes<HTMLTextAreaElement>;                 // id, className, aria-*, disabled, placeholder
  minRows?: number;                                                        // default 4; grows with lines to 16
}): ReactElement;
export const RICH_TEXT_SENTENCES: { detected(f: RichFormat, labels: string[]): string; none: string; picked(f: RichFormat): string; markdownWarning: string; sizeGuard: string };
```
`SyntaxToggle` is two `aria-pressed` buttons in a `role="group"` labelled "Syntax". The textarea stays mounted (`hidden`) on Preview so an outside `<label htmlFor>` and focus return still work.

**Tests:**
- `role="tablist"` with `Write`/`Preview` tabs, `aria-selected`, `role="tabpanel"`; Ctrl+Shift+P from the textarea switches to Preview and back; ArrowRight/ArrowLeft move between tabs.
- With `format="auto"`, typing `h3. Title` shows the detected-wiki hint naming `h3.`; typing `## Title` then flips the toggle to Markdown and shows the Markdown warning.
- Picking Markdown calls `onFormatChange("markdown")`; the parent passing `"markdown"` keeps it even when the text gains wiki signals, and the hint reads the picked sentence.
- `onChange` receives exactly the typed string (a `*bold*` and a trailing newline included); rows grow from 4 to at most 16.
- Preview renders `RichText` with the chosen format; `textarea` props (`id`, `aria-invalid`) land on the textarea.

**Done:** tests pass; no sentence contains an em dash (asserted over `RICH_TEXT_SENTENCES`).
**Gate:** `cd frontend/core; npx vitest run; npm run typecheck`
**Commit:** `feat(core): a Write and Preview field with the syntax toggle and its hint`

### Task 5: Comments in the issue detail

**Files:** modify `tam/internal/backend/backend.go`, `tam/internal/backend/jira/jira.go`, `jira_test.go`, `tam/internal/issuerepo/detail.go` (+ its test file), `tam/internal/demo/demo.go`, `tam/internal/demo/demo_test.go`, `tam/internal/backend/demo/demo.go`; regenerate `tam/frontend/wailsjs/**`; modify `tam/frontend/src/api.ts`.

**Produces:**
```go
type Comment struct {
    ID, Author, AuthorName, Created, Updated, Body string   // json: id, author, authorName, created, updated, body
}
// IssueDetail gains:
Comments          []Comment `json:"comments"`
CommentTotal      int       `json:"commentTotal"`
CommentsTruncated bool      `json:"commentsTruncated"`
```
`api.ts`: `export interface IssueComment { id; author; authorName; created; updated; body: string }` (not `Comment`, which is a DOM global) and the three fields on `IssueDetail`.

`jira.GetIssueDetail` adds `comment` to `fields`, decodes `{comments, total}` (`author.name` into `Author`, `author.displayName` into `AuthorName`), normalises both timestamps, and pages `GET /rest/api/2/issue/{key}/comment?startAt=N&maxResults=100` through `b.c.Get` (no `core/jira` change) when `total` exceeds the inline count. `ReadDetail` turns a nil `Comments` into `[]` so details cached before this bundle decode safely. Demo: PLAT-412's curated description becomes the f1 Jira markup (h3 Acceptance criteria, two bullets with `*before*` and `_"This code has expired"_`, a `||Code||Discount||` table, a `{code}` block, `Design: [Figma checkout flow|https://www.figma.com/file/demo]`), two comments (R. Anand, wiki, `Blocked on the gateway sandbox, see PLAT-409.` plus a `{quote}`; M. Soto, Markdown, `` Rounding fixed in `discount.ts`: `` plus `- **2 decimals** everywhere`, `updated` later than `created`), one wiki comment on PLAT-401; bodies rekeyed from `PLAT-` to the profile's project key the way links are.

**Tests (Go):**
- Detail with 2 inline comments and `total: 2`: both decoded in order, the paged endpoint is never hit, `CommentTotal == 2`, not truncated.
- Inline 1 of `total: 150`: the fake serves pages of 100; result has 150 in order, not truncated.
- `total: 812`: 500 comments, `CommentTotal == 812`, `CommentsTruncated` true, no page past `startAt=400`.
- Paged read answers 500: the inline comment kept, truncated true, `err == nil`.
- `created: "2026-09-13T10:14:00.000+0000"` becomes `2026-09-13T10:14:00Z`; `"not a date"` stays raw.
- `WriteDetail` then `ReadDetail` keeps comments; a `detail_json` written without a `comments` key reads back with a non-nil empty slice.
- Demo: `Detail("PLAT", "PLAT-412")` has 2 comments, the Markdown one edited; with project `ACME` the body names `ACME-409`.

**Done:** Go tests pass; `wails generate module` adds `backend.Comment` to the models; `tsc` passes against the new `api.ts` fields.
**Gate:** `cd tam; go test ./...` and `cd tam; wails generate module` and `npm run typecheck --workspaces --if-present` (root)
**Commit:** `feat(tam): the issue detail carries its comments, paged to 500 and cached with it`

### Task 6: The detail panel

**Files:** modify `tam/frontend/src/components/EditableFields.tsx`, `IssueDetailPanel.tsx`, `IssueDetailPanel.test.tsx`, `tam/frontend/src/App.css` (comment card and avatar only, on tokens).

**Consumes:** `RichText`, `RichTextField`, `SyntaxToggle`, `parseRich`, `detectFormat` from core; `BrowserOpenURL`; `browseUrl` from `IssueKeyLink.tsx` for `onIssueKey` (`url && BrowserOpenURL(url)`, and no handler when `browseUrl` answers `""`).

**Changes:**
- `EditableFields` description row: read view (`RichText` of `values.description`, `SyntaxToggle`, `Edit` disabled until `descriptionReady`) or, while editing, `RichTextField` with `textarea={{ id: "edit-description", className: "detail-input", disabled: !descriptionReady }}`. The label keeps `htmlFor="edit-description"`. An empty description reads `No description.` Toggle state from the module map.
- Panel heading `<p className="detail-summary">` renders `<RichText inline text={issue.summary} …/>`.
- New `Section title="Comments" count={detail.data?.commentTotal}` after Fields: pending/error states as Links has them; each comment an `<article>` with initials, author name, `formatWhen(created)`, `edited` when `updated !== created`, a `Markdown`/`Jira markup` chip only when its detected format differs from the description's effective one, and its `RichText` body; newest 5 with `Show all <total>`; truncation sentence; `No comments.`

**Tests:**
- Description `h3. Acceptance criteria\n* one` renders a heading and a list, not a textarea; picking Markdown re-renders; closing and reopening the panel on the same issue keeps Markdown, another issue starts on auto.
- Edit opens Write with the raw text; typing and Save edit calls the edit mutation with `field: "description"` and the exact string; Cancel restores the read view with the base text and nothing journaled; Edit is disabled while the detail loads.
- Summary `apply at {{/payment}} step` renders a `code` in the heading.
- Comments: count `2` in the heading; opening shows both authors, `edited` only on the edited one, the Markdown chip only on the Markdown comment; 7 comments show 5 and `Show all 7`, which shows 7; `commentsTruncated` with total 812 shows `Showing 500 of 812 comments. Open in Jira for the rest.`
- A comment link calls `BrowserOpenURL` with its URL; a `javascript:` link has no button.
- Existing description tests that read a textarea via `getByLabelText("Description")` move to Edit first.

**Done:** panel tests pass, no other suite regresses.
**Gate:** `cd tam/frontend; npx vitest run; npm run typecheck`
**Commit:** `feat(tam): the detail panel renders the description, the summary and read-only comments`

### Task 7: Plain summaries everywhere else

**Files:** modify `IssueTable.tsx`, `BoardCard.tsx`, `EpicRow.tsx`, `SprintList.tsx`, `NewIssueModal.tsx` (parent statement, `epicOptionLabel`), `EditableFields.tsx` (`epicOptionLabel`), `IssueDetailPanel.tsx` (link and test rows), `ConflictCard.tsx`, `PendingChangesModal.tsx`, `CompleteSprintModal.tsx`; tests in `BacklogView.test.tsx`, `BoardsView.test.tsx`, `EpicsView.test.tsx`.

**Change:** every displayed summary, its `title` and any `aria-label` built from it go through `toPlainText`. Inputs that edit a summary stay raw.

**Tests:** a row summary `Fix *login* at {{/auth}}` shows cell text, `title` and accessible name `Fix login at /auth` in the Backlog grid, on a board card and in the Epics tree.

**Done:** `grep -n "\.summary}" tam/frontend/src/components/*.tsx` shows only `toPlainText(...)` uses, the detail heading's `RichText`, and summary inputs.
**Gate:** `cd tam/frontend; npx vitest run; npm run typecheck`
**Commit:** `feat(tam): grids, cards, the tree and dialogs show summaries as plain text`

### Task 8: Write and Preview in New issue and More fields

**Files:** modify `NewIssueModal.tsx`, `NewIssueModal.test.tsx`, `MetaField.tsx`, `MetaField.test.tsx`.

**Changes:** New issue Description row becomes `<div className="edit-row">` with `<label htmlFor="new-description">` (a `<label>` wrapping the tab buttons would forward clicks to them) and `RichTextField` (`format` held in a `useState<RichFormat | "auto">("auto")`). `META_INPUTS.textarea` becomes `LongTextInput`: `RichTextField` with `textarea={shared}` and its own format state. Project key for issue keys: `activeProfile?.projectKey`.

**Tests:** typing `h2. Steps` in New issue's Description, Preview shows a heading, submitting drafts `description: "h2. Steps"` exactly; a `textarea` create-meta field renders Write and Preview tabs, keeps `aria-invalid` on its textarea, and its value submits unchanged; `META_INPUTS.textarea` is `LongTextInput`.

**Done:** both suites pass.
**Gate:** `cd tam/frontend; npx vitest run; npm run typecheck; npm run build`
**Commit:** `feat(tam): Write and Preview for New issue's description and long-text fields`

### Task 9: Docs

**Files:** modify `tam/CLAUDE.md`.

Add a section "Rich text" in its own voice: the renderer lives in `@agile-suite/core/richtext` (AST, wiki parser, `marked` lexer, detection by distinct signals with the stacked-`#` rule, tie to Jira markup); React elements only, links are buttons through `isAllowedLink` and `onOpenLink`, issue keys split at render; why not `renderedFields` (server HTML, wrong for Markdown, no offline or draft preview); comments ride `IssueDetail` in the detail cache, paged to 500, truncation as a value; the raw string is always the saved value; the toggle map's lifetime. Add `richtext/` and `lib/links.ts` to Layout. Give the controller a short User Guide paragraph for Outline (Description read view, Edit, the toggle, the Markdown warning, Comments are read-only).

**Gate:** none beyond a read; no em dashes added.
**Commit:** `docs(tam): rich text rendering and comments`

## Verification

1. Gates after the last task: `cd frontend/core; npx vitest run`, `cd tam/frontend; npx vitest run; npm run build`, `npm run typecheck --workspaces --if-present` (root), `cd tam; go test ./...`.
2. `/qa` with gstack browse against `cd tam; wails dev` (dev server `http://localhost:34115`), demo profile, synced:
   - Backlog, select PLAT-412. Heading shows the summary. Fields: Description renders "Acceptance criteria" as a heading, two bullets (**before**, *"This code has expired"*), the Code/Discount table, the POST code block, and "Figma checkout flow" as a link. Screenshot against f1.
   - Toggle Markdown: the text re-renders as Markdown (bold turns italic, table breaks). Close the panel, reopen PLAT-412: still Markdown. Toggle back.
   - Click Edit: Write shows the raw markup with the detected hint naming `h3., ||table||, {code}`. Ctrl+Shift+P: Preview. Pick Markdown: the warning appears. Pick Jira markup, add a line `* third`, Save edit, Commit: the demo commit lands; reopen and see the same raw text in Write.
   - Edit the summary to `Checkout: apply promo code at {{/payment}} step`, Save edit: the panel heading shows `/payment` as code; the Backlog cell, its hover title, the board card and the Epics tree show `Checkout: apply promo code at /payment step`.
   - Open Comments (2): R. Anand with the quote and plain `PLAT-409`; M. Soto with a Markdown chip, `edited`, `discount.ts` as code and a bold bullet. Click the Figma link: no navigation inside the window (console shows no error, the app stays on the Backlog).
   - `+ New`: Description has Write and Preview; type `h2. Steps`, Preview shows a heading; draft it and see the raw text in the draft's detail Write tab.
   - Toggle dark theme: rich text, code blocks, table and warning stay readable.
3. `/review` on the branch diff against `main`, with attention to the link rule, `innerHTML`-free rendering, the paging loop bounds and the `ReadDetail` nil guard.

## Out of scope (from the spec)

- Adding, editing or deleting comments.
- Loading images and attachments inline.
- Converting between wiki markup and Markdown.
- A WYSIWYG editor for Jira fields.
