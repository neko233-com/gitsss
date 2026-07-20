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
    [string]$InitialAdminUser = "root",
    [string]$InitialAdminPassword = "root",
    [switch]$NoStart,
    [switch]$NoAutostart
)

$ErrorActionPreference = "Stop"

foreach ($value in @($RootUrl, $JobInstanceUrl, $Image, $DataVolume, $InstallDir, $InitialAdminUser, $InitialAdminPassword)) {
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
$sourceStartScript = Join-Path $PSScriptRoot "start.ps1"
if (-not (Test-Path -LiteralPath $sourceStartScript -PathType Leaf)) {
    throw "start.ps1 not found beside installer"
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
New-Item -ItemType Directory -Force -Path (Join-Path $resolvedInstallDir "logs") | Out-Null
Copy-Item -LiteralPath $sourceCompose -Destination (Join-Path $resolvedInstallDir "compose.yaml") -Force
$installedStartScript = Join-Path $resolvedInstallDir "start.ps1"
Copy-Item -LiteralPath $sourceStartScript -Destination $installedStartScript -Force

$envPath = Join-Path $resolvedInstallDir ".env"
$envTempPath = Join-Path $resolvedInstallDir (".env." + [System.Guid]::NewGuid().ToString("N"))
$envContent = @(
    "GITSSS_IMAGE=$Image"
    "GITSSS_ACTIONS_JOB_IMAGE=$Image"
    "GITSSS_HTTP_PORT=$Port"
    "GITSSS_ROOT_URL=$RootUrl"
    "GITSSS_ACTIONS_JOB_INSTANCE_URL=$JobInstanceUrl"
    "GITSSS_DATA_VOLUME=$DataVolume"
    "GITSSS_INITIAL_ADMIN_USER=$InitialAdminUser"
    "GITSSS_INITIAL_ADMIN_PASSWORD=$InitialAdminPassword"
    "GITSSS_MIN_PASSWORD_LENGTH=4"
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

$taskName = "GitSSS Background"
$startupVbsPath = Join-Path ([Environment]::GetFolderPath("Startup")) "GitSSS Background.vbs"
$autostartMode = $null
if ($NoAutostart) {
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $startupVbsPath -Force -ErrorAction SilentlyContinue
} elseif (-not $NoStart) {
    $powershellPath = (Get-Command powershell.exe -ErrorAction Stop).Source
    $taskArgument = '-NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}" -InstallDir "{1}"' -f $installedStartScript, $resolvedInstallDir
    $taskAction = New-ScheduledTaskAction -Execute $powershellPath -Argument $taskArgument
    $taskTrigger = New-ScheduledTaskTrigger -AtLogOn
    $taskSettings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -Hidden -MultipleInstances IgnoreNew
    try {
        Register-ScheduledTask -TaskName $taskName -Action $taskAction -Trigger $taskTrigger -Settings $taskSettings -Description "Starts GitSSS silently after Docker Desktop is available." -Force -ErrorAction Stop | Out-Null
        Remove-Item -LiteralPath $startupVbsPath -Force -ErrorAction SilentlyContinue
        $autostartMode = "Windows Task Scheduler task '$taskName' (hidden, at logon)"
    } catch {
        $startupCommand = '"{0}" {1}' -f $powershellPath, $taskArgument
        $startupVbs = 'CreateObject("WScript.Shell").Run "' + $startupCommand.Replace('"', '""') + '", 0, False'
        [System.IO.File]::WriteAllText($startupVbsPath, $startupVbs + [Environment]::NewLine)
        $autostartMode = "hidden WScript launcher in the current user's Startup folder"
    }
}

Write-Host "Installed: $resolvedInstallDir"
Write-Host "URL: $RootUrl"
if ($autostartMode) {
    Write-Host "Autostart: $autostartMode"
}
Write-Host "Switch port without data migration:"
Write-Host "  .\install.ps1 -InstallDir `"$resolvedInstallDir`" -DataVolume `"$DataVolume`" -Port 3000 -RootUrl `"http://localhost:3000/`""
