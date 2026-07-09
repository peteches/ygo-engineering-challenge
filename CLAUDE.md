# YGO Localization Engineering Challenge

## Purpose

This repo is a job-application engineering challenge for YGO.ai. The goal is a Go program that:

1. Reads two hotel JSON objects as source data
2. Generates native-sounding marketing descriptions in **English, German, and French**
3. Automatically evaluates **factual consistency** across all three languages (weighted 70%) and **nativeness** (weighted 30%)
4. Runs a reliability loop (up to 3 retries with feedback) and prints score progression

All LLM calls use the **Anthropic API** (`ANTHROPIC_API_KEY` env var). All source code is in **Go**.

### Deliverables

- `main.go`, `generator.go`, `evaluator.go` — Go source
- `hotels.json` — source hotel data
- `prompt-log.md` — full dev conversation log (exported via `/export`)
- `README.md` — setup and run instructions
- `EVALUATION.md` — metrics rationale, iteration story, score improvement

---

## File and Code Editing

**Prefer Anvil MCP tools** for all file work in this repo:

- **Read/explore**: use `mcp__anvil-emacs-eval__file-outline`, `mcp__anvil-emacs-eval__file-read` — avoid reading whole files when a targeted read suffices
- **Edit**: use `mcp__anvil-emacs-eval__file-replace-string` for literal replacements, `mcp__anvil-emacs-eval__file-replace-regexp` for pattern changes
- **Batch edits**: use `mcp__anvil-emacs-eval__file-batch` for 3+ edits in one file, `mcp__anvil-emacs-eval__file-batch-across` for changes across multiple files
- **Create files**: use `mcp__anvil-emacs-eval__file-create`
- **Append**: use `mcp__anvil-emacs-eval__file-append`

Only fall back to the generic `Read`, `Edit`, `Write` tools when no Anvil equivalent exists.

---

## Constraints

- Zero manually written code — Claude Code writes everything
- Source code in Go only
- `prompt-log.md` must be exported (via `/export prompt-log.md`) before submitting
- 3-hour hard deadline from declared start time
