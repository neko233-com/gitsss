[CmdletBinding()]
param(
    [ValidateRange(1, 65535)]
    [int]$Port = 3000,
    [string]$RootUrl = "",
    [string]$JobInstanceUrl = "",
    [string]$Image = "gitsss/gitsss:latest",
    [string]$ImageArchive = "",
    [string]$DataVolume = "gitsss-data",
    [string]$InstallDir = (Join-Path (Get-Location) "gitsss-install"),
    [switch]$NoStart
)

$ErrorActionPreference = "Stop"

foreach ($value in @($RootUrl, $JobInstanceUrl, $Image, $DataVolume)) {
    if ($value.Contains("`n") -or $value.Contains("`r")) {
        throw "Configuration values cannot contain newlines"
    }
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "docker is required"
}
docker compose version | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "docker compose is required"
}

$sourceCompose = Join-Path (Split-Path $PSScriptRoot -Parent) "compose.yaml"
if (-not (Test-Path -LiteralPath $sourceCompose -PathType Leaf)) {
    throw "compose.yaml not found beside installer"
}

if ($ImageArchive) {
    if (-not (Test-Path -LiteralPath $ImageArchive -PathType Leaf)) {
        throw "Image archive not found: $ImageArchive"
    }
    docker load --input $ImageArchive
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to load image archive"
    }
}

docker image inspect $Image *> $null
if ($LASTEXITCODE -ne 0) {
    throw "Image $Image is unavailable. Load offline archive with -ImageArchive."
}

if (-not $RootUrl) {
    $RootUrl = "http://localhost:$Port/"
}
if (-not $RootUrl.EndsWith("/")) {
    $RootUrl += "/"
}
if (-not $JobInstanceUrl) {
    $JobInstanceUrl = "http://host.docker.internal:$Port"
}

$resolvedInstallDir = [System.IO.Path]::GetFullPath($InstallDir)
New-Item -ItemType Directory -Force -Path $resolvedInstallDir | Out-Null
Copy-Item -LiteralPath $sourceCompose -Destination (Join-Path $resolvedInstallDir "compose.yaml") -Force

$envPath = Join-Path $resolvedInstallDir ".env"
$envTempPath = Join-Path $resolvedInstallDir (".env." + [System.Guid]::NewGuid().ToString("N"))
$envContent = @(
    "GITSSS_IMAGE=$Image"
    "GITSSS_ACTIONS_JOB_IMAGE=$Image"
    "GITSSS_HTTP_PORT=$Port"
    "GITSSS_ROOT_URL=$RootUrl"
    "GITSSS_ACTIONS_JOB_INSTANCE_URL=$JobInstanceUrl"
    "GITSSS_DATA_VOLUME=$DataVolume"
) -join [Environment]::NewLine

try {
    [System.IO.File]::WriteAllText($envTempPath, $envContent + [Environment]::NewLine)
    Move-Item -LiteralPath $envTempPath -Destination $envPath -Force
} finally {
    Remove-Item -LiteralPath $envTempPath -Force -ErrorAction SilentlyContinue
}

if (-not $NoStart) {
    docker compose --project-directory $resolvedInstallDir up --detach --force-recreate --pull never
    if ($LASTEXITCODE -ne 0) {
        throw "gitsss startup failed"
    }
}

Write-Host "Installed: $resolvedInstallDir"
Write-Host "URL: $RootUrl"
Write-Host "Switch port without data migration:"
Write-Host "  .\install.ps1 -InstallDir `"$resolvedInstallDir`" -DataVolume `"$DataVolume`" -Port 3000 -RootUrl `"http://localhost:3000/`""
