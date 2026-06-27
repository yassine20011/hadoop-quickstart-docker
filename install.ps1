# install.ps1 — install hadoop-dev for Windows (PowerShell 5.1+)
#
# Usage (run in PowerShell as a normal user — no admin required):
#   iwr -useb https://raw.githubusercontent.com/yassine20011/hadoop-quickstart-docker/main/install.ps1 | iex
#
# Optional env overrides (set before running):
#   $env:HADOOP_DEV_VERSION  — install a specific version (default: latest)
#   $env:HADOOP_DEV_DIR      — install directory (default: $env:LOCALAPPDATA\hadoop-dev)

$ErrorActionPreference = 'Stop'

$Repo    = "yassine20011/hadoop-quickstart-docker"
$Binary  = "hadoop-dev"
$InstallDir = if ($env:HADOOP_DEV_DIR) { $env:HADOOP_DEV_DIR } else { Join-Path $env:LOCALAPPDATA "hadoop-dev" }

function Write-Step($msg) { Write-Host "  $msg" -ForegroundColor DarkGray }
function Write-Ok($msg)   { Write-Host "  $([char]0x2713) $msg" -ForegroundColor Green }
function Write-Warn($msg) { Write-Host "  $([char]0x26A0) $msg" -ForegroundColor Yellow }
function Write-Fail($msg) { Write-Host "  $([char]0x2717) $msg" -ForegroundColor Red; exit 1 }

# ── Detect arch ───────────────────────────────────────────────────────────────

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { Write-Fail "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}

# ── Resolve version ───────────────────────────────────────────────────────────

if ($env:HADOOP_DEV_VERSION) {
    $Version = $env:HADOOP_DEV_VERSION
} else {
    Write-Step "Fetching latest release..."
    try {
        $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
        $Version = $Release.tag_name -replace '^v', ''
    } catch {
        Write-Fail "Could not fetch latest version: $_"
    }
}

Write-Ok "Installing hadoop-dev v$Version (windows/$Arch)"

# ── Download and extract ──────────────────────────────────────────────────────

$Archive = "${Binary}_${Version}_windows_${Arch}.zip"
$Url     = "https://github.com/$Repo/releases/download/v$Version/$Archive"
$TmpDir  = Join-Path $env:TEMP "hadoop-dev-install"
$TmpZip  = Join-Path $TmpDir $Archive

if (Test-Path $TmpDir) { Remove-Item $TmpDir -Recurse -Force }
New-Item -ItemType Directory -Path $TmpDir | Out-Null

Write-Step "Downloading $Archive..."
try {
    Invoke-WebRequest -Uri $Url -OutFile $TmpZip -UseBasicParsing
} catch {
    Write-Fail "Download failed from: $Url`n$_"
}

Write-Step "Extracting..."
Expand-Archive -Path $TmpZip -DestinationPath $TmpDir -Force

# ── Install ───────────────────────────────────────────────────────────────────

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

$ExeDst = Join-Path $InstallDir "${Binary}.exe"
# Remove old binary if running (rename trick)
if (Test-Path $ExeDst) {
    try { Remove-Item $ExeDst -Force } catch { Rename-Item $ExeDst "${ExeDst}.old" -Force }
}
Copy-Item (Join-Path $TmpDir "${Binary}.exe") $ExeDst -Force
Remove-Item $TmpDir -Recurse -Force

Write-Ok "Installed to $ExeDst"

# ── PATH check and update ─────────────────────────────────────────────────────

$UserPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
if ($UserPath -notmatch [regex]::Escape($InstallDir)) {
    Write-Step "Adding $InstallDir to your user PATH..."
    $NewPath = "$UserPath;$InstallDir"
    [Environment]::SetEnvironmentVariable('PATH', $NewPath, 'User')
    $env:PATH = "$env:PATH;$InstallDir"
    Write-Ok "PATH updated — open a new terminal window and run: hadoop-dev --help"
} else {
    Write-Ok "hadoop-dev is ready — run: hadoop-dev --help"
}
