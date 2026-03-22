$ErrorActionPreference = 'Stop'

$pluginId = 'com.mattermost.login-help-mailer'
$version = '0.1.1'
$workspaceRoot = Split-Path -Parent $PSScriptRoot
$portableGo = Join-Path $workspaceRoot 'tools\\go\\bin\\go.exe'
$goExe = if (Test-Path $portableGo) { $portableGo } else { 'go' }

$env:GOTOOLCHAIN = 'local'
$env:GOCACHE = Join-Path $workspaceRoot 'tools\\gocache'
$env:GOMODCACHE = Join-Path $workspaceRoot 'tools\\gomodcache'
$env:CGO_ENABLED = '0'

New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
New-Item -ItemType Directory -Force -Path $env:GOMODCACHE | Out-Null

$distRoot = Join-Path $PSScriptRoot 'dist'
$pluginRoot = Join-Path $distRoot $pluginId
$serverDist = Join-Path $pluginRoot 'server\\dist'
$assetsDist = Join-Path $pluginRoot 'assets'

if (Test-Path $distRoot) {
    Remove-Item -Recurse -Force $distRoot
}

New-Item -ItemType Directory -Force -Path $serverDist | Out-Null
New-Item -ItemType Directory -Force -Path $assetsDist | Out-Null

$targets = @(
    @{ GOOS = 'linux'; GOARCH = 'amd64'; Output = 'plugin-linux-amd64' },
    @{ GOOS = 'linux'; GOARCH = 'arm64'; Output = 'plugin-linux-arm64' },
    @{ GOOS = 'darwin'; GOARCH = 'amd64'; Output = 'plugin-darwin-amd64' },
    @{ GOOS = 'darwin'; GOARCH = 'arm64'; Output = 'plugin-darwin-arm64' },
    @{ GOOS = 'windows'; GOARCH = 'amd64'; Output = 'plugin-windows-amd64.exe' }
)

Push-Location $PSScriptRoot
try {
    foreach ($target in $targets) {
        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH
        & $goExe build -o (Join-Path $serverDist $target.Output) ./server
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $($target.GOOS)/$($target.GOARCH)"
        }
    }

    Copy-Item -Path (Join-Path $PSScriptRoot 'plugin.json') -Destination $pluginRoot
    Copy-Item -Path (Join-Path $PSScriptRoot 'assets\\icon.svg') -Destination $assetsDist

    $archivePath = Join-Path $distRoot "$pluginId-$version.tar.gz"
    & $goExe run ./build/package_plugin.go --source $pluginRoot --output $archivePath
    if ($LASTEXITCODE -ne 0) {
        throw 'failed to create tar.gz bundle'
    }
}
finally {
    Pop-Location
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
}

Write-Host "Bundle created at $archivePath"
