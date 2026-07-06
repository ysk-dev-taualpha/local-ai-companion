# ============================================================
# STT Server Startup Script
# ============================================================
# Starts the faster-whisper STT HTTP server on WinPC.
# Listens on 0.0.0.0:8093, accessible from X1C6 (192.168.12.112).
#
# Usage:
#   .\start_stt_server.ps1
#   .\start_stt_server.ps1 -Port 8094
#   .\start_stt_server.ps1 -NoDeps    # skip pip install check
# ============================================================

param(
    [int]   $Port   = 8093,
    [string]$HostIP = "0.0.0.0",
    [switch]$NoDeps
)

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$serverPy  = Join-Path $scriptDir "faster_whisper_server.py"

# ── preflight ──────────────────────────────────────────────────────────

Write-Host "=== STT Server Startup ===" -ForegroundColor Cyan
Write-Host "Script dir : $scriptDir"
Write-Host "Server     : $serverPy"
Write-Host "Listen     : ${HostIP}:${Port}"

# ── dependency check ───────────────────────────────────────────────────

if (-not $NoDeps) {
    Write-Host "`n[1/3] Checking dependencies …" -ForegroundColor Yellow

    $required = @("faster-whisper", "fastapi", "uvicorn", "python-multipart",
                  "soundfile", "numpy")

    $missing = @()
    foreach ($pkg in $required) {
        pip show $pkg 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            $missing += $pkg
        }
    }

    if ($missing.Count -gt 0) {
        Write-Host "  Installing missing packages: $($missing -join ', ')"
        pip install @missing
        if ($LASTEXITCODE -ne 0) {
            Write-Host "  ERROR: pip install failed" -ForegroundColor Red
            exit 1
        }
    } else {
        Write-Host "  All dependencies OK"
    }
} else {
    Write-Host "`n[1/3] Skipping dependency check (-NoDeps)" -ForegroundColor Yellow
}

# ── server start ───────────────────────────────────────────────────────

Write-Host "`n[2/3] Starting faster-whisper server …" -ForegroundColor Yellow
Write-Host "  Model: Systran/faster-whisper-small (CUDA float16)"
Write-Host "  VRAM : ~1.5 GB"
Write-Host "  URL  : http://${HostIP}:${Port}"
Write-Host "  API  : POST /v1/transcribe (multipart WAV)"
Write-Host "  Health: GET /health"
Write-Host ""

$env:PYTHONUNBUFFERED = "1"

uvicorn stt.faster_whisper_server:app `
    --host $HostIP `
    --port $Port `
    --log-level info

# ============================================================
# Task Scheduler Registration Guide
# ============================================================
#
# Steps:
#   1. Win+R → taskschd.msc
#   2. [Create Task] (not Basic Task)
#   3. General tab:
#      - Name: STT Server
#      - Description: faster-whisper STT HTTP server
#      - "Run whether user is logged on or not"
#      - "Run with highest privileges" ✓
#   4. Triggers tab → New:
#      - "At startup"
#      - Delay 30 seconds (wait for GPU driver init)
#   5. Actions tab → New:
#      - Program/script: powershell.exe
#      - Add arguments:
#        -ExecutionPolicy Bypass -File "<REPO>\stt\start_stt_server.ps1"
#      - Start in: <REPO>\stt\
#   6. Conditions tab: uncheck "AC power"
#   7. Settings tab:
#      - Uncheck "Stop the task if it runs longer than"
#      - "If the task is already running…" → "Do not start a new instance"
#
# Test:
#   curl http://192.168.12.107:8093/health
#   curl -X POST -F "file=@test.wav" http://192.168.12.107:8093/v1/transcribe
