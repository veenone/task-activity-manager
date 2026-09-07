# XTM monorepo convergence implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `veenone/xray-testcase-manager` and Gitea `achmarah/xray-test-manager` hold the agile-suite monorepo, keep all three remotes in step with a push fan-out and a sync script, and move XTM releases to `xtm/v*` tags.

**Architecture:** No code under `xtm/`, `core/`, or `tam/` changes. The monorepo's `main` already contains every commit of the standalone XTM repository, so the flip is a fast-forward push. A PowerShell script (`scripts/sync-remotes.ps1`) configures the fan-out and fast-forwards `main` and tags across the remotes; a plain PowerShell test drives it against temporary bare repositories. The release workflow filters on the `xtm/v*` prefix.

**Tech Stack:** git, PowerShell 5.1 (Windows PowerShell, the version on the user's PC), GitHub Actions, `gh` CLI.

**Spec:** `docs/superpowers/specs/2026-09-06-xtm-monorepo-convergence-design.md`.

## Global constraints

- Work happens in the worktree `C:\projects\agile-suite-wt-docs` on branch `docs/xtm-monorepo-convergence` (already created off `main`, already holding the spec commit). The main checkout at `C:\projects\task-activity-manager` is on the user's in-progress branch `feat/plan-1c-write-features` with uncommitted work and must not be touched.
- The three remotes and their URLs, verbatim: `origin` = `git@github.com:veenone/task-activity-manager.git`, `xtm-origin` = `git@github.com:veenone/xray-testcase-manager.git`, `gitea` = `git@gitea2.myhome.lan:achmarah/xray-test-manager.git`. Gitea SSH is on port 2222, already configured in `~/.ssh/config` under `Host 10.88.20.27 gitea2.myhome.lan`.
- Release tags for XTM are `xtm/vX.Y.Z`. Existing `v1.x` tags are left as they are.
- The sync script never uses `--force`, never overwrites a tag, and refuses to run on a dirty tree.
- PowerShell 5.1 rules: no `&&` or `||` chaining, no `?:`, no `??`. Do not redirect a native command's stderr with `2>&1` (5.1 wraps each line in an ErrorRecord). Scripts set `$ErrorActionPreference = "Stop"` only around cmdlets and check `$LASTEXITCODE` after every git call.
- Commit messages use the repository's prefixes (`chore:`, `docs:`, `ci:`, `feat:`, `test:`) with no trailers and no AI attribution or mentions. PR descriptions likewise. Run the `humanizer:humanizer` skill over every piece of prose (docs, PR text, commit bodies, comments in scripts) before it lands.
- No em dashes in any text written by this plan.
- Every commit runs from inside the worktree: `cd C:\projects\agile-suite-wt-docs`.

---

## File structure

| Path | Responsibility |
|---|---|
| `scripts/sync-remotes.ps1` (create) | `-Setup`: write the `origin` push fan-out and the named remotes. Default: fetch `main` and tags from the three remotes into a private ref namespace, pick the newest `main`, fast-forward the laggards, push missing tags, fast-forward local `main`, print a per-remote report. |
| `scripts/sync-remotes.test.ps1` (create) | Plain PowerShell test (no Pester, so it runs on 5.1 and on CI's Pester 5 alike): builds three bare repos and a work clone in a temp directory and exercises every branch of the script. Exit code 1 on any failed assertion. |
| `scripts/sync-xtm-upstream.ps1` (delete) | Retired after its final run in Task 2. |
| `.github/workflows/release.yml` (modify) | Trigger on `xtm/v*`, strip `xtm/v` in both version-resolution steps, header comment. |
| `CLAUDE.md`, `README.md` (modify) | Replace the upstream-sync sections with a "Remotes" section; add the gstack snippet to `CLAUDE.md`. |
| `xtm/CLAUDE.md`, `xtm/README.md` (modify) | Tag convention in the release instructions. |
| `docs/superpowers/specs/2026-09-04-tam-foundation-design.md` (modify) | One-line superseding note in section 3. |

---

### Task 1: Pre-flight on the old XTM repository

**Files:** none in this repository. GitHub state only.

**Interfaces:**
- Produces: `xray-testcase-manager` `main` containing #155 and #154, which Task 2 pulls in.

- [ ] **Step 1: Confirm both PRs are still green and mergeable**

Run:
```powershell
gh pr view 155 -R veenone/xray-testcase-manager --json title,mergeable,statusCheckRollup --jq '{title,mergeable,checks:[.statusCheckRollup[]|.conclusion]}'
gh pr view 154 -R veenone/xray-testcase-manager --json title,mergeable,statusCheckRollup --jq '{title,mergeable,checks:[.statusCheckRollup[]|.conclusion]}'
```
Expected: every `checks` entry is `SUCCESS`; `mergeable` is `MERGEABLE` (GitHub sometimes reports `UNKNOWN` for a minute after a fetch; re-run once if so). If either PR reports `CONFLICTING`, stop and tell the user; do not resolve it here.

- [ ] **Step 2: Merge #154 first, then #155**

The repository's history uses squash merges with the PR number in the title (`fix(ui): ... (#151)`), so:
```powershell
gh pr merge 154 -R veenone/xray-testcase-manager --squash --delete-branch
gh pr merge 155 -R veenone/xray-testcase-manager --squash --delete-branch
```
Expected: each prints `✓ Squashed and merged pull request`. If #155 shows a conflict after #154 landed, stop and tell the user.

- [ ] **Step 3: Close the Dependabot PRs with a pointer**

```powershell
$note = "Closing: this repository is about to become the agile-suite monorepo, whose dependabot.yml watches /xtm and will raise this update again if it is still needed."
gh pr close 148 -R veenone/xray-testcase-manager --comment $note
gh pr close 130 -R veenone/xray-testcase-manager --comment $note
```
Expected: `✓ Closed pull request #148`, `✓ Closed pull request #130`.

- [ ] **Step 4: Comment on #66 and leave it open**

```powershell
gh pr comment 66 -R veenone/xray-testcase-manager --body "Heads-up: main is about to become the agile-suite monorepo, with XTM under xtm/. This branch will show conflicts afterwards. To carry it over, rebase it onto the new main with git rebase -X subtree=xtm origin/main, or close it if the work is not wanted any more."
```
Expected: a comment URL is printed.

- [ ] **Step 5: Record the new upstream tip**

```powershell
gh api repos/veenone/xray-testcase-manager/commits/main --jq '.sha[0:7] + " " + .commit.message' 
```
Expected: the short SHA and the squash title of #155. Note the SHA; Task 2 checks that it is in the merge.

---

### Task 2: Final upstream sync into the monorepo

**Files:**
- Modify: whatever the subtree merge touches under `xtm/` (typically `xtm/app.go`, `xtm/CHANGELOG.md`, and the files #155 changed).

**Interfaces:**
- Consumes: `xray-testcase-manager` `main` from Task 1.
- Produces: monorepo branch `docs/xtm-monorepo-convergence` with `xtm/` at parity with upstream. The `xtm-upstream` remote exists in the shared git config of the repository; the worktree sees it.

- [ ] **Step 1: Install frontend dependencies in the worktree**

The worktree has no `node_modules`; XTM's Vitest suite needs the root install.
```powershell
cd C:\projects\agile-suite-wt-docs
npm ci
```
Expected: ends with `added N packages` and no `ERR!` lines.

- [ ] **Step 2: Run the upstream sync script**

```powershell
cd C:\projects\agile-suite-wt-docs
git status --porcelain --untracked-files=no
.\scripts\sync-xtm-upstream.ps1
```
Expected: the first command prints nothing. The script prints either `Merged without conflicts. Nothing is committed yet.` or `The merge stopped on conflicts.` followed by the check command line. If it warns that upstream changed `docs/` or `.github/`, run `git status --short xtm/docs xtm/.github` and move those files to the root `docs/` or `.github/` with `git mv`, then `git rm -r --cached xtm/docs` as needed.

- [ ] **Step 3: Resolve conflicts, if any**

```powershell
git diff --name-only --diff-filter=U
```
For each file listed, open it, keep both the upstream change and the monorepo's core wiring (in `xtm/app.go` the monorepo side replaces `profile.` and `settings.` constructors with the `core` equivalents; keep those and take upstream's new logic around them), then `git add <file>`. When the list is empty, continue.

- [ ] **Step 4: Run XTM's suites**

```powershell
cd C:\projects\agile-suite-wt-docs\xtm
go build ./...
go test ./internal/...
cd frontend
npx vitest run
npx tsc --noEmit
```
Expected: `go build` silent; `go test` prints `ok` for every package; Vitest prints `Test Files N passed` with no failures; `tsc` prints nothing.

- [ ] **Step 5: Confirm the upstream tip is in the merge and commit**

```powershell
cd C:\projects\agile-suite-wt-docs
git commit -m "chore: sync xtm/ with xray-testcase-manager through #155"
git merge-base --is-ancestor xtm-upstream/main HEAD; if ($LASTEXITCODE -eq 0) { "upstream tip is in HEAD" } else { "NOT in HEAD, stop" }
```
Expected: `upstream tip is in HEAD`.

---

### Task 3: Release workflow on `xtm/v*` tags

**Files:**
- Modify: `.github/workflows/release.yml` lines 3-9, 51-58, 103-110
- Modify: `xtm/CLAUDE.md` line 174
- Modify: `xtm/README.md` lines 98-106

**Interfaces:**
- Produces: the tag convention Task 5's docs describe.

- [ ] **Step 1: Prove the two strip expressions locally**

```powershell
"xtm/v1.10.0" -replace '^xtm/v', ''
bash -c 'r="xtm/v1.10.0"; echo "${r#xtm/v}"'
```
Expected: both print `1.10.0`.

- [ ] **Step 2: Edit the workflow**

Replace the header comment and trigger (lines 3-9):
```yaml
# Tag an XTM release to build and publish Windows + macOS artifacts:
#   git tag xtm/v1.10.0 && git push origin xtm/v1.10.0
# The two jobs upload to the same GitHub Release for the tag. The xtm/ prefix
# is what keeps this workflow from firing on tags for the other apps in the
# repository.

on:
  push:
    tags: ["xtm/v*"]
```

In `release-windows`, replace the `TrimStart` line (line 55):
```yaml
            "${{ github.ref_name }}" -replace '^xtm/v', ''
```

In `release-macos`, replace the shell strip (line 107):
```yaml
            v="${GITHUB_REF_NAME#xtm/v}"
```

- [ ] **Step 3: Check the file still parses as YAML**

```powershell
cd C:\projects\agile-suite-wt-docs
Test-Path node_modules\yaml
node -e "const y=require('yaml');const d=y.parse(require('fs').readFileSync('.github/workflows/release.yml','utf8'));console.log(JSON.stringify(d.on.push.tags))"
```
Expected: `True`, then `["xtm/v*"]`. If `Test-Path` prints `False` (the `yaml` package is not hoisted into the root `node_modules`), install nothing; run `git diff .github/workflows/release.yml` instead and confirm exactly three hunks: the header and trigger, the pwsh strip, the bash strip.

- [ ] **Step 4: Update XTM's release docs**

`xtm/CLAUDE.md` line 174, replace:
```
(`-NoInstaller` skips the installer). Pushing a `vX.Y.Z` tag triggers
```
with:
```
(`-NoInstaller` skips the installer). Pushing an `xtm/vX.Y.Z` tag triggers
```

`xtm/README.md` lines 98-106, replace:
```
**Cut a GitHub release** — push a tag and CI (`.github/workflows/release.yml`)
runs two jobs: `release-windows` (on `windows-latest`) builds the installer and
portable exe, and `release-macos` (on `macos-latest`) builds the universal `.app`.
Both publish to the same GitHub Release. (`scripts/release.ps1` is Windows-only;
the macOS `.app` is built directly with `wails build`.)

```powershell
git tag v0.2.0
git push origin v0.2.0
```
```
with:
```
**Cut a GitHub release**: push an `xtm/v*` tag and CI (`.github/workflows/release.yml`
at the repository root) runs two jobs: `release-windows` (on `windows-latest`)
builds the installer and portable exe, and `release-macos` (on `macos-latest`)
builds the universal `.app`. Both publish to the same GitHub Release. The
prefix says which app the tag belongs to; a bare `v1.10.0` builds nothing.
(`scripts/release.ps1` is Windows-only; the macOS `.app` is built directly with
`wails build`.)

```powershell
git tag xtm/v1.10.0
git push origin xtm/v1.10.0
```
```

- [ ] **Step 5: Commit**

```powershell
git add .github/workflows/release.yml xtm/CLAUDE.md xtm/README.md
git commit -m "ci: release XTM from xtm/v* tags

One repository now ships two apps, so the tag carries the app name and
the release workflow filters on it. Existing v1.x tags stay as history."
```

---

### Task 4: `scripts/sync-remotes.ps1` with its test

**Files:**
- Create: `scripts/sync-remotes.ps1`
- Create: `scripts/sync-remotes.test.ps1`

**Interfaces:**
- Produces: `sync-remotes.ps1 [-Setup] [-OriginUrl <url>] [-XtmUrl <url>] [-GiteaUrl <url>] [-Branch main]`. Exit 0 on success, 1 on a refusal (dirty tree, diverged mains, conflicting tag), 2 on a git failure. Task 5's docs and Task 7's flip use the two modes.

- [ ] **Step 1: Write the failing test**

Create `scripts/sync-remotes.test.ps1`:

```powershell
<#
Exercises scripts/sync-remotes.ps1 against three bare repositories in a temp
directory. Plain PowerShell on purpose: Windows PowerShell 5.1 ships Pester 3
and CI runners ship Pester 5, and the two assertion syntaxes are not
compatible. Run:

  .\scripts\sync-remotes.test.ps1

Exit code 0 when every check passes, 1 otherwise.
#>
$ErrorActionPreference = "Stop"
$script = Join-Path $PSScriptRoot "sync-remotes.ps1"
$failures = 0
$roots = @()

function Check([bool]$cond, [string]$what) {
    if ($cond) { Write-Host "  ok   $what" } else { Write-Host "  FAIL $what"; $script:failures++ }
}

function Invoke-Git([string]$dir, [string[]]$gitArgs) {
    $out = & git -C $dir @gitArgs
    if ($LASTEXITCODE -ne 0) { throw "git -C $dir $($gitArgs -join ' ') failed" }
    return $out
}

function New-Fixture {
    $root = Join-Path ([IO.Path]::GetTempPath()) ("sync-remotes-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory $root | Out-Null
    $script:roots += $root
    foreach ($n in "origin", "xtm", "gitea") { Invoke-Git $root @("init", "-q", "--bare", "-b", "main", $n) | Out-Null }
    $work = Join-Path $root "work"
    Invoke-Git $root @("init", "-q", "-b", "main", "work") | Out-Null
    Invoke-Git $work @("config", "user.email", "t@example.com") | Out-Null
    Invoke-Git $work @("config", "user.name", "t") | Out-Null
    Set-Content (Join-Path $work "a.txt") "a"
    Invoke-Git $work @("add", "a.txt") | Out-Null
    Invoke-Git $work @("commit", "-q", "-m", "A") | Out-Null
    Invoke-Git $work @("remote", "add", "origin", (Join-Path $root "origin")) | Out-Null
    Invoke-Git $work @("remote", "add", "xtm-origin", (Join-Path $root "xtm")) | Out-Null
    Invoke-Git $work @("remote", "add", "gitea", (Join-Path $root "gitea")) | Out-Null
    foreach ($r in "origin", "xtm-origin", "gitea") { Invoke-Git $work @("push", "-q", $r, "main") | Out-Null }
    return @{ Root = $root; Work = $work }
}

function Tip([string]$bare) { return (Invoke-Git $bare @("rev-parse", "main")) }

function Run-Script([string]$work, [hashtable]$scriptArgs = @{}) {
    Push-Location $work
    try {
        & $script @scriptArgs | Out-Host
        return $LASTEXITCODE
    } finally { Pop-Location }
}

Write-Host "1. fast-forwards the laggards and spreads a tag"
$f = New-Fixture
Set-Content (Join-Path $f.Work "b.txt") "b"
Invoke-Git $f.Work @("add", "b.txt") | Out-Null
Invoke-Git $f.Work @("commit", "-q", "-m", "B") | Out-Null
Invoke-Git $f.Work @("push", "-q", "origin", "main") | Out-Null
Invoke-Git $f.Work @("tag", "-a", "t1", "-m", "t1", "HEAD~1") | Out-Null
Invoke-Git $f.Work @("push", "-q", "gitea", "t1") | Out-Null
Invoke-Git $f.Work @("tag", "-d", "t1") | Out-Null
$b = Invoke-Git $f.Work @("rev-parse", "HEAD")
$code = Run-Script $f.Work
Check ($code -eq 0) "exit code 0"
Check ((Tip (Join-Path $f.Root "xtm")) -eq $b) "xtm main moved to B"
Check ((Tip (Join-Path $f.Root "gitea")) -eq $b) "gitea main moved to B"
$originTags = Invoke-Git (Join-Path $f.Root "origin") @("tag")
$xtmTags = Invoke-Git (Join-Path $f.Root "xtm") @("tag")
Check (($originTags -contains "t1") -and ($xtmTags -contains "t1")) "t1 pushed to origin and xtm"
$leftover = @(Invoke-Git $f.Work @("for-each-ref", "refs/synctags/"))
Check ($leftover.Count -eq 0) "private ref namespace cleaned up"

Write-Host "2. refuses when mains have diverged and pushes nothing"
$f = New-Fixture
$a = Invoke-Git $f.Work @("rev-parse", "HEAD")
Set-Content (Join-Path $f.Work "b.txt") "b"
Invoke-Git $f.Work @("add", "b.txt") | Out-Null
Invoke-Git $f.Work @("commit", "-q", "-m", "B") | Out-Null
Invoke-Git $f.Work @("push", "-q", "origin", "main") | Out-Null
$b = Invoke-Git $f.Work @("rev-parse", "HEAD")
Invoke-Git $f.Work @("checkout", "-q", "-b", "other", $a) | Out-Null
Set-Content (Join-Path $f.Work "d.txt") "d"
Invoke-Git $f.Work @("add", "d.txt") | Out-Null
Invoke-Git $f.Work @("commit", "-q", "-m", "D") | Out-Null
Invoke-Git $f.Work @("push", "-q", "xtm-origin", "other:main") | Out-Null
$d = Invoke-Git $f.Work @("rev-parse", "HEAD")
Invoke-Git $f.Work @("checkout", "-q", "main") | Out-Null
$code = Run-Script $f.Work
Check ($code -eq 1) "exit code 1 on divergence"
Check ((Tip (Join-Path $f.Root "origin")) -eq $b) "origin untouched"
Check ((Tip (Join-Path $f.Root "xtm")) -eq $d) "xtm untouched"
Check ((Tip (Join-Path $f.Root "gitea")) -eq $a) "gitea untouched"

Write-Host "3. refuses when the same tag points at different commits"
$f = New-Fixture
Invoke-Git $f.Work @("tag", "t2") | Out-Null
Invoke-Git $f.Work @("push", "-q", "origin", "t2") | Out-Null
Invoke-Git $f.Work @("tag", "-d", "t2") | Out-Null
Set-Content (Join-Path $f.Work "b.txt") "b"
Invoke-Git $f.Work @("add", "b.txt") | Out-Null
Invoke-Git $f.Work @("commit", "-q", "-m", "B") | Out-Null
Invoke-Git $f.Work @("tag", "t2") | Out-Null
Invoke-Git $f.Work @("push", "-q", "xtm-origin", "t2") | Out-Null
Invoke-Git $f.Work @("tag", "-d", "t2") | Out-Null
$code = Run-Script $f.Work
Check ($code -eq 1) "exit code 1 on tag conflict"
$giteaTags = Invoke-Git (Join-Path $f.Root "gitea") @("tag")
Check (-not ($giteaTags -contains "t2")) "t2 not pushed to gitea"

Write-Host "4. refuses on a dirty tree"
$f = New-Fixture
Set-Content (Join-Path $f.Work "a.txt") "changed"
$code = Run-Script $f.Work
Check ($code -eq 1) "exit code 1 on dirty tree"

Write-Host "5. -Setup writes the fan-out and is idempotent"
$f = New-Fixture
Invoke-Git $f.Work @("remote", "remove", "xtm-origin") | Out-Null
Invoke-Git $f.Work @("remote", "remove", "gitea") | Out-Null
$setupArgs = @{ Setup = $true; OriginUrl = (Join-Path $f.Root "origin"); XtmUrl = (Join-Path $f.Root "xtm"); GiteaUrl = (Join-Path $f.Root "gitea") }
$code = Run-Script $f.Work $setupArgs
Check ($code -eq 0) "exit code 0"
$push = @(Invoke-Git $f.Work @("config", "--get-all", "remote.origin.pushurl"))
Check ($push.Count -eq 3) "three push urls"
Check ($push[1] -eq (Join-Path $f.Root "xtm")) "second push url is xtm"
$remotes = @(Invoke-Git $f.Work @("remote"))
Check (($remotes -contains "xtm-origin") -and ($remotes -contains "gitea")) "named remotes created"
$code = Run-Script $f.Work $setupArgs
$push = @(Invoke-Git $f.Work @("config", "--get-all", "remote.origin.pushurl"))
Check ($push.Count -eq 3) "still three push urls after a second run"

Write-Host "6. fast-forwards a checked-out local main"
$f = New-Fixture
$a = Invoke-Git $f.Work @("rev-parse", "HEAD")
Invoke-Git $f.Work @("checkout", "-q", "-b", "side") | Out-Null
Set-Content (Join-Path $f.Work "b.txt") "b"
Invoke-Git $f.Work @("add", "b.txt") | Out-Null
Invoke-Git $f.Work @("commit", "-q", "-m", "B") | Out-Null
Invoke-Git $f.Work @("push", "-q", "origin", "side:main") | Out-Null
$b = Invoke-Git $f.Work @("rev-parse", "HEAD")
Invoke-Git $f.Work @("checkout", "-q", "main") | Out-Null
$code = Run-Script $f.Work
Check ($code -eq 0) "exit code 0"
Check ((Invoke-Git $f.Work @("rev-parse", "main")) -eq $b) "local main fast-forwarded"

foreach ($r in $roots) { Remove-Item -Recurse -Force $r -ErrorAction SilentlyContinue }
if ($failures -eq 0) { Write-Host "all checks passed"; exit 0 }
Write-Host "$failures check(s) failed"
exit 1
```

- [ ] **Step 2: Run it and watch it fail**

```powershell
cd C:\projects\agile-suite-wt-docs
.\scripts\sync-remotes.test.ps1
```
Expected: the first section prints `FAIL` lines (the script does not exist, so `& $script` throws inside `Run-Script`) or the run terminates with `The term '...sync-remotes.ps1' is not recognized`. Either way it does not print `all checks passed`.

- [ ] **Step 3: Write the script**

Create `scripts/sync-remotes.ps1`:

```powershell
<#
Keeps main and tags in step across the three remotes that hold this repository.

  .\scripts\sync-remotes.ps1 -Setup   write the push fan-out on origin and create the named remotes
  .\scripts\sync-remotes.ps1          fast-forward main and tags across the remotes

The named remotes are origin (task-activity-manager), xtm-origin
(xray-testcase-manager) and gitea. After -Setup, a plain git push of a branch
or a tag reaches all three. Run the script without arguments after merging a
pull request on GitHub, which lands on one remote only.

Nothing here forces a push or overwrites a tag. When two remotes disagree in a
way that is not a fast-forward, the script prints both commits and exits 1.
Exit code 2 means a git command failed.
#>
param(
    [switch]$Setup,
    [string]$OriginUrl = "git@github.com:veenone/task-activity-manager.git",
    [string]$XtmUrl = "git@github.com:veenone/xray-testcase-manager.git",
    [string]$GiteaUrl = "git@gitea2.myhome.lan:achmarah/xray-test-manager.git",
    [string]$Branch = "main"
)

$remoteUrls = [ordered]@{ "origin" = $OriginUrl; "xtm-origin" = $XtmUrl; "gitea" = $GiteaUrl }
$names = @($remoteUrls.Keys)
$syncNs = "refs/synctags"

function Invoke-Git([string[]]$gitArgs) {
    # stdout is returned; stderr stays on the console so fetch/push progress and
    # errors are visible. 5.1 must not redirect a native command's stderr.
    $out = & git @gitArgs
    if ($LASTEXITCODE -ne 0) {
        Write-Host "git $($gitArgs -join ' ') failed"
        Clear-SyncRefs
        exit 2
    }
    return $out
}

function Fail([string]$msg) {
    Write-Host $msg
    Clear-SyncRefs
    exit 1
}

# Every exit path goes through here, so a failed run never leaves fetched
# refs behind under refs/synctags.
function Clear-SyncRefs {
    $refs = @(& git for-each-ref --format="%(refname)" "$syncNs/")
    foreach ($r in $refs) { & git update-ref -d $r | Out-Null }
}

if ($Setup) {
    $existing = @(Invoke-Git @("remote"))
    foreach ($n in $names) {
        if ($existing -contains $n) {
            Invoke-Git @("remote", "set-url", $n, $remoteUrls[$n]) | Out-Null
        } else {
            Invoke-Git @("remote", "add", $n, $remoteUrls[$n]) | Out-Null
        }
    }
    # Rewrite the push list from scratch so a second run leaves exactly three.
    # --unset-all exits 5 when the key does not exist yet, which is the normal
    # first run, so this call is the one git call whose exit code is ignored.
    & git config --unset-all remote.origin.pushurl | Out-Null
    foreach ($n in $names) { Invoke-Git @("config", "--add", "remote.origin.pushurl", $remoteUrls[$n]) | Out-Null }
    Write-Host "origin fetches from $OriginUrl and pushes to:"
    foreach ($n in $names) { Write-Host "  $($remoteUrls[$n])" }
    exit 0
}

# 1. Clean tree only. A merge or a stash in flight would make the local
#    fast-forward below ambiguous.
$dirty = @(Invoke-Git @("status", "--porcelain", "--untracked-files=no"))
if ($dirty.Count -gt 0) { Fail "The working tree has uncommitted changes. Commit or stash them first." }

$existing = @(Invoke-Git @("remote"))
foreach ($n in $names) {
    if ($existing -notcontains $n) { Fail "Remote '$n' is missing. Run .\scripts\sync-remotes.ps1 -Setup first." }
}

# 2. Fetch main and every tag from each remote into a private namespace, so
#    local tags are not touched and a tag that differs between remotes does
#    not make git refuse the fetch.
Clear-SyncRefs
$before = [ordered]@{}
foreach ($n in $names) {
    Invoke-Git @("fetch", "--quiet", "--no-tags", $n,
        "+refs/heads/${Branch}:refs/remotes/$n/$Branch",
        "+refs/tags/*:$syncNs/$n/*") | Out-Null
    $before[$n] = (& git rev-parse --verify --quiet "refs/remotes/$n/$Branch")
    if (-not $before[$n]) { Fail "Remote '$n' has no branch '$Branch'." }
}

# 3. The newest main is the one every other main is an ancestor of.
$newest = $null
foreach ($c in $names) {
    $ok = $true
    foreach ($o in $names) {
        if ($o -eq $c) { continue }
        & git merge-base --is-ancestor $before[$o] $before[$c] | Out-Null
        if ($LASTEXITCODE -ne 0) { $ok = $false; break }
    }
    if ($ok) { $newest = $c; break }
}
if (-not $newest) {
    Write-Host "The remotes have diverged on '$Branch'; merge by hand, push, and run again:"
    foreach ($n in $names) { Write-Host ("  {0,-11} {1}" -f $n, $before[$n]) }
    Clear-SyncRefs
    exit 1
}
$newestSha = $before[$newest]

# 4. Fast-forward every remote that is behind. A SHA to a full ref name is
#    an ordinary push, so a non-fast-forward is rejected by the remote.
foreach ($n in $names) {
    if ($before[$n] -ne $newestSha) {
        Invoke-Git @("push", "--quiet", $n, "${newestSha}:refs/heads/$Branch") | Out-Null
    }
}

# 5. Tags: push each one to the remotes that lack it. Same name, different
#    commit across remotes is a stop, never an overwrite.
$tags = @{}
foreach ($n in $names) {
    $lines = @(& git for-each-ref --format="%(refname:strip=3) %(objectname)" "$syncNs/$n/")
    foreach ($line in $lines) {
        $parts = $line -split " ", 2
        if (-not $tags.ContainsKey($parts[0])) { $tags[$parts[0]] = @{} }
        $tags[$parts[0]][$n] = $parts[1]
    }
}
$conflicts = @()
foreach ($t in $tags.Keys) {
    $shas = @($tags[$t].Values | Select-Object -Unique)
    if ($shas.Count -gt 1) { $conflicts += $t }
}
if ($conflicts.Count -gt 0) {
    Write-Host "These tags point at different commits on different remotes; fix them by hand:"
    foreach ($t in $conflicts) {
        foreach ($n in $names) {
            if ($tags[$t].ContainsKey($n)) { Write-Host ("  {0} {1,-11} {2}" -f $t, $n, $tags[$t][$n]) }
        }
    }
    Clear-SyncRefs
    exit 1
}
foreach ($t in $tags.Keys) {
    $source = @($tags[$t].Keys)[0]
    foreach ($n in $names) {
        if (-not $tags[$t].ContainsKey($n)) {
            Invoke-Git @("push", "--quiet", $n, "${syncNs}/${source}/${t}:refs/tags/$t") | Out-Null
        }
    }
}

# 6. Bring the local branch up as well, when that is a fast-forward.
$local = (& git rev-parse --verify --quiet "refs/heads/$Branch")
if ($local -and ($local -ne $newestSha)) {
    & git merge-base --is-ancestor $local $newestSha | Out-Null
    if ($LASTEXITCODE -eq 0) {
        $current = (& git rev-parse --abbrev-ref HEAD)
        if ($current -eq $Branch) {
            Invoke-Git @("merge", "--ff-only", "--quiet", $newestSha) | Out-Null
        } else {
            Invoke-Git @("branch", "-f", $Branch, $newestSha) | Out-Null
        }
        Write-Host "local $Branch fast-forwarded to $($newestSha.Substring(0,7))"
    } else {
        Write-Host "local $Branch has commits the remotes do not; left alone"
    }
}

# 7. Report.
Clear-SyncRefs
foreach ($n in $names) {
    $after = $newestSha
    $mark = if ($before[$n] -eq $after) { "unchanged" } else { "pushed" }
    Write-Host ("  {0,-11} {1} -> {2}  {3}" -f $n, $before[$n].Substring(0,7), $after.Substring(0,7), $mark)
}
exit 0
```

- [ ] **Step 4: Run the test and make it pass**

```powershell
cd C:\projects\agile-suite-wt-docs
.\scripts\sync-remotes.test.ps1
```
Expected: six numbered sections, every line `ok`, then `all checks passed`, exit code 0 (`$LASTEXITCODE` is 0). If section 1 fails on "private ref namespace cleaned up", check that `Clear-SyncRefs` runs on every exit path. If section 3 pushes `t2` to gitea, the conflict check runs after the push loop; it must run before.

- [ ] **Step 5: Run the humanizer over the two scripts' comments, then commit**

Invoke `humanizer:humanizer` on `scripts/sync-remotes.ps1` and `scripts/sync-remotes.test.ps1` (comments only; leave code untouched). Then:
```powershell
git add scripts/sync-remotes.ps1 scripts/sync-remotes.test.ps1
git commit -m "feat: sync-remotes script keeps main and tags equal across the three remotes

-Setup writes the origin push fan-out and the named remotes. The default
mode fetches into a private ref namespace, fast-forwards whichever remote
is behind, spreads missing tags, and stops on any divergence. The test
drives it against bare repositories in a temp directory."
```

---

### Task 5: Docs and the retired script

**Files:**
- Modify: `CLAUDE.md` lines 21-30 (section "Keeping xtm/ in step with its upstream"), append gstack section at the end
- Modify: `README.md` lines 28-33 (section "Syncing XTM from upstream")
- Modify: `docs/superpowers/specs/2026-09-04-tam-foundation-design.md` line 52
- Delete: `scripts/sync-xtm-upstream.ps1`

**Interfaces:**
- Consumes: the script from Task 4 and the tag convention from Task 3.

- [ ] **Step 1: Replace the upstream section in `CLAUDE.md`**

Replace lines 21-30 (from `## Keeping xtm/ in step with its upstream` through the paragraph ending `the script warns when that happens.`) with:

```markdown
## Remotes

Three remotes hold this repository and all three are equal:
`origin` (github.com/veenone/task-activity-manager), `xtm-origin`
(github.com/veenone/xray-testcase-manager, XTM's original home) and `gitea`
(the home Gitea, `achmarah/xray-test-manager`). `origin` pushes to all three,
so a plain `git push` lands everywhere; `.\scripts\sync-remotes.ps1 -Setup`
writes that configuration on a fresh clone.

A pull request merged on GitHub lands on that one remote only. Run
`.\scripts\sync-remotes.ps1` afterwards: it fast-forwards `main` and tags on
whichever remotes are behind and stops, without pushing, if two remotes have
diverged. Feature branches are not synchronised; they are pushed through the
fan-out and deleted where they were merged.

Release tags carry the app name: `xtm/v1.10.0` for XTM, `tam/v0.1.0` for TAM
once it ships. The release workflow filters on `xtm/v*`.
```

- [ ] **Step 2: Append the gstack section to `CLAUDE.md`**

Append at the end of the file, after the "Frontends" section:

```markdown

## gstack (recommended)

This project uses [gstack](https://github.com/garrytan/gstack) for AI-assisted workflows.
Install it for the best experience:

```bash
git clone --depth 1 https://github.com/garrytan/gstack.git ~/.claude/skills/gstack
cd ~/.claude/skills/gstack && ./setup --team
```

Skills like /qa, /ship, /review, /investigate, and /browse become available after install.
Use /browse for all web browsing. Use ~/.claude/skills/gstack/... for gstack file paths.
```

- [ ] **Step 3: Replace the upstream section in `README.md`**

Replace lines 28-33 (from `## Syncing XTM from upstream` through `before committing.`) with:

```markdown
## Remotes

The repository lives on three remotes, all equal: `veenone/task-activity-manager`
and `veenone/xray-testcase-manager` on GitHub, and `achmarah/xray-test-manager`
on the home Gitea. On a fresh clone, run `.\scripts\sync-remotes.ps1 -Setup`
once; from then on `git push` reaches all three. After merging a pull request
on GitHub, run `.\scripts\sync-remotes.ps1` to fast-forward the others.

XTM releases are tagged `xtm/vX.Y.Z` (see `xtm/README.md`).
```

- [ ] **Step 4: Note the change in the foundation spec**

`docs/superpowers/specs/2026-09-04-tam-foundation-design.md` line 52, replace:
```
The monorepo is hosted at `github.com/veenone/task-activity-manager`. The `agile-suite/` name in the layout above is the conceptual root; the GitHub repository can be renamed to match later, and the old `xray-testcase-manager` repository is archived with a pointer once the suite lands here.
```
with:
```
The monorepo is hosted at `github.com/veenone/task-activity-manager`. The `agile-suite/` name in the layout above is the conceptual root; the GitHub repository can be renamed to match later. (Superseded on 2026-09-06: the old `xray-testcase-manager` repository is not archived; it holds the monorepo too. See `2026-09-06-xtm-monorepo-convergence-design.md`.)
```

- [ ] **Step 5: Delete the retired script**

```powershell
cd C:\projects\agile-suite-wt-docs
git rm scripts/sync-xtm-upstream.ps1
Select-String -Path CLAUDE.md, README.md, xtm/CLAUDE.md, xtm/README.md -Pattern "sync-xtm-upstream|xtm-upstream"
```
Expected: `git rm` prints `rm 'scripts/sync-xtm-upstream.ps1'`; `Select-String` prints nothing. If it prints a line, edit that mention out.

- [ ] **Step 6: Humanizer pass and commit**

Invoke `humanizer:humanizer` on `CLAUDE.md` and `README.md` (the new sections only). Then:
```powershell
git add CLAUDE.md README.md docs/superpowers/specs/2026-09-04-tam-foundation-design.md scripts/sync-xtm-upstream.ps1
git commit -m "docs: three equal remotes, the sync script, and the tag convention

The upstream sync script is gone with its upstream."
```

---

### Task 6: Pull request and merge

**Files:** none new.

- [ ] **Step 1: Push the branch and open the PR**

```powershell
cd C:\projects\agile-suite-wt-docs
git push -u origin docs/xtm-monorepo-convergence
$body = @'
xray-testcase-manager and the home Gitea become copies of this repository, kept in step by a push fan-out and scripts/sync-remotes.ps1. XTM releases move to xtm/v* tags. The upstream sync script is retired after one last run, which brings #155 and #154 in.

Design: docs/superpowers/specs/2026-09-06-xtm-monorepo-convergence-design.md
Plan: docs/superpowers/plans/2026-09-06-xtm-monorepo-convergence.md

No code under xtm/, core/, or tam/ changes beyond the upstream merge.
'@
$bodyFile = Join-Path $env:TEMP "pr-body.md"
Set-Content -Path $bodyFile -Value $body -Encoding utf8
gh pr create -R veenone/task-activity-manager --base main --head docs/xtm-monorepo-convergence --title "Fold the XTM repository into the monorepo" --body-file $bodyFile
```
Run the humanizer over the body first. Expected: a PR URL.

- [ ] **Step 2: Wait for CI**

```powershell
gh pr checks --watch -R veenone/task-activity-manager docs/xtm-monorepo-convergence
```
Expected: `test`, `frontend`, `build-windows`, `build-macos` all pass. If `test` fails under `xtm/`, the upstream merge in Task 2 left something out; fix on the branch and push.

- [ ] **Step 3: Merge**

```powershell
gh pr merge -R veenone/task-activity-manager docs/xtm-monorepo-convergence --squash --delete-branch
```
Expected: `✓ Squashed and merged pull request`.

---

### Task 7: The flip

**Files:** none. Remote state and local git config.

**Interfaces:**
- Consumes: `origin/main` containing Task 6's merge; `scripts/sync-remotes.ps1` from Task 4.

- [ ] **Step 1: Bring the worktree to the merged main**

```powershell
cd C:\projects\agile-suite-wt-docs
git checkout main
git pull --ff-only origin main
git log --oneline -1
```
Expected: the squash commit `Fold the XTM repository into the monorepo (#NN)`.

- [ ] **Step 2: Confirm both pushes will be fast-forwards**

```powershell
git fetch xtm-origin main
git merge-base --is-ancestor xtm-origin/main main; if ($LASTEXITCODE -eq 0) { "xtm-origin: fast-forward" } else { "xtm-origin: NOT fast-forward, stop" }
git ls-remote git@gitea2.myhome.lan:achmarah/xray-test-manager.git refs/heads/main
```
Expected: `xtm-origin: fast-forward`; the Gitea line shows the SHA of the standalone tip from 2026-09-05 (`caa163b...` or the #155 squash if the user pushed since). Check it with `git merge-base --is-ancestor <that sha> main`. If either check says not fast-forward, the other PC pushed to the old layout in the meantime: stop and tell the user.

- [ ] **Step 3: Push main and tags to the two XTM remotes**

```powershell
.\scripts\sync-remotes.ps1 -Setup
git push xtm-origin main --tags
git push gitea main --tags
```
Expected: `-Setup` prints the three push URLs. Each push prints `main -> main` and any tags the remote lacked. No `[rejected]` lines.

- [ ] **Step 4: First real sync run and remote clean-up**

```powershell
.\scripts\sync-remotes.ps1
git remote remove xtm-upstream
foreach ($u in "git@github.com:veenone/task-activity-manager.git", "git@github.com:veenone/xray-testcase-manager.git", "git@gitea2.myhome.lan:achmarah/xray-test-manager.git") { git ls-remote $u refs/heads/main }
```
Expected: the sync reports every remote `unchanged` at the same SHA; the three `ls-remote` lines print the same SHA.

- [ ] **Step 5: Exercise the script once with a throwaway tag**

```powershell
git tag sync-check
git push xtm-origin sync-check
git tag -d sync-check
.\scripts\sync-remotes.ps1
git ls-remote --tags origin refs/tags/sync-check
git ls-remote --tags gitea refs/tags/sync-check
git push origin :refs/tags/sync-check
```
Expected: after the sync, both `ls-remote` lines show the tag. The final push goes through the fan-out and prints `[deleted]` once per remote, three times in all.

- [ ] **Step 6: Check CI on xray-testcase-manager**

```powershell
gh run list -R veenone/xray-testcase-manager --branch main --limit 3
```
Expected: a `Build check` run for the new main, `completed success`. If it is still `in_progress`, wait with `gh run watch -R veenone/xray-testcase-manager <id>`.

---

### Task 8: Local clean-up, memory, Outline

**Files:**
- Modify: `C:\Users\ARaha\.claude\projects\C--projects-xray-test-manager\memory\tam-fork-project.md`
- Outline: a new document under the TAM Foundation design page.

- [ ] **Step 1: Remove the worktree**

```powershell
cd C:\projects\task-activity-manager
git worktree remove C:\projects\agile-suite-wt-docs
git worktree prune
git branch -d docs/xtm-monorepo-convergence
```
Expected: no output from `remove`; `Deleted branch docs/xtm-monorepo-convergence`. The main checkout stays on `feat/plan-1c-write-features` with its uncommitted files untouched: confirm with `git status --short | Select-Object -First 5`.

- [ ] **Step 2: Tell the user what to do with the old folder and the other PC**

Print this in the final report, verbatim:
```
C:\projects\xray-test-manager is retired. Delete it when convenient; its
only real uncommitted change (the gstack CLAUDE.md section) is now in the
monorepo. On the other PC: cd into the xray-testcase-manager clone, git pull
(it fast-forwards into the monorepo layout), npm install at the root,
.\scripts\sync-remotes.ps1 -Setup, then work inside xtm\.
```

- [ ] **Step 3: Update the memory note**

Edit `tam-fork-project.md`: replace the sentence starting `The user keeps veenone/xray-testcase-manager active` through `which plan 0b documents.` with:
```
Since 2026-09-06 (convergence spec docs/superpowers/specs/2026-09-06-xtm-monorepo-convergence-design.md) xray-testcase-manager, task-activity-manager and Gitea achmarah/xray-test-manager all hold the monorepo and are equal; origin pushes to all three and scripts/sync-remotes.ps1 fast-forwards laggards after a GitHub merge. There is no standalone XTM any more and no upstream sync. XTM releases are tagged xtm/vX.Y.Z.
```
Also update the `Status` line's date and the mention of the old folder (`C:/projects/xray-test-manager folder is to be deleted by the user` stays true).

- [ ] **Step 4: Mirror the spec into Outline**

Use the Outline MCP tools: `mcp__outline__list_collections` to get the id of the "Xray Test Manager" collection, `mcp__outline__list_collection_documents` with that id to find the document titled `Task Activity Manager: foundation design`, then `mcp__outline__create_document` with that document as `parentDocumentId`, title `XTM joins the monorepo: repository convergence design`, and the spec's markdown body from section 1 onwards (Outline stores the title separately; do not start the body with an H1). Expected: a document URL, reported to the user.

---

## Self-review

**Spec coverage.** Section 2 decisions: repository fate and canonical remotes (Task 7), no rename (nothing to do), fan-out plus script (Task 4, Task 7 step 3), tag prefix (Task 3), open PRs (Task 1), script deletion (Task 5). Section 4.1 fan-out config (Task 4 `-Setup`), 4.2 and 4.3 script behaviour steps 1-7 (Task 4 script, each numbered comment maps to a spec step; the spec's "dirty checked-out main is left alone" is covered by the early refusal in step 1). Section 5 (Task 3). Section 6 order (Tasks 1-8 in that order). Section 7 verification: suites (Task 2 step 4), CI green (Task 6 step 2), equal SHAs (Task 7 step 4), throwaway tag (Task 7 step 5), release dry run through `workflow_dispatch` is left to the next real release as the spec allows. Section 8 risks: non-fast-forward stop (Task 7 step 2), diverged mains (script). Gitea has no CI: nothing to do.

**Placeholders.** None; every step has the command or the text.

**Type consistency.** The script's parameters (`-Setup`, `-OriginUrl`, `-XtmUrl`, `-GiteaUrl`, `-Branch`) match between Task 4's script, its test, and Task 7's invocations. Remote names `origin`, `xtm-origin`, `gitea` are the same everywhere, including the docs in Task 5. Exit codes 0/1/2 as declared in Task 4's interface.
