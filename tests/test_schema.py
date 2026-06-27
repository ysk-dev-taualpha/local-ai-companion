import unittest

from local_ai_companion.schema import ResponseValidationError, validate_assistant_response


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


if __name__ == "__main__":
    unittest.main()
