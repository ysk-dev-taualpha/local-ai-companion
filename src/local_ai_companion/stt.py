"""STT (Speech-to-Text) client for faster-whisper integration.

Sends accumulated PCM audio data to the faster-whisper HTTP server on WinPC
for speech recognition. Handles connection errors, timeouts, and empty results
with graceful fallback responses.
"""

import io
import logging
import json
import urllib.request
import urllib.error
import wave
from dataclasses import dataclass
from typing import Optional

logger = logging.getLogger(__name__)


@dataclass
class STTConfig:
    """STT client configuration parameters."""

    server_url: str = "http://192.168.12.107:8093/v1/transcribe"
    timeout_seconds: float = 5.0
    language: str = "ja"
    sample_rate: int = 16000


def pcm_to_wav(pcm_data: bytes, sample_rate: int = 16000) -> bytes:
    """Convert raw PCM int16 mono audio to WAV format."""
    buf = io.BytesIO()
    with wave.open(buf, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(sample_rate)
        wf.writeframes(pcm_data)
    return buf.getvalue()


class STTClient:
    """HTTP client for faster-whisper speech recognition."""

    def __init__(self, config: Optional[STTConfig] = None):
        self.config = config or STTConfig()

    def transcribe(self, pcm_bytes: bytes) -> dict:
        wav_bytes = pcm_to_wav(pcm_bytes, self.config.sample_rate)
        result = self._send_request(wav_bytes)
        if result["error"] is None:
            return result
        logger.info("STT first attempt failed (%s), retrying...", result["error"])
        return self._send_request(wav_bytes)

    def _send_request(self, wav_bytes: bytes) -> dict:
        boundary = "----STTFormBoundary"
        body = self._build_multipart(wav_bytes, boundary)
        req = urllib.request.Request(
            self.config.server_url,
            data=body,
            headers={"Content-Type": f"multipart/form-data; boundary={boundary}"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=self.config.timeout_seconds) as resp:
                data = json.loads(resp.read().decode("utf-8"))
                text = data.get("text", "").strip()
                if not text:
                    return {"text": "", "error": "no speech detected"}
                return {"text": text, "error": None}
        except urllib.error.URLError as e:
            reason = str(e.reason) if e.reason else str(e)
            if "refused" in reason.lower() or "connection" in reason.lower():
                return {"text": "", "error": "connection refused"}
            return {"text": "", "error": "connection refused"}
        except TimeoutError:
            return {"text": "", "error": "timeout"}
        except Exception as e:
            logger.exception("Unexpected STT error")
            return {"text": "", "error": "connection refused"}

    @staticmethod
    def _build_multipart(wav_bytes: bytes, boundary: str) -> bytes:
        lines = [
            f"--{boundary}",
            'Content-Disposition: form-data; name="audio_file"; filename="audio.wav"',
            "Content-Type: audio/wav",
            "",
        ]
        body = "\r\n".join(lines).encode("utf-8") + b"\r\n"
        body += wav_bytes
        body += f"\r\n--{boundary}".encode("utf-8")
        body += b"\r\n"
        body += 'Content-Disposition: form-data; name="language"'.encode("utf-8")
        body += b"\r\n\r\nja"
        body += f"\r\n--{boundary}--\r\n".encode("utf-8")
        return body


def create_stt_client(config: Optional[STTConfig] = None) -> STTClient:
    return STTClient(config)
