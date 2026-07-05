param(
    [string]$Config,
    [string]$Message,
    [string]$ConversationId,
    [string]$RequestId,
    [string]$LogDir
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$env:PYTHONPATH = Join-Path $repoRoot "src"

$argsList = @()
if ($Config) {
    $argsList += @("--config", $Config)
}
if ($Message) {
    $argsList += @("--message", $Message)
}
if ($ConversationId) {
    $argsList += @("--conversation-id", $ConversationId)
}
if ($RequestId) {
    $argsList += @("--request-id", $RequestId)
}
if ($LogDir) {
    $argsList += @("--log-dir", $LogDir)
}

Push-Location $repoRoot
try {
    python -m local_ai_companion @argsList
}
finally {
    Pop-Location
}
