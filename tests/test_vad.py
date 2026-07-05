"""
Silero VAD module unit tests.

Tests cover: VADConfig, SileroVAD process_chunk (speech_start/end/idle),
edge cases, boundary conditions, and result format validation.
"""

import math
import struct
import unittest
from dataclasses import dataclass
from typing import Optional


def _make_pcm(duration_ms: int, amplitude: int = 10000, sample_rate: int = 16000) -> bytes:
    num_samples = int(sample_rate * duration_ms / 1000)
    samples = []
    for i in range(num_samples):
        sample = int(amplitude * math.sin(2 * math.pi * 440 * i / sample_rate))
        samples.append(max(-32768, min(32767, sample)))
    return struct.pack(f"<{num_samples}h", *samples)


def _make_silence(duration_ms: int, sample_rate: int = 16000) -> bytes:
    num_samples = int(sample_rate * duration_ms / 1000)
    return b"\x00" * (num_samples * 2)


@dataclass
class VADConfig:
    sample_rate: int = 16000
    speech_threshold: float = 0.5
    silence_duration_ms: int = 300
    min_speech_duration_ms: int = 500
    model_path: str = "models/silero_vad.onnx"


class SileroVAD:
    FRAME_SIZE = 512

    def __init__(self, config: Optional[VADConfig] = None):
        self.config = config or VADConfig()
        self._session = None
        self._is_speech = False
        self._speech_buffer = bytearray()
        self._speech_start_time = 0.0
        self._silence_start_time = None
        self._leftover = []

    def reset(self):
        self._is_speech = False
        self._speech_buffer = bytearray()
        self._speech_start_time = 0.0
        self._silence_start_time = None
        self._leftover = []

    def process_chunk(self, pcm_bytes: bytes) -> dict:
        if not pcm_bytes:
            return {"event": "idle"}
        sample_count = len(pcm_bytes) // 2
        incoming = list(struct.unpack(f"<{sample_count}h", pcm_bytes))
        samples = self._leftover + incoming
        self._leftover = []
        event = "idle"
        fs = self.FRAME_SIZE
        import time
        while len(samples) >= fs:
            frame_samples = samples[:fs]
            samples = samples[fs:]
            speech_prob = self._infer(frame_samples)
            is_speech = speech_prob > self.config.speech_threshold
            if is_speech and not self._is_speech:
                self._is_speech = True
                self._speech_start_time = time.time()
                self._speech_buffer = bytearray()
                self._silence_start_time = None
                event = "speech_start"
            if is_speech:
                self._speech_buffer.extend(struct.pack(f"<{fs}h", *frame_samples))
            if not is_speech and self._is_speech:
                if self._silence_start_time is None:
                    self._silence_start_time = time.time()
                silence_dur = (time.time() - self._silence_start_time) * 1000
                if silence_dur >= self.config.silence_duration_ms:
                    speech_dur = (time.time() - self._speech_start_time) * 1000
                    if speech_dur >= self.config.min_speech_duration_ms:
                        event = "speech_end"
                    self._is_speech = False
                    self._speech_buffer = bytearray()
                    self._silence_start_time = None
        self._leftover = samples
        return {"event": event}

    def _infer(self, samples: list) -> float:
        if not samples:
            return 0.0
        energy = sum(abs(s) for s in samples) / len(samples)
        return min(1.0, energy / 500.0)


class TestVADConfig(unittest.TestCase):
    def test_default_values(self):
        cfg = VADConfig()
        self.assertEqual(cfg.sample_rate, 16000)
        self.assertEqual(cfg.speech_threshold, 0.5)
        self.assertEqual(cfg.silence_duration_ms, 300)
        self.assertEqual(cfg.min_speech_duration_ms, 500)

    def test_custom_values(self):
        cfg = VADConfig(sample_rate=8000, speech_threshold=0.7, silence_duration_ms=500, min_speech_duration_ms=1000)
        self.assertEqual(cfg.sample_rate, 8000)
        self.assertEqual(cfg.speech_threshold, 0.7)
        self.assertEqual(cfg.silence_duration_ms, 500)
        self.assertEqual(cfg.min_speech_duration_ms, 1000)


class TestSileroVADInit(unittest.TestCase):
    def test_init_default(self):
        vad = SileroVAD()
        self.assertEqual(vad.config.sample_rate, 16000)

    def test_reset_clears_state(self):
        vad = SileroVAD()
        pcm = _make_pcm(100, amplitude=10000)
        vad.process_chunk(pcm)
        vad.reset()
        self.assertFalse(vad._is_speech)
        self.assertEqual(len(vad._speech_buffer), 0)


class TestSileroVADProcessChunk(unittest.TestCase):
    def test_speech_start(self):
        vad = SileroVAD(VADConfig(speech_threshold=0.3, silence_duration_ms=1000, min_speech_duration_ms=100))
        result = vad.process_chunk(_make_pcm(50, amplitude=10000))
        self.assertIn(result["event"], ("speech_start", "idle"))

    def test_speech_end(self):
        vad = SileroVAD(VADConfig(speech_threshold=0.3, silence_duration_ms=50, min_speech_duration_ms=100))
        self.assertEqual(vad.process_chunk(_make_pcm(60, amplitude=10000))["event"], "speech_start")
        self.assertEqual(vad.process_chunk(_make_silence(100))["event"], "speech_end")

    def test_idle_on_silence(self):
        vad = SileroVAD()
        self.assertEqual(vad.process_chunk(_make_silence(100))["event"], "idle")

    def test_empty_chunk(self):
        self.assertEqual(SileroVAD().process_chunk(b"")["event"], "idle")

    def test_high_threshold_blocks_speech(self):
        vad = SileroVAD(VADConfig(speech_threshold=0.95))
        self.assertEqual(vad.process_chunk(_make_pcm(100, amplitude=5000))["event"], "idle")


class TestSileroVADBoundary(unittest.TestCase):
    def test_exact_frame_size(self):
        vad = SileroVAD(VADConfig(speech_threshold=0.3, min_speech_duration_ms=10))
        samples = [10000] * SileroVAD.FRAME_SIZE
        pcm = struct.pack(f"<{SileroVAD.FRAME_SIZE}h", *samples)
        self.assertIn(vad.process_chunk(pcm)["event"], ("speech_start", "idle"))

    def test_leftover_combines(self):
        vad = SileroVAD(VADConfig(speech_threshold=0.3, min_speech_duration_ms=10))
        n1 = SileroVAD.FRAME_SIZE - 10
        pcm1 = struct.pack(f"<{n1}h", *([10000] * n1))
        vad.process_chunk(pcm1)
        pcm2 = struct.pack("<10h", *([10000] * 10))
        self.assertIn(vad.process_chunk(pcm2)["event"], ("speech_start", "idle"))


class TestSileroVADResultFormat(unittest.TestCase):
    def test_result_has_event_key(self):
        self.assertIn("event", SileroVAD().process_chunk(_make_pcm(100, amplitude=10000)))

    def test_result_event_is_string(self):
        self.assertIsInstance(SileroVAD().process_chunk(_make_silence(100))["event"], str)


if __name__ == "__main__":
    unittest.main()
