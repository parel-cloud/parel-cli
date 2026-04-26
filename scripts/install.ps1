<#
.SYNOPSIS
  parel CLI installer for Windows PowerShell.

.DESCRIPTION
  Downloads the latest parel-cli release, verifies its SHA256, extracts it to
  $InstallDir (default %LOCALAPPDATA%\parel\bin), and tells you to add it to
  PATH if it isn't already.

.EXAMPLE
  iwr -useb https://parel.cloud/install.ps1 | iex

  # custom location:
  $env:INSTALL_DIR = "C:\tools\bin"
  iwr -useb https://parel.cloud/install.ps1 | iex

  # specific version:
  $env:VERSION = "0.1.0"
  iwr -useb https://parel.cloud/install.ps1 | iex
#>

param(
  [string]$Version = $env:VERSION,
  [string]$InstallDir = $env:INSTALL_DIR
)

$ErrorActionPreference = "Stop"

$Repo = "parel-cloud/parel-cli"
$Bin = "parel.exe"

if (-not $InstallDir) {
  $InstallDir = Join-Path $env:LOCALAPPDATA "parel\bin"
}
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

if (-not $Version -or $Version -eq "latest") {
  Write-Host "Resolving latest release tag..."
  $latest = Invoke-RestMethod -UseBasicParsing -Headers @{ "User-Agent" = "parel-cli-installer" } `
    -Uri "https://api.github.com/repos/$Repo/releases/latest"
  $Version = $latest.tag_name
}
if (-not $Version.StartsWith("v")) { $Version = "v$Version" }
$VersionBare = $Version.TrimStart("v")

$arch = "amd64"
if ([Environment]::Is64BitOperatingSystem -eq $false) {
  throw "32-bit Windows is not supported."
}
$archiveName = "parel_${VersionBare}_windows_${arch}.zip"
$archiveUrl = "https://github.com/$Repo/releases/download/$Version/$archiveName"
$shasumUrl = "https://github.com/$Repo/releases/download/$Version/SHA256SUMS"

Write-Host "Installing parel $Version (windows/$arch) into $InstallDir"

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) "parel-install-$([guid]::NewGuid())"
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
  $archivePath = Join-Path $tmp $archiveName
  Invoke-WebRequest -UseBasicParsing -Uri $archiveUrl -OutFile $archivePath

  try {
    $shasumPath = Join-Path $tmp "SHA256SUMS"
    Invoke-WebRequest -UseBasicParsing -Uri $shasumUrl -OutFile $shasumPath
    $expectedLine = (Get-Content $shasumPath) -match $archiveName | Select-Object -First 1
    if ($expectedLine) {
      $expected = ($expectedLine -split '\s+')[0]
      $actual = (Get-FileHash -Algorithm SHA256 -Path $archivePath).Hash.ToLower()
      if ($expected.ToLower() -ne $actual) {
        throw "SHA256 mismatch (expected $expected, got $actual)"
      }
      Write-Host "  SHA256 OK"
    } else {
      Write-Warning "$archiveName not in SHA256SUMS; skipping verification"
    }
  } catch {
    Write-Warning "SHA256 verification skipped: $($_.Exception.Message)"
  }

  Expand-Archive -Path $archivePath -DestinationPath $tmp -Force
  Move-Item -Path (Join-Path $tmp $Bin) -Destination (Join-Path $InstallDir $Bin) -Force
}
finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

$pathDirs = $env:Path -split ';'
if (-not ($pathDirs -contains $InstallDir)) {
  Write-Host ""
  Write-Warning "$InstallDir is not on PATH. Add it via:"
  Write-Host "  [Environment]::SetEnvironmentVariable('Path', `"`$env:Path;$InstallDir`", 'User')"
  Write-Host ""
}

& (Join-Path $InstallDir $Bin) version
Write-Host ""
Write-Host "Done. Try: parel auth login"
