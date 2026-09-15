# Probing Jira's create metadata

Five probes against your own Jira Data Center, fifteen minutes, to confirm what TAM's create
dialog now assumes about the two createmeta endpoints. Nothing in the plan waits on them: the
code prefers the per-type endpoint, falls back to the classic one on a 404, and never sends an
extra field the draft's own metadata did not list. The answers decide whether the screen check
needs one more rule.

## Before you start

Your personal access token goes in the `Authorization` header and nowhere else. Do not paste
it into this file.

```bash
JIRA=https://jira.example.com   # base URL, no trailing slash
read -rs PAT                    # paste the token, it will not echo
PROJECT=KEY                     # the project from the ticket that failed with customfield_10253
```

Probes 1 to 4 are reads. Probe 5 attempts one create that is expected to be refused; if it is
not refused, delete the issue it made.

---

## Probe 1: does the per-type endpoint exist, and what are the type ids?

```bash
curl -sS -i "$JIRA/rest/api/2/issue/createmeta/$PROJECT/issuetypes" \
  -H "Authorization: Bearer $PAT"
```

**Assumed:** `200`, a paged body `{"values":[{"id":"...","name":"Story",...},...]}`. Data Center
before 8.4 answers `404`.

**Record:** status code; the `id` of Story and of the sub-task type (named "Technical task" on
the instance in the ticket); whether the body has `isLast`, `total`, or both.

```bash
STORY=10001      # from the answer above
SUBTASK=10003    # from the answer above
```

## Probe 2: the per-type fields of Story

```bash
curl -sS "$JIRA/rest/api/2/issue/createmeta/$PROJECT/issuetypes/$STORY?maxResults=200" \
  -H "Authorization: Bearer $PAT"
```

**Assumed:** a body `{"values":[{"fieldId":"summary","required":true,"schema":{...},...}]}`
that does **not** list `customfield_10253`.

**Record:** is `customfield_10253` listed (yes or no); if yes, its `required`, `hasDefaultValue`
and `operations`; the `schema.custom` of Epic Link and Sprint.

**What it changes.** If `customfield_10253` is listed here too, the per-type answer is not a
screen and `CreateFields` in `tam/internal/backend/jira/writes.go` needs a second rule: leave
out an optional field whose `operations` does not contain `set`. Open a follow-up task; the
draft's stored field ids still keep the field out of the payload.

## Probe 3: the per-type fields of the sub-task type

```bash
curl -sS "$JIRA/rest/api/2/issue/createmeta/$PROJECT/issuetypes/$SUBTASK?maxResults=200" \
  -H "Authorization: Bearer $PAT"
```

**Assumed:** `parent` is listed, `required: true`, `schema.system: "parent"`.

**Record:** whether `parent` is listed and how its `schema` reads.

**What it changes.** Nothing if `parent` is listed with `fieldId: "parent"` or
`schema.system: "parent"`: both are already base fields. Any other shape is added to
`isBaseField`.

## Probe 4: the classic call for the same two types

```bash
curl -sS "$JIRA/rest/api/2/issue/createmeta?projectKeys=$PROJECT&issuetypeIds=$STORY&expand=projects.issuetypes.fields" \
  -H "Authorization: Bearer $PAT"
```

**Assumed:** `customfield_10253` is listed here, which is how the ticket's draft picked it up.

**Record:** is `customfield_10253` listed (yes or no).

## Probe 5: a create carrying the field

```bash
curl -sS -i -X POST "$JIRA/rest/api/2/issue" \
  -H "Authorization: Bearer $PAT" -H "Content-Type: application/json" \
  -d '{"fields":{"project":{"key":"'"$PROJECT"'"},"issuetype":{"id":"'"$STORY"'"},"summary":"TAM createmeta probe","customfield_10253":"x"}}'
```

**Assumed:** `400` with `"customfield_10253":"Field 'customfield_10253' cannot be set. It is
not on the appropriate screen, or unknown."`

**Record:** status code and the `errors` object. If it answered `201`, delete the issue.

## Answers

| probe | answer |
|---|---|
| 1 status / Story id / sub-task id / paging keys | |
| 2 customfield_10253 listed? required / hasDefaultValue / operations | |
| 3 parent listed? schema | |
| 4 customfield_10253 listed in classic? | |
| 5 status and errors | |
