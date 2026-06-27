import json
import os
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


class OpenAICompatibleProvider(LLMProvider):
    name = "openai_compatible"

    def __init__(self, config):
        self._config = config

    def generate(self, user_text, history):
        raise RuntimeError(
            "OpenAI compatible provider is not yet implemented. "
            "Configured: base_url={}, model={}, api_key_env={}".format(
                repr(self._config.base_url) if self._config.base_url else "(not set)",
                repr(self._config.model) if self._config.model else "(not set)",
                repr(self._config.api_key_env) if self._config.api_key_env else "(not set)",
            )
        )


def create_provider(name, config=None):
    if name == "mock":
        return MockLLMProvider()
    if name == "openai_compatible":
        if config is None:
            return OpenAICompatibleProvider(config=None)
        return OpenAICompatibleProvider(config)
    raise ValueError("unsupported llm provider: {}".format(name))
