param(
    [string]$Version,
    [string]$BinDir,
    [switch]$Help
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Repository = "Yacobolo/toolbelt"

if ($Help) {
    @"
Install Sourcebook from a GitHub release.

Usage:
  .\install.ps1 [-Version <version>] [-BinDir <directory>]

Environment:
  SOURCEBOOK_VERSION                   Release version (default: latest)
  SOURCEBOOK_INSTALL_DIR               Install directory (default: %LOCALAPPDATA%\Programs\OpenAI\Codex\bin)
  SOURCEBOOK_RELEASE_BASE_URL          Override release download base URL
  SOURCEBOOK_RELEASES_API_URL          Override GitHub releases API URL
"@
    return
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = $env:SOURCEBOOK_VERSION
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = "latest"
}
if ($Version -eq "latest") {
    $ReleasesAPIURL = $env:SOURCEBOOK_RELEASES_API_URL
    if ([string]::IsNullOrWhiteSpace($ReleasesAPIURL)) {
        $ReleasesAPIURL = "https://api.github.com/repos/$Repository/releases?per_page=100"
    }
    $ReleasesURI = [System.Uri]$ReleasesAPIURL
    if ($ReleasesURI.IsFile) {
        $Releases = Get-Content -LiteralPath $ReleasesURI.LocalPath -Raw | ConvertFrom-Json
    }
    else {
        $Releases = Invoke-RestMethod -UseBasicParsing -Uri $ReleasesAPIURL
    }
    $LatestRelease = @($Releases) |
        Where-Object {
            -not $_.draft -and
            -not $_.prerelease -and
            $_.tag_name -match '^sourcebook/v[0-9]+\.[0-9]+\.[0-9]+$'
        } |
        Sort-Object { [System.Version]($_.tag_name -replace '^sourcebook/v', '') } -Descending |
        Select-Object -First 1
    if ($null -eq $LatestRelease) {
        throw "sourcebook installer: could not determine the latest Sourcebook release"
    }
    $Version = $LatestRelease.tag_name
}
$Version = $Version -replace '^sourcebook/v', '' -replace '^v', ''
if ($Version -notmatch '^[0-9A-Za-z._-]+$') {
    throw "sourcebook installer: invalid version: $Version"
}

if ([string]::IsNullOrWhiteSpace($BinDir)) {
    $BinDir = $env:SOURCEBOOK_INSTALL_DIR
}
if ([string]::IsNullOrWhiteSpace($BinDir)) {
    $LocalAppData = $env:LOCALAPPDATA
    if ([string]::IsNullOrWhiteSpace($LocalAppData)) {
        $LocalAppData = Join-Path $HOME "AppData\Local"
    }
    $BinDir = Join-Path $LocalAppData "Programs\OpenAI\Codex\bin"
}
$BinDir = [System.IO.Path]::GetFullPath($BinDir)

$TargetOS = $env:SOURCEBOOK_OS
if ([string]::IsNullOrWhiteSpace($TargetOS)) {
    if ($env:OS -ne "Windows_NT") {
        throw "sourcebook installer: install.ps1 supports Windows only"
    }
    $TargetOS = "windows"
}
$TargetOS = $TargetOS.Trim().ToLowerInvariant()
if ($TargetOS -ne "windows") {
    throw "sourcebook installer: unsupported operating system: $TargetOS"
}

$TargetArch = $env:SOURCEBOOK_ARCH
if ([string]::IsNullOrWhiteSpace($TargetArch)) {
    # PROCESSOR_ARCHITEW6432 identifies the native OS architecture when a
    # 32-bit PowerShell process is running under WOW64.
    $DetectedArch = $env:PROCESSOR_ARCHITEW6432
    if ([string]::IsNullOrWhiteSpace($DetectedArch)) {
        $DetectedArch = $env:PROCESSOR_ARCHITECTURE
    }
    if ([string]::IsNullOrWhiteSpace($DetectedArch)) {
        throw "sourcebook installer: could not determine the Windows architecture"
    }
    $DetectedArch = $DetectedArch.Trim().ToUpperInvariant()
    switch ($DetectedArch) {
        "AMD64" { $TargetArch = "amd64" }
        "ARM64" { $TargetArch = "arm64" }
        default { throw "sourcebook installer: unsupported architecture: $DetectedArch" }
    }
}
$TargetArch = $TargetArch.Trim().ToLowerInvariant()
if ($TargetArch -notin @("amd64", "arm64")) {
    throw "sourcebook installer: unsupported architecture: $TargetArch"
}

$ReleaseBaseURL = $env:SOURCEBOOK_RELEASE_BASE_URL
if ([string]::IsNullOrWhiteSpace($ReleaseBaseURL)) {
    $ReleaseBaseURL = "https://github.com/$Repository/releases/download"
}
$ReleaseBaseURL = $ReleaseBaseURL.TrimEnd('/')
$Asset = "sourcebook_${Version}_windows_${TargetArch}.zip"
$ReleaseURL = "$ReleaseBaseURL/sourcebook/v$Version"

function Get-SourcebookFile {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][string]$Destination
    )

    $ParsedURI = [System.Uri]$Uri
    if ($ParsedURI.IsFile) {
        Copy-Item -LiteralPath $ParsedURI.LocalPath -Destination $Destination -Force
        return
    }
    Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $Destination
}

