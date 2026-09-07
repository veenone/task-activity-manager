# XTM joins the monorepo: repository convergence design

**Status:** proposed · **Date:** 2026-09-06

## 1. What we're doing

Xray Test Manager still has two homes. The monorepo (`veenone/task-activity-manager`) carries a copy under `xtm/` that has been refactored onto the shared Go core and the shared React package. The original repository (`veenone/xray-testcase-manager`) carries the older single-app layout with no core at all, and a script merges its commits into `xtm/` by hand.

That split defeats the point of the suite. Every XTM change lands twice, the refactor onto core exists only on one side, and the two copies drift a little with each sync. This design ends the split: the original repository, and the Gitea mirror the user pushed yesterday, receive the monorepo itself. From then on there is one codebase, held by three remotes that are all equal.

This is a repository-topology change. No code inside `xtm/`, `core/`, or `tam/` changes.

## 2. Decisions

| Decision | Choice | Why |
|---|---|---|
| What the standalone XTM repository becomes | The monorepo, by fast-forward push | The monorepo's `main` already contains every commit of `xray-testcase-manager`, so the push is a plain fast-forward. No history rewrite, and an existing clone on another PC updates in place. |
| Which repository is canonical | All three: `task-activity-manager`, `xray-testcase-manager`, and Gitea `achmarah/xray-test-manager` | The user's choice. XTM users keep the repository they know, TAM keeps its own, and the home Gitea holds a full copy. The cost is a sync discipline, covered by section 4. |
| Repository names | Unchanged | A rename to `agile-suite` was offered and declined for now. GitHub redirects would make it cheap later. |
| How the remotes stay in step | Local push fan-out plus a sync script | It needs no tokens in CI and no mirror workflow that can fail without anyone noticing. Every push reaches all three remotes, and a script fast-forwards whichever lag after a PR is merged on GitHub. |
| Release tags | Per-app prefix: `xtm/v1.10.0`, later `tam/v0.1.0` | One repository will ship two apps. A prefix says which app a tag belongs to, and the release workflow filters on it. Existing `v1.x` tags stay as history. |
| Open PRs on the old repository | Merge the two ready ones first, then flip | #155 and #154 are green. Merging them and syncing once more means nothing is lost and the sync script retires with a clean last run. |
| The upstream sync script | Deleted after its final run | There is no upstream any more. |

## 3. Repository shape after the move

Nothing in the tree changes. The layout stays as the foundation design describes it:

```
go.work
core/            shared Go module (store, profiles, settings, credentials, Jira transport, journal, importfile)
xtm/             Xray Test Manager, importing core
tam/             Task Activity Manager, importing core
frontend/core/   @agile-suite/core, the shared React package
docs/            specs and plans for the suite
scripts/         sync-remotes.ps1 (new), sync-xtm-upstream.ps1 (removed)
.github/         build and release workflows for the whole repository
```

What changes is where that tree lives: after the flip, `main` on all three remotes points at the same commit, and the tree at the root of `xray-testcase-manager` is this monorepo rather than XTM alone. GitHub renders the root `README.md`, which already describes both apps and points at `xtm/README.md`.

The foundation design (2026-09-04, section 3) said the old repository would be archived with a pointer. That sentence is superseded by this document; a one-line note in the foundation spec says so.

## 4. Keeping three remotes in step

### 4.1 Push fan-out

On each developer PC the `origin` remote fetches from `task-activity-manager` and pushes to all three:

```
[remote "origin"]
    url = git@github.com:veenone/task-activity-manager.git
    pushurl = git@github.com:veenone/task-activity-manager.git
    pushurl = git@github.com:veenone/xray-testcase-manager.git
    pushurl = git@gitea2.myhome.lan:achmarah/xray-test-manager.git
```

With that in place, one `git push` of a branch or a tag lands on every remote. Git pushes to the URLs in order and reports each result, so a remote that is down shows up in the terminal rather than being skipped quietly.

