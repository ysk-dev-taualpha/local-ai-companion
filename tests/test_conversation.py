import unittest

from local_ai_companion.conversation import ConversationCore
from local_ai_companion.providers import MockLLMProvider


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


if __name__ == "__main__":
    unittest.main()
