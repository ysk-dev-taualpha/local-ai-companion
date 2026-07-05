# faster-whisper STT Server 起動スクリプト (WinPC)
#
# 初回セットアップ（管理者 PowerShell で実行）:
#   pip install faster-whisper fastapi uvicorn python-multipart
#
# 配置:
#   WinPC の local-ai-companion\stt\ に配置
#
# 起動方法:
#   .\start_stt_server.ps1
#   または PowerShell から直接:
#   python faster_whisper_server.py
#
# Task Scheduler 登録（ログオン時自動起動）:
#   1. taskschd.msc を開く
#   2. 「タスクの作成」
#   3. トリガー: ログオン時
#   4. 操作: プログラムの開始 → powershell.exe
#      引数: -ExecutionPolicy Bypass -File "C:\path\to\local-ai-companion\stt\start_stt_server.ps1"
#   5. 「ユーザーがログオンしているかどうかにかかわらず実行する」にチェック
#       → 「パスワードを保存しない」にチェック

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

Write-Host "=== faster-whisper STT Server ===" -ForegroundColor Cyan
Write-Host "Model: small (CUDA float16)" -ForegroundColor Yellow
Write-Host "Port: 8093" -ForegroundColor Yellow
Write-Host "GPU VRAM usage: ~1.5GB (total with Ollama: ~7.5GB/8GB)" -ForegroundColor DarkYellow
Write-Host ""

# CUDA が使えるか簡易チェック
$cudaCheck = python -c "import torch; print(torch.cuda.is_available())" 2>$null
if ($cudaCheck -ne "True") {
    Write-Host "[WARN] CUDA not available in PyTorch. Check torch installation." -ForegroundColor Red
    Write-Host "[WARN] Server will fall back to CPU (slow)." -ForegroundColor Red
}

Write-Host "[start] Launching server on 0.0.0.0:8093..."
python faster_whisper_server.py
