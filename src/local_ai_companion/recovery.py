import json
import re

_MD_FENCE_PATTERN = re.compile(r"```(?:json)?\s*(.*?)```", re.DOTALL)


def try_extract_json(raw_text):
    if not isinstance(raw_text, str):
        return None

    text = raw_text.strip()

    # 1) plain parse
    parsed = _safe_parse(text)
    if parsed is not None:
        return parsed

    # 2) code fence
    fence_match = _MD_FENCE_PATTERN.search(text)
    if fence_match:
        parsed = _safe_parse(fence_match.group(1).strip())
        if parsed is not None:
            return parsed

    # 3) brace block scan
    start = text.find("{")
    while start != -1:
        end = _find_matching_brace(text, start)
        if end != -1:
            candidate = text[start : end + 1]
            parsed = _safe_parse(candidate)
            if parsed is not None:
                return parsed
            start = text.find("{", start + 1)
        else:
            break

    return None


def _safe_parse(s):
    try:
        obj = json.loads(s)
        if isinstance(obj, dict):
            return obj
    except (json.JSONDecodeError, ValueError):
        pass
    return None


def _find_matching_brace(text, start):
    depth = 0
    for i in range(start, len(text)):
        ch = text[i]
        if ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0:
                return i
    return -1