Two named remotes stay alongside `origin` for fetching: `xtm-origin` for `xray-testcase-manager` and `gitea` for the Gitea repository. The sync script uses them, and `gh` can target either GitHub repository through them.

### 4.2 The gap fan-out does not cover

A pull request merged in the GitHub UI creates a merge commit on that one repository only. Until someone pulls it and pushes, the other two remotes are behind. Dependabot PRs merged on either GitHub repository have the same effect.

`scripts/sync-remotes.ps1` closes that gap. Run it after merging a PR on GitHub, or whenever the remotes might disagree.

### 4.3 sync-remotes.ps1

Two modes.

`.\scripts\sync-remotes.ps1 -Setup` writes the fan-out configuration from 4.1 and creates the `xtm-origin` and `gitea` remotes if they are missing. It is idempotent: running it again changes nothing. This replaces a paragraph of manual `git remote` commands in the docs and makes the other PC's setup one command.

`.\scripts\sync-remotes.ps1` (no arguments) synchronises `main` and tags:

1. Refuse to run if tracked files have uncommitted changes.
2. Fetch `main` and all tags from the three remotes.
3. Compare the three `main` tips. If one is a descendant of the other two, it is the newest. If any pair has diverged, stop and print the two commits; the user merges by hand.
4. Push the newest `main` to each remote that is behind it, as `<sha>:refs/heads/main`, never with force.
5. Push tags to any remote missing them. If a tag name exists on two remotes with different commits, stop and report it; tags are never overwritten.
6. Fast-forward the local `main` if it is behind: `git merge --ff-only` when `main` is checked out, `git branch -f main <sha>` otherwise. A checked-out `main` with a dirty tree is left alone, with a message.
7. Print one line per remote: the `main` SHA before and after.

Branches other than `main` are not synchronised. Feature branches go through the fan-out when pushed and are deleted from wherever they were merged; they do not need to agree across remotes.

The script needs the same SSH access the user already has: the `gitea2` key for Gitea on port 2222, configured in `~/.ssh/config`, and the GitHub key.

## 5. Release workflow

`.github/workflows/release.yml` changes in two places:

- The trigger becomes `tags: ["xtm/v*"]` instead of `["v*"]`.
- The version resolution strips `xtm/v` from the tag name instead of `v`. The `workflow_dispatch` input keeps accepting a bare `x.y.z`.

The job bodies, the release name (`Xray Test Manager vX.Y.Z`), the artifact names, and `xtm/scripts/release.ps1` do not change. A TAM release job is a later addition; the prefix convention is what makes room for it.

Because tags go through the fan-out, an `xtm/v1.10.0` tag reaches both GitHub repositories and each runs the release workflow. Two GitHub Releases with the same artifacts is the intended result of keeping both repositories canonical, and it is what keeps the XTM repository's Releases page useful.

`xtm/CLAUDE.md` and `xtm/README.md` change their release instructions from `git tag vX.Y.Z` to `git tag xtm/vX.Y.Z`.

## 6. Order of work

Steps 1 and 2 are the pre-flight on the old repository. Steps 3 to 5 are one PR on the monorepo. Steps 6 and 7 are the flip and the local clean-up.

