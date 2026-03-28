# ctx installer for Windows — installs the latest ctx binary
# Usage: irm https://getctx.org/install.ps1 | iex
#
# Environment variables:
#   CTX_INSTALL_DIR  — installation directory (default: $env:LOCALAPPDATA\ctx)
#   CTX_VERSION      — specific version to install (default: latest)

$ErrorActionPreference = 'Stop'

$Repo = 'ctx-hq/ctx'
$Binary = 'ctx.exe'
$DefaultInstallDir = Join-Path $env:LOCALAPPDATA 'ctx'
$InstallDir = if ($env:CTX_INSTALL_DIR) { $env:CTX_INSTALL_DIR } else { $DefaultInstallDir }

# --- Detect architecture ------------------------------------------------------

$RawArch = $env:PROCESSOR_ARCHITECTURE
switch ($RawArch) {
    'AMD64'   { $Arch = 'amd64' }
    'x86'     {
        # 32-bit process on 64-bit OS? PROCESSOR_ARCHITEW6432 can be AMD64 or ARM64.
        # See: https://learn.microsoft.com/windows/win32/winprog64/wow64-implementation-details
        switch ($env:PROCESSOR_ARCHITEW6432) {
            'AMD64' { $Arch = 'amd64' }
            'ARM64' { $Arch = 'arm64' }
            default {
                Write-Error "Unsupported architecture: x86 (32-bit). ctx requires a 64-bit system."
                exit 1
            }
        }
    }
    'ARM64'   { $Arch = 'arm64' }
    default   {
        Write-Error "Unsupported architecture: $RawArch"
        exit 1
    }
}

$Platform = "windows-$Arch"

# --- Resolve version ----------------------------------------------------------

if ($env:CTX_VERSION) {
    $Version = $env:CTX_VERSION -replace '^v', ''
    Write-Host "-> Installing ctx v$Version for $Platform..."
} else {
    Write-Host '-> Detecting latest version...'
    try {
        # Use TLS 1.2+ for GitHub API
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13

        $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{
            'Accept' = 'application/vnd.github+json'
            'User-Agent' = 'ctx-installer/1.0'
        } -TimeoutSec 30
        $Version = $Release.tag_name -replace '^v', ''
    } catch {
        Write-Error "Could not determine latest version: $_"
        exit 1
    }

    if (-not $Version) {
        Write-Error 'Could not determine latest version from GitHub API.'
        exit 1
    }
    Write-Host "-> Installing ctx v$Version for $Platform..."
}

# --- Download and verify ------------------------------------------------------

$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "ctx-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    $ArchiveName = "ctx-$Platform.zip"
    $BaseUrl = "https://github.com/$Repo/releases/download/v$Version"
    $ArchiveUrl = "$BaseUrl/$ArchiveName"
    $ChecksumUrl = "$BaseUrl/checksums.txt"
    $ArchivePath = Join-Path $TmpDir $ArchiveName
    $ChecksumPath = Join-Path $TmpDir 'checksums.txt'

    Write-Host '-> Downloading archive...'
    try {
        Invoke-WebRequest -Uri $ArchiveUrl -OutFile $ArchivePath -UseBasicParsing -TimeoutSec 120
    } catch {
        Write-Error "Failed to download archive from $ArchiveUrl`n$_"
        exit 1
    }

    Write-Host '-> Downloading checksums...'
    try {
        Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumPath -UseBasicParsing -TimeoutSec 30
    } catch {
        Write-Error "Failed to download checksums from $ChecksumUrl`n$_"
        exit 1
    }

    Write-Host '-> Verifying SHA256 checksum...'
    $ChecksumLines = Get-Content $ChecksumPath
    $ExpectedLine = $ChecksumLines | Where-Object { $_ -match [regex]::Escape($ArchiveName) }

    if (-not $ExpectedLine) {
        Write-Error "Checksum not found for $ArchiveName in checksums.txt"
        exit 1
    }

    $Expected = ($ExpectedLine -split '\s+')[0]
    $Actual = (Get-FileHash -Path $ArchivePath -Algorithm SHA256).Hash.ToLower()

    if ($Expected -ne $Actual) {
        Write-Error "Checksum mismatch!`n  Expected: $Expected`n  Actual:   $Actual"
        exit 1
    }
    Write-Host '-> Checksum verified'

    # --- Extract and install --------------------------------------------------

    Write-Host '-> Extracting...'
    $ExtractDir = Join-Path $TmpDir 'extract'
    Expand-Archive -Path $ArchivePath -DestinationPath $ExtractDir -Force

    # Find the binary (may be at root or in a subdirectory)
    $BinaryPath = Get-ChildItem -Path $ExtractDir -Recurse -Filter $Binary | Select-Object -First 1

    if (-not $BinaryPath) {
        Write-Error "Binary $Binary not found in archive."
        exit 1
    }

    # Ensure install directory exists
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    $Destination = Join-Path $InstallDir $Binary
    Copy-Item -Path $BinaryPath.FullName -Destination $Destination -Force

    # --- Add to PATH if needed ------------------------------------------------

    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($UserPath -notlike "*$InstallDir*") {
        Write-Host "-> Adding $InstallDir to user PATH..."
        $NewPath = "$InstallDir;$UserPath"
        [Environment]::SetEnvironmentVariable('Path', $NewPath, 'User')
        # Also update current session so ctx is immediately available
        $env:Path = "$InstallDir;$env:Path"
    }

    Write-Host ''
    Write-Host "ctx v$Version installed to $Destination"
    Write-Host ''
    Write-Host '  Get started:'
    Write-Host '    ctx search "code review"'
    Write-Host '    ctx install @scope/name'
    Write-Host '    ctx --help'

} finally {
    # Clean up temp directory
    if (Test-Path $TmpDir) {
        Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
