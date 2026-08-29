# AGIS Installer for Windows
# Usage: iwr -useb https://raw.githubusercontent.com/SalvucciFacundo/agis/main/install.ps1 | iex

$ErrorActionPreference = "Stop"
$Repo = "SalvucciFacundo/agis"
$BinaryName = "agis-windows-amd64.exe"
$InstallDir = "$env:LOCALAPPDATA\Programs\agis"

Write-Host "🚀 Installing AGIS (Autonomous Go Intelligent System)..." -ForegroundColor Cyan

# 1. Determine download URL from latest GitHub release
$LatestRelease = ""
try {
    $ReleaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    $LatestRelease = $ReleaseInfo.tag_name
} catch {
    Write-Host "⚠️ Unable to fetch release tag via API, using latest download endpoint..." -ForegroundColor Yellow
}

if ($LatestRelease) {
    $DownloadUrl = "https://github.com/$Repo/releases/download/$LatestRelease/$BinaryName"
    Write-Host "📦 Downloading AGIS release $LatestRelease..." -ForegroundColor Green
} else {
    $DownloadUrl = "https://github.com/$Repo/releases/latest/download/$BinaryName"
}

# 2. Download binary
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$DestPath = Join-Path $InstallDir "agis.exe"
$DownloadSuccess = $false

try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $DestPath -UseBasicParsing
    $DownloadSuccess = $true
} catch {
    Write-Host "⚠️ Direct binary download failed. Checking if Go compiler is available..." -ForegroundColor Yellow
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Host "🔨 Building and installing from source via 'go install'..." -ForegroundColor Cyan
        go install "github.com/$Repo/cmd/agis@latest"
        Write-Host "✅ AGIS installed successfully via 'go install'!" -ForegroundColor Green
        exit 0
    } else {
        Write-Error "❌ Failed to download binary and Go compiler is not installed."
        exit 1
    }
}

# 3. Add to user PATH if not present
$UserPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", [EnvironmentVariableTarget]::User)
    $env:Path += ";$InstallDir"
    Write-Host "✨ Added $InstallDir to user PATH." -ForegroundColor Yellow
}

Write-Host "✅ AGIS installed successfully at $DestPath!" -ForegroundColor Green
Write-Host ""
Write-Host "Type 'agis' in a new PowerShell window to start." -ForegroundColor Cyan
