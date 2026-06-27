import json
import os


class JSONLLogWriter:
    def __init__(self, log_dir):
        os.makedirs(log_dir, exist_ok=True)
        self._log_dir = log_dir
        self._file = None

    def write(self, entry):
        if self._file is None:
            path = os.path.join(self._log_dir, "conversation.jsonl")
            self._file = open(path, "a", encoding="utf-8")

        json.dump(entry, self._file, ensure_ascii=False, sort_keys=True)
        self._file.write("\n")
        self._file.flush()

    def close(self):
        if self._file is not None:
            self._file.close()
            self._file = None
