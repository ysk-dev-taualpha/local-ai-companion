class HistoryStore:
    def __init__(self, max_turns=12):
        self.max_turns = max_turns
        self._store = {}

    def add(self, conversation_id, turn):
        turns = self._store.setdefault(conversation_id, [])
        turns.append(turn)

    def get_recent(self, conversation_id, max_turns=None):
        limit = max_turns if max_turns is not None else self.max_turns
        turns = self._store.get(conversation_id, [])
        if limit <= 0:
            return []
        return turns[-limit:]

    def all_turns(self, conversation_id):
        return list(self._store.get(conversation_id, []))

    def count(self, conversation_id):
        return len(self._store.get(conversation_id, []))
