"""
Unit tests for STT (Speech-to-Text) client module.

Tests cover:
- STTConfig creation and defaults
- PCM to WAV conversion
- Successful transcription via mock HTTP server
- Error handling: connection refused, timeout, no speech detected
- Retry behavior (single retry on first failure)
- Factory function
"""

import io
import json
import math
import struct
import sys
import unittest
import wave
from http.server import HTTPServer, BaseHTTPRequestHandler
from threading import Thread

sys.path.insert(0, "src")

from local_ai_companion.stt import (
    STTClient,
    STTConfig,
    create_stt_client,
    pcm_to_wav,
)


def _make_pcm_bytes(duration_ms: int, sample_rate: int = 16000) -> bytes:
    num_samples = int(sample_rate * duration_ms / 1000)
    samples = []
    for i in range(num_samples):
        sample = int(0.5 * 32767 * math.sin(2 * math.pi * 440 * i / sample_rate))
        samples.append(sample)
    return struct.pack(f"<{num_samples}h", *samples)


def _start_mock_server(handler_class) -> tuple:
    server = HTTPServer(("127.0.0.1", 0), handler_class)
    port = server.server_address[1]
    url = f"http://127.0.0.1:{port}/v1/transcribe"
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server, url


class TestSTTConfig(unittest.TestCase):

    def test_default_values(self):
        cfg = STTConfig()
        self.assertEqual(cfg.server_url, "http://192.168.12.107:8093/v1/transcribe")
        self.assertEqual(cfg.timeout_seconds, 5.0)
        self.assertEqual(cfg.language, "ja")
        self.assertEqual(cfg.sample_rate, 16000)

    def test_custom_values(self):
        cfg = STTConfig(server_url="http://localhost:9999/transcribe", timeout_seconds=10.0, language="en", sample_rate=8000)
        self.assertEqual(cfg.server_url, "http://localhost:9999/transcribe")
        self.assertEqual(cfg.timeout_seconds, 10.0)
        self.assertEqual(cfg.language, "en")
        self.assertEqual(cfg.sample_rate, 8000)


class TestPCMToWAV(unittest.TestCase):

    def test_valid_wav_output(self):
        pcm = _make_pcm_bytes(500)
        wav_bytes = pcm_to_wav(pcm, 16000)
        with wave.open(io.BytesIO(wav_bytes), "rb") as wf:
            self.assertEqual(wf.getnchannels(), 1)
            self.assertEqual(wf.getsampwidth(), 2)
            self.assertEqual(wf.getframerate(), 16000)
            frames = wf.readframes(wf.getnframes())
            self.assertEqual(frames, pcm)

    def test_zero_length_wav(self):
        wav_bytes = pcm_to_wav(b"", 16000)
        with wave.open(io.BytesIO(wav_bytes), "rb") as wf:
            self.assertEqual(wf.getnchannels(), 1)
            self.assertEqual(wf.getnframes(), 0)

    def test_different_sample_rate(self):
        pcm = _make_pcm_bytes(200, sample_rate=8000)
        wav_bytes = pcm_to_wav(pcm, 8000)
        with wave.open(io.BytesIO(wav_bytes), "rb") as wf:
            self.assertEqual(wf.getframerate(), 8000)
            frames = wf.readframes(wf.getnframes())
            self.assertEqual(frames, pcm)


class _TranscribeHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"text": "\\u3053\\u3093\\u306b\\u3061\\u306f"}')  # こんにちは
    def log_message(self, format, *args):
        pass


class TestSTTClientTranscribe(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        cls.server, cls.server_url = _start_mock_server(_TranscribeHandler)

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()

    def test_successful_transcription(self):
        cfg = STTConfig(server_url=self.server_url)
        client = STTClient(cfg)
        result = client.transcribe(_make_pcm_bytes(1000))
        self.assertEqual(result["text"], "こんにちは")
        self.assertIsNone(result["error"])

    def test_empty_pcm_produces_result(self):
        cfg = STTConfig(server_url=self.server_url)
        client = STTClient(cfg)
        result = client.transcribe(b"")
        self.assertIn("text", result)
        self.assertIn("error", result)


class _EmptyTextHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"text": ""}')
    def log_message(self, format, *args):
        pass


