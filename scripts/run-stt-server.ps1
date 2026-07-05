# faster-whisper STT サーバー起動スクリプト (WinPC)
#
# 必要条件:
#   pip install faster-whisper fastapi uvicorn python-multipart
#
# GPU メモリ使用量 (RTX 2080 SUPER 8GB):
#   Ollama g4v100   ~6.0 GB
#   faster-whisper  ~1.5 GB

$ErrorActionPreference = "Stop"
$port = 8093

$existing = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue
if ($existing) {
    Write-Warning "Port $port is already in use. Stopping existing process..."
    Stop-Process -Id $existing.OwningProcess -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$env:PYTHONPATH = Join-Path $repoRoot "src"

Write-Host "=== faster-whisper STT Server ===" -ForegroundColor Cyan
Write-Host "Model: small | Device: cuda | Compute: float16" -ForegroundColor Gray
Write-Host "Listening on 0.0.0.0:${port}" -ForegroundColor Gray
Write-Host "Health check: http://192.168.12.107:${port}/health" -ForegroundColor Gray
Write-Host ""

Push-Location $repoRoot
try {
    python -m uvicorn local_ai_companion.stt_server:create_app `
        --factory `
        --host 0.0.0.0 `
        --port $port `
        --log-level info
}
finally {
    Pop-Location
}
