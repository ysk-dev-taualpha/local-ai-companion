import unittest

from local_ai_companion.history import HistoryStore


class HistoryStoreTests(unittest.TestCase):
    def test_add_and_get_recent(self):
        hs = HistoryStore(max_turns=2)
        hs.add("c1", {"n": 1})
        hs.add("c1", {"n": 2})
        hs.add("c1", {"n": 3})
        recent = hs.get_recent("c1")
        self.assertEqual(len(recent), 2)
        self.assertEqual(recent[0]["n"], 2)
        self.assertEqual(recent[1]["n"], 3)

    def test_conversation_id_isolation(self):
        hs = HistoryStore()
        hs.add("c1", {"msg": "a"})
        hs.add("c2", {"msg": "b"})
        self.assertEqual(hs.get_recent("c1"), [{"msg": "a"}])
        self.assertEqual(hs.get_recent("c2"), [{"msg": "b"}])

    def test_zero_max_turns_returns_empty(self):
        hs = HistoryStore(max_turns=0)
        hs.add("c1", {"n": 1})
        self.assertEqual(hs.get_recent("c1"), [])

    def test_count_returns_turn_count(self):
        hs = HistoryStore()
        self.assertEqual(hs.count("c1"), 0)
        hs.add("c1", {"n": 1})
        self.assertEqual(hs.count("c1"), 1)
        hs.add("c1", {"n": 2})
        self.assertEqual(hs.count("c1"), 2)

    def test_all_turns_returns_full_list(self):
        hs = HistoryStore(max_turns=2)
        for i in range(5):
            hs.add("c1", {"n": i})
        self.assertEqual(len(hs.all_turns("c1")), 5)

    def test_unknown_conversation_returns_empty(self):
        hs = HistoryStore()
        self.assertEqual(hs.get_recent("unknown"), [])


if __name__ == "__main__":
    unittest.main()