class TestSTTClientNoSpeech(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        cls.server, cls.server_url = _start_mock_server(_EmptyTextHandler)

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()

    def test_empty_text_returns_no_speech_error(self):
        cfg = STTConfig(server_url=self.server_url)
        client = STTClient(cfg)
        result = client.transcribe(_make_pcm_bytes(1000))
        self.assertEqual(result["text"], "")
        self.assertEqual(result["error"], "no speech detected")


class _TimeoutHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        import time
        time.sleep(10)
    def log_message(self, format, *args):
        pass


class TestSTTClientTimeout(unittest.TestCase):

    def test_timeout_returns_error(self):
        server, url = _start_mock_server(_TimeoutHandler)
        try:
            cfg = STTConfig(server_url=url, timeout_seconds=0.1)
            client = STTClient(cfg)
            result = client.transcribe(_make_pcm_bytes(100))
            self.assertEqual(result["text"], "")
            self.assertEqual(result["error"], "timeout")
        finally:
            server.shutdown()


class _RetryHandler(BaseHTTPRequestHandler):
    request_count = 0

    @classmethod
    def reset(cls):
        cls.request_count = 0

    def do_POST(self):
        type(self).request_count += 1
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        if self.request_count == 1:
            self.send_error(500, "Internal Server Error")
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"text": "\\u304a\\u306f\\u3088\\u3046"}')  # おはよう
    def log_message(self, format, *args):
        pass


class TestSTTClientRetry(unittest.TestCase):

    def test_retries_once_on_failure(self):
        _RetryHandler.reset()
        server, url = _start_mock_server(_RetryHandler)
        try:
            cfg = STTConfig(server_url=url)
            client = STTClient(cfg)
            result = client.transcribe(_make_pcm_bytes(1000))
            self.assertEqual(result["text"], "おはよう")
            self.assertIsNone(result["error"])
            self.assertEqual(_RetryHandler.request_count, 2)
        finally:
            server.shutdown()


class _AlwaysFailingHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        self.send_error(500, "Internal Server Error")
    def log_message(self, format, *args):
        pass


class TestSTTClientConnectionRefused(unittest.TestCase):

    def test_server_error_returns_connection_refused(self):
        server, url = _start_mock_server(_AlwaysFailingHandler)
        try:
            cfg = STTConfig(server_url=url)
            client = STTClient(cfg)
            result = client.transcribe(_make_pcm_bytes(1000))
            self.assertEqual(result["text"], "")
            self.assertEqual(result["error"], "connection refused")
        finally:
            server.shutdown()

    def test_invalid_url_returns_connection_refused(self):
        cfg = STTConfig(server_url="http://127.0.0.1:19999/transcribe")
        client = STTClient(cfg)
        result = client.transcribe(_make_pcm_bytes(100))
        self.assertEqual(result["text"], "")
        self.assertIsNotNone(result["error"])


class TestCreateSTTClient(unittest.TestCase):

    def test_create_no_config(self):
        client = create_stt_client()
        self.assertIsInstance(client, STTClient)
        self.assertEqual(client.config.server_url, "http://192.168.12.107:8093/v1/transcribe")

    def test_create_with_config(self):
        cfg = STTConfig(language="en")
        client = create_stt_client(cfg)
        self.assertEqual(client.config.language, "en")


class TestMultipartFormData(unittest.TestCase):

    def test_build_multipart_contains_audio_file(self):
        wav = pcm_to_wav(_make_pcm_bytes(100))
        body = STTClient._build_multipart(wav, "----Boundary")
        body_str = body.decode("utf-8", errors="replace")
        self.assertIn('name="audio_file"', body_str)
        self.assertIn("audio/wav", body_str)
        self.assertIn(wav, body)

    def test_build_multipart_contains_language_ja(self):
        wav = pcm_to_wav(_make_pcm_bytes(100))
        body = STTClient._build_multipart(wav, "----Boundary")
        body_str = body.decode("utf-8", errors="replace")
        self.assertIn('name="language"', body_str)
        self.assertIn("ja", body_str)


if __name__ == "__main__":
    unittest.main()
