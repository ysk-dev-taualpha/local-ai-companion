import json
import os
import tempfile
import unittest

from local_ai_companion.config import AppConfig, ConversationConfig, LLMConfig, load_config


class ConfigDefaultsTests(unittest.TestCase):
    """設定ファイルなし (None) の場合のデフォルト値をテストする。"""

    def test_load_config_none_returns_defaults(self):
        config = load_config(None)
        self.assertIsInstance(config, AppConfig)
        self.assertEqual(config.conversation.default_conversation_id, "default")
        self.assertEqual(config.conversation.max_history_turns, 12)
        self.assertEqual(config.llm.provider, "mock")

    def test_default_app_config_is_frozen(self):
        config = AppConfig()
        with self.assertRaises(Exception):
            config.conversation = ConversationConfig()  # type: ignore[misc]


class ConfigFileLoadingTests(unittest.TestCase):
    """実際の JSON ファイルからの読み込みをテストする。"""

    def test_load_full_config_json(self):
        full = {
            "conversation": {
                "default_conversation_id": "my-session",
                "max_history_turns": 5,
            },
            "llm": {
                "provider": "mock",
            },
        }
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".json", delete=False, encoding="utf-8"
        ) as f:
            json.dump(full, f)
            path = f.name

        try:
            config = load_config(path)
            self.assertEqual(config.conversation.default_conversation_id, "my-session")
            self.assertEqual(config.conversation.max_history_turns, 5)
            self.assertEqual(config.llm.provider, "mock")
        finally:
            os.unlink(path)

    def test_load_partial_config_uses_defaults_for_missing(self):
        partial = {"llm": {"provider": "mock"}}
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".json", delete=False, encoding="utf-8"
        ) as f:
            json.dump(partial, f)
            path = f.name

        try:
            config = load_config(path)
            self.assertEqual(config.llm.provider, "mock")
            self.assertEqual(config.conversation.default_conversation_id, "default")
            self.assertEqual(config.conversation.max_history_turns, 12)
        finally:
            os.unlink(path)

    def test_load_config_handles_missing_llm_section(self):
        partial = {"conversation": {"max_history_turns": 3}}
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".json", delete=False, encoding="utf-8"
        ) as f:
            json.dump(partial, f)
            path = f.name

        try:
            config = load_config(path)
            self.assertEqual(config.conversation.max_history_turns, 3)
            self.assertEqual(config.llm.provider, "mock")
        finally:
            os.unlink(path)

    def test_load_config_preserves_example_json_structure(self):
        """config.example.json が load_config で正しく読めることを確認。"""
        example_path = os.path.join(
            os.path.dirname(__file__), "..", "config.example.json"
        )
        normalized = os.path.normpath(example_path)
        config = load_config(normalized)
        self.assertEqual(config.conversation.default_conversation_id, "default")
        self.assertEqual(config.conversation.max_history_turns, 12)
        self.assertEqual(config.llm.provider, "mock")


if __name__ == "__main__":
    unittest.main()
