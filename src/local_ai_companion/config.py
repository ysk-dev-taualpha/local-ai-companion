import json
from dataclasses import dataclass, field

@dataclass(frozen=True)
class ConversationConfig:
    default_conversation_id: str = "default"
    max_history_turns: int = 12

@dataclass(frozen=True)
class LLMConfig:
    provider: str = "mock"; base_url: str = ""; model: str = ""
    api_key_env: str = ""; timeout_seconds: float = 30.0

@dataclass(frozen=True)
class PromptConfig:
    system_prompt_path: str = ""; response_format_path: str = ""

@dataclass(frozen=True)
class LoggingConfig:
    enabled: bool = False; log_dir: str = ""
    include_user_text: bool = False; include_raw_response: bool = False

@dataclass(frozen=True)
class VADConfig:
    enabled: bool = False; sample_rate: int = 16000
    speech_threshold: float = 0.5; silence_duration_ms: int = 300
    min_speech_duration_ms: int = 500; model_path: str = "models/silero_vad.onnx"

@dataclass(frozen=True)
class AppConfig:
    conversation: ConversationConfig = field(default_factory=ConversationConfig)
    llm: LLMConfig = field(default_factory=LLMConfig)
    prompt: PromptConfig = field(default_factory=PromptConfig)
    logging: LoggingConfig = field(default_factory=LoggingConfig)
    vad: VADConfig = field(default_factory=VADConfig)

def load_config(path=None):
    if path is None: return AppConfig()
    with open(path, "r", encoding="utf-8") as f: raw = json.load(f)
    c = raw.get("conversation", {}); l = raw.get("llm", {})
    p = raw.get("prompt", {}); g = raw.get("logging", {}); v = raw.get("vad", {})
    return AppConfig(
        conversation=ConversationConfig(default_conversation_id=c.get("default_conversation_id","default"),max_history_turns=int(c.get("max_history_turns",12))),
        llm=LLMConfig(provider=l.get("provider","mock"),base_url=l.get("base_url",""),model=l.get("model",""),api_key_env=l.get("api_key_env",""),timeout_seconds=float(l.get("timeout_seconds",30.0))),
        prompt=PromptConfig(system_prompt_path=p.get("system_prompt_path",""),response_format_path=p.get("response_format_path","")),
        logging=LoggingConfig(enabled=g.get("enabled",False),log_dir=g.get("log_dir",""),include_user_text=g.get("include_user_text",False),include_raw_response=g.get("include_raw_response",False)),
        vad=VADConfig(enabled=v.get("enabled",False),sample_rate=int(v.get("sample_rate",16000)),speech_threshold=float(v.get("speech_threshold",0.5)),silence_duration_ms=int(v.get("silence_duration_ms",300)),min_speech_duration_ms=int(v.get("min_speech_duration_ms",500)),model_path=v.get("model_path","models/silero_vad.onnx")),
    )

def load_prompt_text(path):
    if not path: return ""
    with open(path,"r",encoding="utf-8") as f: return f.read().strip()

DEFAULT_SYSTEM_PROMPT = "あなたは常駐型AIアシスタントです。応答は必ずJSON形式で返してください。"
DEFAULT_RESPONSE_FORMAT = "応答はJSONオブジェクトのみを返してください。"

def build_prompts(config):
    s = load_prompt_text(config.prompt.system_prompt_path) or DEFAULT_SYSTEM_PROMPT
    f = load_prompt_text(config.prompt.response_format_path) or DEFAULT_RESPONSE_FORMAT
    return s, f
