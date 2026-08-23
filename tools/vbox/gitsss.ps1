[CmdletBinding()]
param(
    [ValidateSet("check", "pull", "load", "install", "verify", "stop", "logs", "remove")]
    [string]$Action = "check",
    [string]$VboxPath = "",
    [string]$Image = "gitsss/gitsss:latest",
    [string]$ImageArchive = "",
    [string]$DataDir = "",
    [ValidateRange(1, 65535)]
    [int]$HttpPort = 3000,
    [ValidateRange(1, 65535)]
    [int]$GitSshPort = 2222,
    [switch]$KeepRunning
)

$ErrorActionPreference = "Stop"
$vboxRoot = "C:\tools\vbox"
$containerName = "gitsss"

if (-not $DataDir) {
    $DataDir = Join-Path $env:LOCALAPPDATA "gitsss\vbox-data"
}

function Resolve-Vbox {
    if ($VboxPath) {
        $resolved = [IO.Path]::GetFullPath($VboxPath)
        if (-not (Test-Path -LiteralPath $resolved -PathType Leaf)) {
            throw "vbox executable not found: $resolved"
        }
        return $resolved
    }

    $command = Get-Command vbox.exe -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    $candidate = Join-Path $vboxRoot "target\release\vbox.exe"
    if (Test-Path -LiteralPath $candidate -PathType Leaf) {
        return $candidate
    }

    $installed = Join-Path $env:LOCALAPPDATA "vbox\bin\vbox.exe"
    if (Test-Path -LiteralPath $installed -PathType Leaf) {
        return $installed
    }

    throw "GitHub neko233-com/vbox was not found. Build/install it from $vboxRoot or pass -VboxPath."
}

function Invoke-Vbox {
    param([Parameter(Mandatory)][string[]]$Arguments)

    $output = & $script:vbox @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "vbox $($Arguments -join ' ') failed with exit code $LASTEXITCODE`n$($output -join "`n")"
    }
    return $output
}

function Ensure-DataDir {
    New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
}

function Invoke-VboxRun {
    if ($ImageArchive) {
        Invoke-VboxLoad
    }
    Ensure-DataDir
    $args = @(
        "run", "--detach", "--name", $containerName,
        "--publish", "$HttpPort`:3000",
        "--publish", "$GitSshPort`:22",
        "--volume", "$DataDir`:/data",
        "--env", "USER_UID=1000",
        "--env", "USER_GID=1000",
        "--env", "GITSSS__server__HTTP_PORT=3000",
        "--env", "GITSSS__server__SSH_PORT=2222",
        "--env", "GITSSS__server__SSH_LISTEN_PORT=22",
        "--env", "GITSSS__server__LFS_START_SERVER=true",
        "--env", "GITSSS__security__INSTALL_LOCK=false",
        "--env", "GITSSS__server__ROOT_URL=http://127.0.0.1:$HttpPort/",
        "--entrypoint", "/usr/local/bin/gitsss",
        $Image,
        "web", "--port", "3000"
    )
    Invoke-Vbox -Arguments $args | Out-Host
}

function Invoke-VboxLoad {
    $resolvedArchive = [IO.Path]::GetFullPath($ImageArchive)
    if (-not (Test-Path -LiteralPath $resolvedArchive -PathType Leaf)) {
        throw "GitSSS image archive not found: $resolvedArchive"
    }
    Invoke-Vbox -Arguments @("load", "--input", $resolvedArchive, "--tag", $Image) | Out-Host
}

function Wait-Health {
    param([int]$TimeoutSeconds = 120)

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$HttpPort/api/healthz" -TimeoutSec 5
            if ($response.StatusCode -eq 200) {
                return
            }
        } catch {
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)

    throw "gitsss did not become healthy at http://127.0.0.1:$HttpPort/api/healthz"
}

$script:vbox = Resolve-Vbox

switch ($Action) {
    "check" {
        Write-Host "Provider: GitHub neko233-com/vbox"
        Write-Host "Source: $vboxRoot"
        Invoke-Vbox -Arguments @("info") | Out-Host
        Invoke-Vbox -Arguments @("doctor", "--json") | Out-Host
    }
    "pull" {
        if ($ImageArchive) {
            Invoke-VboxLoad
        } else {
            Invoke-Vbox -Arguments @("pull", $Image) | Out-Host
        }
    }
    "load" {
        if (-not $ImageArchive) {
            throw "-ImageArchive is required for -Action load"
        }
        Invoke-VboxLoad
    }
    "install" {
        Invoke-VboxRun
        Wait-Health
        Write-Host "gitsss installed by GitHub neko233-com/vbox"
        Write-Host "URL: http://127.0.0.1:$HttpPort/"
        Write-Host "Persistent data: $DataDir"
    }
    "verify" {
        try {
            Invoke-VboxRun
            Wait-Health
            Write-Host "Health check passed: http://127.0.0.1:$HttpPort/api/healthz"
            Invoke-Vbox -Arguments @("inspect", $containerName) | Out-Host
        } catch {
            Write-Host "vbox container logs:" -ForegroundColor Yellow
            & $script:vbox logs $containerName 2>&1 | Out-Host
            throw
        } finally {
            if (-not $KeepRunning) {
                & $script:vbox stop $containerName 2>&1 | Out-Host
            }
        }
    }
    "stop" {
        Invoke-Vbox -Arguments @("stop", $containerName) | Out-Host
    }
    "logs" {
        Invoke-Vbox -Arguments @("logs", $containerName) | Out-Host
    }
    "remove" {
        Invoke-Vbox -Arguments @("rm", $containerName, "--force") | Out-Host
        Write-Host "Container removed; persistent data was kept at $DataDir"
    }
}
