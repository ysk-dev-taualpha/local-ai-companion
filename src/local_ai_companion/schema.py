ALLOWED_EMOTIONS = {
    "neutral",
    "happy",
    "sad",
    "thinking",
    "surprised",
    "angry",
    "sleepy",
    "confident",
}

ALLOWED_MOTIONS = {
    "idle",
    "nod",
    "shake_head",
    "wave",
    "look_away",
    "think",
    "point",
}

ALLOWED_SPEAK_STYLES = {
    "normal",
    "soft",
    "fast",
    "slow",
    "serious",
    "playful",
}

MAX_TEXT_LENGTH = 500


class ResponseValidationError(ValueError):
    pass


def validate_assistant_response(value):
    if not isinstance(value, dict):
        raise ResponseValidationError("assistant response must be an object")

    text = value.get("text")
    emotion = value.get("emotion")
    motion = value.get("motion")
    speak_style = value.get("speak_style")
    interruptible = value.get("interruptible")

    if not isinstance(text, str) or not text.strip():
        raise ResponseValidationError("text must be a non-empty string")
    if len(text.strip()) > MAX_TEXT_LENGTH:
        raise ResponseValidationError(
            "text exceeds max length of {} characters".format(MAX_TEXT_LENGTH)
        )
    if emotion not in ALLOWED_EMOTIONS:
        raise ResponseValidationError("emotion is not allowed: {}".format(emotion))
    if motion not in ALLOWED_MOTIONS:
        raise ResponseValidationError("motion is not allowed: {}".format(motion))
    if speak_style not in ALLOWED_SPEAK_STYLES:
        raise ResponseValidationError("speak_style is not allowed: {}".format(speak_style))
    if not isinstance(interruptible, bool):
        raise ResponseValidationError("interruptible must be a boolean")

    return {
        "text": text.strip(),
        "emotion": emotion,
        "motion": motion,
        "speak_style": speak_style,
        "interruptible": interruptible,
    }


def fallback_response():
    return {
        "text": "すみません、応答を整えるところで失敗しました。もう一度お願いします。",
        "emotion": "neutral",
        "motion": "idle",
        "speak_style": "soft",
        "interruptible": True,
    }
