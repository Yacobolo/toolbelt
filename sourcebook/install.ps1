param(
    [string]$Version,
    [string]$BinDir,
    [switch]$Help
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$DefaultVersion = "0.1.0"
$Repository = "Yacobolo/toolbelt"

if ($Help) {
    @"
Install Sourcebook from a GitHub release.

Usage:
  .\install.ps1 [-Version <version>] [-BinDir <directory>]

Environment:
  SOURCEBOOK_VERSION                   Release version (default: 0.1.0)
  SOURCEBOOK_INSTALL_DIR               Install directory
  SOURCEBOOK_RELEASE_BASE_URL          Override release download base URL
"@
    return
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = $env:SOURCEBOOK_VERSION
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = $DefaultVersion
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
    $BinDir = Join-Path $LocalAppData "Programs\Sourcebook"
}
$BinDir = [System.IO.Path]::GetFullPath($BinDir)

$TargetOS = $env:SOURCEBOOK_OS
if ([string]::IsNullOrWhiteSpace($TargetOS)) {
    $IsWindowsPlatform = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [System.Runtime.InteropServices.OSPlatform]::Windows
    )
    if (-not $IsWindowsPlatform) {
        throw "sourcebook installer: install.ps1 supports Windows only"
    }
    $TargetOS = "windows"
}
if ($TargetOS -ne "windows") {
    throw "sourcebook installer: unsupported operating system: $TargetOS"
}

$TargetArch = $env:SOURCEBOOK_ARCH
if ([string]::IsNullOrWhiteSpace($TargetArch)) {
    $DetectedArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
    switch ($DetectedArch) {
        "x64" { $TargetArch = "amd64" }
        "arm64" { $TargetArch = "arm64" }
        default { throw "sourcebook installer: unsupported architecture: $DetectedArch" }
    }
}
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
    $ActualChecksum = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($ActualChecksum -ne $ExpectedChecksum) {
        throw "sourcebook installer: checksum verification failed for $Asset"
    }

    $ExtractedDir = Join-Path $TemporaryDir "extracted"
    Expand-Archive -LiteralPath $Archive -DestinationPath $ExtractedDir
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
}
finally {
    if (-not [string]::IsNullOrWhiteSpace($StagedBinary) -and (Test-Path -LiteralPath $StagedBinary)) {
        Remove-Item -LiteralPath $StagedBinary -Force
    }
    if (Test-Path -LiteralPath $TemporaryDir) {
        Remove-Item -LiteralPath $TemporaryDir -Recurse -Force
    }
}
