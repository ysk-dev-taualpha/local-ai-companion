import json


class LLMProvider:
    name = "base"

    def generate(self, user_text, history):
        raise NotImplementedError


class MockLLMProvider(LLMProvider):
    name = "mock"

    def generate(self, user_text, history):
        response = {
            "text": "受け取りました: {}".format(user_text),
            "emotion": "neutral",
            "motion": "nod",
            "speak_style": "normal",
            "interruptible": True,
        }
        return json.dumps(response, ensure_ascii=False)


def create_provider(name):
    if name == "mock":
        return MockLLMProvider()
    raise ValueError("unsupported llm provider: {}".format(name))
