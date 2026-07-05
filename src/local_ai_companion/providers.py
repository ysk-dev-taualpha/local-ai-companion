import json
import os
import time
import urllib.error
import urllib.request
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

    def __init__(self, config, system_prompt="", response_format=""):
        self._config = config
        self._system_prompt = system_prompt
        self._response_format = response_format

    def generate(self, user_text, history):
        self._validate_config()
        start = time.monotonic()
        payload = {
            "model": self._config.model,
            "messages": self._build_messages(user_text, history),
        }
        request = urllib.request.Request(
            self._chat_completions_url(),
            data=json.dumps(payload).encode("utf-8"),
            headers=self._headers(),
            method="POST",
        )
        try:
            with urllib.request.urlopen(
                request,
                timeout=self._config.timeout_seconds,
            ) as response:
                body = response.read().decode("utf-8")
        except urllib.error.HTTPError as exc:
            raise RuntimeError(
                "OpenAI compatible provider request failed with HTTP status {}".format(
                    exc.code
                )
            ) from exc
        except urllib.error.URLError as exc:
            raise RuntimeError(
                "OpenAI compatible provider request failed: {}".format(exc.reason)
            ) from exc

        latency = (time.monotonic() - start) * 1000
        return ProviderResult(
            raw_response=self._extract_content(body),
            provider_name=self.name,
            latency_ms=latency,
        )

    def _validate_config(self):
        if self._config is None:
            raise RuntimeError("OpenAI compatible provider requires llm config")
        if not self._config.base_url:
            raise RuntimeError("OpenAI compatible provider requires llm.base_url")
        if not self._config.model:
            raise RuntimeError("OpenAI compatible provider requires llm.model")
        if self._config.api_key_env and self._config.api_key_env not in os.environ:
            raise RuntimeError(
                "OpenAI compatible provider API key env var is not set: {}".format(
                    self._config.api_key_env
                )
            )

    def _chat_completions_url(self):
        return self._config.base_url.rstrip("/") + "/chat/completions"

    def _headers(self):
        headers = {
            "Accept": "application/json",
            "Content-Type": "application/json",
        }
        if self._config.api_key_env:
            headers["Authorization"] = "Bearer {}".format(
                os.environ[self._config.api_key_env]
            )
        return headers

    def _build_messages(self, user_text, history):
        messages = []
        system_content = self._combined_system_prompt()
        if system_content:
            messages.append({"role": "system", "content": system_content})
        for turn in history:
            if turn.get("user_text"):
                messages.append({"role": "user", "content": turn["user_text"]})
            assistant = turn.get("assistant") or {}
            assistant_text = assistant.get("text")
            if assistant_text:
                messages.append({"role": "assistant", "content": assistant_text})
        messages.append({"role": "user", "content": user_text})
        return messages

    def _combined_system_prompt(self):
        parts = []
        if self._system_prompt:
            parts.append(self._system_prompt)
        if self._response_format:
            parts.append(self._response_format)
        return "\n\n".join(parts)

    def _extract_content(self, body):
        try:
            parsed = json.loads(body)
            return parsed["choices"][0]["message"]["content"]
        except (KeyError, IndexError, TypeError, json.JSONDecodeError) as exc:
            raise RuntimeError(
                "OpenAI compatible provider response did not include "
                "choices[0].message.content"
            ) from exc


def create_provider(name, config=None, system_prompt="", response_format=""):
    if name == "mock":
        return MockLLMProvider()
    if name == "openai_compatible":
        if config is None:
            return OpenAICompatibleProvider(config=None)
        return OpenAICompatibleProvider(
            config,
            system_prompt=system_prompt,
            response_format=response_format,
        )
    raise ValueError("unsupported llm provider: {}".format(name))
