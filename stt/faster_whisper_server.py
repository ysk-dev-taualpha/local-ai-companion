"""
faster-whisper STT HTTP server.

FastAPI + faster-whisper small (CUDA float16).
Provides /v1/transcribe (multipart WAV) and a health endpoint.
Designed to run on WinPC (RTX 2080 SUPER 8GB), coexisting with Ollama (~6GB).
GPU VRAM usage: ~1.5 GB.
"""

from __future__ import annotations

import io
import logging
import sys
from contextlib import asynccontextmanager

import numpy as np
import soundfile as sf
from fastapi import FastAPI, File, HTTPException, UploadFile

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    stream=sys.stdout,
)
logger = logging.getLogger("faster_whisper_server")

# ── model state ──────────────────────────────────────────────────────────

_model = None
_model_name: str = "Systran/faster-whisper-small"
_device: str = "cuda"
_compute_type: str = "float16"


def load_model() -> None:
    """Load the faster-whisper model at startup (preload)."""
    global _model

    try:
        from faster_whisper import WhisperModel
    except ImportError:
        logger.error(
            "faster-whisper is not installed. "
            "Run: pip install faster-whisper"
        )
        raise

    logger.info(
        "Loading model %s on %s (compute_type=%s) …",
        _model_name, _device, _compute_type,
    )
    _model = WhisperModel(
        _model_name,
        device=_device,
        compute_type=_compute_type,
    )
    logger.info("Model loaded successfully — ready for transcription.")


# ── lifespan ─────────────────────────────────────────────────────────────

@asynccontextmanager
async def lifespan(app: FastAPI):
    load_model()
    yield


app = FastAPI(
    title="faster-whisper STT Server",
    version="0.1.0",
    lifespan=lifespan,
)


# ── helpers ──────────────────────────────────────────────────────────────

def _read_wav_from_bytes(raw: bytes) -> np.ndarray:
    """Read WAV bytes into a float32 mono numpy array at 16 kHz."""
    audio, sr = sf.read(io.BytesIO(raw), dtype="float32")
    # Convert stereo → mono
    if audio.ndim == 2:
        audio = audio.mean(axis=1)
    # Resample if needed — faster-whisper handles 16kHz best
    if sr != 16000:
        logger.warning("Resampling %d Hz → 16000 Hz", sr)
        import librosa
        audio = librosa.resample(audio, orig_sr=sr, target_sr=16000)
    return audio.astype(np.float32)


# ── endpoints ────────────────────────────────────────────────────────────

@app.get("/health")
async def health():
    """Health check — returns model load status."""
    return {
        "status": "ok" if _model is not None else "model_not_loaded",
        "model": _model_name,
        "device": _device,
    }


@app.post("/v1/transcribe")
async def transcribe(file: UploadFile = File(...)):
    """
    Transcribe a WAV audio file.

    Accepts multipart file upload (Content-Type: multipart/form-data).
    Returns ``{"text": "..."}``.
    """
    if _model is None:
        raise HTTPException(status_code=503, detail="Model not loaded yet")

    if file.content_type and "audio" not in file.content_type:
        raise HTTPException(
            status_code=400,
            detail=f"Expected audio/*, got {file.content_type}",
        )

    try:
        raw = await file.read()
        audio = _read_wav_from_bytes(raw)

        segments, _info = _model.transcribe(
            audio,
            language="ja",
            beam_size=5,
            vad_filter=True,
        )
        text = "".join(seg.text for seg in segments).strip()

        logger.info("Transcription (%d B): %s", len(raw), text[:80])
        return {"text": text}

    except Exception:
        logger.exception("Transcription failed")
        return {"error": "transcription failed", "text": ""}
