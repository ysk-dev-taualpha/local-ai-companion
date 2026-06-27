import json
import time
from dataclasses import dataclass


@dataclass(frozen=True)
class ProviderResult:
    raw_response: str
    provider_name: str
    latency_ms: float


class LLMProvider:
    name = "base"

    def generate(self, user_text, history):
        raise NotImplementedError


class MockLLMProvider(LLMProvider):
    name = "mock"

    def generate(self, user_text, history):
        start = time.monotonic()
        response = {
            "text": "受け取りました: {}".format(user_text),
            "emotion": "neutral",
            "motion": "nod",
            "speak_style": "normal",
            "interruptible": True,
        }
        raw = json.dumps(response, ensure_ascii=False)
        latency = (time.monotonic() - start) * 1000
        return ProviderResult(
            raw_response=raw,
            provider_name=self.name,
            latency_ms=latency,
        )


def create_provider(name):
    if name == "mock":
        return MockLLMProvider()
    raise ValueError("unsupported llm provider: {}".format(name))
