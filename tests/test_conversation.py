import unittest

from local_ai_companion.conversation import ConversationCore
from local_ai_companion.providers import MockLLMProvider, ProviderResult


class BrokenProvider:
    name = "broken"

    def generate(self, user_text, history):
        return "not json"


class ConversationTests(unittest.TestCase):
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


if __name__ == "__main__":
    unittest.main()
