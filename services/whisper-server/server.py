"""faster-whisper HTTP Transcription Server (WinPC v0.5 STT Engine).

Provides /v1/transcribe endpoint for voice input transcription.
Model: small, CUDA float16, ~1.5GB GPU, preloaded at startup.
Host: 0.0.0.0:8093 (accessible from X1C6 at 192.168.12.112).

Usage:
    python server.py [--port 8093] [--host 0.0.0.0] [--model small]
"""

import argparse
import io
import logging
import os
import time

import uvicorn
from fastapi import FastAPI, File, Form, HTTPException, UploadFile

from faster_whisper import WhisperModel

# Logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("whisper-server")

# Configuration (env vars override defaults)
MODEL_SIZE = os.environ.get("WHISPER_MODEL", "small")
DEVICE = os.environ.get("WHISPER_DEVICE", "cuda")
COMPUTE_TYPE = os.environ.get("WHISPER_COMPUTE_TYPE", "float16")
HOST = os.environ.get("WHISPER_HOST", "0.0.0.0")
PORT = int(os.environ.get("WHISPER_PORT", "8093"))

app = FastAPI(title="faster-whisper Transcription Server", version="1.0.0")

# Model — preloaded at module level (import time)
_model = None
_load_error = None

logger.info(
    "Loading faster-whisper model: %s (device=%s, compute_type=%s)",
    MODEL_SIZE, DEVICE, COMPUTE_TYPE,
)
try:
    _model = WhisperModel(MODEL_SIZE, device=DEVICE, compute_type=COMPUTE_TYPE)
    logger.info("Model loaded successfully")
except Exception as exc:
    _load_error = str(exc)
    logger.error("Failed to load model: %s", exc)


@app.get("/health")
async def health():
    if _model is not None:
        return {"status": "ok", "model": MODEL_SIZE, "device": DEVICE}
    return {"status": "degraded", "model": None, "error": _load_error}


@app.post("/v1/transcribe")
async def transcribe(
    file: UploadFile = File(...),
    language: str = Form("ja"),
):
    if _model is None:
        return {"text": "", "duration": 0, "error": f"Model not loaded: {_load_error}"}

    start = time.monotonic()

    try:
        audio_bytes = await file.read()
        if not audio_bytes:
            return {"text": "", "duration": 0, "error": "Empty audio file"}

        logger.info(
            "Transcribing %s: %d bytes, language=%s",
            file.filename, len(audio_bytes), language,
        )

        segments, info = _model.transcribe(
            io.BytesIO(audio_bytes),
            language=language,
            beam_size=5,
            vad_filter=True,
            vad_parameters=dict(min_silence_duration_ms=500),
        )

        text = "".join(seg.text for seg in segments)
        elapsed = time.monotonic() - start

        logger.info("Transcription complete: %.1fs, text_len=%d", elapsed, len(text))

        return {
            "text": text.strip(),
            "duration": round(elapsed, 3),
            "error": None,
        }

    except Exception as exc:
        elapsed = time.monotonic() - start
        logger.exception("Transcription failed")
        raise HTTPException(
            status_code=500,
            detail={
                "text": "",
                "duration": round(elapsed, 3),
                "error": str(exc),
            },
        )


def main():
    global MODEL_SIZE, DEVICE, COMPUTE_TYPE, HOST, PORT

    parser = argparse.ArgumentParser(description="faster-whisper STT Server")
    parser.add_argument("--port", type=int, default=PORT)
    parser.add_argument("--host", type=str, default=HOST)
    parser.add_argument("--model", type=str, default=MODEL_SIZE)
    parser.add_argument("--device", type=str, default=DEVICE)
    parser.add_argument("--compute-type", type=str, default=COMPUTE_TYPE, dest="compute_type")
    args = parser.parse_args()

    if "WHISPER_MODEL" not in os.environ:
        MODEL_SIZE = args.model
    if "WHISPER_DEVICE" not in os.environ:
        DEVICE = args.device
    if "WHISPER_COMPUTE_TYPE" not in os.environ:
        COMPUTE_TYPE = args.compute_type
    if "WHISPER_HOST" not in os.environ:
        HOST = args.host
    if "WHISPER_PORT" not in os.environ:
        PORT = args.port

    logger.info("Starting server on %s:%d", HOST, PORT)
    uvicorn.run(app, host=HOST, port=PORT, log_level="info")


if __name__ == "__main__":
    main()
