param(
    [string]$Config = "config.example.json"
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$env:PYTHONPATH = Join-Path $repoRoot "src"

Push-Location $repoRoot
try {
    python -m local_ai_companion --config $Config --serve
}
finally {
    Pop-Location
}
