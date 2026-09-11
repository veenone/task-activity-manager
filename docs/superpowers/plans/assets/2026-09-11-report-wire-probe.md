# Probing the changelog read

Four requests against your own Data Center, before the sprint report is built on assumptions about
them. Three of the reviews' findings resolve to "we guessed" without this, and one of the four
could make the phase better rather than worse.

## Before you start

Your PAT goes in the `Authorization` header and nowhere else. Do not paste it into this file or
into a script you commit; if your shell keeps history, read it into a variable in the current
session only.

```bash
JIRA=https://jira.example.com          # your base URL, no trailing slash
read -rs PAT                           # paste the token, it will not echo
SPRINT=12                              # a closed sprint with a decent number of issues
PROJ=PLAT                              # its project key
```

Everything here is a read. Nothing writes.

---

## Probe 1: does a search truncate an issue's changelog?

```bash
curl -sS "$JIRA/rest/api/2/search?jql=sprint%3D$SPRINT&expand=changelog&maxResults=50" \
  -H "Authorization: Bearer $PAT" \
  | python -c "
import json,sys
d=json.load(sys.stdin)
print('issues returned:', len(d.get('issues',[])), 'of', d.get('total'))
for i in d.get('issues',[]):
    c=i.get('changelog') or {}
    got=len(c.get('histories',[]))
    tot=c.get('total')
    mark=' <-- TRUNCATED' if tot is not None and tot>got else ''
    print(i['key'], 'histories', got, 'of', tot, 'maxResults', c.get('maxResults'), mark)
"
```

**Assumed:** every issue carries a `changelog` object with `startAt`, `maxResults`, `total` and
`histories`, and at least one long lived card shows `total` greater than the histories returned.

**What it changes.** Task 1 Step 2 reads those three numbers so a cut history stops decoding
identically to a complete one, and Task 5 shows a state naming the cards it could not read in full.
**Record the actual cap**, because that number decides how often the state fires. If nothing is
ever truncated on your instance, say so: the handling stays, but it moves from likely to defensive.

---

## Probe 2: what does a Sprint field change actually look like?

This is the one most likely to differ from what the plan assumes, and the reconstruction's
membership rule is built on it.

```bash
curl -sS "$JIRA/rest/api/2/search?jql=sprint%3D$SPRINT&expand=changelog&maxResults=50" \
  -H "Authorization: Bearer $PAT" \
  | python -c "
import json,sys
d=json.load(sys.stdin)
for i in d.get('issues',[]):
    for h in (i.get('changelog') or {}).get('histories',[]):
        for it in h.get('items',[]):
            if it.get('field','').lower()=='sprint' or 'sprint' in (it.get('fieldId') or '').lower():
                print(i['key'], h['created'])
                print('   field   ', repr(it.get('field')), 'fieldId', repr(it.get('fieldId')))
                print('   from    ', repr(it.get('from')), '->', repr(it.get('to')))
                print('   fromStr ', repr(it.get('fromString')), '->', repr(it.get('toString')))
" | head -40
```

**Assumed:** `field` is `Sprint`, `fieldId` is a `customfield_NNNNN`, `from` and `to` are comma
separated lists of sprint **ids**, and `fromString`/`toString` are comma separated **names**.

**What it changes.** Task 1 Step 4 matches the field by the custom field id TAM already discovers
per instance, because a matcher keyed on a literal id would match `status` and silently never match
this one. Task 2 Step 4 treats membership as set difference over those lists rather than a boolean
toggle. **If `from`/`to` turn out to be empty and only the `String` forms are populated**, which
some DC versions do, the membership rule has to match on name and that is worth knowing now rather
than at Task 2.

Also note the `created` format. It should look like `2026-09-09T10:42:00.000+0200`, with **no colon
in the offset**, which is why every timestamp parses through `internal/sprintdate` rather than
`time.Parse(time.RFC3339, ...)`.

---

## Probe 3: can a removed card be seen at all?

The one that could make this phase better. Find a card you know was taken **out** of sprint 12 and
never put back, then:

```bash
# Does the membership query return it?
curl -sS "$JIRA/rest/api/2/search?jql=sprint%3D$SPRINT%20AND%20key%3DPROJ-123" \
  -H "Authorization: Bearer $PAT" | python -c "import json,sys; print('returned:', json.load(sys.stdin).get('total'))"

# Does Jira accept a historical operator on the Sprint field?
curl -sS --get "$JIRA/rest/api/2/search" --data-urlencode "jql=sprint WAS $SPRINT" \
  -H "Authorization: Bearer $PAT" | head -c 400; echo
```

**Assumed:** the first returns 0, and the second is rejected with a JQL error, because `WAS` is
limited to status, assignee, priority, resolution, reporter and fixVersion.

**What it changes.** This is the blind spot the spec's section 3 is written around: removals are
only visible for cards that left and came back, committed is a floor rather than a total, and every
surface showing either says so. **If the second request succeeds**, that section is wrong, the
labels come off, and the phase delivers what it originally promised. Worth the thirty seconds.

---

## Probe 4: what does the expansion cost?

On the largest sprint you have:

```bash
for n in 50 25; do
  echo "maxResults=$n"
  curl -sS -o /dev/null -w "  with changelog:    %{time_total}s  %{size_download} bytes\n" \
    "$JIRA/rest/api/2/search?jql=sprint%3D$SPRINT&expand=changelog&maxResults=$n" \
    -H "Authorization: Bearer $PAT"
  curl -sS -o /dev/null -w "  without changelog: %{time_total}s  %{size_download} bytes\n" \
    "$JIRA/rest/api/2/search?jql=sprint%3D$SPRINT&maxResults=$n" \
    -H "Authorization: Bearer $PAT"
done
```

**Assumed:** the expansion is several times slower and much larger, and 25 is meaningfully better
than 50 per request.

**What it changes.** Task 4 Step 2 sets the page size from this rather than inheriting the sync's
50, and decides how loudly progress needs reporting. If a page takes over a second, opening the
view without progress would read as a freeze.

---

## Recording the answers

Write what you actually got beside each **Assumed** line and commit this file. Probe 2's real shape
and probe 3's answer are the two that change code rather than constants, and the next reader of the
membership rule needs to know whether they were verified or assumed.
