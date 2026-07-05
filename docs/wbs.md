# WBS

## Policy

Phase 1 is detailed because it is the next implementation target.

Later phases are intentionally coarse. They should be refined when the preceding milestone is close to completion.

## v0.1: Python Conversation Core

### 1. Project Scaffold

- Create Python package layout: done
- Add CLI entry point: done
- Add config file loading: done
- Add development dependency management: initial `pyproject.toml` added
- Add test runner: initial `unittest` tests added

### 2. Response Schema

- Define response JSON schema
- Define allowed emotion values
- Define allowed motion values
- Define allowed speak_style values
- Validate interruptible as boolean
- Add schema tests

### 3. LLM Provider Interface

- Define provider interface
- Implement mock provider
- Implement OpenAI-compatible provider
- Prepare local provider adapter boundary
- Add provider selection from config

### 4. Prompt Management

- Define system prompt
- Define response format instruction
- Define character tone instruction
- Add max length guidance
- Keep prompts editable outside code

### 5. Conversation Flow

- Accept user text
- Build prompt context
- Call selected LLM provider
- Parse response
- Validate response JSON
- Return normalized response

### 6. Recovery and Fallback

- Detect invalid JSON
- Try JSON extraction
- Try minimal repair
- Fall back to safe response
- Log raw invalid response

### 7. History Management

- Store conversation turns
- Limit history passed to LLM
- Keep conversation_id
- Support new conversation creation

### 8. Logging

- Write JSONL logs
- Log request_id
- Log provider name
- Log latency
- Log validation result
- Avoid logging secrets

### 9. Tests

- Test valid response
- Test invalid JSON fallback
- Test mock provider
- Test history trimming
- Test config loading

### 10. Documentation

- Document how to run CLI
- Document config format
- Document response schema
- Document provider behavior

## v0.2: Go Runtime Minimum

- Create Go service scaffold
- Add config loading
- Add structured logging
- Add request_id generation
- Add HTTP client for Python service
- Add timeout handling
- Add cancellation handling
- Add health check
- Add process boundary documentation

## v0.3: Unity Text Connection

- Create Unity project scaffold
- Add minimal text input UI
- Add Go Runtime client
- Send user text to Go
- Display response text
- Display raw JSON debug view
- Receive emotion / motion / speak_style fields

## v0.4: TTS Output

- Select initial TTS backend
- Add TTS service boundary
- Generate audio from response text
- Add playback queue
- Add stop / interrupt
- Sync subtitle with playback state

## v0.5: Voice Input

- Select initial STT backend
- Add microphone input
- Add VAD
- Add speech start detection
- Add speech end detection
- Display recognized text
- Add cancellation before send
- Prevent TTS feedback loop

## v0.6: Character Control

- Select first character display method
- Map emotion to expression
- Map motion to animation
- Add speaking state
- Add idle state
- Add subtitle display
- Add external control API

## Later: Agent Tool Calling

- Add Go Runtime AgentLoop for Ollama `/api/chat`
- Add ToolRegistry for tool schemas and executors
- Add ToolPolicy for allowlist, loop limit, and audit decisions
- Add ToolExecutor boundary with timeout and cancellation
- Add initial safe tools: `web_search`, `web_fetch`, `audio_control`, `set_state`
- Add Ollama Web Search provider
- Add structured tool call audit logs
- Add tests for tool call parsing, policy denial, executor errors, and loop limit
