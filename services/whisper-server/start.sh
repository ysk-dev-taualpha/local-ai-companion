#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
echo "faster-whisper STT Server v0.5"
DEVICE="cuda"
python -c "import torch; torch.cuda.is_available()" 2>/dev/null || DEVICE="cpu"
exec python server.py --host 0.0.0.0 --port 8093 --device "$DEVICE"
