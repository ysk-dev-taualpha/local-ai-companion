import unittest

from local_ai_companion.schema import MAX_TEXT_LENGTH, ResponseValidationError, validate_assistant_response


class SchemaTests(unittest.TestCase):
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


if __name__ == "__main__":
    unittest.main()
