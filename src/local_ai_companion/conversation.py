import json
import uuid

from .schema import fallback_response, validate_assistant_response


class ConversationCore:
    def __init__(self, provider, max_history_turns=12, log_writer=None):
        self.provider = provider
        self.max_history_turns = max_history_turns
        self.history = []
        self.log_writer = log_writer

    def send(self, user_text, conversation_id="default", request_id=None):
        request_id = request_id or str(uuid.uuid4())
        raw_response = self.provider.generate(user_text, self.history[-self.max_history_turns :])

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
            "provider": getattr(self.provider, "name", "unknown"),
        }
        self.history.append(turn)

        if self.log_writer is not None:
            self.log_writer.write(turn)

        return {
            "request_id": request_id,
            "conversation_id": conversation_id,
            "assistant": assistant,
        }
