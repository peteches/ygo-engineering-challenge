# YGO Localisation Engineering Challenge

A Go program that generates native-sounding hotel marketing descriptions in English, German,
and French, then automatically evaluates factual consistency and linguistic nativeness using
an LLM-as-judge pipeline. Failing descriptions are refined iteratively with targeted feedback.

## Dependencies

- **Go 1.21+**
- **[github.com/sashabaranov/go-openai](https://github.com/sashabaranov/go-openai)** — OpenAI-compatible API client (fetched automatically by `go mod download`)

## Configuration

The program is configured via environment variables:

| Variable | Required | Description |
|---|---|---|
| `LLM_API_KEY` | Yes | API key (any non-empty string for KoboldCPP) |
| `LLM_BASE_URL` | No | Base URL of the API endpoint (defaults to OpenAI) |
| `LLM_MODEL` | No | Model name to request (defaults to `gpt-4o-mini`) |

## Setup

### Option 1 — OpenAI

Create an API key at [platform.openai.com](https://platform.openai.com) and export it:

```bash
export LLM_API_KEY="sk-..."
```

No `LLM_BASE_URL` needed. Set `LLM_MODEL` if you want a model other than `gpt-4o-mini`:

```bash
export LLM_MODEL="gpt-4o"
```

### Option 2 — Local KoboldCPP instance

> **Note:** All development and testing for this project was done against a local
> [KoboldCPP](https://github.com/LostRuins/koboldcpp) instance running
> `qwen2.5-coder-14b-instruct-q6_k` — not an actual OpenAI endpoint. The code uses the
> OpenAI-compatible `/v1/chat/completions` interface which KoboldCPP exposes, so it should
> be compatible with any real OpenAI endpoint, but this has not been tested directly.

Install KoboldCPP following the [official documentation](https://github.com/LostRuins/koboldcpp?tab=readme-ov-file#usage).

The instance used for this project was started with:

```bash
koboldcpp \
  --host :: \
  --port 5001 \
  --model /home/peteches/.local/share/models/qwen2.5-coder-14b-instruct-q6_k.gguf \
  --usecuda \
  --websearch \
  --draftamount 16 \
  --gpulayers 999 \
  --contextsize 16384 \
  --flashattention \
  --ssl /home/peteches/.local/share/certs/nug.peteches.co.uk.pem \
        /home/peteches/.local/share/certs/nug.peteches.co.uk.pem \
  --ttsmodel /home/peteches/.local/share/models/Kokoro_no_espeak_Q4.gguf \
  --draftmodel /home/peteches/.local/share/models/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf \
  --whispermodel /home/peteches/.local/share/models/whisper-small-q5_1.bin
```

Then export:

```bash
export LLM_API_KEY="dummy"
export LLM_BASE_URL="https://your-koboldcpp-host:5001/v1"
```

## Running the main program

Generates and prints EN/DE/FR descriptions for each hotel in `hotels.json`:

```bash
go run .
```

## Running the tests

Tests generate descriptions and evaluate them using the full LLM pipeline. Each test run
makes many LLM calls so allow several minutes.

```bash
# Run all tests (default: up to 3 feedback iterations per hotel)
go test . -v -timeout 30m

# Limit feedback iterations (faster, fewer LLM calls)
go test . -v -timeout 15m -max-iterations=1

# Run only the combined score gate
go test . -v -run TestCombinedScore -timeout 15m

# Compare prompt variants (only meaningful once extra .md files exist in prompts/<slot>/)
go test . -v -run TestPromptVariants -timeout 60m
```

### Test flags

| Flag | Default | Description |
|---|---|---|
| `-max-iterations` | `3` | Maximum feedback-loop attempts per hotel before accepting the best result |

### Test structure

| Test | What it checks |
|---|---|
| `TestClaimAccuracyCoverage` | Every English fact appears in the foreign text (≥ 0.90) |
| `TestClaimAccuracyHallucinationPrecision` | Foreign text contains no invented facts (≥ 0.90) |
| `TestClaimAccuracyBackTranslation` | Back-translating the foreign text recovers the English meaning (≥ 0.90) |
| `TestLanguageNativenessRegister` | Formality appropriate for luxury hotel marketing (≥ 7/10) |
| `TestLanguageNativenessIdiom` | Natural target-language idioms, no calques (≥ 7/10) |
| `TestLanguageNativenessFlow` | Sentence structure follows target-language patterns (≥ 7/10) |
| `TestLanguageNativenessCulturalResonance` | Appeals to what target-language travellers value (≥ 7/10) |
| `TestCombinedScore` | Combined gate: ClaimAccuracy ≥ 0.90, LanguageNativeness ≥ 0.70, combined ≥ 0.85 |
| `TestPromptVariants` | Benchmarks every `.md` file in each `prompts/<slot>/` directory |

## Prompt variants

Prompts live in `prompts/<slot>/default.md` and are embedded into the binary at compile
time. To test an alternative wording, add a new `.md` file to the relevant slot directory
(e.g. `prompts/gen_de/strict.md`) and run `TestPromptVariants`. No code changes are needed.
