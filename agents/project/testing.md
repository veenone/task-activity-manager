# Testing playbook

Read before writing tests, UI work, or platform checks.

## Where tests live

| Area | Files | Run |
|---|---|---|
| Go, all three modules | `*_test.go` beside the source | `go test ./... -count=1` from the module |
| Frontend, three workspaces | `*.test.ts`, `*.test.tsx` beside the source | `npm test --workspaces --if-present` from the root |

`xtm` runs `go test ./internal/...` in CI rather than `./...`.

## Counts to beat, never lower

46 in `frontend/core`, 159 in `xtm/frontend`, 507 in `tam/frontend`. A change
that lowers one has deleted coverage; say so in the commit message or put it
back. C6 forbids assertions that cannot fail, so a count kept up by existence
checks is worse than a smaller honest one.

## What has actually caught bugs here

- A test that seeds the pre-change value explicitly, then asserts the
  post-change value. A migration test that lets the new schema create the
  column it is meant to migrate proves nothing; seed the old shape, rewind
  the recorded version, reopen, and assert the conversion ran.
- A regression test run once against the unfixed code. Several fixes in this
  repository were proven by reverting the fix, watching the named test fail
  with the exact symptom, then restoring it.
- Asserting both halves of a deletion: the rows that should go, and a second
  profile's or board's rows that should stay. A purge that deletes everything
  passes a one-sided test.

## Traps in this repository

- `waitFor(expect(select).toBeEnabled())` then selecting an option is a race.
  The control enables on its query's loading flag while the options come from
  that query's data, so there is a window where it is enabled and empty. It
  never opens on a developer's machine and opens on a loaded CI runner. Wait
  for the option, not the control.
- `getByText` finds content inside a collapsed `<details>`. "In the DOM" and
  "visible to a user" are different assertions; use the one you mean.
- A `<dd>` takes no accessible name from its `<dt>`, so
  `getByRole("definition", { name })` never matches. Query the container's
  `aria-label` instead.
- Mock factories that spread the real module and override named exports fail
  open: a binding added later reaches the real implementation and throws far
  from its cause.

## Demo mode

Both apps run without Jira through demo profiles. Demo data must never
fabricate state that only a real operation produces (see
`agents/project/domain-context.md`).
