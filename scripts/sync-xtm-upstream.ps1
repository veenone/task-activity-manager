<#
Pulls the latest Xray Test Manager commits from its own repository into xtm/.

XTM is still developed in veenone/xray-testcase-manager. This repository
carries a copy under xtm/ that has to follow it. Both histories share every
commit up to the monorepo restructure, so a subtree-shifted merge gives Git a
real merge base and lines the two trees up.

Usage:
  .\scripts\sync-xtm-upstream.ps1            # merge upstream main, leave it uncommitted
  .\scripts\sync-xtm-upstream.ps1 -Branch dev

Afterwards: resolve any conflicts, run the XTM suites, then commit.
#>
param(
    [string]$Remote = "xtm-upstream",
    [string]$Url = "git@github.com:veenone/xray-testcase-manager.git",
    [string]$Branch = "main"
)

$ErrorActionPreference = "Stop"
$root = (git rev-parse --show-toplevel).Trim()
# Push/Pop so the operator's shell stays where it was, whatever happens below.
$script:mergeExit = 0
Push-Location $root
try {
    if ((git status --porcelain --untracked-files=no).Length -ne 0) {
        throw "The working tree has uncommitted changes. Commit or stash them first."
    }

    $remotes = git remote
    if ($remotes -notcontains $Remote) {
        git remote add $Remote $Url
    }
    git fetch $Remote $Branch
    if ($LASTEXITCODE -ne 0) {
        throw "git fetch $Remote $Branch failed."
    }

    git merge -X subtree=xtm --no-commit --no-ff "$Remote/$Branch"
    $script:mergeExit = $LASTEXITCODE

    # Files that sit at the root of XTM's repository (docs/, .github/) get shifted
    # under xtm/ by the subtree merge. This repository keeps them at the root.
    foreach ($dir in @("docs", ".github")) {
        $shifted = Join-Path "xtm" $dir
        if (Test-Path $shifted) {
            Write-Warning "Upstream changed $dir and the merge put it under $shifted. Move those changes into $dir before committing."
        }
    }

    if ($script:mergeExit -ne 0) {
        Write-Host "The merge stopped on conflicts. Resolve them, then run the suites and commit."
    } else {
        Write-Host "Merged without conflicts. Nothing is committed yet."
    }
    Write-Host "Check: cd xtm; go test ./internal/...; cd frontend; npx vitest run"
} finally {
    Pop-Location
}

exit $script:mergeExit
