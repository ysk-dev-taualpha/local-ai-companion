import argparse
import json
import sys

from .config import load_config
from .conversation import ConversationCore
from .log_writer import JSONLLogWriter
from .providers import create_provider


def build_parser():
    parser = argparse.ArgumentParser(description="Local AI Companion conversation CLI")
    parser.add_argument("--config", help="Path to JSON config file")
    parser.add_argument("--message", help="Single user message")
    parser.add_argument("--conversation-id", help="Conversation session ID")
    parser.add_argument("--request-id", help="Request ID for tracing")
    parser.add_argument("--log-dir", help="Enable JSONL logging to directory")
    parser.add_argument("--serve", action="store_true", help="Start as HTTP server")
    return parser


def _make_log_writer(config, log_dir_arg):
    include_user = config.logging.include_user_text
    include_raw = config.logging.include_raw_response
    if log_dir_arg:
        return JSONLLogWriter(log_dir_arg, include_user_text=include_user, include_raw_response=include_raw)
    if config.logging.enabled and config.logging.log_dir:
        return JSONLLogWriter(config.logging.log_dir, include_user_text=include_user, include_raw_response=include_raw)
    return None


def run_once(config, conversation_id, request_id, message, log_writer):
    provider = create_provider(config.llm.provider, config.llm)
    core = ConversationCore(
        provider=provider,
        max_history_turns=config.conversation.max_history_turns,
        log_writer=log_writer,
    )
    response = core.send(
        message,
        conversation_id=conversation_id,
        request_id=request_id,
    )
    print(json.dumps(response, ensure_ascii=False, indent=2))
    if log_writer:
        log_writer.close()
    return 0


def run_repl(config, conversation_id, log_writer):
    provider = create_provider(config.llm.provider, config.llm)
    core = ConversationCore(
        provider=provider,
        max_history_turns=config.conversation.max_history_turns,
        log_writer=log_writer,
    )

    print("Local AI Companion CLI. Type /exit to quit.")
    try:
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
                conversation_id=conversation_id,
            )
            print(json.dumps(response, ensure_ascii=False, indent=2))
    finally:
        if log_writer:
            log_writer.close()


def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)
    config = load_config(args.config)

    conversation_id = args.conversation_id or config.conversation.default_conversation_id
    log_writer = _make_log_writer(config, args.log_dir)

    if args.serve:
        from .server import run_server
        run_server(config)
        return 0
    if args.message:
        return run_once(config, conversation_id, args.request_id, args.message, log_writer)
    return run_repl(config, conversation_id, log_writer)


if __name__ == "__main__":
    sys.exit(main())
