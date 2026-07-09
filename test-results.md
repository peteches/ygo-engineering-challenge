# Prompt Variant Test Results

Run: `go test . -v -run TestPromptVariants -max-iterations=1 -timeout 60m`
Model: qwen2.5-coder-14b-instruct-q6_k via local KoboldCPP

> **Note:** The test run was killed partway through `gen_de/strict` due to the KoboldCPP
> server producing intermittent i/o timeouts on longer generation tasks (Pinecrest Lodge
> in particular). `gen_english/strict` also lost its Pinecrest Lodge result to a timeout.
> Results for `gen_de/strict`, `gen_fr/*` slots were not collected.

Thresholds: ClaimAccuracy ≥ 0.90 · LanguageNativeness ≥ 0.70 · Combined ≥ 0.85

---

## Default setup scores (from `setup()` cache, attempt 1)

| Hotel | Lang | ClaimAccuracy | LangNativeness | Combined | Pass |
|---|---|---|---|---|---|
| Strandhaus Aurora | de | 0.87 | 0.84 | 0.86 | ✗ |
| Strandhaus Aurora | fr | 0.82 | 0.73 | 0.80 | ✗ |
| Pinecrest Lodge | de | 0.79 | 0.75 | 0.78 | ✗ |
| Pinecrest Lodge | fr | 0.88 | 0.86 | 0.88 | ✗ |

---

## gen_english variants

| Variant | Hotel | Lang | ClaimAcc | LangNat | Combined | Pass |
|---|---|---|---|---|---|---|
| **default** | Strandhaus Aurora | de | 0.93 | 0.80 | 0.89 | ✓ |
| **default** | Strandhaus Aurora | fr | 0.91 | 0.81 | 0.88 | ✓ |
| **default** | Pinecrest Lodge | de | 0.75 | 0.84 | 0.78 | ✗ |
| **default** | Pinecrest Lodge | fr | 0.82 | 0.82 | 0.82 | ✗ |
| factled | Strandhaus Aurora | de | 0.80 | 0.75 | 0.79 | ✗ |
| factled | Strandhaus Aurora | fr | 0.79 | 0.78 | 0.79 | ✗ |
| factled | Pinecrest Lodge | de | 0.80 | 0.82 | 0.81 | ✗ |
| factled | Pinecrest Lodge | fr | 0.85 | 0.82 | 0.84 | ✗ |
| strict | Strandhaus Aurora | de | 0.88 | 0.85 | 0.87 | ✗ |
| strict | Strandhaus Aurora | fr | 0.79 | 0.78 | 0.78 | ✗ |
| strict | Pinecrest Lodge | — | — | — | — | timeout |

**Winner: `default`** — only variant to pass any cases; best ClaimAccuracy on Strandhaus Aurora (0.93/0.91). `factled` underperformed across the board. `strict` showed a small improvement on LangNativeness for Strandhaus Aurora/de (0.85) but couldn't clear the ClaimAccuracy bar.

---

## gen_de variants

| Variant | Hotel | Lang | ClaimAcc | LangNat | Combined | Pass |
|---|---|---|---|---|---|---|
| cultural | Strandhaus Aurora | de | 0.87 | 0.73 | 0.83 | ✗ |
| cultural | Strandhaus Aurora | fr | 0.80 | 0.82 | 0.80 | ✗ |
| cultural | Pinecrest Lodge | de | 0.79 | 0.79 | 0.79 | ✗ |
| cultural | Pinecrest Lodge | fr | 0.88 | 0.78 | 0.85 | ✗ |
| **default** | Strandhaus Aurora | de | 0.91 | 0.74 | **0.86** | ✓ |
| **default** | Strandhaus Aurora | fr | 0.90 | 0.84 | **0.88** | ✓ |
| **default** | Pinecrest Lodge | de | 0.73 | 0.84 | 0.77 | ✗ |
| **default** | Pinecrest Lodge | fr | 0.78 | 0.84 | 0.80 | ✗ |
| strict | — | — | — | — | — | killed |

**Winner: `default`** — passed both Strandhaus Aurora cases with ClaimAccuracy 0.91/0.90. The `cultural` variant (prompt written in German) produced lower ClaimAccuracy, likely because the target-language prompt introduces more creative licence. `strict` data not collected.

---

## gen_fr variants

Not collected — run killed before gen_fr slot was reached.

---

## Overall observations

- **`default` prompts outperform alternatives on ClaimAccuracy** — the existing constraints ("NOTHING that is not in this list") already work reasonably well; the `strict` and `cultural` rewrites did not reliably improve on them with this model.
- **Pinecrest Lodge is the consistent hard case** across all variants and all runs. The hotel's alpine ski context may be less represented in the model's training data, leading to more hallucination.
- **Network stability** of the KoboldCPP server is the main practical obstacle to exhaustive variant testing; retrying on timeout would make `TestPromptVariants` more robust for long-running comparisons.
- **A frontier model** (GPT-4o, Claude 3.5 Sonnet) would be expected to clear the 0.90 ClaimAccuracy bar consistently without requiring multiple feedback iterations, making variant differences more meaningful.
