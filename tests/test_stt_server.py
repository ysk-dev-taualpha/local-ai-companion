"""faster-whisper STT server のテスト。"""

import io
import json
import unittest
import wave

from fastapi.testclient import TestClient
from local_ai_companion.stt_server import STTResponse, WhisperEngine, create_app


def _make_wav_bytes(sample_rate=16000, channels=1, sample_width=2, duration_sec=1.0):
    buf = io.BytesIO()
    with wave.open(buf, "wb") as wf:
        wf.setnchannels(channels)
        wf.setsampwidth(sample_width)
        wf.setframerate(sample_rate)
        n_frames = int(sample_rate * duration_sec)
        wf.writeframes(b"\x00" * (n_frames * channels * sample_width))
    return buf.getvalue()


class WhisperEngineTest(unittest.TestCase):
    def test_default_config(self):
        e = WhisperEngine()
        self.assertEqual(e.model_size, "small")
        self.assertEqual(e.device, "cuda")
        self.assertEqual(e.compute_type, "float16")

    def test_custom_config(self):
        e = WhisperEngine(model_size="medium", device="cpu", compute_type="int8")
        self.assertEqual(e.model_size, "medium")
        self.assertEqual(e.device, "cpu")
        self.assertEqual(e.compute_type, "int8")


class STTResponseTest(unittest.TestCase):
    def test_success(self):
        r = STTResponse(text="こんにちは", duration=1.5)
        d = r.to_dict()
        self.assertEqual(d["text"], "こんにちは")
        self.assertEqual(d["duration"], 1.5)
        self.assertIsNone(d["error"])

    def test_error(self):
        r = STTResponse(text="", duration=0, error="model not loaded")
        d = r.to_dict()
        self.assertEqual(d["error"], "model not loaded")

    def test_json(self):
        r = STTResponse(text="テスト", duration=0.8)
        j = json.dumps(r.to_dict(), ensure_ascii=False)
        parsed = json.loads(j)
        self.assertEqual(parsed["text"], "テスト")


class TranscribeEndpointTest(unittest.TestCase):
    def setUp(self):
        def mock_transcribe(audio_path, language):
            return STTResponse(text="ダミー認識結果", duration=0.5)
        self.app = create_app(fallback_transcribe=mock_transcribe)
        self.client = TestClient(self.app)

    def test_valid_request(self):
        wav = _make_wav_bytes()
        resp = self.client.post("/v1/transcribe",
            files={"file": ("test.wav", io.BytesIO(wav), "audio/wav")},
            data={"language": "ja"})
        self.assertEqual(resp.status_code, 200)
        data = resp.json()
        self.assertIsNone(data["error"])

    def test_missing_file_returns_422(self):
        resp = self.client.post("/v1/transcribe", data={"language": "ja"})
        self.assertEqual(resp.status_code, 422)

    def test_empty_filename_returns_422(self):
        resp = self.client.post("/v1/transcribe",
            files={"file": ("", io.BytesIO(b""), "application/octet-stream")},
            data={"language": "ja"})
        self.assertEqual(resp.status_code, 422)

    def test_default_language(self):
        wav = _make_wav_bytes()
        resp = self.client.post("/v1/transcribe",
            files={"file": ("test.wav", io.BytesIO(wav), "audio/wav")})
        self.assertEqual(resp.status_code, 200)

    def test_transcribe_error_returns_500(self):
        def failing(audio_path, language):
            raise RuntimeError("CUDA out of memory")
        app = create_app(fallback_transcribe=failing)
        client = TestClient(app)
        wav = _make_wav_bytes()
        resp = client.post("/v1/transcribe",
            files={"file": ("test.wav", io.BytesIO(wav), "audio/wav")},
            data={"language": "ja"})
        self.assertEqual(resp.status_code, 500)

    def test_health(self):
        resp = self.client.get("/health")
        self.assertEqual(resp.status_code, 200)
        self.assertEqual(resp.json()["status"], "ok")


if __name__ == "__main__":
    unittest.main()
