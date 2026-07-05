$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$env:PYTHONPATH = Join-Path $repoRoot "src"

Push-Location $repoRoot
try {
    python -m unittest discover -s tests -v
}
finally {
    Pop-Location
}
