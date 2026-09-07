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
