# Development Automation

This project is easiest to develop through the PowerShell scripts in `scripts/`.

## Common Commands

```powershell
.\scripts\test-python.ps1
.\scripts\check.ps1
.\scripts\run-cli.ps1 -Message "hello"
.\scripts\run-cli.ps1 -Config config.ollama.example.json -Message "hello"
.\scripts\run-server.ps1 -Config config.ollama.example.json
.\scripts\smoke-ollama.ps1
```

The scripts set `PYTHONPATH` to `src`, so commands work from a fresh shell without installing the package.

## Ollama

`config.ollama.example.json` uses Ollama's OpenAI-compatible API:

```text
http://127.0.0.1:11434/v1
```

The default model is `g4v100`, a local Ollama model configured for V100 use with `num_ctx 8192`. To use another Ollama model, edit:

```json
"model": "g4v100"
```

For example:

```json
"model": "gemma4:12b"
```

## Development Loop

1. Run `.\scripts\check.ps1`.
2. Make the smallest behavior change.
3. Add or update a focused test.
4. Run `.\scripts\test-python.ps1`.
5. Run a CLI smoke test with `.\scripts\run-cli.ps1`.
