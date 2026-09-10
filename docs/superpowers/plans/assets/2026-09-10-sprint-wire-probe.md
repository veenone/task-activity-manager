# Probing Jira's sprint write calls

Four requests against your own Data Center, ten minutes, before any of the sprint write path is
built on assumptions about them. Three tasks are shaped by the answers, and one of them is a
promise the app makes to a user about their data.

## Before you start

Your PAT goes in the `Authorization` header and nowhere else. Do not paste it into this file, do
not put it in a shell script you commit, and if your shell keeps history, prefer reading it from
an environment variable you set in the current session only.

```bash
JIRA=https://jira.example.com          # your base URL, no trailing slash
read -rs PAT                           # paste the token, it will not echo
BOARD=1                                # a scrum board id in a project you can safely test in
```

Use a throwaway project if you have one. Probe 2 deletes a sprint.

---

## Probe 1: what does create answer with?

```bash
curl -sS -i -X POST "$JIRA/rest/agile/1.0/sprint" \
  -H "Authorization: Bearer $PAT" \
  -H "Content-Type: application/json" \
  -d '{"name":"TAM probe sprint","originBoardId":'"$BOARD"',"startDate":"2026-10-01T09:00:00.000+0000","endDate":"2026-10-15T09:00:00.000+0000","goal":"probing the create endpoint"}'
```

**Assumed:** `200` or `201`, with a JSON body carrying the new sprint's `id`, and its `goal`
echoed back.

**What it changes.** The `id` shapes `CreateSprint`'s signature and everything downstream of it.
The `goal` is the reason for the whole schema change in Task 2: if Jira does not return a goal
here, check whether it returns one on a plain `GET /rest/agile/1.0/board/{id}/sprint`, because
that is the read TAM actually caches from.

**If the body is empty or has no `id`:** nothing breaks. Task 1 already treats that as "created,
the refresh will find it". Note it and move on.

Keep the id it returns. Call it `SPRINT`.

---

## Probe 2: does a delete return the issues, or destroy them?

Put one issue you do not care about into the sprint first, from the Jira UI or with:

```bash
curl -sS -i -X POST "$JIRA/rest/agile/1.0/sprint/$SPRINT/issue" \
  -H "Authorization: Bearer $PAT" -H "Content-Type: application/json" \
  -d '{"issues":["PROJ-123"]}'
```

Then:

```bash
curl -sS -i -X DELETE "$JIRA/rest/agile/1.0/sprint/$SPRINT" \
  -H "Authorization: Bearer $PAT"
```

Now look up `PROJ-123`. Is it still there, with no sprint?

**Assumed:** the sprint is gone and the issue is back in the backlog, unharmed.

**What it changes.** This is not a code shape, it is **a sentence the app tells a user at the one
moment it cannot be undone.** The delete confirmation says "Jira moves its 6 issues back to the
backlog. The issues themselves are not deleted." If that turns out to be false on your instance,
that copy is a lie, and whether delete ships at all goes back on the table. Do not skip this one.

---

## Probe 3: what does a missing permission look like?

Repeat probe 1 as an account without Manage Sprints on that board. A second PAT, or ask a
colleague, or temporarily remove the permission from a test project.

**Assumed:** `403` with a message in the body.

**What it changes.** Only an error message. TAM deliberately does not guess at permissions before
trying, because that hides capability from people who have it, so a 403 is expected and is turned
into one readable line. **What would matter is a `200` that silently does nothing**, because then
TAM would report success for a write that never happened.

---

## Probe 4: does Jira refuse editing a closed sprint?

Find a sprint that is already closed on that board, or close the probe sprint first. Then:

```bash
curl -sS -i -X POST "$JIRA/rest/agile/1.0/sprint/$CLOSED" \
  -H "Authorization: Bearer $PAT" -H "Content-Type: application/json" \
  -d '{"name":"TAM probe rename"}'
```

**Assumed:** refused.

**What it changes.** TAM's edit guard refuses a closed sprint because its dates are what velocity
and burndown are computed from. The guard is currently written to re-read the sprint's state from
Jira before deciding, which is safe whichever way this lands. If Jira refuses closed edits itself,
that re-read is belt and braces and could later be relaxed to a cache read; if Jira **accepts**
them, the guard is the only thing standing between a stale local cache and a silently rewritten
chart, and the re-read stays mandatory forever. Either way, record the answer next to the guard.

---

## Cleaning up

If probe 2 left anything behind, or probe 1's sprint still exists, delete it from the Jira UI.
Nothing here writes to TAM's local database, so there is nothing to clean up on this side.

## Recording the answers

Write what you actually got beside each "Assumed" line above and commit this file. The next reader
of the delete guard, and of the delete confirmation's copy, needs to know whether these were
verified or assumed.
