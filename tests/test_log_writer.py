import json
import os
import tempfile
import unittest

from local_ai_companion.conversation import ConversationCore
from local_ai_companion.log_writer import JSONLLogWriter
from local_ai_companion.providers import MockLLMProvider


class JSONLLogWriterTests(unittest.TestCase):
    def test_write_creates_jsonl_file(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            writer = JSONLLogWriter(tmpdir)
            writer.write({"key": "value"})
            writer.close()

            log_path = os.path.join(tmpdir, "conversation.jsonl")
            self.assertTrue(os.path.isfile(log_path))

            with open(log_path, "r", encoding="utf-8") as f:
                lines = f.readlines()
            self.assertEqual(len(lines), 1)

            parsed = json.loads(lines[0])
            self.assertEqual(parsed["key"], "value")

    def test_multiple_writes_append(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            writer = JSONLLogWriter(tmpdir)
            writer.write({"n": 1})
            writer.write({"n": 2})
            writer.close()

            log_path = os.path.join(tmpdir, "conversation.jsonl")
            with open(log_path, "r", encoding="utf-8") as f:
                lines = f.readlines()
            self.assertEqual(len(lines), 2)

    def test_default_filename_is_conversation_jsonl(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            writer = JSONLLogWriter(tmpdir)
            writer.write({"x": 1})
            writer.close()
            self.assertTrue(os.path.isfile(os.path.join(tmpdir, "conversation.jsonl")))


class ConversationLoggingTests(unittest.TestCase):
    def test_core_writes_to_log_writer(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            writer = JSONLLogWriter(tmpdir)
            core = ConversationCore(MockLLMProvider(), log_writer=writer)
            core.send("hello", request_id="req-1")
            writer.close()

            log_path = os.path.join(tmpdir, "conversation.jsonl")
            with open(log_path, "r", encoding="utf-8") as f:
                lines = f.readlines()
            self.assertEqual(len(lines), 1)

            entry = json.loads(lines[0])
            self.assertEqual(entry["request_id"], "req-1")
            self.assertEqual(entry["user_text"], "hello")
            self.assertEqual(entry["provider"], "mock")
            self.assertIn("assistant", entry)
            self.assertTrue(entry["valid"])

    def test_core_without_log_writer_does_not_crash(self):
        core = ConversationCore(MockLLMProvider())
        response = core.send("hello")
        self.assertEqual(response["assistant"]["emotion"], "neutral")

    def test_log_contains_required_fields(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            writer = JSONLLogWriter(tmpdir)
            core = ConversationCore(MockLLMProvider(), log_writer=writer)
            core.send("test", request_id="r1", conversation_id="c1")
            writer.close()

            log_path = os.path.join(tmpdir, "conversation.jsonl")
            with open(log_path, "r", encoding="utf-8") as f:
                entry = json.loads(f.readline())

            required = {
                "request_id", "conversation_id", "user_text",
                "assistant", "valid", "error", "provider",
            }
            for field in required:
                self.assertIn(field, entry, "missing field: {}".format(field))


if __name__ == "__main__":
    unittest.main()
