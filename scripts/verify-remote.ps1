# Read-only handoff validation for an internal Gitsss-compatible server.
# The password is read from GITSSS_REMOTE_PASSWORD or a hidden prompt and is
# never written to the console or a repository URL.
param(
  [string] $BaseUrl = "http://192.168.110.42:3000",
  [string] $Repo = "poke-game/server-project-sf-go",
  [string] $Username = "robot_cicd",
  [string] $Password = "",
  [switch] $NoClone
)

$ErrorActionPreference = "Stop"
$base = $BaseUrl.TrimEnd('/')
$repoPath = $Repo.Trim('/')

if (-not $Password) { $Password = $env:GITSSS_REMOTE_PASSWORD }
if (-not $Password) {
  $secure = Read-Host "Remote password" -AsSecureString
  $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
  try { $Password = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
  finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
}

$auth = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("${Username}:${Password}"))
$headers = @{ Authorization = "Basic $auth" }

function Test-Endpoint([string] $Path) {
  $uri = "$base$Path"
  $response = Invoke-WebRequest -Uri $uri -Headers $headers -UseBasicParsing
  if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 300) {
    throw "endpoint failed: $Path ($($response.StatusCode))"
  }
  Write-Host "[remote] $($response.StatusCode) $Path"
}

Test-Endpoint "/api/healthz"
Test-Endpoint "/api/swagger"
Test-Endpoint "/$repoPath"

$remote = "$base/$repoPath.git"
$headerConfig = "http.extraheader=Authorization: Basic $auth"
$env:GIT_TERMINAL_PROMPT = "0"
git -c $headerConfig ls-remote $remote refs/heads/main refs/heads/master
if ($LASTEXITCODE -ne 0) { throw "git ls-remote failed" }
Write-Host "[remote] smart HTTP refs passed"

if (-not $NoClone) {
  $tempRoot = Join-Path ([IO.Path]::GetTempPath()) "gitsss-remote-verify-$PID"
  New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
  try {
    git -c $headerConfig clone --depth 1 --filter=blob:none $remote (Join-Path $tempRoot "repo")
    if ($LASTEXITCODE -ne 0) { throw "read-only clone failed" }
    $workflowFiles = Get-ChildItem -Path (Join-Path $tempRoot "repo") -Recurse -File -Force |
      Where-Object { $_.FullName -match '[\\/](\.gitea|\.github)[\\/]workflows[\\/].+\.(yml|yaml)$' }
    Write-Host "[remote] workflow files: $($workflowFiles.Count)"
    if ($workflowFiles.Count -eq 0) { throw "no workflow files found" }
    if (Get-Command git-lfs -ErrorAction SilentlyContinue) { git lfs version }
  }
  finally {
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
  }
}

Write-Host "[remote] read-only validation passed"