1. Merge the ready PRs on `xray-testcase-manager`. #155 (About box sizing, profile clone) and #154 (changelog for the Kiwi work). Close the Dependabot PRs #148 and #130; the monorepo's `dependabot.yml` already watches `/xtm` and re-raises anything still relevant. Comment on #66 that its branch will need re-basing under `xtm/` after the flip, and leave it open for the user to decide.
2. Final upstream sync. On a branch off the monorepo `main`, run `scripts/sync-xtm-upstream.ps1`, resolve conflicts (usually `xtm/app.go`), run `go test ./internal/...` in `xtm/` and `npx vitest run` in `xtm/frontend`, commit.
3. Release workflow. Apply section 5.
4. Scripts and docs. Add `scripts/sync-remotes.ps1`, delete `scripts/sync-xtm-upstream.ps1`. In the root `CLAUDE.md` and `README.md`, replace the "Keeping xtm/ in step with its upstream" and "Syncing XTM from upstream" sections with a "Remotes" section that names the three remotes, the fan-out, when to run the sync script, and the tag convention. In `xtm/CLAUDE.md` and `xtm/README.md`, update the tag convention. Add the gstack team snippet the user set up yesterday to the root `CLAUDE.md`, so the monorepo carries it instead of the retired folder. Add the superseding note to the foundation spec.
5. PR and merge into `task-activity-manager`, with `build.yml` green.
6. The flip. From `main` after the merge:
   - `git push xtm-origin main --tags` and `git push gitea main --tags`. Both are fast-forwards; if either is rejected as non-fast-forward, stop, because the other PC pushed something in the meantime.
   - `.\scripts\sync-remotes.ps1 -Setup` on this PC, then `.\scripts\sync-remotes.ps1` as the first real run, which should report all three at the same SHA and push nothing.
   - Remove the `xtm-upstream` remote from this PC.
   - Check that `build.yml` ran and passed on `xray-testcase-manager` for the new `main`.
7. Local clean-up. `C:\projects\xray-test-manager` is retired. Its only meaningful uncommitted change, the gstack section in `CLAUDE.md`, moved in step 4; the rest is line-ending noise in generated files. The user deletes the folder when convenient. On the other PC, `git pull` in the existing `xray-testcase-manager` clone fast-forwards into the monorepo layout; then `npm install` once at the root, `.\scripts\sync-remotes.ps1 -Setup`, and work continues inside `xtm/`.

The user's in-progress branch `feat/plan-1c-write-features` is untouched throughout.

## 7. Verification

- After step 2: XTM's Go suite under `xtm/internal/...` and its Vitest suite pass at the counts they pass on `main` today.
- After step 5: `build.yml` green on the PR.
- After step 6: `git ls-remote <remote> refs/heads/main` prints the same SHA for all three remotes; `git ls-remote --tags` agrees too. `sync-remotes.ps1` reports nothing to push. `build.yml` passes on `xray-testcase-manager`.
- The script is exercised once on purpose: push a throwaway tag to one remote only, run the script, confirm it lands on the other two, delete the tag from all three.
- The release workflow change is exercised on the next real XTM release, `xtm/v1.10.0`. A dry run through `workflow_dispatch` with a version input on `task-activity-manager` confirms the job still resolves the version before that.

## 8. Risks

| Risk | Handling |
|---|---|
| The other PC has unpushed XTM commits on the old layout | The flip is a fast-forward push, so its later push would be rejected as non-fast-forward rather than lost. It pulls first, and any local commit needs re-basing under `xtm/` with `git rebase -X subtree=xtm`. The spec calls this out so it is done knowingly. |
| A remote is unreachable during a push | Git reports it per URL. The next `sync-remotes.ps1` run catches it up. |
| Two remotes diverge because someone merged different PRs on each | The script refuses to guess and prints both commits. The user merges once, pushes through the fan-out, and the script is happy again. |
| The `xtm/v*` filter means a plain `v1.10.0` tag builds nothing | `xtm/CLAUDE.md` and `xtm/README.md` document the new form, and the release script's semver check unchanged means a wrong tag fails loudly at the workflow rather than shipping. |
| Gitea has no CI | Nothing runs there. GitHub remains where checks and releases happen; Gitea is a full copy of the code and history. |

## 9. Out of scope

- A TAM release job.
- GitHub Actions mirroring between the repositories. It can be layered on later without changing anything here.
- Renaming either GitHub repository.
- Any change to code, schema, or build inside `xtm/`, `core/`, or `tam/`.
- Rewriting the old `docs/` history that the subtree merges left under `xtm/`.
