import json
from dataclasses import dataclass


@dataclass(frozen=True)
class ConversationConfig:
    default_conversation_id: str = "default"
    max_history_turns: int = 12


@dataclass(frozen=True)
class LLMConfig:
    provider: str = "mock"


@dataclass(frozen=True)
class AppConfig:
    conversation: ConversationConfig = ConversationConfig()
    llm: LLMConfig = LLMConfig()


def load_config(path=None):
    if path is None:
        return AppConfig()

    with open(path, "r", encoding="utf-8") as file:
        raw = json.load(file)

    conversation = raw.get("conversation", {})
    llm = raw.get("llm", {})

    return AppConfig(
        conversation=ConversationConfig(
            default_conversation_id=conversation.get("default_conversation_id", "default"),
            max_history_turns=int(conversation.get("max_history_turns", 12)),
        ),
        llm=LLMConfig(provider=llm.get("provider", "mock")),
    )
