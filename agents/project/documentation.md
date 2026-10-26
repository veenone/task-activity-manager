# Documentation playbook

Read before editing user or developer documentation.

## Scope

- User-visible behavior changes update `docs/user-guide/`.
- New APIs, components, services, or traced paths update `docs/superpowers/specs/`.
- A changed user-visible flow updates its call-flow trace. Add a trace only
  for a main journey not already covered.
- Developer docs teach a competent programmer who does not know this
  stack. Explain a framework mechanism where it first affects behavior.

## Style

- Write and edit with the slop-mop skill. The rules below add what it does
  not cover.
- Write like a developer explaining to a colleague. No headline-style
  headings ("The X", "A deep dive into Y") and no news-article cadence.
- Examples must grep-hit the codebase unless marked simplified. Cite
  symbols, never line numbers; line numbers rot within hours.
- Developer docs cite AGENTS.md rule IDs or AGENTS.project.md contract
  names instead of copying process rules.
- Reports and retrospectives are documentation; every rule above applies.
- A claim cites the commit on the default branch that landed the work, not one
  from the branch it was written on. This repo squash-merges, so a branch
  commit is never on main and resolves only while the branch is kept. Work
  being landed cites its issue or PR number instead, because its own hashes
  will not survive the merge. Existing was the old test and it passed for six
  citations that main did not have (#355). Gate: the instruction gate's hash
  check, which measures reachability from main.
