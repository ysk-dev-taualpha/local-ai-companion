import uuid

from .recovery import try_extract_json
from .schema import fallback_response, validate_assistant_response


class ConversationCore:
    def __init__(self, provider, max_history_turns=12):
        self.provider = provider
        self.max_history_turns = max_history_turns
        self.history = []

    def send(self, user_text, conversation_id="default", request_id=None):
        request_id = request_id or str(uuid.uuid4())
        raw_response = self.provider.generate(user_text, self.history[-self.max_history_turns :])

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
        }
        self.history.append(turn)

        return {
            "request_id": request_id,
            "conversation_id": conversation_id,
            "assistant": assistant,
        }
