# Evaluation

## Metrics chosen and why

The system uses a two-tier weighted scoring scheme: **ClaimAccuracy** (70%) and
**LanguageNativeness** (30%). The weighting deliberately prioritises factual fidelity
because an elegant hallucination is worse than a clunky truth — a hotel description that
invents an amenity or misstates a distance causes real harm.

**ClaimAccuracy** combines three sub-metrics:

- *ClaimCoverage* (40% of tier) — extract every factual claim from the English base
  description, then verify each one against the foreign text. Measures completeness.
- *HallucinationPrecision* (35% of tier) — extract claims from the foreign text and
  check each against the original hotel JSON. Measures how much the model invented.
- *BackTranslation* (25% of tier) — back-translate the foreign text to English and score
  semantic equivalence against the English base. Measures meaning drift.

**LanguageNativeness** is a single LLM call that scores four dimensions 1–10: Register
(formality), Idiom (naturalness vs. calques), Flow (sentence structure), and
CulturalResonance (appeal to the target audience). Combined threshold is 0.70.

## Evaluating factual consistency without speaking the target languages

All three ClaimAccuracy metrics reduce the problem to English-to-English comparison. For
ClaimCoverage the LLM reads the foreign text but we only reason about the English claims
it should cover. For HallucinationPrecision the LLM translates the foreign claims to
English before checking them against the source JSON. BackTranslation explicitly
round-trips the foreign text back to English so the scoring prompt compares two English
paragraphs. No human knowledge of German or French is required at any point.

## What was wrong initially

A single-attempt baseline with hardcoded prompts produced the following combined scores:

| Hotel | Lang | ClaimAccuracy | Combined |
|---|---|---|---|
| Strandhaus Aurora | de | ~0.90 | 0.88 |
| Strandhaus Aurora | fr | ~0.93 | 0.90 |
| Pinecrest Lodge | de | 0.77 | 0.75 |
| Pinecrest Lodge | fr | 0.89 | 0.85 |

HallucinationPrecision was the main culprit (0.75–0.85 across hotels) — the model
invented facts not present in the source JSON. BackTranslation also fell short (0.75–0.85)
indicating meaning drift between the English base and the foreign output.

## What changed

A **language-aware feedback loop** (up to 3 iterations, configurable via `--max-iterations`)
was added. After each failing attempt the evaluator captures missed claims, hallucinated
claims, back-translation drift notes, and per-dimension linguistic notes. These are fed
into a refinement prompt alongside the previous description so the model has concrete,
stateful context for corrections. Only languages that failed are regenerated; passing
languages are copied unchanged to avoid score regression.

All prompts were also **externalised** from Go code into `prompts/<slot>/default.md` and
loaded at runtime via `embed.FS` with `text/template` substitution. This separates wording
from logic and enables the prompt variant testing infrastructure described below.

## Score progression with the feedback loop

With the feedback loop enabled (3 attempts), Strandhaus Aurora/fr improved from 0.86
combined on attempt 1 to 0.91 on attempt 2 and held there. Pinecrest Lodge remained the
harder case — ClaimAccuracy stalled at 0.79–0.85 across all three attempts, which reflects
the capability ceiling of the local KoboldCPP inference server used for development rather
than a flaw in the evaluation design.

## What would be done with more time

The most valuable next step is **systematic prompt comparison**. The variant testing
infrastructure is already built: dropping `prompts/gen_de/strict.md` alongside
`default.md` causes `go test -run TestPromptVariants` to benchmark both variants
side-by-side across all hotels and languages without touching production code. The `strict`
variants add explicit constraints ("do NOT add any information not in the list"); the
`cultural` variants write the generation prompt in the target language itself to improve
idiomatic output. Running these against a frontier model would quickly identify which
wording reliably pushes HallucinationPrecision above 0.90.

One process note: roughly 20 minutes were spent early in the session investigating
automated export of the development conversation log before realising that `/export
prompt-log.md` produces the same result in seconds. That time would have been better spent
on prompt iteration.
