import json
import os
import tempfile
import unittest
from unittest.mock import patch

from local_ai_companion.config import AppConfig, LLMConfig, load_config
from local_ai_companion.providers import (
    OpenAICompatibleProvider,
    ProviderResult,
    create_provider,
)


class OpenAIConfigTests(unittest.TestCase):
    def test_load_llm_config_with_openai_fields(self):
        data = {
            "llm": {
                "provider": "openai_compatible",
                "base_url": "http://localhost:8080/v1",
                "model": "test-model",
                "api_key_env": "OPENAI_KEY",
                "timeout_seconds": 5,
            }
        }
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".json", delete=False, encoding="utf-8"
        ) as f:
            json.dump(data, f)
            path = f.name

        try:
            config = load_config(path)
            self.assertEqual(config.llm.provider, "openai_compatible")
            self.assertEqual(config.llm.base_url, "http://localhost:8080/v1")
            self.assertEqual(config.llm.model, "test-model")
            self.assertEqual(config.llm.api_key_env, "OPENAI_KEY")
            self.assertEqual(config.llm.timeout_seconds, 5.0)
        finally:
            os.unlink(path)

    def test_llm_config_defaults_are_empty(self):
        c = LLMConfig()
        self.assertEqual(c.base_url, "")
        self.assertEqual(c.model, "")
        self.assertEqual(c.api_key_env, "")
        self.assertEqual(c.timeout_seconds, 30.0)


class OpenAIProviderTests(unittest.TestCase):
    def test_create_openai_provider(self):
        config = LLMConfig(
            base_url="http://localhost:1234/v1",
            model="test",
            api_key_env="FAKE_KEY",
        )
        provider = create_provider("openai_compatible", config)
        self.assertIsInstance(provider, OpenAICompatibleProvider)

    def test_openai_provider_posts_chat_completion_request(self):
        config = LLMConfig(
            base_url="http://localhost:1234/v1",
            model="test",
            api_key_env="FAKE_KEY",
            timeout_seconds=3.0,
        )
        provider = create_provider("openai_compatible", config)

        response_body = json.dumps(
            {
                "choices": [
                    {
                        "message": {
                            "content": json.dumps(
                                {
                                    "text": "ok",
                                    "emotion": "neutral",
                                    "motion": "nod",
                                    "speak_style": "normal",
                                    "interruptible": True,
                                }
                            )
                        }
                    }
                ]
            }
        ).encode("utf-8")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def read(self):
                return response_body

        with patch.dict(os.environ, {"FAKE_KEY": "secret-value"}), patch(
            "urllib.request.urlopen",
            return_value=FakeResponse(),
        ) as urlopen:
            result = provider.generate(
                "hello",
                [
                    {
                        "user_text": "before",
                        "assistant": {"text": "previous answer"},
                    }
                ],
            )

        self.assertIsInstance(result, ProviderResult)
        self.assertEqual(result.provider_name, "openai_compatible")
        self.assertIn('"text": "ok"', result.raw_response)

        request = urlopen.call_args[0][0]
        self.assertEqual(
            request.get_full_url(),
            "http://localhost:1234/v1/chat/completions",
        )
        self.assertEqual(urlopen.call_args[1]["timeout"], 3.0)
        self.assertEqual(request.headers["Authorization"], "Bearer secret-value")
        payload = json.loads(request.data.decode("utf-8"))
        self.assertEqual(payload["model"], "test")
        self.assertEqual(
            payload["messages"],
            [
                {"role": "user", "content": "before"},
                {"role": "assistant", "content": "previous answer"},
                {"role": "user", "content": "hello"},
            ],
        )

    def test_openai_provider_includes_prompts_as_system_message(self):
        config = LLMConfig(
            base_url="http://localhost:1234/v1",
            model="test",
        )
        provider = create_provider(
            "openai_compatible",
            config,
            system_prompt="system rules",
            response_format="format rules",
        )

        response_body = json.dumps(
            {
                "choices": [
                    {
                        "message": {
                            "content": json.dumps(
                                {
                                    "text": "ok",
                                    "emotion": "neutral",
                                    "motion": "nod",
                                    "speak_style": "normal",
                                    "interruptible": True,
                                }
                            )
                        }
                    }
                ]
            }
        ).encode("utf-8")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def read(self):
                return response_body

        with patch("urllib.request.urlopen", return_value=FakeResponse()) as urlopen:
            provider.generate("hello", [])

        request = urlopen.call_args[0][0]
        payload = json.loads(request.data.decode("utf-8"))
        self.assertEqual(payload["messages"][0]["role"], "system")
        self.assertIn("system rules", payload["messages"][0]["content"])
        self.assertIn("format rules", payload["messages"][0]["content"])

    def test_error_message_does_not_expose_secret_values(self):
        config = LLMConfig(
            base_url="http://localhost:1234/v1",
            model="test",
            api_key_env="FAKE_KEY",
        )
        provider = create_provider("openai_compatible", config)
        with self.assertRaises(RuntimeError) as ctx:
            provider.generate("hello", [])
        msg = str(ctx.exception)
        self.assertIn("FAKE_KEY", msg)
        self.assertNotIn("secret-value", msg)

    def test_unconfigured_llm_config_is_safe(self):
        config = LLMConfig()
        provider = create_provider("openai_compatible", config)
        with self.assertRaises(RuntimeError) as ctx:
            provider.generate("hello", [])
        msg = str(ctx.exception)
        self.assertIn("llm.base_url", msg)

    def test_mock_provider_unchanged(self):
        provider = create_provider("mock")
        result = provider.generate("hello", [])
        self.assertEqual(result.provider_name, "mock")


if __name__ == "__main__":
    unittest.main()
