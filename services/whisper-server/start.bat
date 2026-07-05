@echo off
cd /d "%~dp0"
echo faster-whisper STT Server v0.5
echo Port: 8093 | Model: small | Device: cuda
python server.py --host 0.0.0.0 --port 8093
