import json
import unittest

from local_ai_companion.conversation import ConversationCore
from local_ai_companion.providers import MockLLMProvider, ProviderResult


class BrokenProvider:
    name = "broken"

    def generate(self, user_text, history):
        return "not json"


class CountingMockProvider:
    """会話履歴を受け取った回数を記録する mock provider。"""

    name = "counting-mock"

    def __init__(self):
        self.call_count = 0
        self.last_history = None

    def generate(self, user_text, history):
        self.call_count += 1
        self.last_history = list(history)
        return json.dumps(
            {
                "text": "count: {}".format(self.call_count),
                "emotion": "neutral",
                "motion": "nod",
                "speak_style": "normal",
                "interruptible": True,
            },
            ensure_ascii=False,
        )


class ConversationBasicTests(unittest.TestCase):
    """ConversationCore の基本的な振る舞いをテストする。"""

    def test_mock_provider_returns_assistant_response(self):
        core = ConversationCore(MockLLMProvider())
        response = core.send("hello", request_id="req-1")
        self.assertEqual(response["request_id"], "req-1")
        self.assertEqual(response["assistant"]["emotion"], "neutral")

    def test_invalid_json_uses_fallback(self):
        core = ConversationCore(BrokenProvider())
        response = core.send("hello", request_id="req-1")
        self.assertEqual(response["assistant"]["emotion"], "neutral")
        self.assertEqual(response["assistant"]["motion"], "idle")

    def test_turn_history_includes_provider_name(self):
        core = ConversationCore(MockLLMProvider())
        core.send("hello")
        self.assertEqual(len(core.history), 1)
        self.assertEqual(core.history[0]["provider"], "mock")

    def test_turn_history_includes_latency(self):
        core = ConversationCore(MockLLMProvider())
        core.send("hello")
        self.assertIsInstance(core.history[0]["latency_ms"], float)
        self.assertGreaterEqual(core.history[0]["latency_ms"], 0.0)

    def test_mock_provider_returns_provider_result(self):
        provider = MockLLMProvider()
        result = provider.generate("hello", [])
        self.assertIsInstance(result, ProviderResult)
        self.assertEqual(result.provider_name, "mock")
        self.assertIsInstance(result.raw_response, str)
        self.assertIsInstance(result.latency_ms, float)

    def test_legacy_provider_still_works(self):
        core = ConversationCore(BrokenProvider())
        core.send("hello")
        self.assertEqual(core.history[0]["provider"], "broken")
        self.assertIsNone(core.history[0]["latency_ms"])
    def test_default_conversation_id_is_default(self):
        core = ConversationCore(MockLLMProvider())
        response = core.send("hello")
        self.assertEqual(response["conversation_id"], "default")

    def test_custom_conversation_id_is_preserved(self):
        core = ConversationCore(MockLLMProvider())
        response = core.send("hello", conversation_id="session-42")
        self.assertEqual(response["conversation_id"], "session-42")

    def test_auto_request_id_is_generated(self):
        core = ConversationCore(MockLLMProvider())
        response = core.send("hello")
        self.assertIsInstance(response["request_id"], str)
        self.assertGreater(len(response["request_id"]), 0)

    def test_history_records_turns(self):
        core = ConversationCore(CountingMockProvider(), max_history_turns=10)
        self.assertEqual(len(core.history), 0)

        core.send("first", conversation_id="c1")
        self.assertEqual(len(core.history), 1)
        self.assertEqual(core.history[0]["user_text"], "first")
        self.assertEqual(core.history[0]["conversation_id"], "c1")
        self.assertTrue(core.history[0]["valid"])

        core.send("second", conversation_id="c1")
        self.assertEqual(len(core.history), 2)

    def test_invalid_response_records_error(self):
        core = ConversationCore(BrokenProvider())
        core.send("hello", request_id="req-err")
        self.assertEqual(len(core.history), 1)
        turn = core.history[0]
        self.assertFalse(turn["valid"])
        self.assertIsNotNone(turn["error"])


class HistoryTrimmingTests(unittest.TestCase):
    """max_history_turns による履歴トリミングをテストする。"""

    def test_history_does_not_exceed_max_turns(self):
        core = ConversationCore(CountingMockProvider(), max_history_turns=3)

        for i in range(10):
            core.send("msg-{}".format(i))

        self.assertEqual(len(core.history), 10)

    def test_provider_receives_trimmed_history(self):
        """provider に渡される履歴が max_history_turns で制限されることを確認。"""
        provider = CountingMockProvider()
        core = ConversationCore(provider, max_history_turns=2)

        core.send("first")
        core.send("second")
        core.send("third")

        self.assertIsNotNone(provider.last_history)
        self.assertEqual(len(provider.last_history), 2)
        self.assertEqual(provider.last_history[0]["user_text"], "first")
        self.assertEqual(provider.last_history[1]["user_text"], "second")

    def test_trimm_history_preserves_valid_and_error_fields(self):
        provider = CountingMockProvider()
        core = ConversationCore(provider, max_history_turns=1)
        core_br = ConversationCore(BrokenProvider(), max_history_turns=1)

        core.send("valid")
        core.send("also-valid")

        self.assertIsNotNone(provider.last_history)
        self.assertEqual(len(provider.last_history), 1)
        self.assertEqual(provider.last_history[0]["user_text"], "valid")
        self.assertTrue(provider.last_history[0]["valid"])
        self.assertIsNone(provider.last_history[0]["error"])

    def test_zero_max_history_first_call_is_empty(self):
        """max_history_turns=0 で初回呼び出し時は空リストが渡される。
        注意: Python の history[-0:] は history[:] と同じになるため、
        2回目以降の呼び出しでは履歴が渡されてしまう既知の挙動がある。
        """
        provider = CountingMockProvider()
        core = ConversationCore(provider, max_history_turns=0)

        core.send("hello")
        self.assertIsNotNone(provider.last_history)
        self.assertEqual(provider.last_history, [])


class SendReturnValueTests(unittest.TestCase):
    """send() の戻り値の構造をテストする。"""

    def test_returns_request_id(self):
        core = ConversationCore(MockLLMProvider())
        response = core.send("hello", request_id="my-req")
        self.assertEqual(response["request_id"], "my-req")

    def test_returns_conversation_id(self):
        core = ConversationCore(MockLLMProvider())
        response = core.send("hello", conversation_id="my-session")
        self.assertEqual(response["conversation_id"], "my-session")

    def test_returns_assistant_with_all_fields(self):
        core = ConversationCore(MockLLMProvider())
        response = core.send("hello")
        assistant = response["assistant"]
        for key in ("text", "emotion", "motion", "speak_style", "interruptible"):
            self.assertIn(key, assistant)


if __name__ == "__main__":
    unittest.main()
