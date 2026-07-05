"""faster-whisper STT HTTP サーバー。

WinPC (RTX 2080 SUPER 8GB) 上で稼働し、X1C6 の Python AI Service から
HTTP POST /v1/transcribe で WAV ファイルを受け取り、faster-whisper で
音声認識を実行してテキストを返す。

使い方:
    uvicorn local_ai_companion.stt_server:app --host 0.0.0.0 --port 8093
    python -m local_ai_companion.stt_server
"""

from __future__ import annotations

import argparse
import logging
import os
import tempfile
import time
from dataclasses import dataclass
from typing import Callable, Optional

from fastapi import FastAPI, File, Form, UploadFile
from fastapi.responses import JSONResponse

logger = logging.getLogger(__name__)


# ============================================================================
# データモデル
# ============================================================================


@dataclass
class STTResponse:
    """transcribe 結果。"""

    text: str
    duration: float
    error: Optional[str] = None

    def to_dict(self) -> dict:
        return {"text": self.text, "duration": self.duration, "error": self.error}


# ============================================================================
# モデルロードと推論
# ============================================================================


class WhisperEngine:
    """faster-whisper モデルのラッパー。起動時にプリロード。"""

    def __init__(
        self,
        model_size: str = "small",
        device: str = "cuda",
        compute_type: str = "float16",
    ):
        self.model_size = model_size
        self.device = device
        self.compute_type = compute_type
        self._model = None

    @property
    def model(self):
        if self._model is None:
            self._model = self._load()
        return self._model

    def _load(self):
        from faster_whisper import WhisperModel

        logger.info(
            "Loading faster-whisper model_size=%s device=%s compute_type=%s",
            self.model_size, self.device, self.compute_type,
        )
        model = WhisperModel(self.model_size, device=self.device, compute_type=self.compute_type)
        logger.info("Model loaded successfully")
        return model

    def transcribe(self, audio_path: str, language: str) -> STTResponse:
        start = time.monotonic()
        segments, info = self.model.transcribe(audio_path, language=language, beam_size=5)
        text = "".join(seg.text for seg in segments).strip()
        elapsed = time.monotonic() - start
        logger.info("Transcribed in %.2fs (audio %.1fs): %s", elapsed, info.duration, text[:100] if text else "(empty)")
        return STTResponse(text=text, duration=round(info.duration, 2))


# ============================================================================
# FastAPI アプリケーション構築
# ============================================================================


def create_app(
    engine: Optional[WhisperEngine] = None,
    fallback_transcribe: Optional[Callable] = None,
) -> FastAPI:
    app = FastAPI(title="faster-whisper STT Server", version="0.1.0")

    if fallback_transcribe is not None:
        _transcribe_fn = fallback_transcribe
    elif engine is not None:
        _transcribe_fn = engine.transcribe
    else:
        real_engine = WhisperEngine()
        _ = real_engine.model  # プリロード
        _transcribe_fn = real_engine.transcribe

    @app.get("/health")
    async def health():
        return {"status": "ok"}

    @app.post("/v1/transcribe")
    async def transcribe(file: UploadFile = File(...), language: str = Form(default="ja")):
        suffix = ".wav"
        with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as tmp:
            content = await file.read()
            tmp.write(content)
            tmp_path = tmp.name
        try:
            result = _transcribe_fn(tmp_path, language)
            return JSONResponse(status_code=200, content=result.to_dict())
        except Exception as exc:
            logger.exception("Transcription failed")
            error_response = STTResponse(text="", duration=0, error=str(exc))
            return JSONResponse(status_code=500, content=error_response.to_dict())
        finally:
            try:
                os.unlink(tmp_path)
            except OSError:
                pass

    return app


# ============================================================================
# エントリポイント
# ============================================================================


def main():
    parser = argparse.ArgumentParser(description="faster-whisper STT HTTP Server")
    parser.add_argument("--host", default="0.0.0.0", help="Listen address")
    parser.add_argument("--port", type=int, default=8093, help="Listen port")
    parser.add_argument("--model-size", default="small",
                        choices=["tiny", "base", "small", "medium", "large-v3"])
    parser.add_argument("--device", default="cuda", choices=["cuda", "cpu"])
    parser.add_argument("--compute-type", default="float16",
                        choices=["float16", "int8_float16", "int8"])
    args = parser.parse_args()

    engine = WhisperEngine(model_size=args.model_size, device=args.device, compute_type=args.compute_type)
    _ = engine.model  # 即時ロード
    app = create_app(engine=engine)

    import uvicorn
    logger.info("Starting STT server on %s:%d", args.host, args.port)
    uvicorn.run(app, host=args.host, port=args.port, log_level="info")


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
    main()
