import json
from http.server import HTTPServer, BaseHTTPRequestHandler

from .config import AppConfig, load_config
from .conversation import ConversationCore
from .providers import create_provider


class ConversationHandler(BaseHTTPRequestHandler):
    core = None
    config = None

    def do_POST(self):
        if self.path != "/v1/conversation":
            self._send_json(404, {"error": {"code": "not_found", "message": "not found"}})
            return

        try:
            length = int(self.headers.get("Content-Length", "0"))
            body = self.rfile.read(length)
            request_data = json.loads(body)
        except Exception:
            self._send_json(400, {"error": {"code": "invalid_request", "message": "invalid JSON body"}})
            return

        message = request_data.get("message", "")
        if not isinstance(message, str) or not message.strip():
            self._send_json(400, {"error": {"code": "invalid_request", "message": "message is required"}})
            return

        conversation_id = request_data.get("conversation_id", self.config.conversation.default_conversation_id)
        request_id = request_data.get("request_id")

        response = self.core.send(
            message,
            conversation_id=conversation_id,
            request_id=request_id,
        )
        self._send_json(200, response)

    def _send_json(self, status, data):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        pass


def run_server(config: AppConfig, host="127.0.0.1", port=8090):
    provider = create_provider(config.llm.provider, config.llm)
    core = ConversationCore(
        provider=provider,
        max_history_turns=config.conversation.max_history_turns,
    )

    handler = _make_handler(core, config)
    server = HTTPServer((host, port), handler)
    print("Python AI Service listening on {}:{}".format(host, port))
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    server.server_close()


def _make_handler(core, config):
    class BoundHandler(ConversationHandler):
        pass
    BoundHandler.core = core
    BoundHandler.config = config
    return BoundHandler
