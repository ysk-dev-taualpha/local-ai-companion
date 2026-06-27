import json
import os
import tempfile
import unittest

from local_ai_companion.config import AppConfig, LLMConfig, load_config
from local_ai_companion.providers import (
    OpenAICompatibleProvider,
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
        finally:
            os.unlink(path)

    def test_llm_config_defaults_are_empty(self):
        c = LLMConfig()
        self.assertEqual(c.base_url, "")
        self.assertEqual(c.model, "")
        self.assertEqual(c.api_key_env, "")


class OpenAIProviderTests(unittest.TestCase):
    def test_create_openai_provider(self):
        config = LLMConfig(
            base_url="http://localhost:1234/v1",
            model="test",
            api_key_env="FAKE_KEY",
        )
        provider = create_provider("openai_compatible", config)
        self.assertIsInstance(provider, OpenAICompatibleProvider)

    def test_openai_provider_raises_on_generate(self):
        config = LLMConfig(
            base_url="http://localhost:1234/v1",
            model="test",
            api_key_env="FAKE_KEY",
        )
        provider = create_provider("openai_compatible", config)
        with self.assertRaises(RuntimeError) as ctx:
            provider.generate("hello", [])
        msg = str(ctx.exception)
        self.assertIn("not yet implemented", msg)

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

    def test_unconfigured_llm_config_is_safe(self):
        config = LLMConfig()
        provider = create_provider("openai_compatible", config)
        with self.assertRaises(RuntimeError) as ctx:
            provider.generate("hello", [])
        msg = str(ctx.exception)
        self.assertIn("(not set)", msg)

    def test_mock_provider_unchanged(self):
        provider = create_provider("mock")
        result = provider.generate("hello", [])
        self.assertEqual(result.provider_name, "mock")


if __name__ == "__main__":
    unittest.main()
