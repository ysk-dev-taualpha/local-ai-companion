import json
import unittest

from local_ai_companion.conversation import ConversationCore
from local_ai_companion.history import HistoryStore
from local_ai_companion.providers import MockLLMProvider, ProviderResult


class BrokenProvider:
    name = "broken"

    def generate(self, user_text, history):
        return "not json"


class CountingMockProvider:
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
        turn = core.history_store.all_turns("default")[0]
        self.assertEqual(turn["provider"], "broken")
        self.assertIsNone(turn["latency_ms"])

    def test_history_store_tracks_turns(self):
        hs = HistoryStore()
        core = ConversationCore(MockLLMProvider(), history_store=hs)
        core.send("hello", conversation_id="c1")
        self.assertEqual(hs.count("c1"), 1)
        self.assertEqual(hs.all_turns("c1")[0]["user_text"], "hello")

    def test_history_store_isolates_conversations(self):
        hs = HistoryStore()
        core = ConversationCore(MockLLMProvider(), history_store=hs)
        core.send("a", conversation_id="c1")
        core.send("b", conversation_id="c2")
        self.assertEqual(hs.count("c1"), 1)
        self.assertEqual(hs.count("c2"), 1)

    def test_max_history_turns_respected(self):
        hs = HistoryStore(max_turns=2)
        core = ConversationCore(MockLLMProvider(), max_history_turns=2, history_store=hs)
        for i in range(5):
            core.send("msg-{}".format(i), conversation_id="c1")
        self.assertEqual(len(hs.all_turns("c1")), 5)
        self.assertEqual(len(hs.get_recent("c1", 2)), 2)

    def test_turn_history_has_provider_and_latency(self):
        hs = HistoryStore()
        core = ConversationCore(MockLLMProvider(), history_store=hs)
        core.send("hello")
        turn = hs.all_turns("default")[0]
        self.assertEqual(turn["provider"], "mock")
        self.assertIsInstance(turn["latency_ms"], float)
        self.assertGreaterEqual(turn["latency_ms"], 0.0)

    def test_invalid_response_records_error(self):
        hs = HistoryStore()
        core = ConversationCore(BrokenProvider(), history_store=hs)
        core.send("hello", request_id="req-err")
        turn = hs.all_turns("default")[0]
        self.assertFalse(turn["valid"])
        self.assertIsNotNone(turn["error"])


class HistoryTrimmingTests(unittest.TestCase):
    def test_provider_receives_trimmed_history(self):
        provider = CountingMockProvider()
        hs = HistoryStore()
        core = ConversationCore(provider, max_history_turns=2, history_store=hs)
        core.send("first")
        core.send("second")
        core.send("third")
        self.assertEqual(len(provider.last_history), 2)
        self.assertEqual(provider.last_history[0]["user_text"], "first")
        self.assertEqual(provider.last_history[1]["user_text"], "second")

    def test_trimming_preserves_valid_and_error_fields(self):
        provider = CountingMockProvider()
        hs = HistoryStore()
        core = ConversationCore(provider, max_history_turns=1, history_store=hs)
        core.send("valid")
        core.send("also-valid")
        self.assertEqual(len(provider.last_history), 1)
        self.assertEqual(provider.last_history[0]["user_text"], "valid")
        self.assertTrue(provider.last_history[0]["valid"])
        self.assertIsNone(provider.last_history[0]["error"])

    def test_zero_max_history_first_call_is_empty(self):
        provider = CountingMockProvider()
        hs = HistoryStore()
        core = ConversationCore(provider, max_history_turns=0, history_store=hs)
        core.send("hello")
        self.assertEqual(provider.last_history, [])


class SendReturnValueTests(unittest.TestCase):
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
