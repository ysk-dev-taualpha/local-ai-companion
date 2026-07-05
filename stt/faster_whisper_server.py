#!/usr/bin/env python3
"""
faster-whisper STT HTTP Server (WinPC)

WinPC (192.168.12.107) 上で稼働する faster-whisper Speech-to-Text サーバー。
X1C6 の local-ai-companion Go Runtime から HTTP で利用される。

要件:
- Python 3.11+ (WinPC の Torch 2.12+cu130 環境)
- pip install faster-whisper fastapi uvicorn python-multipart
- GPU メモリ: faster-whisper small ~1.5GB (Ollama 6GB との合計 ~7.5GB / 8GB)

起動:
    python faster_whisper_server.py
    # または start_stt_server.ps1 から起動

API:
    POST /v1/transcribe
    Content-Type: multipart/form-data
    file: <WAV audio (16kHz mono)>
    language: "ja"

    Response 200:
    {"text": "認識テキスト", "duration": 3.2, "error": null}

    Response 500:
    {"text": "", "duration": 0, "error": "error message"}
"""

import tempfile
import time
from pathlib import Path

from fastapi import FastAPI, File, Form, UploadFile
from faster_whisper import WhisperModel
import uvicorn

# ---- 設定 ----
MODEL_SIZE = "small"
DEVICE = "cuda"
COMPUTE_TYPE = "float16"
HOST = "0.0.0.0"
PORT = 8093
SUPPORTED_LANGUAGES = {"ja", "en", "zh", "ko", "auto"}

app = FastAPI(title="faster-whisper STT Server", version="0.1.0")

# ---- モデルプリロード ----
print(f"[init] Loading model '{MODEL_SIZE}' on {DEVICE} ({COMPUTE_TYPE})...")
model: WhisperModel | None = None
_load_error: str | None = None

try:
    model = WhisperModel(MODEL_SIZE, device=DEVICE, compute_type=COMPUTE_TYPE)
    print(f"[init] Model loaded successfully.")
except Exception as e:
    _load_error = str(e)
    print(f"[init] ERROR loading model: {e}")
    print("[init] Server will start but requests will return 503.")


@app.on_event("startup")
async def startup():
    if _load_error:
        print(f"[startup] WARNING: Model not loaded. Reason: {_load_error}")


@app.get("/health")
async def health():
    """ヘルスチェックエンドポイント"""
    return {
        "status": "ok" if model is not None else "degraded",
        "model": MODEL_SIZE if model is not None else None,
        "device": DEVICE,
        "error": _load_error,
    }


@app.post("/v1/transcribe")
async def transcribe(
    file: UploadFile = File(...),
    language: str = Form(default="ja"),
):
    """
    WAV 音声ファイルを受け取り、faster-whisper で文字起こしする。

    Args:
        file: WAV audio (16kHz mono 推奨)
        language: 言語コード ("ja", "en", "zh", "ko", "auto")

    Returns:
        {"text": str, "duration": float, "error": str | None}
    """
    # モデル未ロード時は 503
    if model is None:
        return {
            "text": "",
            "duration": 0,
            "error": f"Model not loaded: {_load_error}",
        }

    # 言語バリデーション
    lang = language if language in SUPPORTED_LANGUAGES else "ja"

    # アップロードファイルを一時ファイルに保存
    suffix = Path(file.filename).suffix if file.filename else ".wav"
    with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as tmp:
        tmp.write(await file.read())
        tmp_path = tmp.name

    try:
        start = time.monotonic()
        segments, info = model.transcribe(
            tmp_path,
            language=None if lang == "auto" else lang,
            beam_size=5,
        )
        # segments はジェネレータ。全セグメントをリスト化
        segments_list = list(segments)
        elapsed = time.monotonic() - start

        text = "".join(seg.text for seg in segments_list).strip()

        print(
            f"[transcribe] lang={lang} "
            f"duration={info.duration:.1f}s "
            f"elapsed={elapsed:.2f}s "
            f"text_len={len(text)}"
        )

        return {
            "text": text,
            "duration": round(elapsed, 2),
            "error": None,
        }
    except Exception as e:
        print(f"[transcribe] ERROR: {e}")
        return {
            "text": "",
            "duration": 0,
            "error": str(e),
        }
    finally:
        # 一時ファイル削除
        Path(tmp_path).unlink(missing_ok=True)


if __name__ == "__main__":
    print(f"[server] Starting on {HOST}:{PORT}")
    uvicorn.run(app, host=HOST, port=PORT, log_level="info")
