import unittest

from local_ai_companion.recovery import try_extract_json


class JsonExtractionTests(unittest.TestCase):
    def test_plain_json(self):
        obj = try_extract_json('{"text":"hello","emotion":"neutral","motion":"idle","speak_style":"normal","interruptible":true}')
        self.assertIsNotNone(obj)
        self.assertEqual(obj["text"], "hello")

    def test_code_fence_json(self):
        obj = try_extract_json('```json\n{"text":"hi","emotion":"happy","motion":"wave","speak_style":"fast","interruptible":false}\n```')
        self.assertIsNotNone(obj)
        self.assertEqual(obj["text"], "hi")
        self.assertEqual(obj["emotion"], "happy")

    def test_code_fence_without_lang(self):
        obj = try_extract_json('```\n{"text":"test","emotion":"sad","motion":"idle","speak_style":"slow","interruptible":true}\n```')
        self.assertIsNotNone(obj)
        self.assertEqual(obj["text"], "test")

    def test_text_before_json(self):
        obj = try_extract_json('Here is the response: {"text":"ok","emotion":"thinking","motion":"nod","speak_style":"normal","interruptible":true}')
        self.assertIsNotNone(obj)
        self.assertEqual(obj["text"], "ok")

    def test_text_after_json(self):
        obj = try_extract_json('{"text":"done","emotion":"neutral","motion":"idle","speak_style":"normal","interruptible":true} Hope this helps!')
        self.assertIsNotNone(obj)
        self.assertEqual(obj["text"], "done")

    def test_text_around_json(self):
        obj = try_extract_json('Sure! Here you go:\n{"text":"here","emotion":"confident","motion":"point","speak_style":"normal","interruptible":true}\nLet me know.')
        self.assertIsNotNone(obj)
        self.assertEqual(obj["text"], "here")

    def test_invalid_text_returns_none(self):
        obj = try_extract_json("No JSON here at all.")
        self.assertIsNone(obj)

    def test_non_dict_json_returns_none(self):
        obj = try_extract_json("[1, 2, 3]")
        self.assertIsNone(obj)

    def test_empty_string_returns_none(self):
        obj = try_extract_json("")
        self.assertIsNone(obj)

    def test_none_input_returns_none(self):
        obj = try_extract_json(None)
        self.assertIsNone(obj)

    def test_nested_json_objects(self):
        obj = try_extract_json('{"text":"nested","emotion":"neutral","motion":"idle","speak_style":"normal","interruptible":true,"extra":{"inner":"value"}}')
        self.assertIsNotNone(obj)
        self.assertEqual(obj["extra"]["inner"], "value")


if __name__ == "__main__":
    unittest.main()
