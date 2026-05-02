<#
.SYNOPSIS
    Bumps the Flutter app version and builds a release APK pointing to production.

.DESCRIPTION
    Single-purpose helper for the local release flow:
      1. Reads `version: X.Y.Z+N` from mobile/pubspec.yaml
      2. Increments the chosen segment plus the build number
      3. Writes the new version back to pubspec.yaml
      4. Runs `flutter build apk --release` with the production API_BASE_URL baked in
      5. Copies the APK to `mobile/build/releases/jullius-scan-X.Y.Z.apk`

    All steps run from the repository root regardless of the caller's CWD.

.PARAMETER Bump
    Which segment to increment. Default: patch. Use `minor` or `major` for those.

.PARAMETER ApiBaseUrl
    Backend URL injected into the APK via --dart-define=API_BASE_URL.
    Default: https://jullius-scan.erielmiquilino.me

.PARAMETER SkipBuild
    Bump the version in pubspec.yaml but do not run `flutter build apk`.
    Useful when you only want to commit a version bump.

.EXAMPLE
    pwsh scripts/release.ps1
    # 0.0.1+1 -> 0.0.2+2, builds APK, copies to build/releases/jullius-scan-0.0.2.apk

.EXAMPLE
    pwsh scripts/release.ps1 -Bump minor
    # 0.0.2+2 -> 0.1.0+3
#>

param(
    [ValidateSet('patch', 'minor', 'major')]
    [string]$Bump = 'patch',

    [string]$ApiBaseUrl = 'https://jullius-scan.erielmiquilino.me',

    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'

# --- Locate repo root + pubspec --------------------------------------------------

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')
$pubspecPath = Join-Path $repoRoot 'mobile/pubspec.yaml'
if (-not (Test-Path $pubspecPath)) {
    throw "pubspec.yaml not found at $pubspecPath"
}

# --- Read + bump version ---------------------------------------------------------

$pubspec = Get-Content $pubspecPath -Raw
$versionPattern = '(?m)^version:\s+(\d+)\.(\d+)\.(\d+)\+(\d+)\s*$'
$match = [regex]::Match($pubspec, $versionPattern)
if (-not $match.Success) {
    throw "version line not found in pubspec.yaml (expected: 'version: X.Y.Z+N')"
}

$major = [int]$match.Groups[1].Value
$minor = [int]$match.Groups[2].Value
$patch = [int]$match.Groups[3].Value
$build = [int]$match.Groups[4].Value

$oldVersion = "$major.$minor.$patch+$build"

switch ($Bump) {
    'major' { $major++; $minor = 0; $patch = 0 }
    'minor' { $minor++; $patch = 0 }
    'patch' { $patch++ }
}
$build++

$newSemver = "$major.$minor.$patch"
$newVersion = "$newSemver+$build"
$pubspec = [regex]::Replace($pubspec, $versionPattern, "version: $newVersion")
Set-Content -Path $pubspecPath -Value $pubspec -NoNewline -Encoding UTF8

Write-Host "Version bumped: $oldVersion -> $newVersion ($Bump)" -ForegroundColor Green

if ($SkipBuild) {
    Write-Host 'SkipBuild flag set — pubspec.yaml updated but no APK built.' -ForegroundColor Yellow
    return
}

# --- Build APK --------------------------------------------------------------------

Push-Location (Join-Path $repoRoot 'mobile')
try {
    Write-Host 'Running flutter pub get...' -ForegroundColor Cyan
    & flutter pub get
    if ($LASTEXITCODE -ne 0) { throw 'flutter pub get failed' }

    Write-Host "Building APK with API_BASE_URL=$ApiBaseUrl ..." -ForegroundColor Cyan
    & flutter build apk --release "--dart-define=API_BASE_URL=$ApiBaseUrl"
    if ($LASTEXITCODE -ne 0) { throw 'flutter build apk failed' }

    $sourceApk = Join-Path (Get-Location) 'build/app/outputs/flutter-apk/app-release.apk'
    if (-not (Test-Path $sourceApk)) {
        throw "expected APK not found at $sourceApk"
    }

    $releaseDir = Join-Path (Get-Location) 'build/releases'
    if (-not (Test-Path $releaseDir)) {
        New-Item -ItemType Directory -Path $releaseDir | Out-Null
    }
    $destApk = Join-Path $releaseDir "jullius-scan-$newSemver.apk"
    Copy-Item -Path $sourceApk -Destination $destApk -Force

    $sizeMb = [math]::Round((Get-Item $destApk).Length / 1MB, 1)
    Write-Host ''
    Write-Host '=== Release ready ===' -ForegroundColor Green
    Write-Host "Version : $newVersion"
    Write-Host "APK     : $destApk"
    Write-Host "Size    : ${sizeMb} MB"
}
finally {
    Pop-Location
}
