"""Tests for faster-whisper Transcription Server.

Run on WinPC:  pytest test_server.py -v
Run on X1C6:   python -m unittest test_mock_server.py -v  (mock mode)
"""

import io
import json
import os
import sys
import tempfile
import unittest
from unittest.mock import MagicMock, patch

# ---------------------------------------------------------------------------
# Mock faster_whisper before importing server
# ---------------------------------------------------------------------------
_mock_whisper = MagicMock()
_mock_whisper.WhisperModel = MagicMock()
sys.modules["faster_whisper"] = _mock_whisper

# Now safe to import
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "."))
from server import app  # noqa: E402

from fastapi.testclient import TestClient


class TestWhisperServerMocked(unittest.TestCase):
    """Unit tests for whisper-server with mocked faster-whisper."""

    def setUp(self):
        self.client = TestClient(app)
        # Reset mock state for each test
        _mock_whisper.WhisperModel.reset_mock()
        self.mock_model = _mock_whisper.WhisperModel.return_value
        # Clear any side effects from previous tests
        self.mock_model.transcribe.side_effect = None

    def test_health_endpoint(self):
        """GET /health returns status ok."""
        resp = self.client.get("/health")
        self.assertEqual(resp.status_code, 200)
        data = resp.json()
        self.assertEqual(data["status"], "ok")
        self.assertIn("model", data)

    def test_transcribe_returns_text(self):
        """POST /v1/transcribe returns transcribed text."""
        # Mock segments
        mock_seg1 = MagicMock()
        mock_seg1.text = "こんにちは"
        mock_seg2 = MagicMock()
        mock_seg2.text = "元気ですか"
        self.mock_model.transcribe.return_value = (
            [mock_seg1, mock_seg2],
            MagicMock(),  # info
        )

        # Create a fake WAV file
        wav_content = b"RIFF\x24\x00\x00\x00WAVEfakeaudio"  # minimal header
        files = {"file": ("test.wav", io.BytesIO(wav_content), "audio/wav")}
        data = {"language": "ja"}

        resp = self.client.post("/v1/transcribe", files=files, data=data)

        self.assertEqual(resp.status_code, 200)
        body = resp.json()
        self.assertEqual(body["text"], "こんにちは元気ですか")
        self.assertIsNone(body["error"])
        self.assertGreaterEqual(body["duration"], 0)

    def test_transcribe_with_default_language(self):
        """POST /v1/transcribe defaults to ja when language not specified."""
        mock_seg = MagicMock()
        mock_seg.text = "テスト"
        self.mock_model.transcribe.return_value = (
            [mock_seg],
            MagicMock(),
        )

        wav_content = b"RIFF\x24\x00\x00\x00WAVEfakeaudio"
        files = {"file": ("test.wav", io.BytesIO(wav_content), "audio/wav")}

        resp = self.client.post("/v1/transcribe", files=files)

        self.assertEqual(resp.status_code, 200)
        self.assertEqual(resp.json()["text"], "テスト")

        # Verify language default
        kwargs = self.mock_model.transcribe.call_args[1]
        self.assertEqual(kwargs["language"], "ja")

    def test_transcribe_custom_language(self):
        """POST /v1/transcribe accepts language parameter."""
        mock_seg = MagicMock()
        mock_seg.text = "hello world"
        self.mock_model.transcribe.return_value = (
            [mock_seg],
            MagicMock(),
        )

        wav_content = b"RIFF\x24\x00\x00\x00WAVEfakeaudio"
        files = {"file": ("test.wav", io.BytesIO(wav_content), "audio/wav")}
        data = {"language": "en"}

        resp = self.client.post("/v1/transcribe", files=files, data=data)

        self.assertEqual(resp.status_code, 200)
        self.assertEqual(resp.json()["text"], "hello world")
        kwargs = self.mock_model.transcribe.call_args[1]
        self.assertEqual(kwargs["language"], "en")

    def test_transcribe_error_returns_500(self):
        """POST /v1/transcribe returns 500 on model failure."""
        self.mock_model.transcribe.side_effect = RuntimeError("CUDA out of memory")

        wav_content = b"RIFF\x24\x00\x00\x00WAVEfakeaudio"
        files = {"file": ("test.wav", io.BytesIO(wav_content), "audio/wav")}
        data = {"language": "ja"}

        resp = self.client.post("/v1/transcribe", files=files, data=data)

        self.assertEqual(resp.status_code, 500)
        detail = resp.json()["detail"]
        self.assertEqual(detail["text"], "")
        self.assertIn("CUDA out of memory", detail["error"])

    def test_transcribe_missing_file_returns_422(self):
        """POST /v1/transcribe without file returns 422."""
        resp = self.client.post("/v1/transcribe")
        self.assertEqual(resp.status_code, 422)

    def test_transcribe_empty_segments_returns_empty_text(self):
        """POST /v1/transcribe with no segments returns empty string."""
        self.mock_model.transcribe.return_value = ([], MagicMock())

        wav_content = b"RIFF\x24\x00\x00\x00WAVEfakeaudio"
        files = {"file": ("test.wav", io.BytesIO(wav_content), "audio/wav")}

        resp = self.client.post("/v1/transcribe", files=files)

        self.assertEqual(resp.status_code, 200)
        self.assertEqual(resp.json()["text"], "")

    def test_model_environment_variables(self):
        """Model configuration is read from environment variables."""
        model_size = os.environ.get("WHISPER_MODEL", "small")
        device = os.environ.get("WHISPER_DEVICE", "cuda")
        compute_type = os.environ.get("WHISPER_COMPUTE_TYPE", "float16")

        self.assertEqual(model_size, "small")
        self.assertIn(device, ("cuda", "cpu"))
        self.assertIn(compute_type, ("float16", "float32", "int8"))


if __name__ == "__main__":
    unittest.main()
