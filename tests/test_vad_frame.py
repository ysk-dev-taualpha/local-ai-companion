"""Silero VAD unit tests with mocked ONNX inference — frame-counter version."""
import io, sys, unittest, wave
from unittest.mock import MagicMock

sys.path.insert(0, "src")

from local_ai_companion.vad import SileroVAD, VADConfig


class _MockSession:
    def __init__(self, prob: float = 0.0):
        self._prob = prob
        self._run_count = 0

    def set_prob(self, prob: float):
        self._prob = prob

    def run(self, outputs, inputs):
        import numpy as np
        self._run_count += 1
        return [np.array([[self._prob]], dtype=np.float32),
                np.zeros((2, 1, 128), dtype=np.float32)]


class TestSileroVAD(unittest.TestCase):
    def setUp(self):
        self.vad = SileroVAD.__new__(SileroVAD)
        self.vad.config = VADConfig(
            speech_threshold=0.5, silence_duration_ms=300,
            min_speech_duration_ms=500, model_path="models/silero_vad.onnx")
        self.vad._silence_frames = 9
        self.vad._min_speech = 15
        self._mock = _MockSession()
        self.vad._session = self._mock
        self.vad._load_attempted = True
        self.vad._vad_state = None
        self.vad._is_speech = False
        self.vad._speech_buffer = bytearray()
        self.vad._speech_count = 0
        self.vad._silence_count = 0
        self.vad._valid = False
        self.vad._leftover = []

    def _pcm(self, n: int) -> bytes:
        import struct
        return struct.pack(f"<{n}h", *([100] * n))

    def test_initial_idle(self):
        self.assertFalse(self.vad.is_speech)

    def test_speech_start(self):
        self._mock.set_prob(0.9)
        r = self.vad.process_chunk(self._pcm(512))
        self.assertEqual(r["event"], "speech_start")
        self.assertTrue(self.vad.is_speech)

    def test_ongoing_idle(self):
        self._mock.set_prob(0.9)
        self.vad.process_chunk(self._pcm(512))
        r = self.vad.process_chunk(self._pcm(512))
        self.assertEqual(r["event"], "idle")

    def test_idle_when_silent(self):
        self._mock.set_prob(0.1)
        r = self.vad.process_chunk(self._pcm(512))
        self.assertEqual(r["event"], "idle")
        self.assertFalse(self.vad.is_speech)

    def test_speech_end(self):
        self._mock.set_prob(0.9)
        self.vad.process_chunk(self._pcm(512))
        self._mock.set_prob(0.9)
        for _ in range(14):
            self.vad.process_chunk(self._pcm(512))
        self._mock.set_prob(0.1)
        for _ in range(8):
            r = self.vad.process_chunk(self._pcm(512))
        r = self.vad.process_chunk(self._pcm(512))
        self.assertEqual(r["event"], "speech_end")
        self.assertIn("audio_wav", r)
        self.assertFalse(self.vad.is_speech)

    def test_short_speech_ignored(self):
        self._mock.set_prob(0.9)
        self.vad.process_chunk(self._pcm(512))
        self.vad.process_chunk(self._pcm(512))
        self._mock.set_prob(0.1)
        for _ in range(9):
            r = self.vad.process_chunk(self._pcm(512))
        self.assertEqual(r["event"], "idle")
        self.assertFalse(self.vad.is_speech)

    def test_pcm_to_wav(self):
        pcm = self._pcm(16000)
        wav_bytes = self.vad._pcm_to_wav(pcm)
        with wave.open(io.BytesIO(wav_bytes), "rb") as wf:
            self.assertEqual(wf.getnchannels(), 1)
            self.assertEqual(wf.getsampwidth(), 2)
            self.assertEqual(wf.getframerate(), 16000)

    def test_reset(self):
        self._mock.set_prob(0.9)
        self.vad.process_chunk(self._pcm(512))
        self.vad.reset()
        self.assertFalse(self.vad.is_speech)
        self.assertEqual(len(self.vad._speech_buffer), 0)
        self.assertIsNone(self.vad._vad_state)
        self.assertEqual(self.vad._speech_count, 0)


class TestVADConfig(unittest.TestCase):
    def test_default(self):
        from local_ai_companion.config import AppConfig
        c = AppConfig()
        self.assertFalse(c.vad.enabled)
        self.assertEqual(c.vad.sample_rate, 16000)
        self.assertEqual(c.vad.speech_threshold, 0.5)

    def test_load_json(self):
        import json, tempfile, os
        from local_ai_companion.config import load_config
        data = {"conversation": {}, "llm": {"provider": "mock"}, "vad": {
            "enabled": True, "sample_rate": 8000, "speech_threshold": 0.7,
            "silence_duration_ms": 500, "min_speech_duration_ms": 1000}}
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(data, f)
            p = f.name
        try:
            c = load_config(p)
            self.assertTrue(c.vad.enabled)
            self.assertEqual(c.vad.sample_rate, 8000)
            self.assertEqual(c.vad.speech_threshold, 0.7)
        finally:
            os.unlink(p)


if __name__ == "__main__":
    unittest.main()
