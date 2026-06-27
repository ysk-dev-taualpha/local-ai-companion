import json
import uuid

from .providers import ProviderResult
from .schema import fallback_response, validate_assistant_response


class ConversationCore:
    def __init__(self, provider, max_history_turns=12):
        self.provider = provider
        self.max_history_turns = max_history_turns
        self.history = []

    def send(self, user_text, conversation_id="default", request_id=None):
        request_id = request_id or str(uuid.uuid4())
        result = self.provider.generate(
            user_text, self.history[-self.max_history_turns :]
        )

        if isinstance(result, ProviderResult):
            raw_response = result.raw_response
            provider_name = result.provider_name
            latency_ms = result.latency_ms
        else:
            raw_response = result
            provider_name = getattr(self.provider, "name", "unknown")
            latency_ms = None

        try:
            parsed = json.loads(raw_response)
            assistant = validate_assistant_response(parsed)
            valid = True
            error = None
        except Exception as exc:
            assistant = fallback_response()
            valid = False
            error = str(exc)

        turn = {
            "request_id": request_id,
            "conversation_id": conversation_id,
            "user_text": user_text,
            "assistant": assistant,
            "valid": valid,
            "error": error,
            "provider": provider_name,
            "latency_ms": latency_ms,
        }
        self.history.append(turn)

        return {
            "request_id": request_id,
            "conversation_id": conversation_id,
            "assistant": assistant,
        }
