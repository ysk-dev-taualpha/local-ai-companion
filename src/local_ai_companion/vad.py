"""Silero VAD module for real-time voice activity detection on PCM audio chunks.

Uses ONNX Runtime (lightweight, no torch dependency) to run Silero VAD
stateful streaming model. Accepts PCM int16 chunks, emits speech events.
"""

import base64, io, logging, struct, wave
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Optional
import numpy as np

logger = logging.getLogger(__name__)

@dataclass
class VADConfig:
    sample_rate: int = 16000
    speech_threshold: float = 0.5
    silence_duration_ms: int = 300
    min_speech_duration_ms: int = 500
    model_path: str = "models/silero_vad.onnx"

class SileroVAD:
    FRAME_SIZE = 512
    def __init__(self, config=None):
        self.config = config or VADConfig()
        ms = self.FRAME_SIZE / self.config.sample_rate * 1000
        self._silence_frames = max(1, int(self.config.silence_duration_ms / ms))
        self._min_speech = max(1, int(self.config.min_speech_duration_ms / ms))
        self._session = None
        self._load_attempted = False
        self._vad_state = None
        self._is_speech = False
        self._speech_buffer = bytearray()
        self._speech_count = 0
        self._silence_count = 0
        self._valid = False
        self._leftover = []

    def reset(self):
        self._vad_state = None; self._is_speech = False
        self._speech_buffer = bytearray(); self._speech_count = 0
        self._silence_count = 0; self._valid = False; self._leftover = []

    def process_chunk(self, pcm_bytes):
        self._ensure_model()
        n = len(pcm_bytes) // 2
        samples = self._leftover + list(struct.unpack(f"<{n}h", pcm_bytes))
        self._leftover = []
        sr = np.array(self.config.sample_rate, dtype=np.int64)
        s0 = np.zeros((2, 1, 128), dtype=np.float32)
        fs = self.FRAME_SIZE
        event = "idle"
        wav = None
        while len(samples) >= fs:
            frame = samples[:fs]; samples = samples[fs:]
            arr = np.array(frame, dtype=np.float32).reshape(1, fs) / 32768.0
            st = self._vad_state if self._vad_state is not None else s0
            if self._session is None: raise RuntimeError("no model")
            out = self._session.run(None, {"input": arr, "state": st, "sr": sr})
            prob = float(out[0][0][0]); self._vad_state = out[1]
            self._speech_buffer.extend(struct.pack(f"<{fs}h", *frame))
            if prob >= self.config.speech_threshold:
                self._speech_count += 1; self._silence_count = 0
                if not self._is_speech:
                    self._is_speech = True; self._valid = False
                    event = "speech_start"; break
                if self._speech_count >= self._min_speech: self._valid = True
            elif self._is_speech:
                self._silence_count += 1
                if self._silence_count >= self._silence_frames:
                    if self._valid:
                        wb = self._pcm_to_wav(bytes(self._speech_buffer))
                        wav = base64.b64encode(wb).decode("ascii")
                        event = "speech_end"
                    self.reset(); break
        self._leftover = samples
        r = {"event": event}
        if wav: r["audio_wav"] = wav
        return r

    @property
    def is_speech(self): return self._is_speech

    def _ensure_model(self):
        if self._load_attempted:
            if self._session is None: raise RuntimeError("VAD model failed to load")
            return
        self._load_attempted = True
        try: import onnxruntime as ort
        except ImportError: raise RuntimeError("onnxruntime required")
        mp = Path(self.config.model_path)
        if not mp.is_absolute():
            mp = Path(__file__).resolve().parent.parent.parent / self.config.model_path
        if not mp.exists(): raise FileNotFoundError(f"model not found: {mp}")
        self._session = ort.InferenceSession(str(mp), providers=["CPUExecutionProvider"])

    def _pcm_to_wav(self, data):
        b = io.BytesIO()
        with wave.open(b, "wb") as wf:
            wf.setnchannels(1); wf.setsampwidth(2)
            wf.setframerate(self.config.sample_rate); wf.writeframes(data)
        return b.getvalue()

def create_vad(config=None):
    return SileroVAD(config)
