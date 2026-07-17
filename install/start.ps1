[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateNotNullOrEmpty()]
    [string]$InstallDir
)

$ErrorActionPreference = "Stop"

$resolvedInstallDir = [System.IO.Path]::GetFullPath($InstallDir)
$logDir = Join-Path $resolvedInstallDir "logs"
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$logPath = Join-Path $logDir "autostart.log"

$deadline = (Get-Date).AddMinutes(2)
do {
    & docker version --format '{{.Server.Version}}' *>> $logPath
    if ($LASTEXITCODE -eq 0) {
        break
    }
    Start-Sleep -Seconds 2
} while ((Get-Date) -lt $deadline)

if ($LASTEXITCODE -ne 0) {
    Add-Content -LiteralPath $logPath -Value "$(Get-Date -Format o) Docker did not become ready within two minutes."
    exit 1
}

& docker compose --project-directory $resolvedInstallDir up --detach --remove-orphans --pull never *>> $logPath
exit $LASTEXITCODE
