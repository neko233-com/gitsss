[CmdletBinding()]
param(
    [string]$Version = "",
    [string]$OutputDirectory = "",
    [switch]$SkipTests,
    [string]$SigningKeyEnvironment = "",
    [string]$UploadTag = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repository = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$utf8WithoutBom = New-Object Text.UTF8Encoding($false)

function Invoke-Checked {
    param(
        [Parameter(Mandatory)][string]$Executable,
        [Parameter(Mandatory)][string[]]$Arguments,
        [Parameter(Mandatory)][string]$WorkingDirectory
    )

    Push-Location $WorkingDirectory
    try {
        & $Executable @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "$Executable exited with code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }
}

Invoke-Checked git @("diff", "--quiet") $repository
Invoke-Checked git @("diff", "--cached", "--quiet") $repository
$commit = (& git -C $repository rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $commit -notmatch '^[0-9a-f]{40}$') {
    throw "Unable to resolve the source commit"
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = "dev-$($commit.Substring(0, 7))"
}
if ($Version -notmatch '^[0-9A-Za-z][0-9A-Za-z._-]*$') {
    throw "Version may contain only letters, digits, dot, underscore and hyphen"
}
if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
    $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $OutputDirectory = Join-Path ([IO.Path]::GetTempPath()) "gitsss-release-$Version-$timestamp"
}
$OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)
if (Test-Path -LiteralPath $OutputDirectory) {
    throw "Output directory already exists: $OutputDirectory"
}

$stageDirectory = Join-Path $OutputDirectory "stage"
$releaseDirectory = Join-Path $OutputDirectory "release"
New-Item -ItemType Directory -Path $stageDirectory, $releaseDirectory | Out-Null

if (-not $SkipTests) {
    Invoke-Checked pnpm @("install", "--frozen-lockfile") (Join-Path $repository "web")
    Invoke-Checked pnpm @("run", "lint") (Join-Path $repository "web")
    Invoke-Checked pnpm @("run", "typecheck") (Join-Path $repository "web")
    Invoke-Checked pnpm @("run", "test") (Join-Path $repository "web")
    Invoke-Checked pnpm @("run", "build") (Join-Path $repository "web")

    $previousCgo = $env:CGO_ENABLED
    try {
        $env:CGO_ENABLED = "1"
        Invoke-Checked go @("test", "-race", "./...") (Join-Path $repository "server")
        Invoke-Checked go @("vet", "./...") (Join-Path $repository "server")
    }
    finally {
        $env:CGO_ENABLED = $previousCgo
    }
}
elseif (-not (Test-Path -LiteralPath (Join-Path $repository "web/dist"))) {
    throw "web/dist is required when -SkipTests is used"
}

$targets = @(
    @{ OS = "linux"; Architecture = "amd64"; Archive = "tar.gz" },
    @{ OS = "linux"; Architecture = "arm64"; Archive = "tar.gz" },
    @{ OS = "darwin"; Architecture = "amd64"; Archive = "tar.gz" },
    @{ OS = "darwin"; Architecture = "arm64"; Archive = "tar.gz" },
    @{ OS = "windows"; Architecture = "amd64"; Archive = "zip" },
    @{ OS = "windows"; Architecture = "arm64"; Archive = "zip" }
)

$previousGoOS = $env:GOOS
$previousGoArch = $env:GOARCH
$previousCgo = $env:CGO_ENABLED
try {
    foreach ($target in $targets) {
        $goos = $target.OS
        $goarch = $target.Architecture
        $name = "gitsss-$Version-$goos-$goarch"
        $packageDirectory = Join-Path $stageDirectory $name
        $webDirectory = Join-Path $packageDirectory "web"
        $docsDirectory = Join-Path $packageDirectory "docs"
        New-Item -ItemType Directory -Path $webDirectory, $docsDirectory | Out-Null

        $suffix = if ($goos -eq "windows") { ".exe" } else { "" }
        $env:GOOS = $goos
        $env:GOARCH = $goarch
        $env:CGO_ENABLED = "0"
        Invoke-Checked go @("build", "-trimpath", "-ldflags=-s -w", "-o", (Join-Path $packageDirectory "gitsss$suffix"), "./cmd/gitsss") (Join-Path $repository "server")
        Invoke-Checked go @("build", "-trimpath", "-ldflags=-s -w", "-o", (Join-Path $packageDirectory "gitsss-agent$suffix"), "./cmd/gitsss-agent") (Join-Path $repository "server")

        foreach ($binary in @("gitsss$suffix", "gitsss-agent$suffix")) {
            $metadata = (& go version -m (Join-Path $packageDirectory $binary)) -join "`n"
            if ($LASTEXITCODE -ne 0 -or $metadata -notmatch "GOOS=$goos" -or $metadata -notmatch "GOARCH=$goarch") {
                throw "Architecture verification failed for $name/$binary"
            }
        }

        Copy-Item -Recurse -Force (Join-Path $repository "web/dist/*") $webDirectory
        Copy-Item -Force (Join-Path $repository "server/README.md") (Join-Path $packageDirectory "README.md")
        Copy-Item -Force (Join-Path $repository "docs/operations/agent-autostart.md") (Join-Path $docsDirectory "agent-autostart.md")
        $buildInfo = [ordered]@{
            version = $Version
            commit = $commit
            goos = $goos
            goarch = $goarch
        } | ConvertTo-Json
        [IO.File]::WriteAllText((Join-Path $packageDirectory "BUILDINFO.json"), $buildInfo + "`n", $utf8WithoutBom)

        $archivePath = Join-Path $releaseDirectory "$name.$($target.Archive)"
        if ($target.Archive -eq "zip") {
            Compress-Archive -Path $packageDirectory -DestinationPath $archivePath -CompressionLevel Optimal
        }
        else {
            Invoke-Checked tar @("-C", $stageDirectory, "-czf", $archivePath, $name) $repository
        }
    }
}
finally {
    $env:GOOS = $previousGoOS
    $env:GOARCH = $previousGoArch
    $env:CGO_ENABLED = $previousCgo
}

