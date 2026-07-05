import json
from http.server import HTTPServer, BaseHTTPRequestHandler
from .config import AppConfig, build_prompts
from .conversation import ConversationCore
from .providers import create_provider
from .vad import create_vad, VADConfig as VADModelConfig

class ConversationHandler(BaseHTTPRequestHandler):
    core = None; config = None; vad = None

    def do_POST(self):
        if self.path == "/v1/conversation": self._handle_conversation()
        elif self.path == "/vad/chunk": self._handle_vad_chunk()
        else: self._drain_body(); self._send_json(404, {"error":{"code":"not_found","message":"not found"}})

    def _handle_conversation(self):
        try:
            length = int(self.headers.get("Content-Length","0"))
            d = json.loads(self.rfile.read(length))
        except Exception:
            self._send_json(400, {"error":{"code":"invalid_request","message":"invalid JSON body"}}); return
        msg = d.get("message","")
        if not isinstance(msg, str) or not msg.strip():
            self._send_json(400, {"error":{"code":"invalid_request","message":"message is required"}}); return
        cid = d.get("conversation_id", self.config.conversation.default_conversation_id)
        resp = self.core.send(msg, conversation_id=cid, request_id=d.get("request_id"))
        self._send_json(200, resp)

    def _handle_vad_chunk(self):
        try: length = int(self.headers.get("Content-Length","0")); data = self.rfile.read(length)
        except Exception: self._send_json(400, {"error":{"code":"invalid_request","message":"invalid body"}}); return
        if not data or len(data) % 2 != 0:
            self._send_json(400, {"error":{"code":"invalid_request","message":"PCM must be int16"}}); return
        if self.vad is None:
            self._send_json(503, {"error":{"code":"vad_unavailable","message":"VAD not enabled"}}); return
        self._send_json(200, self.vad.process_chunk(data))

    def _drain_body(self):
        try:
            n = int(self.headers.get("Content-Length","0"))
            if n > 0: self.rfile.read(n)
        except: pass

    def _send_json(self, status, data):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type","application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers(); self.wfile.write(body)

    def log_message(self, *args): pass

def run_server(config, host="127.0.0.1", port=8090):
    sp, rf = build_prompts(config)
    prov = create_provider(config.llm.provider, config.llm, system_prompt=sp, response_format=rf)
    core = ConversationCore(provider=prov, max_history_turns=config.conversation.max_history_turns)
    vad = None
    if config.vad.enabled:
        try:
            from .vad import SileroVAD
            vm = VADModelConfig(sample_rate=config.vad.sample_rate, speech_threshold=config.vad.speech_threshold, silence_duration_ms=config.vad.silence_duration_ms, min_speech_duration_ms=config.vad.min_speech_duration_ms, model_path=config.vad.model_path)
            vad = SileroVAD(vm)
        except Exception as e: print(f"VAD unavailable: {e}")
    handler = _make_handler(core, config, vad)
    srv = HTTPServer((host, port), handler)
    print(f"Python AI Service on {host}:{port}")
    try: srv.serve_forever()
    except KeyboardInterrupt: pass
    srv.server_close()

def _make_handler(core, config, vad=None):
    class H(ConversationHandler): pass
    H.core = core; H.config = config; H.vad = vad
    return H
