<#
.SYNOPSIS
  Build, version and bundle one app in this repository for distribution (Windows).

.DESCRIPTION
  Produces, under <app>/dist/:
    - a portable single-file executable        <binary>-<ver>-windows-amd64.exe
    - an Inno Setup installer (unless          <binary>-<ver>-windows-amd64-installer.exe
      -NoInstaller)
    - the user guide bundle, for an app        <binary>-<ver>-user-guide.zip
      that ships one
    - SHA256SUMS.txt for all of the above

  The product name and the binary name are read from the app's wails.json
  rather than written here, so this script serves every app in the repository
  and gains nothing to edit when another is added.

  The version is stamped into wails.json (info.productVersion), which Wails
  bakes into the .exe version resource; the same version is passed to the Inno
  Setup compiler for the installer.

.PARAMETER App
  Which app to release: the directory holding its wails.json, e.g. xtm or tam.

.PARAMETER Version
  Semver to release, e.g. 0.2.0. If omitted, the app's current wails.json
  info.productVersion is used.

.PARAMETER NoInstaller
  Build only the portable .exe (skips the Inno Setup installer - useful when
  ISCC.exe isn't installed).

.EXAMPLE
  ./scripts/release-app.ps1 -App xtm -Version 1.10.0

.EXAMPLE
  ./scripts/release-app.ps1 -App tam -Version 0.1.0
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$App,
  [string]$Version,
  [switch]$NoInstaller
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$root = Join-Path $repoRoot $App

function Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }

# --- Resolve the app ---------------------------------------------------------
$wailsJsonPath = Join-Path $root "wails.json"
if (-not (Test-Path $wailsJsonPath)) {
  throw "No app at '$App': expected $wailsJsonPath. Pass the directory holding the app's wails.json."
}
Set-Location $root

$wailsJson = Get-Content $wailsJsonPath -Raw
# The two names every artifact is built from. Reading them here is what keeps
# this script free of any one app's literals.
if ($wailsJson -notmatch '"outputfilename"\s*:\s*"([^"]*)"') {
  throw "$wailsJsonPath has no outputfilename; the built exe cannot be located without it."
}
$binary = $Matches[1]
if ($wailsJson -notmatch '"productName"\s*:\s*"([^"]*)"') {
  throw "$wailsJsonPath has no info.productName; the release name cannot be built without it."
}
$productName = $Matches[1]

# --- Resolve version (arg wins; otherwise read wails.json) -------------------
if ($Version) {
  if ($Version -notmatch '^\d+\.\d+\.\d+') {
    throw "Version '$Version' is not semver (expected x.y.z)."
  }
  # Targeted text replace so the file's formatting/key order is preserved.
  if ($wailsJson -match '"productVersion"\s*:\s*"[^"]*"') {
    $wailsJson = $wailsJson -replace '("productVersion"\s*:\s*")[^"]*(")', "`${1}$Version`${2}"
  } else {
    throw "wails.json has no info.productVersion to stamp; add an info block first."
  }
  # Write UTF-8 WITHOUT a BOM. Windows PowerShell 5.1's `Set-Content -Encoding utf8`
  # prepends a BOM (EF BB BF), which Wails' JSON parser rejects with
  # "invalid character 'ï' looking for beginning of value".
  [System.IO.File]::WriteAllText($wailsJsonPath, $wailsJson, (New-Object System.Text.UTF8Encoding($false)))
  Write-Host "Stamped wails.json productVersion = $Version"
} else {
  if ($wailsJson -match '"productVersion"\s*:\s*"([^"]*)"') { $Version = $Matches[1] }
  if (-not $Version) { throw "No version: pass -Version x.y.z or set info.productVersion in wails.json." }
}
Step "Releasing $productName v$Version"

# --- Locate the Wails CLI ----------------------------------------------------
$wailsExe = (Get-Command wails -ErrorAction SilentlyContinue).Source
if (-not $wailsExe) { $wailsExe = Join-Path $env:USERPROFILE "go\bin\wails.exe" }
if (-not (Test-Path $wailsExe)) {
  throw "wails CLI not found. Install it: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
}

# --- Build -------------------------------------------------------------------
# Build the portable exe only; the installer is built from it with Inno Setup
# below (we no longer pass -nsis).
Step "Building (windows/amd64, production)"
& $wailsExe build -platform windows/amd64 -clean -trimpath
if ($LASTEXITCODE -ne 0) { throw "wails build failed (exit $LASTEXITCODE)." }

# --- Stage dist/ -------------------------------------------------------------
Step "Staging artifacts"
$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null
Get-ChildItem $dist -File -ErrorAction SilentlyContinue | Remove-Item -Force

$portableSrc = Join-Path $root "build\bin\$binary.exe"
if (-not (Test-Path $portableSrc)) { throw "Expected build output not found: $portableSrc" }
$portable = Join-Path $dist "$binary-$Version-windows-amd64.exe"
Copy-Item $portableSrc $portable -Force

