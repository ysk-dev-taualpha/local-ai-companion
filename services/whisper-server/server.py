"""faster-whisper HTTP Transcription Server (WinPC v0.5 STT Engine).

Provides /v1/transcribe endpoint for voice input transcription.
Model: small, CUDA float16, ~1.5GB GPU, preloaded at startup.
Host: 0.0.0.0:8093 (accessible from X1C6 at 192.168.12.112).
"""

import io
import logging
import os
import tempfile
import time

import uvicorn
from fastapi import FastAPI, File, Form, HTTPException, UploadFile

from faster_whisper import WhisperModel

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("whisper-server")

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
MODEL_SIZE = os.environ.get("WHISPER_MODEL", "small")
DEVICE = os.environ.get("WHISPER_DEVICE", "cuda")
COMPUTE_TYPE = os.environ.get("WHISPER_COMPUTE_TYPE", "float16")
HOST = os.environ.get("WHISPER_HOST", "0.0.0.0")
PORT = int(os.environ.get("WHISPER_PORT", "8093"))

# ---------------------------------------------------------------------------
# App & model (preloaded at startup)
# ---------------------------------------------------------------------------
app = FastAPI(title="faster-whisper Transcription Server", version="1.0.0")

logger.info("Loading faster-whisper model: %s (device=%s, compute_type=%s)",
            MODEL_SIZE, DEVICE, COMPUTE_TYPE)
model: WhisperModel = WhisperModel(
    MODEL_SIZE,
    device=DEVICE,
    compute_type=COMPUTE_TYPE,
)
logger.info("Model loaded successfully")


# ---------------------------------------------------------------------------
# Health check
# ---------------------------------------------------------------------------
@app.get("/health")
async def health():
    """Health check endpoint."""
    return {"status": "ok", "model": MODEL_SIZE, "device": DEVICE}


# ---------------------------------------------------------------------------
# Transcription endpoint
# ---------------------------------------------------------------------------
@app.post("/v1/transcribe")
async def transcribe(
    file: UploadFile = File(...),
    language: str = Form("ja"),
):
    """Transcribe uploaded WAV audio (16kHz mono).

    Args:
        file: WAV audio file (16kHz mono recommended).
        language: Language code (default: "ja").

    Returns:
        {"text": "認識テキスト", "duration": 3.2, "error": None}
    """
    start = time.monotonic()
    tmp_path: str | None = None

    try:
        # Save uploaded file to temp
        suffix = os.path.splitext(file.filename or "audio.wav")[1] or ".wav"
        with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as tmp:
            content = await file.read()
            tmp.write(content)
            tmp_path = tmp.name

        # Transcribe
        segments, info = model.transcribe(
            tmp_path,
            language=language,
            beam_size=5,
            vad_filter=True,
        )

        # Concatenate segments
        text = "".join(seg.text for seg in segments)
        elapsed = time.monotonic() - start

        logger.info("Transcription complete: %.1fs, text length=%d", elapsed, len(text))

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

    finally:
        # Clean up temp file
        if tmp_path and os.path.exists(tmp_path):
            os.unlink(tmp_path)


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------
if __name__ == "__main__":
    logger.info("Starting server on %s:%d", HOST, PORT)
    uvicorn.run(app, host=HOST, port=PORT, log_level="info")
