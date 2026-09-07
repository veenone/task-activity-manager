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
$urls = @{}
foreach ($n in $names) {
    if ($existing -notcontains $n) { Fail "Remote '$n' is missing. Run .\scripts\sync-remotes.ps1 -Setup first." }
    # Pushes below go to the remote's URL, not its name: after -Setup a push
    # to "origin" would fan out to all three remotes at once.
    $urls[$n] = (Invoke-Git @("remote", "get-url", $n))
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
        Invoke-Git @("push", "--quiet", $urls[$n], "${newestSha}:refs/heads/$Branch") | Out-Null
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
            Invoke-Git @("push", "--quiet", $urls[$n], "${syncNs}/${source}/${t}:refs/tags/$t") | Out-Null
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
            Write-Host "local $Branch fast-forwarded to $($newestSha.Substring(0,7))"
        } else {
            # Refused when main is checked out in another worktree; the remotes
            # are already in step, so that is not a failure.
            & git branch -f $Branch $newestSha | Out-Null
            if ($LASTEXITCODE -eq 0) {
                Write-Host "local $Branch fast-forwarded to $($newestSha.Substring(0,7))"
            } else {
                Write-Host "local $Branch is checked out in another worktree; left alone"
            }
        }
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