function Get-SourcebookSHA256 {
    param(
        [Parameter(Mandatory = $true)][string]$Path
    )

    $Algorithm = [System.Security.Cryptography.SHA256]::Create()
    $Stream = [System.IO.File]::OpenRead($Path)
    try {
        $Hash = $Algorithm.ComputeHash($Stream)
        return [System.BitConverter]::ToString($Hash).Replace("-", "").ToLowerInvariant()
    }
    finally {
        $Stream.Dispose()
        $Algorithm.Dispose()
    }
}

function Expand-SourcebookZip {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Destination
    )

    $ExpandArchive = Get-Command Expand-Archive -ErrorAction SilentlyContinue
    if ($null -ne $ExpandArchive) {
        Expand-Archive -LiteralPath $Path -DestinationPath $Destination
        return
    }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($Path, $Destination)
}

$TemporaryDir = Join-Path ([System.IO.Path]::GetTempPath()) ("sourcebook-install-" + [System.Guid]::NewGuid())
$StagedBinary = $null
New-Item -ItemType Directory -Path $TemporaryDir | Out-Null

try {
    $Archive = Join-Path $TemporaryDir $Asset
    $Checksums = Join-Path $TemporaryDir "checksums.txt"
    Get-SourcebookFile -Uri "$ReleaseURL/$Asset" -Destination $Archive
    Get-SourcebookFile -Uri "$ReleaseURL/checksums.txt" -Destination $Checksums

    $EscapedAsset = [System.Text.RegularExpressions.Regex]::Escape($Asset)
    $ChecksumLine = Get-Content -LiteralPath $Checksums |
        Where-Object { $_ -match "^[a-fA-F0-9]{64}\s+\*?$EscapedAsset$" } |
        Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($ChecksumLine)) {
        throw "sourcebook installer: checksum not found for $Asset"
    }
    $ExpectedChecksum = ($ChecksumLine -split '\s+')[0].ToLowerInvariant()
    $ActualChecksum = Get-SourcebookSHA256 -Path $Archive
    if ($ActualChecksum -ne $ExpectedChecksum) {
        throw "sourcebook installer: checksum verification failed for $Asset"
    }

    $ExtractedDir = Join-Path $TemporaryDir "extracted"
    Expand-SourcebookZip -Path $Archive -Destination $ExtractedDir
    $SourceBinary = Join-Path $ExtractedDir "sourcebook.exe"
    if (-not (Test-Path -LiteralPath $SourceBinary -PathType Leaf)) {
        throw "sourcebook installer: archive does not contain sourcebook.exe"
    }

    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    $StagedBinary = Join-Path $BinDir (".sourcebook.exe.tmp." + $PID)
    Copy-Item -LiteralPath $SourceBinary -Destination $StagedBinary -Force
    Move-Item -LiteralPath $StagedBinary -Destination (Join-Path $BinDir "sourcebook.exe") -Force
    $StagedBinary = $null

    $UserPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
    $PathEntries = @($UserPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($BinDir -notin $PathEntries) {
        $NewUserPath = if ([string]::IsNullOrWhiteSpace($UserPath)) { $BinDir } else { "$UserPath;$BinDir" }
        try {
            [System.Environment]::SetEnvironmentVariable("Path", $NewUserPath, "User")
        }
        catch {
            Write-Warning "Installed Sourcebook, but could not add $BinDir to your user PATH: $_"
        }
    }
    if ($BinDir -notin @($env:Path -split ';')) {
        $env:Path = "$env:Path;$BinDir"
    }

    Write-Output "Sourcebook v$Version installed to $(Join-Path $BinDir 'sourcebook.exe')"
    Write-Output "Open a new terminal if Sourcebook still reports an older version."
}
finally {
    if (-not [string]::IsNullOrWhiteSpace($StagedBinary) -and (Test-Path -LiteralPath $StagedBinary)) {
        Remove-Item -LiteralPath $StagedBinary -Force
    }
    if (Test-Path -LiteralPath $TemporaryDir) {
        Remove-Item -LiteralPath $TemporaryDir -Recurse -Force
    }
}
