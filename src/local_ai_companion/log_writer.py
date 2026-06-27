import json
import os


class JSONLLogWriter:
    def __init__(self, log_dir, include_user_text=False, include_raw_response=False):
        os.makedirs(log_dir, exist_ok=True)
        self._log_dir = log_dir
        self._include_user_text = include_user_text
        self._include_raw_response = include_raw_response
        self._file = None

    def write(self, entry):
        if self._file is None:
            path = os.path.join(self._log_dir, "conversation.jsonl")
            self._file = open(path, "a", encoding="utf-8")

        json.dump(self._filter_entry(entry), self._file, ensure_ascii=False, sort_keys=True)
        self._file.write("\n")
        self._file.flush()

    def _filter_entry(self, entry):
        filtered = dict(entry)
        if not self._include_user_text:
            filtered.pop("user_text", None)
        if not self._include_raw_response:
            filtered.pop("raw_response", None)
        return filtered

    def close(self):
        if self._file is not None:
            self._file.close()
            self._file = None
