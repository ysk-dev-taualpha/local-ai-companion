import uuid

from .recovery import try_extract_json
from .providers import ProviderResult
from .schema import fallback_response, validate_assistant_response


class ConversationCore:
    def __init__(self, provider, max_history_turns=12, log_writer=None):
        self.provider = provider
        self.max_history_turns = max_history_turns
        self.history = []
        self.log_writer = log_writer

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
            parsed = try_extract_json(raw_response)
            if parsed is None:
                raise ValueError("no valid JSON object found in response")
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
            "provider": getattr(self.provider, "name", "unknown"),
            "provider": provider_name,
            "latency_ms": latency_ms,
        }
        self.history.append(turn)

        if self.log_writer is not None:
            self.log_writer.write(turn)

        return {
            "request_id": request_id,
            "conversation_id": conversation_id,
            "assistant": assistant,
        }
