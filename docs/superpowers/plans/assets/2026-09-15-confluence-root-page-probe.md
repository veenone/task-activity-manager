# Probing Confluence for a missing rituals root page

Six probes against your own Confluence Data Center, twenty minutes. They check the answers the
plan `2026-09-15-tam-02-rituals-toolbar-root-page.md` was written against. **This probe does
not block the plan.** The code ships with the messages written in the plan. What you record here
decides whether a follow-up changes three sentences and one status-code check.

## Before you start

Your personal access tokens go in the `Authorization` header and nowhere else. Do not paste a
token into this file.

You need two tokens: one that can create pages in the space (`PAT`) and one that can only read
it (`RO`). If you have no read-only token, skip the probes marked **RO** and say so.

```bash
CONF=https://confluence.example.com   # base URL, no trailing slash
read -rs PAT                          # a token that can create pages in $SPACE
read -rs RO                           # a token that can read $SPACE but not create in it
SPACE=TEAM                            # the space key from Profile settings
MISSING=653264152                     # the root page id from the ticket, which answers 404
```

Probe 4 creates a page at the top of the space. Delete it afterwards.

---

## Probe 1: is the space readable? (the first half of the permission probe)

```bash
curl -sS -i "$CONF/rest/api/content?spaceKey=$SPACE&limit=1" -H "Authorization: Bearer $PAT"
curl -sS -i "$CONF/rest/api/content?spaceKey=NOSUCHSPACE&limit=1" -H "Authorization: Bearer $PAT"
```

**Assumed:** `200` with a `results` array for `$SPACE`. For a space that does not exist, either
`200` with an empty `results` array or `404`.

**Record:** both status codes and the first 300 characters of each body.

**What it changes.** TAM reads `401`, `403` and `404` here as "cannot create". If a missing space
answers `200` with no results, TAM falls through to Probe 2 and treats the answer as unknown,
which offers the dialog. That is the documented behaviour, so nothing changes unless the code is
something else entirely.

## Probe 2: does the space report the token's operations? (the second half)

```bash
curl -sS "$CONF/rest/api/space/$SPACE?expand=operations" -H "Authorization: Bearer $PAT"
curl -sS "$CONF/rest/api/space/$SPACE?expand=operations" -H "Authorization: Bearer $RO"   # RO
```

**Assumed:** an `operations` array. For `$PAT` it holds `{"operation":"create","targetType":"page"}`.
For `$RO` it holds `read` entries and no create-page entry. It is also possible the instance
ignores the expansion and leaves `operations` out.

**Record:** the `operations` value from both answers, or "not present".

**What it changes.** `confluence.Client.CanCreatePages` answers `yes` when a create-page operation
is listed, `no` when operations are listed without one, and `unknown` when the field is absent. An
unknown answer offers the dialog and lets the create's own `403` speak. If this instance never
reports operations, every token gets the dialog, and a read-only token only learns the truth when
it presses Create. That still works, but note it: the dialog wording may then want to say so.

## Probe 3: what does a page the token cannot see answer?

Pick a page in `$SPACE` that has a view restriction excluding the `$RO` user (or skip this
probe). Call its id `RESTRICTED`.

```bash
curl -sS -i "$CONF/rest/api/content/$MISSING?expand=body.storage,version,ancestors" -H "Authorization: Bearer $PAT"
curl -sS -i "$CONF/rest/api/content/RESTRICTED?expand=body.storage,version,ancestors" -H "Authorization: Bearer $RO"   # RO
```

**Assumed:** `404` with `No content found with id: ContentId{id=653264152}` for the missing page.
The restricted page may also answer `404`, since Confluence hides what a user cannot view.

**Record:** both status codes and messages.

**What it changes.** TAM reads `404` on the root as "root missing" and offers to create one. If a
restricted root also answers `404`, the dialog is offered to a user whose root exists but is
hidden from them. The dialog sentence already says the page "may have been deleted, moved out of
reach, or mistyped". If this answers `403` instead, TAM keeps the existing "could not be read"
refusal for it, and nothing changes.

## Probe 4: create a page at the top of the space (no ancestors)

```bash
curl -sS -i -X POST "$CONF/rest/api/content?expand=body.storage,version,ancestors" \
  -H "Authorization: Bearer $PAT" -H "Content-Type: application/json" \
  -d '{"type":"page","title":"TAM probe root","space":{"key":"'"$SPACE"'"},"body":{"storage":{"representation":"storage","value":"<p>probe</p>"}}}'
```

**Assumed:** `200`. The page has an empty `ancestors` array and sits at the top of the space
(in Confluence 8 it may show under the space home page in the tree, while still having no
ancestor through the API).

**Record:** status code; the `id`; the `ancestors` value; where the page shows in the space tree.

**What it changes.** If Confluence places a page with no ancestors under the space home page
*and* reports that home page as its ancestor, the "top of the space" check in
`ritualsync.CreateRoot` (an adoption is offered only for a page with no ancestors) must also
accept the space home page id as top level. Record the home page id if so.

## Probe 5: create the same top-level title again

Run Probe 4's command a second time, unchanged.

**Assumed:** `400` with a message like `A page with this title already exists: A page already
exists with the title TAM probe root in the space with key TEAM`. It may be `409`.

**Record:** status code and the full message.

**What it changes.** `ritualsync.CreateRoot` treats either `400` or `409` on a create as "maybe the
title is taken" and then looks the title up. It never reads the message. If the code is
something else (say `403` or `500`), the check in `root.go` needs that code added. Then delete
the probe page:

```bash
curl -sS -i -X DELETE "$CONF/rest/api/content/<id from Probe 4>" -H "Authorization: Bearer $PAT"
```

## Probe 6: create with the read-only token  (RO)

Run Probe 4's command with `$RO` in place of `$PAT` and the title `TAM probe root RO`.

**Assumed:** `403` with a message like `Could not create content with type page`.

**Record:** status code and message.

**What it changes.** TAM shows "Your token cannot create pages in TEAM. Ask a space admin, or set
an existing page id in Profile settings." only for `403`. If this answers `401` or `400`, the
forbidden check in `root.go` must include that code, or the user sees the raw Confluence line
instead.

---

## Answers

Fill in below and commit on the branch that carries the plan.

| Probe | Status | What came back |
|---|---|---|
| 1 readable space | | |
| 1 missing space | | |
| 2 PAT operations | | |
| 2 RO operations | | |
| 3 missing id | | |
| 3 restricted id | | |
| 4 top-level create | | |
| 5 duplicate title | | |
| 6 RO create | | |
