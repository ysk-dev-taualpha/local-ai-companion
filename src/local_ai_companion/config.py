import json
from dataclasses import dataclass, field


@dataclass(frozen=True)
class ConversationConfig:
    default_conversation_id: str = "default"
    max_history_turns: int = 12


@dataclass(frozen=True)
class LLMConfig:
    provider: str = "mock"


@dataclass(frozen=True)
class PromptConfig:
    system_prompt_path: str = ""
    response_format_path: str = ""


@dataclass(frozen=True)
class LoggingConfig:
    enabled: bool = False
    log_dir: str = ""


@dataclass(frozen=True)
class AppConfig:
    conversation: ConversationConfig = field(default_factory=ConversationConfig)
    llm: LLMConfig = field(default_factory=LLMConfig)
    prompt: PromptConfig = field(default_factory=PromptConfig)
    logging: LoggingConfig = field(default_factory=LoggingConfig)


def load_config(path=None):
    if path is None:
        return AppConfig()

    with open(path, "r", encoding="utf-8") as file:
        raw = json.load(file)

    conversation = raw.get("conversation", {})
    llm = raw.get("llm", {})
    prompt_raw = raw.get("prompt", {})
    logging_raw = raw.get("logging", {})

    return AppConfig(
        conversation=ConversationConfig(
            default_conversation_id=conversation.get("default_conversation_id", "default"),
            max_history_turns=int(conversation.get("max_history_turns", 12)),
        ),
        llm=LLMConfig(provider=llm.get("provider", "mock")),
        prompt=PromptConfig(
            system_prompt_path=prompt_raw.get("system_prompt_path", ""),
            response_format_path=prompt_raw.get("response_format_path", ""),
        ),
        logging=LoggingConfig(
            enabled=logging_raw.get("enabled", False),
            log_dir=logging_raw.get("log_dir", ""),
        ),
    )


def load_prompt_text(path):
    if not path:
        return ""
    with open(path, "r", encoding="utf-8") as f:
        return f.read().strip()


DEFAULT_SYSTEM_PROMPT = "あなたは常駐型AIアシスタントです。応答は必ずJSON形式で返してください。"
DEFAULT_RESPONSE_FORMAT = "応答はJSONオブジェクトのみを返してください。"


def build_prompts(config):
    system = load_prompt_text(config.prompt.system_prompt_path) or DEFAULT_SYSTEM_PROMPT
    fmt = load_prompt_text(config.prompt.response_format_path) or DEFAULT_RESPONSE_FORMAT
    return system, fmt
