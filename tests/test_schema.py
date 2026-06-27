import unittest

from local_ai_companion.schema import MAX_TEXT_LENGTH, ResponseValidationError, validate_assistant_response
from local_ai_companion.schema import (
    ALLOWED_EMOTIONS,
    ALLOWED_MOTIONS,
    ALLOWED_SPEAK_STYLES,
    ResponseValidationError,
    fallback_response,
    validate_assistant_response,
)


class SchemaValidationTests(unittest.TestCase):
    """valid response の各フィールドが正しく検証・正規化されることを確認する。"""

    def test_valid_response_is_normalized(self):
        response = validate_assistant_response(
            {
                "text": " hello ",
                "emotion": "neutral",
                "motion": "idle",
                "speak_style": "normal",
                "interruptible": True,
            }
        )
        self.assertEqual(response["text"], "hello")
        self.assertEqual(response["emotion"], "neutral")
        self.assertEqual(response["motion"], "idle")
        self.assertEqual(response["speak_style"], "normal")
        self.assertTrue(response["interruptible"])

    def test_all_emotions_are_accepted(self):
        for emotion in ALLOWED_EMOTIONS:
            with self.subTest(emotion=emotion):
                response = validate_assistant_response(
                    {
                        "text": "test",
                        "emotion": emotion,
                        "motion": "idle",
                        "speak_style": "normal",
                        "interruptible": True,
                    }
                )
                self.assertEqual(response["emotion"], emotion)

    def test_all_motions_are_accepted(self):
        for motion in ALLOWED_MOTIONS:
            with self.subTest(motion=motion):
                response = validate_assistant_response(
                    {
                        "text": "test",
                        "emotion": "neutral",
                        "motion": motion,
                        "speak_style": "normal",
                        "interruptible": True,
                    }
                )
                self.assertEqual(response["motion"], motion)

    def test_all_speak_styles_are_accepted(self):
        for style in ALLOWED_SPEAK_STYLES:
            with self.subTest(style=style):
                response = validate_assistant_response(
                    {
                        "text": "test",
                        "emotion": "neutral",
                        "motion": "idle",
                        "speak_style": style,
                        "interruptible": True,
                    }
                )
                self.assertEqual(response["speak_style"], style)

    def test_interruptible_false_is_accepted(self):
        response = validate_assistant_response(
            {
                "text": "test",
                "emotion": "neutral",
                "motion": "idle",
                "speak_style": "normal",
                "interruptible": False,
            }
        )
        self.assertFalse(response["interruptible"])


class SchemaRejectionTests(unittest.TestCase):
    """不正なレスポンスが ResponseValidationError で拒否されることを確認する。"""

    def test_invalid_emotion_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "emotion": "invalid",
                    "motion": "idle",
                    "speak_style": "normal",
                    "interruptible": True,
                }
            )

    def test_text_exceeds_max_length_fails(self):
        long_text = "a" * (MAX_TEXT_LENGTH + 1)
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": long_text,
                    "emotion": "neutral",
                    "motion": "idle",
                    "speak_style": "normal",
    def test_invalid_motion_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "emotion": "neutral",
                    "motion": "dance",
                    "speak_style": "normal",
                    "interruptible": True,
                }
            )

    def test_invalid_speak_style_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "emotion": "neutral",
                    "motion": "idle",
                    "speak_style": "shout",
                    "interruptible": True,
                }
            )

    def test_empty_text_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "   ",
                    "emotion": "neutral",
                    "motion": "idle",
                    "speak_style": "normal",
                    "interruptible": True,
                }
            )

    def test_missing_text_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "emotion": "neutral",
                    "motion": "idle",
                    "speak_style": "normal",
                    "interruptible": True,
                }
            )

    def test_non_dict_input_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response("not a dict")

    def test_none_input_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(None)

    def test_interruptible_not_bool_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "emotion": "neutral",
                    "motion": "idle",
                    "speak_style": "normal",
                    "interruptible": "yes",
                }
            )

    def test_interruptible_as_int_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "emotion": "neutral",
                    "motion": "idle",
                    "speak_style": "normal",
                    "interruptible": 1,
                }
            )

    def test_empty_emotion_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "emotion": "",
                    "motion": "idle",
                    "speak_style": "normal",
                    "interruptible": True,
                }
            )

    def test_text_at_max_length_is_accepted(self):
        max_text = "あ" * MAX_TEXT_LENGTH
        response = validate_assistant_response(
            {
                "text": max_text,
                "emotion": "neutral",
                "motion": "idle",
                "speak_style": "normal",
                "interruptible": True,
            }
        )
        self.assertEqual(len(response["text"]), MAX_TEXT_LENGTH)
    def test_missing_emotion_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "motion": "idle",
                    "speak_style": "normal",
                    "interruptible": True,
                }
            )

    def test_missing_motion_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "emotion": "neutral",
                    "speak_style": "normal",
                    "interruptible": True,
                }
            )

    def test_missing_speak_style_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "emotion": "neutral",
                    "motion": "idle",
                    "interruptible": True,
                }
            )

    def test_missing_interruptible_fails(self):
        with self.assertRaises(ResponseValidationError):
            validate_assistant_response(
                {
                    "text": "hello",
                    "emotion": "neutral",
                    "motion": "idle",
                    "speak_style": "normal",
                }
            )


class FallbackResponseTests(unittest.TestCase):
    """fallback_response が所定のスキーマを満たすことを確認する。"""

    def test_fallback_is_valid(self):
        fb = fallback_response()
        validated = validate_assistant_response(fb)
        self.assertEqual(validated["emotion"], "neutral")
        self.assertEqual(validated["motion"], "idle")
        self.assertIsInstance(validated["text"], str)
        self.assertGreater(len(validated["text"]), 0)
        self.assertTrue(validated["interruptible"])

    def test_fallback_uses_soft_speak_style(self):
        fb = fallback_response()
        self.assertEqual(fb["speak_style"], "soft")


if __name__ == "__main__":
    unittest.main()
