param(
    [string]$Config = "config.ollama.example.json",
    [string]$Message = "Reply with only a short valid JSON assistant response."
)

$ErrorActionPreference = "Stop"

try {
    Invoke-RestMethod -Uri "http://127.0.0.1:11434/api/tags" -Method Get | Out-Null
}
catch {
    throw "Ollama is not reachable at http://127.0.0.1:11434. Start Ollama and try again."
}

& (Join-Path $PSScriptRoot "run-cli.ps1") -Config $Config -Message $Message -ConversationId "ollama-smoke" -RequestId "ollama-smoke-1"
