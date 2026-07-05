"""Silero VAD module for real-time voice activity detection on PCM audio chunks.

Uses ONNX Runtime (lightweight, no torch dependency) to run Silero VAD
stateful streaming model. Accepts PCM int16 chunks, emits speech events.
"""

import base64
import io
import logging
import os
import struct
import wave
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Optional

import numpy as np

logger = logging.getLogger(__name__)


@dataclass
class VADConfig:
    """VAD configuration parameters."""
    enabled: bool = False
    sample_rate: int = 16000
    speech_threshold: float = 0.5
    silence_duration_ms: int = 300
    min_speech_duration_ms: int = 500
    model_path: str = "models/silero_vad.onnx"


class SileroVAD:
    """Stateful streaming Silero VAD processor using ONNX Runtime."""

    FRAME_SIZE = 512

    def __init__(self, config: Optional[VADConfig] = None):
        self.config = config or VADConfig()
        self._session = None
        self._load_attempted = False
        self._vad_state: Optional[np.ndarray] = None
        self._is_speech: bool = False
        self._speech_buffer: bytearray = bytearray()
        self._speech_start_time: float = 0.0
        self._silence_start_time: Optional[float] = None
        self._leftover: list[int] = []

    def reset(self) -> None:
        self._vad_state = None
        self._is_speech = False
        self._speech_buffer = bytearray()
        self._speech_start_time = 0.0
        self._silence_start_time = None
        self._leftover = []

    def process_chunk(self, pcm_bytes: bytes) -> dict[str, Any]:
        self._ensure_model()
        sample_count = len(pcm_bytes) // 2
        incoming = list(struct.unpack(f"<{sample_count}h", pcm_bytes))
        samples = self._leftover + incoming
        self._leftover = []

        sr_arr = np.array(self.config.sample_rate, dtype=np.int64)
        init_state = np.zeros((2, 1, 128), dtype=np.float32)
        fs = self.FRAME_SIZE

        import time
        event = "idle"
        wav_b64 = None

        while len(samples) >= fs:
            frame_samples = samples[:fs]
            samples = samples[fs:]
            frame = (np.array(frame_samples, dtype=np.float32).reshape(1, fs) / 32768.0)
            state = self._vad_state if self._vad_state is not None else init_state
            outputs = self._session.run(None, {"input": frame, "state": state, "sr": sr_arr})
            speech_prob = float(outputs[0][0][0])
            self._vad_state = outputs[1]
            self._speech_buffer.extend(struct.pack(f"<{fs}h", *frame_samples))

            is_voice = speech_prob >= self.config.speech_threshold
            now = time.time()
            if is_voice:
                voice_event = self._handle_voice_inner(now)
                if voice_event:
                    event = voice_event
                    break
            else:
                silence_result = self._handle_silence_inner(now)
                if silence_result:
                    event, wav_b64 = silence_result
                    break

        self._leftover = samples
        result: dict[str, Any] = {"event": event}
        if wav_b64 is not None:
            result["audio_wav"] = wav_b64
        return result

    @property
    def is_speech(self) -> bool:
        return self._is_speech

    def _ensure_model(self) -> None:
        if self._load_attempted:
            if self._session is None:
                raise RuntimeError("Silero VAD ONNX model failed to load.")
            return
        self._load_attempted = True
        try:
            import onnxruntime as ort
        except ImportError:
            raise RuntimeError("onnxruntime is required. Install: pip install onnxruntime")
        model_path = Path(self.config.model_path)
        if not model_path.is_absolute():
            module_dir = Path(__file__).resolve().parent.parent.parent
            model_path = module_dir / self.config.model_path
        if not model_path.exists():
            raise FileNotFoundError(f"Silero VAD model not found at {model_path}")
        logger.info("Loading Silero VAD model from %s", model_path)
        self._session = ort.InferenceSession(str(model_path), providers=["CPUExecutionProvider"])

    def _handle_voice_inner(self, now: float) -> Optional[str]:
        self._silence_start_time = None
        if not self._is_speech:
            self._is_speech = True
            self._speech_start_time = now
            return "speech_start"
        return None

    def _handle_silence_inner(self, now: float):
        if not self._is_speech:
            return None
        if self._silence_start_time is None:
            self._silence_start_time = now
        silence_ms = (now - self._silence_start_time) * 1000
        if silence_ms < self.config.silence_duration_ms:
            return None
        speech_ms = (now - self._speech_start_time) * 1000
        if speech_ms < self.config.min_speech_duration_ms:
            self.reset()
            return ("idle", None)
        wav_bytes = self._pcm_to_wav(bytes(self._speech_buffer))
        wav_b64 = base64.b64encode(wav_bytes).decode("ascii")
        self.reset()
        return ("speech_end", wav_b64)

    def _pcm_to_wav(self, pcm_data: bytes) -> bytes:
        buf = io.BytesIO()
        with wave.open(buf, "wb") as wf:
            wf.setnchannels(1)
            wf.setsampwidth(2)
            wf.setframerate(self.config.sample_rate)
            wf.writeframes(pcm_data)
        return buf.getvalue()


def create_vad(config: Optional[VADConfig] = None) -> SileroVAD:
    return SileroVAD(config)
