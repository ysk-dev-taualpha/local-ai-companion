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

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------


@dataclass
class VADConfig:
    """VAD configuration parameters.

    Attributes:
        sample_rate: Audio sample rate in Hz (must be 16000 for Silero VAD).
        speech_threshold: Speech probability threshold (0.0-1.0).
        silence_duration_ms: Continuous silence duration to trigger speech_end.
        min_speech_duration_ms: Minimum speech duration; shorter utterances ignored.
        model_path: Path to the Silero VAD ONNX model file.
    """

    sample_rate: int = 16000
    speech_threshold: float = 0.5
    silence_duration_ms: int = 300
    min_speech_duration_ms: int = 500
    model_path: str = "models/silero_vad.onnx"


# ---------------------------------------------------------------------------
# VAD Processor
# ---------------------------------------------------------------------------


class SileroVAD:
    """Stateful streaming Silero VAD processor using ONNX Runtime.

    Accepts PCM int16 audio chunks and emits speech events. The internal
    ONNX model maintains a recurrent state between frames.

    Events:
        * ``speech_start`` – voice detected after silence.
        * ``speech_end`` – voice ended (includes ``audio_wav`` base64).
        * ``idle`` – no speech activity.
    """

    FRAME_SIZE = 512  # samples per VAD frame (32ms @ 16kHz)

    def __init__(self, config: Optional[VADConfig] = None):
        self.config = config or VADConfig()
        self._session = None
        self._load_attempted = False

        # State tracking
        self._vad_state: Optional[np.ndarray] = None  # ONNX recurrent state
        self._is_speech: bool = False
        self._speech_buffer: bytearray = bytearray()
        self._speech_start_time: float = 0.0
        self._silence_start_time: Optional[float] = None
        self._leftover: list[int] = []  # partial frame samples

    # ---- public API -------------------------------------------------------

    def reset(self) -> None:
        """Reset internal state (buffers, counters, VAD recurrent state)."""
        self._vad_state = None
        self._is_speech = False
        self._speech_buffer = bytearray()
        self._speech_start_time = 0.0
        self._silence_start_time = None
        self._leftover = []

    def process_chunk(self, pcm_bytes: bytes) -> dict[str, Any]:
        """Process a PCM int16 little-endian audio chunk.

        Args:
            pcm_bytes: Raw PCM int16 LE audio data.

        Returns:
            dict with ``"event"`` key ("speech_start", "speech_end", "idle").
            For ``speech_end``, includes ``"audio_wav"`` (str) with base64
            encoded WAV audio.
        """
        self._ensure_model()

        # Convert bytes to int16 samples and prepend leftovers
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

        # Process complete frames
        while len(samples) >= fs:
            frame_samples = samples[:fs]
            samples = samples[fs:]

            # Convert to float32 normalized
            frame = (
                np.array(frame_samples, dtype=np.float32).reshape(1, fs) / 32768.0
            )

            # Run ONNX inference
            state = (
                self._vad_state if self._vad_state is not None else init_state
            )
            outputs = self._session.run(
                None,
                {"input": frame, "state": state, "sr": sr_arr},
            )
            speech_prob = float(outputs[0][0][0])
            self._vad_state = outputs[1]

            # Collect speech samples
            self._speech_buffer.extend(
                struct.pack(f"<{fs}h", *frame_samples)
            )

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

        # Save leftovers for next chunk
        self._leftover = samples

        result: dict[str, Any] = {"event": event}
        if wav_b64 is not None:
            result["audio_wav"] = wav_b64
        return result

    @property
    def is_speech(self) -> bool:
        """Whether currently in a speech segment."""
        return self._is_speech

    # ---- internal helpers -------------------------------------------------

    def _ensure_model(self) -> None:
        """Lazy-load the ONNX model."""
        if self._load_attempted:
            if self._session is None:
                raise RuntimeError(
                    "Silero VAD ONNX model failed to load. "
                    "Ensure the model file exists and onnxruntime is installed."
                )
            return
        self._load_attempted = True

        try:
            import onnxruntime as ort
        except ImportError:
            raise RuntimeError(
                "onnxruntime is required for Silero VAD. "
                "Install with: pip install onnxruntime"
            )

        model_path = Path(self.config.model_path)
        if not model_path.is_absolute():
            module_dir = Path(__file__).resolve().parent.parent.parent
            model_path = module_dir / self.config.model_path

        if not model_path.exists():
            raise FileNotFoundError(
                f"Silero VAD model not found at {model_path}. "
                "Download from: https://github.com/snakers4/silero-vad"
            )

        logger.info("Loading Silero VAD model from %s", model_path)
        self._session = ort.InferenceSession(
            str(model_path),
            providers=["CPUExecutionProvider"],
        )

    def _handle_voice_inner(self, now: float) -> Optional[str]:
        """Handle voice detection. Returns event if state changed."""
        self._silence_start_time = None

        if not self._is_speech:
            self._is_speech = True
            self._speech_start_time = now
            return "speech_start"
        return None  # already speaking

    def _handle_silence_inner(self, now: float):
        """Handle silence detection. Returns (event, wav_b64) or None."""
        if not self._is_speech:
            return None

        if self._silence_start_time is None:
            self._silence_start_time = now

        silence_ms = (now - self._silence_start_time) * 1000
        if silence_ms < self.config.silence_duration_ms:
            return None

        # Check minimum speech duration
        speech_ms = (now - self._speech_start_time) * 1000
        if speech_ms < self.config.min_speech_duration_ms:
            # Too short, discard
            self.reset()
            return ("idle", None)

        # Build WAV from accumulated PCM
        wav_bytes = self._pcm_to_wav(bytes(self._speech_buffer))
        wav_b64 = base64.b64encode(wav_bytes).decode("ascii")
        self.reset()
        return ("speech_end", wav_b64)

    def _pcm_to_wav(self, pcm_data: bytes) -> bytes:
        """Convert PCM int16 data to WAV format."""
        buf = io.BytesIO()
        with wave.open(buf, "wb") as wf:
            wf.setnchannels(1)
            wf.setsampwidth(2)  # 16-bit
            wf.setframerate(self.config.sample_rate)
            wf.writeframes(pcm_data)
        return buf.getvalue()


# ---------------------------------------------------------------------------
# Factory
# ---------------------------------------------------------------------------


def create_vad(config: Optional[VADConfig] = None) -> SileroVAD:
    """Create a SileroVAD instance."""
    return SileroVAD(config)
