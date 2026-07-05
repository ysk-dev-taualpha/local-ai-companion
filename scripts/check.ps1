param(
    [switch]$SkipGo
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")

& (Join-Path $PSScriptRoot "test-python.ps1")

if (-not $SkipGo) {
    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($go) {
        Push-Location $repoRoot
        try {
            go test ./...
        }
        finally {
            Pop-Location
        }
    }
    else {
        Write-Warning "Go is not on PATH; skipped Go tests. Install Go or rerun with -SkipGo to make this explicit."
    }
}
