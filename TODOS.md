# TODOS

Deferred work, with enough context to pick it up cold.

## Render XTM's test descriptions with core RichText

**What:** use `@agile-suite/core`'s `richtext` (added by TAM bundle 06) to render XTM's test descriptions, preconditions and step text, instead of showing raw Jira wiki markup.

**Why:** the renderer was put in `frontend/core` so both apps could use it. XTM shows the same Jira wiki markup as plain text today, so a test whose description carries `h3.`, `{code}` or a table reads as symbols.

**Pros:** one rendering behaviour across the suite; no new code, only wiring. **Cons:** XTM has its own views, fixtures and tests to update, and its own review; it does not belong in the TAM bundle branch.

**Context:** after bundle 06 lands, core exports `RichText`, `parseRich`, `detectFormat`, `toPlainText`, `RichTextField`, `isAllowedLink`. XTM's link rule is its own; check whether it should move onto `core/src/lib/links.ts` too. TAM wires `onOpenLink` to `BrowserOpenURL` and `onIssueKey` to `browseUrl`; XTM needs the same two handlers.

**Depends on:** TAM bundle 06 merged (branch `feat/tam-bundles-01-06`).

**Raised by:** /plan-eng-review of the bundle 06 plan, 2026-09-16.
