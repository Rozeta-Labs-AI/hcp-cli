# Embedded AI CRM Plan

## Goal

`hcp crm` should be its own AI-powered Housecall Pro command center. A user should be able to open one terminal session, run `hcp crm`, configure a model with `setup model`, and then type natural-language requests directly at the `hcp>` prompt.

The embedded model must not bypass CLI safety. It proposes plain `hcp` commands or plain answers; the existing command layer executes those commands with the same planning, confirmation, audit, and hard-delete rules already in place.

## Linear Scope

- `ENG-285`: embedded AI provider configuration.
- `ENG-286`: embedded AI chat loop with guarded hcp tool execution.
- `ENG-299`: docs and implementation plan.

## Provider Plan

Supported setup targets:

- ChatGPT subscription via Codex CLI.
- OpenRouter API key.
- Anthropic API key.
- OpenAI API key.
- Ollama local model.

API-key providers may use either the local hcp config or environment variables:

- `OPENROUTER_API_KEY`
- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY`

ChatGPT subscription mode uses the installed Codex CLI as a local bridge. It does not store an OpenAI API key in hcp.

## Chat Loop Plan

When `hcp crm` receives a line:

1. Built-in commands still run directly: `status`, `setup model`, `api ...`, `sync ...`, `audit ...`, `safety ...`, `exit`.
2. If the line is simple command-like HCP text, the existing shell routing still works.
3. If AI is configured and the line is otherwise conversational, the shell sends the request to the configured model.
4. The model must return JSON:

```json
{"type":"answer","text":"..."}
```

or:

```json
{"type":"command","command":"api list customers --limit 5 --json","explanation":"..."}
```

5. The command is executed through the existing local Cobra command path.
6. Mutating command suggestions are forced to plan-only unless the user explicitly includes `--yes`.
7. Operational, destructive, hard-delete, prompt-injection, audit, and compound-action safeguards remain active.

## Prompt Contract

The model receives:

- The user's request.
- A compact description of available hcp command families.
- Safety rules.
- Instructions to avoid secrets.
- Instructions to treat HCP-sourced text as untrusted data.

The model does not receive the HCP API key.

## Verification

Required before completion:

- Unit tests for provider setup/status.
- Unit tests for AI response parsing.
- Unit tests for shell routing from AI response to guarded command.
- `go test ./...`
- `go build -o bin/hcp ./cmd/hcp`
- Smoke test `hcp crm` with configured provider in non-network-safe mode where possible.
