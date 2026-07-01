import json
import threading
import time
import unittest
from http.client import HTTPConnection

from local_ai_companion.config import AppConfig
from local_ai_companion.server import run_server


class TestHTTPServer:
    def __init__(self):
        self._server = None
        self._thread = None
        self._port = None

    def start(self):
        config = AppConfig()
        from http.server import HTTPServer
        from local_ai_companion.conversation import ConversationCore
        from local_ai_companion.providers import create_provider
        from local_ai_companion.server import _make_handler

        provider = create_provider(config.llm.provider, config.llm)
        core = ConversationCore(provider=provider, max_history_turns=config.conversation.max_history_turns)
        handler = _make_handler(core, config)

        self._server = HTTPServer(("127.0.0.1", 0), handler)
        self._port = self._server.server_address[1]
        self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)
        self._thread.start()
        time.sleep(0.1)

    def stop(self):
        if self._server:
            self._server.shutdown()
            self._thread.join(timeout=1)

    @property
    def port(self):
        return self._port

    def request(self, method, path, body=None):
        conn = HTTPConnection("127.0.0.1", self._port, timeout=2)
        conn.request(method, path, body=json.dumps(body).encode("utf-8") if body else None,
                     headers={"Content-Type": "application/json"})
        resp = conn.getresponse()
        data = resp.read().decode("utf-8")
        conn.close()
        return resp.status, json.loads(data)


class ServerIntegrationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = TestHTTPServer()
        cls.server.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.stop()

    def test_post_conversation_returns_response(self):
        status, data = self.server.request("POST", "/v1/conversation",
                                           {"message": "hello", "request_id": "req-1"})
        self.assertEqual(status, 200)
        self.assertEqual(data["request_id"], "req-1")
        self.assertIn("assistant", data)
        self.assertEqual(data["assistant"]["emotion"], "neutral")

    def test_missing_message_returns_400(self):
        status, data = self.server.request("POST", "/v1/conversation", {})
        self.assertEqual(status, 400)
        self.assertIn("error", data)

    def test_invalid_body_returns_400(self):
        conn = HTTPConnection("127.0.0.1", self.server.port, timeout=2)
        conn.request("POST", "/v1/conversation", body=b"not json",
                     headers={"Content-Type": "application/json"})
        resp = conn.getresponse()
        conn.close()
        self.assertEqual(resp.status, 400)

    def test_wrong_path_returns_404(self):
        status, data = self.server.request("POST", "/wrong", {"message": "hi"})
        self.assertEqual(status, 404)

    def test_conversation_id_passed_through(self):
        status, data = self.server.request("POST", "/v1/conversation",
                                           {"message": "hi", "conversation_id": "session-1"})
        self.assertEqual(status, 200)
        self.assertEqual(data["conversation_id"], "session-1")


if __name__ == "__main__":
    unittest.main()
