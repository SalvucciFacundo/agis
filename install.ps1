# AGIS Installer for Windows (PowerShell 5.1 & PowerShell 7+)
# Usage: iwr -useb https://raw.githubusercontent.com/SalvucciFacundo/agis/main/install.ps1 | iex

$ErrorActionPreference = "Stop"
$Repo = "SalvucciFacundo/agis"
$InstallDir = "$env:LOCALAPPDATA\Programs\agis"

Write-Host "🚀 Installing AGIS (Autonomous Go Intelligent System)..." -ForegroundColor Cyan

# 1. Determine architecture
$Arch = "amd64"
if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        $Arch = "arm64"
    } else {
        $Arch = "amd64"
    }
} else {
    $Arch = "386"
}

Write-Host "🔍 Detected Platform: windows/$Arch" -ForegroundColor Gray

# 2. Query latest release
$LatestRelease = ""
try {
    $ReleaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    $LatestRelease = $ReleaseInfo.tag_name
} catch {
    Write-Host "⚠️ Unable to query latest release tag from GitHub API..." -ForegroundColor Yellow
}

$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$DestPath = Join-Path $InstallDir "agis.exe"
$Success = $false

if ($LatestRelease) {
    $CleanVer = $LatestRelease.TrimStart('v')
    $ZipName = "agis_${CleanVer}_windows_${Arch}.zip"
    $ZipUrl = "https://github.com/$Repo/releases/download/$LatestRelease/$ZipName"
    $ExeUrl = "https://github.com/$Repo/releases/download/$LatestRelease/agis-windows-$Arch.exe"

    Write-Host "📦 Attempting download for release $LatestRelease..." -ForegroundColor Green

    # Try Zip archive first
    try {
        $ZipPath = Join-Path $TempDir "agis.zip"
        Invoke-WebRequest -Uri $ZipUrl -OutFile $ZipPath -UseBasicParsing
        Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
        $ExtractedExe = Join-Path $TempDir "agis.exe"
        if (Test-Path $ExtractedExe) {
            Move-Item -Path $ExtractedExe -Destination $DestPath -Force
            $Success = $true
        }
    } catch {
        # Fallback to direct exe
        try {
            Invoke-WebRequest -Uri $ExeUrl -OutFile $DestPath -UseBasicParsing
            $Success = $true
        } catch {}
    }
}

if (-not $Success) {
    Write-Host "⚠️ Prebuilt release binary download not available." -ForegroundColor Yellow
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Host "🔨 Building and installing from source via 'go install github.com/$Repo/cmd/agis@latest'..." -ForegroundColor Cyan
        try {
            & go install "github.com/$Repo/cmd/agis@latest"
            Write-Host "✅ AGIS installed successfully via 'go install'!" -ForegroundColor Green
            Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
            exit 0
        } catch {
            Write-Error "❌ go install failed."
        }
    } else {
        Write-Error "❌ Could not download release binary and Go compiler is not installed."
        Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
        exit 1
    }
}

Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue

# 3. Add to user PATH if not present
$UserPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", [EnvironmentVariableTarget]::User)
    $env:Path += ";$InstallDir"
    Write-Host "✨ Added $InstallDir to user PATH." -ForegroundColor Yellow
}

Write-Host "✅ AGIS installed successfully at $DestPath!" -ForegroundColor Green
Write-Host ""
Write-Host "🚀 Type 'agis' in a new PowerShell window to start!" -ForegroundColor Cyan
