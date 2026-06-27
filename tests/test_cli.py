import unittest

from local_ai_companion.cli import build_parser, run_once
from local_ai_companion.config import AppConfig


class CLITests(unittest.TestCase):
    def test_parser_accepts_message(self):
        parser = build_parser()
        args = parser.parse_args(["--message", "hello"])
        self.assertEqual(args.message, "hello")

    def test_parser_accepts_conversation_id(self):
        parser = build_parser()
        args = parser.parse_args(["--conversation-id", "session-1", "--message", "hi"])
        self.assertEqual(args.conversation_id, "session-1")

    def test_parser_accepts_request_id(self):
        parser = build_parser()
        args = parser.parse_args(["--request-id", "req-123", "--message", "hi"])
        self.assertEqual(args.request_id, "req-123")

    def test_parser_defaults_are_none(self):
        parser = build_parser()
        args = parser.parse_args(["--message", "hi"])
        self.assertIsNone(args.conversation_id)
        self.assertIsNone(args.request_id)

    def test_run_once_uses_conversation_id(self):
        config = AppConfig()
        exit_code = run_once(config, "my-session", "req-1", "hello")
        self.assertEqual(exit_code, 0)

    def test_run_once_uses_request_id_in_output(self):
        import io
        import sys

        config = AppConfig()
        old_stdout = sys.stdout
        try:
            buf = io.StringIO()
            sys.stdout = buf
            run_once(config, "c1", "req-xyz", "hello")
            output = buf.getvalue()
            self.assertIn("req-xyz", output)
            self.assertIn("c1", output)
        finally:
            sys.stdout = old_stdout


if __name__ == "__main__":
    unittest.main()