if (-not [string]::IsNullOrWhiteSpace($SigningKeyEnvironment)) {
    if ($SigningKeyEnvironment -notmatch '^[A-Z_][A-Z0-9_]{0,127}$') {
        throw "SigningKeyEnvironment must be a safe environment variable name"
    }
    if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($SigningKeyEnvironment))) {
        throw "$SigningKeyEnvironment is empty"
    }
    $publishedAt = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
    Invoke-Checked go @(
        "run", "./cmd/gitsss-release", "manifest",
        "--release-dir", $releaseDirectory,
        "--version", $Version,
        "--commit", $commit,
        "--published-at", $publishedAt,
        "--private-key-env", $SigningKeyEnvironment
    ) (Join-Path $repository "server")
}

$manifestPath = Join-Path $releaseDirectory "SHA256SUMS"
$manifest = Get-ChildItem -LiteralPath $releaseDirectory -File |
    Where-Object Name -ne "SHA256SUMS" |
    Sort-Object Name |
    ForEach-Object {
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
        "$hash  $($_.Name)"
    }
[IO.File]::WriteAllLines($manifestPath, $manifest, $utf8WithoutBom)

$resolvedStage = [IO.Path]::GetFullPath($stageDirectory)
$resolvedOutput = [IO.Path]::GetFullPath($OutputDirectory).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
if (-not $resolvedStage.StartsWith($resolvedOutput, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Unsafe staging cleanup path: $resolvedStage"
}
Remove-Item -LiteralPath $resolvedStage -Recurse -Force

if (-not [string]::IsNullOrWhiteSpace($UploadTag)) {
    if ($UploadTag -notmatch '^v[0-9A-Za-z][0-9A-Za-z._-]*$') {
        throw "UploadTag must start with v and contain only release-safe characters"
    }
    $assets = Get-ChildItem -LiteralPath $releaseDirectory -File | Sort-Object Name | ForEach-Object FullName
    Invoke-Checked gh @("release", "create", $UploadTag, "--repo", "neko233-com/gitsss", "--target", $commit, "--title", "gitsss $UploadTag", "--generate-notes", "--prerelease", "--draft") $repository
    foreach ($asset in $assets) {
        Invoke-Checked gh @("release", "upload", $UploadTag, $asset, "--repo", "neko233-com/gitsss") $repository
    }

    $remote = (& gh release view $UploadTag --repo neko233-com/gitsss --json assets,isDraft,targetCommitish | ConvertFrom-Json)
    if ($LASTEXITCODE -ne 0 -or -not $remote.isDraft -or $remote.targetCommitish -ne $commit -or $remote.assets.Count -ne $assets.Count) {
        throw "Draft release metadata does not match the local release"
    }
    foreach ($asset in $remote.assets) {
        $localPath = Join-Path $releaseDirectory $asset.name
        if (-not (Test-Path -LiteralPath $localPath)) {
            throw "Remote release contains unexpected asset: $($asset.name)"
        }
        $localDigest = (Get-FileHash -Algorithm SHA256 -LiteralPath $localPath).Hash.ToLowerInvariant()
        if ($asset.digest -ne "sha256:$localDigest") {
            throw "Remote digest does not match: $($asset.name)"
        }
    }
    Invoke-Checked gh @("release", "edit", $UploadTag, "--repo", "neko233-com/gitsss", "--draft=false", "--prerelease") $repository
}

Write-Host "Release artifacts: $releaseDirectory"
