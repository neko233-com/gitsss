# gitsss-run.ps1
# Kill any stale gitsss processes bound to the port (so old PIDs never hold
# port 3000), then start the server. Usage:
#   .\scripts\gitsss-run.ps1                 # default port 3000, exe gitsss.exe, args "web"
#   .\scripts\gitsss-run.ps1 -Port 3001 -Exe gitsss.exe -ExtraArgs "web --port 3001"
param(
  [int]    $Port = 3000,
  [string] $Exe = "gitsss.exe",
  [string] $ExtraArgs = "web",
  [switch] $NoLaunch   # cleanup only, do not start the server
)

$ErrorActionPreference = "SilentlyContinue"

function Kill-PortOwner {
  param([int]$p)
  $conns = Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue
  foreach ($c in $conns) {
    $owner = $c.OwningProcess
    if ($owner -and $owner -ne $PID) {
      Write-Host "[gitsss-run] killing stale process on port $p (PID $owner)"
      Stop-Process -Id $owner -Force
    }
  }
}

# 1) Kill whatever holds the port
Kill-PortOwner -p $Port
Start-Sleep -Seconds 1

# 2) Kill any leftover gitsss* processes (covers stale builds from earlier runs)
Get-Process -Name gitsss* -ErrorAction SilentlyContinue |
  Where-Object { $_.Id -ne $PID } |
  ForEach-Object {
    Write-Host "[gitsss-run] killing stale gitsss process (PID $($_.Id))"
    Stop-Process -Id $_.Id -Force
  }
Start-Sleep -Seconds 1

if ($NoLaunch) {
  Write-Host "[gitsss-run] cleanup done; -NoLaunch specified, not starting server"
  exit 0
}

# 3) Start fresh
Write-Host "[gitsss-run] starting $Exe $ExtraArgs"
$argsArray = ($ExtraArgs -split '\s+') | Where-Object { $_ -ne '' }
& ".\$Exe" @argsArray
