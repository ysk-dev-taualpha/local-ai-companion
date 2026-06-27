import argparse
import json
import sys

from .config import load_config
from .conversation import ConversationCore
from .providers import create_provider


def build_parser():
    parser = argparse.ArgumentParser(description="Local AI Companion conversation CLI")
    parser.add_argument("--config", help="Path to JSON config file")
    parser.add_argument("--message", help="Single user message")
    return parser


def run_once(config, message):
    provider = create_provider(config.llm.provider)
    core = ConversationCore(
        provider=provider,
        max_history_turns=config.conversation.max_history_turns,
    )
    response = core.send(
        message,
        conversation_id=config.conversation.default_conversation_id,
    )
    print(json.dumps(response, ensure_ascii=False, indent=2))
    return 0


def run_repl(config):
    provider = create_provider(config.llm.provider)
    core = ConversationCore(
        provider=provider,
        max_history_turns=config.conversation.max_history_turns,
    )

    print("Local AI Companion CLI. Type /exit to quit.")
    while True:
        try:
            user_text = input("> ").strip()
        except EOFError:
            return 0

        if user_text in {"/exit", "/quit"}:
            return 0
        if not user_text:
            continue

        response = core.send(
            user_text,
            conversation_id=config.conversation.default_conversation_id,
        )
        print(json.dumps(response, ensure_ascii=False, indent=2))


def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)
    config = load_config(args.config)

    if args.message:
        return run_once(config, args.message)
    return run_repl(config)


if __name__ == "__main__":
    sys.exit(main())
