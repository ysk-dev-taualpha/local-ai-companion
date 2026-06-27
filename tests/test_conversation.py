import unittest

from local_ai_companion.conversation import ConversationCore
from local_ai_companion.history import HistoryStore
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


if __name__ == "__main__":
    unittest.main()
