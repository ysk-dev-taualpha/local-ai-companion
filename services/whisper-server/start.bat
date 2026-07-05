@echo off
REM ============================================================
REM  faster-whisper Transcription Server Startup Script (WinPC)
REM
REM  事前セットアップ:
REM    pip install -r requirements.txt
REM
REM  起動: double-click start.bat
REM  停止: Ctrl+C
REM ============================================================

cd /d "%~dp0"
echo [whisper-server] Installing dependencies...
pip install -r requirements.txt

echo.
echo [whisper-server] Starting faster-whisper server (small, CUDA)...
echo [whisper-server] Endpoint: http://192.168.12.107:8093/v1/transcribe
echo [whisper-server] Health:   http://192.168.12.107:8093/health
echo.

python server.py
pause
