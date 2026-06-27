import json
import uuid

from .history import HistoryStore
from .schema import fallback_response, validate_assistant_response


class ConversationCore:
    def __init__(self, provider, max_history_turns=12, history_store=None):
        self.provider = provider
        self.max_history_turns = max_history_turns
        self.history_store = history_store or HistoryStore(max_turns=max_history_turns)

    def send(self, user_text, conversation_id="default", request_id=None):
        request_id = request_id or str(uuid.uuid4())
        past = self.history_store.get_recent(conversation_id, self.max_history_turns)
        raw_response = self.provider.generate(user_text, past)

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
        }
        self.history_store.add(conversation_id, turn)

        return {
            "request_id": request_id,
            "conversation_id": conversation_id,
            "assistant": assistant,
        }
