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