# --- Installer (Inno Setup) --------------------------------------------------
# Compile build\windows\installer\installer.iss with ISCC, which packages the
# portable exe (built above) into an installer written straight into dist/.
if (-not $NoInstaller) {
  Step "Building installer (Inno Setup)"

  $iss = Join-Path $root "build\windows\installer\installer.iss"
  if (-not (Test-Path $iss)) {
    throw "No installer script at $iss. Add one (see another app's for the shape, with its OWN AppId GUID) or pass -NoInstaller."
  }

  # Locate the Inno Setup compiler (ISCC.exe): PATH first, then the default
  # install location.
  $iscc = (Get-Command iscc -ErrorAction SilentlyContinue).Source
  if (-not $iscc) {
    foreach ($p in @(
        (Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"),
        (Join-Path $env:ProgramFiles "Inno Setup 6\ISCC.exe"))) {
      if ($p -and (Test-Path $p)) { $iscc = $p; break }
    }
  }

  if (-not $iscc) {
    Write-Warning "Inno Setup (ISCC.exe) not found - skipping installer. Install it (choco install innosetup) or pass -NoInstaller."
  } else {
    # Best-effort: fetch the Evergreen WebView2 bootstrapper so the installer can
    # install the runtime when it's missing. If the download fails, the installer
    # still builds and relies on WebView2 already being present.
    $wv2 = Join-Path $root "build\windows\installer\tmp\MicrosoftEdgeWebview2Setup.exe"
    if (-not (Test-Path $wv2)) {
      New-Item -ItemType Directory -Force -Path (Split-Path $wv2) | Out-Null
      try {
        Invoke-WebRequest -Uri "https://go.microsoft.com/fwlink/p/?LinkId=2124703" -OutFile $wv2 -UseBasicParsing
      } catch {
        Write-Warning "Could not download the WebView2 bootstrapper ($($_.Exception.Message)); the installer will rely on WebView2 already being present."
      }
    }

    $isccArgs = @(
      "/DAppVersion=$Version",
      "/DSourceDir=$(Join-Path $root 'build\bin')",
      "/DOutputDir=$dist"
    )
    if (Test-Path $wv2) { $isccArgs += "/DWebView2Bootstrapper=$wv2" }
    & $iscc @isccArgs $iss
    if ($LASTEXITCODE -ne 0) { throw "Inno Setup compile failed (exit $LASTEXITCODE)." }
  }
}

# --- User guide --------------------------------------------------------------
# Bundle the app's user guide (markdown + screenshots) into a versioned zip so
# it ships alongside the binaries. The generated docs/user-guide/dist build
# output and the images/.gitkeep placeholder are deliberately excluded.
#
# docs/user-guide is XTM's guide, titled for it and versioned with it. TAM's
# documentation is kept outside this repository, so TAM ships no guide and the
# step is skipped rather than failing a release over a file that was never
# meant to be here. An app that declares a guide must still ship it: a missing
# file for XTM is an error, not a skip.
$guideDirs = @{ xtm = "docs\user-guide" }
if ($guideDirs.ContainsKey($App)) {
  Step "Bundling user guide"
  $guideSrc = Join-Path $repoRoot $guideDirs[$App]
  if (-not (Test-Path (Join-Path $guideSrc "USER_GUIDE.md"))) {
    throw "User guide not found at $guideSrc; a release of $productName must ship it."
  }
  # The archive root reads as the product, e.g. Xray-Test-Manager-User-Guide/.
  $guideFolder = ($productName -replace '\s+', '-') + "-User-Guide"
  $guideStage = Join-Path $env:TEMP "$App-user-guide-$Version"
  if (Test-Path $guideStage) { Remove-Item $guideStage -Recurse -Force }
  $guideInner = Join-Path $guideStage $guideFolder
  New-Item -ItemType Directory -Force -Path $guideInner | Out-Null
  Copy-Item (Join-Path $guideSrc "USER_GUIDE.md") $guideInner -Force
  Copy-Item (Join-Path $guideSrc "images") (Join-Path $guideInner "images") -Recurse -Force
  Remove-Item (Join-Path $guideInner "images\.gitkeep") -Force -ErrorAction SilentlyContinue
  $guideZip = Join-Path $dist "$binary-$Version-user-guide.zip"
  if (Test-Path $guideZip) { Remove-Item $guideZip -Force }
  # Compress the folder itself so the archive root is the folder, not its files.
  Compress-Archive -Path $guideInner -DestinationPath $guideZip -Force
  Remove-Item $guideStage -Recurse -Force
  Write-Host "Bundled user guide -> $(Split-Path $guideZip -Leaf)"
} else {
  Write-Host "$productName ships no user guide from this repository; skipping that step."
}

# --- Checksums ---------------------------------------------------------------
Step "Writing SHA256SUMS.txt"
Push-Location $dist
try {
  $lines = Get-ChildItem -File | Where-Object { $_.Name -ne "SHA256SUMS.txt" } | ForEach-Object {
    $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower()
    "$hash  $($_.Name)"
  }
  Set-Content -Path "SHA256SUMS.txt" -Value $lines -Encoding ascii
} finally { Pop-Location }

Write-Host ""
Step "Done - artifacts in $App/dist/"
Get-ChildItem $dist | Select-Object Name, @{ N = "Size"; E = { "{0:N1} MB" -f ($_.Length / 1MB) } } | Format-Table -AutoSize
Write-Host "Next: tag the release ->  git tag $App/v$Version && git push origin $App/v$Version" -ForegroundColor Green
