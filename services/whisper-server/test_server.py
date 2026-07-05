"""Tests for faster-whisper Transcription Server (mock mode)."""

import io
import os
import sys
import unittest
from unittest.mock import MagicMock

_mock_whisper = MagicMock()
_mock_whisper.WhisperModel = MagicMock()
sys.modules["faster_whisper"] = _mock_whisper

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "."))
from server import app
from fastapi.testclient import TestClient


class TestWhisperServer(unittest.TestCase):
    def setUp(self):
        self.client = TestClient(app)
        _mock_whisper.WhisperModel.reset_mock()
        self.mock_model = _mock_whisper.WhisperModel.return_value
        self.mock_model.transcribe.side_effect = None

    def test_health_endpoint(self):
        resp = self.client.get("/health")
        self.assertEqual(resp.status_code, 200)
        self.assertEqual(resp.json()["status"], "ok")

    def test_transcribe_returns_text(self):
        mock_seg1 = MagicMock()
        mock_seg1.text = "こんにちは"
        mock_seg2 = MagicMock()
        mock_seg2.text = "元気ですか"
        self.mock_model.transcribe.return_value = ([mock_seg1, mock_seg2], MagicMock())
        files = {"file": ("test.wav", io.BytesIO(b"fakeaudio"), "audio/wav")}
        resp = self.client.post("/v1/transcribe", files=files, data={"language": "ja"})
        self.assertEqual(resp.status_code, 200)
        body = resp.json()
        self.assertEqual(body["text"], "こんにちは元気ですか")
        self.assertIsNone(body["error"])

    def test_transcribe_default_language(self):
        mock_seg = MagicMock()
        mock_seg.text = "テスト"
        self.mock_model.transcribe.return_value = ([mock_seg], MagicMock())
        files = {"file": ("test.wav", io.BytesIO(b"fakeaudio"), "audio/wav")}
        resp = self.client.post("/v1/transcribe", files=files)
        self.assertEqual(resp.status_code, 200)
        kwargs = self.mock_model.transcribe.call_args[1]
        self.assertEqual(kwargs["language"], "ja")

    def test_transcribe_error_returns_500(self):
        self.mock_model.transcribe.side_effect = RuntimeError("CUDA OOM")
        files = {"file": ("test.wav", io.BytesIO(b"fakeaudio"), "audio/wav")}
        resp = self.client.post("/v1/transcribe", files=files, data={"language": "ja"})
        self.assertEqual(resp.status_code, 500)
        detail = resp.json()["detail"]
        self.assertIn("CUDA OOM", detail["error"])

    def test_transcribe_missing_file_returns_422(self):
        resp = self.client.post("/v1/transcribe")
        self.assertEqual(resp.status_code, 422)

    def test_empty_segments(self):
        self.mock_model.transcribe.return_value = ([], MagicMock())
        files = {"file": ("test.wav", io.BytesIO(b"fakeaudio"), "audio/wav")}
        resp = self.client.post("/v1/transcribe", files=files)
        self.assertEqual(resp.status_code, 200)
        self.assertEqual(resp.json()["text"], "")


if __name__ == "__main__":
    unittest.main()
