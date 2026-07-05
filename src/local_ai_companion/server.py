import json
from http.server import HTTPServer, BaseHTTPRequestHandler

from .config import AppConfig, build_prompts, load_config
from .conversation import ConversationCore
from .providers import create_provider


class ConversationHandler(BaseHTTPRequestHandler):
    core = None
    config = None
    vad_module = None

    def do_POST(self):
        if self.path == "/v1/conversation":
            self._handle_conversation()
        elif self.path == "/vad/chunk":
            self._handle_vad_chunk()
        else:
            self._drain_body()
            self._send_json(404, {"error": {"code": "not_found", "message": "not found"}})

    def _handle_conversation(self):
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

    def _handle_vad_chunk(self):
        if self.vad_module is None:
            self._send_json(503, {"error": {"code": "vad_unavailable", "message": "VAD is not enabled"}})
            return

        try:
            length = int(self.headers.get("Content-Length", "0"))
            if length == 0:
                self._send_json(400, {"error": {"code": "invalid_request", "message": "empty body"}})
                return
            pcm_bytes = self.rfile.read(length)
        except Exception:
            self._send_json(400, {"error": {"code": "invalid_request", "message": "failed to read body"}})
            return

        try:
            result = self.vad_module.process_chunk(pcm_bytes)
            self._send_json(200, result)
        except Exception as e:
            self._send_json(500, {"error": {"code": "vad_error", "message": str(e)}})

    def _drain_body(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if length > 0:
                self.rfile.read(length)
        except Exception:
            pass

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
    system_prompt, response_format = build_prompts(config)
    provider = create_provider(
        config.llm.provider,
        config.llm,
        system_prompt=system_prompt,
        response_format=response_format,
    )
    core = ConversationCore(
        provider=provider,
        max_history_turns=config.conversation.max_history_turns,
    )

    vad_module = None
    if config.vad.enabled:
        from .vad import SileroVAD, VADConfig as VADRuntimeConfig
        vad_runtime_config = VADRuntimeConfig(
            sample_rate=config.vad.sample_rate,
            speech_threshold=config.vad.speech_threshold,
            silence_duration_ms=config.vad.silence_duration_ms,
            min_speech_duration_ms=config.vad.min_speech_duration_ms,
            model_path=config.vad.model_path,
        )
        vad_module = SileroVAD(vad_runtime_config)

    handler = _make_handler(core, config, vad_module)
    server = HTTPServer((host, port), handler)
    print("Python AI Service listening on {}:{}".format(host, port))
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    server.server_close()


def _make_handler(core, config, vad_module=None):
    class BoundHandler(ConversationHandler):
        pass
    BoundHandler.core = core
    BoundHandler.config = config
    BoundHandler.vad_module = vad_module
    return BoundHandler
