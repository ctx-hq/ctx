# ctx installer for Windows — installs the latest ctx binary
# Usage: irm https://getctx.org/install.ps1 | iex
#
# Environment variables:
#   CTX_INSTALL_DIR    — override installation directory (default: $env:LOCALAPPDATA\ctx)
#   CTX_VERSION        — specific version to install (default: latest)
#   CTX_NO_MODIFY_PATH — skip PATH auto-configuration (set to 1)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'  # Massively speeds up Invoke-WebRequest

$Repo = 'ctx-hq/ctx'
$Binary = 'ctx.exe'
$DefaultInstallDir = Join-Path $env:LOCALAPPDATA 'ctx'
$InstallDir = if ($env:CTX_INSTALL_DIR) { $env:CTX_INSTALL_DIR } else { $DefaultInstallDir }

# --- Helpers ----------------------------------------------------------------

function Write-Info  { param([string]$Msg) Write-Host "> $Msg" -ForegroundColor Cyan }
function Write-Ok    { param([string]$Msg) Write-Host "> $Msg" -ForegroundColor Green }
function Write-Warn  { param([string]$Msg) Write-Host "> $Msg" -ForegroundColor Yellow }

# --- Detect architecture ----------------------------------------------------

$RawArch = $env:PROCESSOR_ARCHITECTURE
switch ($RawArch) {
    'AMD64' { $Arch = 'amd64' }
    'x86' {
        switch ($env:PROCESSOR_ARCHITEW6432) {
            'AMD64' { $Arch = 'amd64' }
            'ARM64' { $Arch = 'arm64' }
            default {
                Write-Error "Unsupported: 32-bit x86. ctx requires a 64-bit system."
                exit 1
            }
        }
    }
    'ARM64' { $Arch = 'arm64' }
    default {
        Write-Error "Unsupported architecture: $RawArch"
        exit 1
    }
}

$Platform = "windows-$Arch"

# --- Resolve version --------------------------------------------------------

# Ensure TLS 1.2+ for GitHub API
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13

if ($env:CTX_VERSION) {
    $Version = $env:CTX_VERSION -replace '^v', ''
} else {
    Write-Info 'detecting latest version...'
    try {
        $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{
            'Accept'     = 'application/vnd.github+json'
            'User-Agent' = 'ctx-installer/1.0'
        } -TimeoutSec 30
        $Version = $Release.tag_name -replace '^v', ''
    } catch {
        Write-Error "Could not detect latest version (GitHub API may be rate-limited). Set `$env:CTX_VERSION = '0.1.0' and retry."
        exit 1
    }
    if (-not $Version) {
        Write-Error 'Could not determine latest version from GitHub API.'
        exit 1
    }
}

# --- Check for upgrade ------------------------------------------------------

$Destination = Join-Path $InstallDir $Binary
$Upgrade = $null

if (Test-Path $Destination) {
    try {
        $VersionOutput = & $Destination version 2>$null | ConvertFrom-Json
        $Current = $VersionOutput.data.version
        if ($Current -eq $Version) {
            Write-Ok "ctx v$Version is already installed"
            exit 0
        }
        if ($Current) {
            $Upgrade = $Current
            Write-Info "upgrading ctx v$Current -> v$Version ($Platform)"
        }
    } catch {
        # Can't determine current version — treat as fresh install
    }
}

if (-not $Upgrade) {
    Write-Info "installing ctx v$Version ($Platform)"
}

# --- Download and verify ----------------------------------------------------

$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "ctx-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    $ArchiveName = "ctx-$Platform.zip"
    $BaseUrl = "https://github.com/$Repo/releases/download/v$Version"
    $ArchiveUrl = "$BaseUrl/$ArchiveName"
    $ChecksumUrl = "$BaseUrl/checksums.txt"
    $ArchivePath = Join-Path $TmpDir $ArchiveName
    $ChecksumPath = Join-Path $TmpDir 'checksums.txt'

    try {
        Invoke-WebRequest -Uri $ArchiveUrl -OutFile $ArchivePath -UseBasicParsing -TimeoutSec 120
    } catch {
        Write-Error "failed to download $ArchiveUrl`n$_"
        exit 1
    }

    try {
        Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumPath -UseBasicParsing -TimeoutSec 30
    } catch {
        Write-Error "failed to download checksums`n$_"
        exit 1
    }

    # Exact match on archive name (avoid SBOM filename collision)
    $ChecksumLines = Get-Content $ChecksumPath
    $ExpectedLine = $ChecksumLines | Where-Object { $_ -match "  $([regex]::Escape($ArchiveName))$" }

    if (-not $ExpectedLine) {
        Write-Error "checksum not found for $ArchiveName"
        exit 1
    }

    # Handle case where multiple lines match (shouldn't happen with exact match, but be safe)
    if ($ExpectedLine -is [array]) { $ExpectedLine = $ExpectedLine[0] }
    $Expected = ($ExpectedLine -split '\s+')[0]
    $Actual = (Get-FileHash -Path $ArchivePath -Algorithm SHA256).Hash.ToLower()

    if ($Expected -ne $Actual) {
        Write-Error "checksum mismatch (expected $Expected, got $Actual)"
        exit 1
    }

    # --- Extract and install ------------------------------------------------

    $ExtractDir = Join-Path $TmpDir 'extract'
    Expand-Archive -Path $ArchivePath -DestinationPath $ExtractDir -Force

    $BinaryPath = Get-ChildItem -Path $ExtractDir -Recurse -Filter $Binary | Select-Object -First 1
    if (-not $BinaryPath) {
        Write-Error "binary $Binary not found in archive"
        exit 1
    }

    # Ensure install directory exists
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    Copy-Item -Path $BinaryPath.FullName -Destination $Destination -Force

    # --- Auto-configure PATH ------------------------------------------------

    if ($env:CTX_NO_MODIFY_PATH -ne '1') {
        $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        if ($UserPath -notlike "*$InstallDir*") {
            # Append to user PATH (persistent across sessions)
            $NewPath = "$InstallDir;$UserPath"
            [Environment]::SetEnvironmentVariable('Path', $NewPath, 'User')
            # Update current session
            $env:Path = "$InstallDir;$env:Path"
            Write-Info "added $InstallDir to user PATH"
        }
    }

    # --- Done ---------------------------------------------------------------

    if ($Upgrade) {
        Write-Ok "ctx upgraded v$Upgrade -> v$Version"
    } else {
        Write-Ok "ctx v$Version installed to $Destination"
    }
    Write-Host ''
    Write-Host '  run `ctx --help` to get started'

    # PowerShell may need profile reload for PATH changes
    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($UserPath -like "*$InstallDir*" -and $env:Path -notlike "*$InstallDir*") {
        Write-Host ''
        Write-Info 'restart your terminal for PATH changes to take effect'
    }

} finally {
    if (Test-Path $TmpDir) {
        Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
