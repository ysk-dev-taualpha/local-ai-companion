import json
import os
import tempfile
import unittest

from local_ai_companion.config import (
    DEFAULT_SYSTEM_PROMPT,
    DEFAULT_RESPONSE_FORMAT,
    AppConfig,
    PromptConfig,
    build_prompts,
    load_config,
    load_prompt_text,
)


class PromptLoadingTests(unittest.TestCase):
    def test_load_prompt_text_reads_file(self):
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".md", delete=False, encoding="utf-8"
        ) as f:
            f.write("hello prompt\n")
            path = f.name
        try:
            text = load_prompt_text(path)
            self.assertEqual(text, "hello prompt")
        finally:
            os.unlink(path)

    def test_load_prompt_text_empty_path_returns_empty(self):
        self.assertEqual(load_prompt_text(""), "")

    def test_build_prompts_uses_defaults_when_no_paths(self):
        config = AppConfig()
        system, fmt = build_prompts(config)
        self.assertEqual(system, DEFAULT_SYSTEM_PROMPT)
        self.assertEqual(fmt, DEFAULT_RESPONSE_FORMAT)

    def test_build_prompts_reads_files(self):
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".md", delete=False, encoding="utf-8"
        ) as f_sys, tempfile.NamedTemporaryFile(
            mode="w", suffix=".md", delete=False, encoding="utf-8"
        ) as f_fmt:
            f_sys.write("custom system\n")
            f_fmt.write("custom format\n")
            sys_path = f_sys.name
            fmt_path = f_fmt.name

        try:
            config = AppConfig(
                prompt=PromptConfig(
                    system_prompt_path=sys_path,
                    response_format_path=fmt_path,
                )
            )
            system, fmt = build_prompts(config)
            self.assertEqual(system, "custom system")
            self.assertEqual(fmt, "custom format")
        finally:
            os.unlink(sys_path)
            os.unlink(fmt_path)

    def test_config_loads_prompt_paths_from_json(self):
        data = {
            "prompt": {
                "system_prompt_path": "/tmp/sys.md",
                "response_format_path": "/tmp/fmt.md",
            }
        }
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".json", delete=False, encoding="utf-8"
        ) as f:
            json.dump(data, f)
            path = f.name

        try:
            config = load_config(path)
            self.assertEqual(config.prompt.system_prompt_path, "/tmp/sys.md")
            self.assertEqual(config.prompt.response_format_path, "/tmp/fmt.md")
        finally:
            os.unlink(path)

    def test_prompt_config_defaults_are_empty_strings(self):
        pc = PromptConfig()
        self.assertEqual(pc.system_prompt_path, "")
        self.assertEqual(pc.response_format_path, "")


if __name__ == "__main__":
    unittest.main()
