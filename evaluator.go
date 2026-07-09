package main

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// --- Tier 1 ---

// scoreClaimCoverage implements metric 1a.
// It extracts claims from the English description then verifies each one
// against the foreign text.
func scoreClaimCoverage(ctx context.Context, client *openai.Client, englishDesc, foreignDesc, language string) (float64, []string, error) {
	model := defaultModel()

	// Step 1: extract claims from the English description
	extractPrompt := fmt.Sprintf(`Read this English hotel description and extract every specific factual claim it makes.
Return ONLY a JSON array of short English strings, one claim per item. No duplicates. No vague generalities — only concrete facts.

Description:
%s`, englishDesc)

	var claims []string
	if err := chatJSON(ctx, client, model, extractPrompt, &claims); err != nil {
		return 0, nil, fmt.Errorf("claim extraction: %w", err)
	}
	if len(claims) == 0 {
		return 0, nil, fmt.Errorf("no claims extracted from English description")
	}

	// Step 2: verify each claim against the foreign description
	claimsJSON, _ := json.Marshal(claims)
	verifyPrompt := fmt.Sprintf(`You are checking whether a %s hotel description covers a set of facts.

For each claim in the list below, decide:
  "yes"     — the %s description clearly states this fact
  "partial" — the %s description mentions it but with less detail or a weaker form
  "no"      — the %s description does not mention this fact

Return ONLY a JSON array of objects in this exact shape, one per claim, in the same order:
[{"claim": "...", "result": "yes|partial|no"}, ...]

Claims to check:
%s

%s description to check:
%s`, language, language, language, language, string(claimsJSON), language, foreignDesc)

	type claimCheck struct {
		Claim  string `json:"claim"`
		Result string `json:"result"`
	}
	var checks []claimCheck
	if err := chatJSON(ctx, client, model, verifyPrompt, &checks); err != nil {
		return 0, nil, fmt.Errorf("claim verification: %w", err)
	}

	var score float64
	var missed []string
	for _, c := range checks {
		switch c.Result {
		case "yes":
			score += 1.0
		case "partial":
			score += 0.5
		default:
			missed = append(missed, c.Claim)
		}
	}
	return score / float64(len(claims)), missed, nil
}

// scoreHallucinationPrecision implements metric 1b.
// It extracts claims from the foreign description and checks each against
// the original source hotel JSON.
func scoreHallucinationPrecision(ctx context.Context, client *openai.Client, foreignDesc, language string, hotel Hotel) (float64, []string, error) {
	model := defaultModel()
	hotelJSON, _ := json.MarshalIndent(hotel, "", "  ")

	// Step 1: extract claims from the foreign description (translated to English)
	extractPrompt := fmt.Sprintf(`Read this %s hotel description. Extract every specific factual claim it makes.
Translate each claim to English. Return ONLY a JSON array of short English strings, one claim per item.

%s description:
%s`, language, language, foreignDesc)

	var claims []string
	if err := chatJSON(ctx, client, model, extractPrompt, &claims); err != nil {
		return 0, nil, fmt.Errorf("foreign claim extraction: %w", err)
	}
	if len(claims) == 0 {
		return 1.0, nil, nil // no claims = no hallucinations
	}

	// Step 2: verify each claim against the source hotel JSON
	claimsJSON, _ := json.Marshal(claims)
	verifyPrompt := fmt.Sprintf(`You are fact-checking hotel marketing claims against the original hotel data.

For each claim below, decide if it is supported by the hotel data:
  "yes"     — clearly supported by the hotel data
  "partial" — roughly supported but slightly exaggerated or imprecise
  "no"      — not supported or contradicts the hotel data

Return ONLY a JSON array in this shape, one per claim, in order:
[{"claim": "...", "result": "yes|partial|no"}, ...]

Hotel source data:
%s

Claims to fact-check:
%s`, string(hotelJSON), string(claimsJSON))

	type claimCheck struct {
		Claim  string `json:"claim"`
		Result string `json:"result"`
	}
	var checks []claimCheck
	if err := chatJSON(ctx, client, model, verifyPrompt, &checks); err != nil {
		return 0, nil, fmt.Errorf("hallucination check: %w", err)
	}

	var score float64
	var hallucinated []string
	for _, c := range checks {
		switch c.Result {
		case "yes":
			score += 1.0
		case "partial":
			score += 0.5
		default:
			hallucinated = append(hallucinated, c.Claim)
		}
	}
	return score / float64(len(claims)), hallucinated, nil
}

// scoreBackTranslation implements metric 1c.
// It back-translates the foreign description to English then scores semantic
// equivalence against the original English base.
func scoreBackTranslation(ctx context.Context, client *openai.Client, englishDesc, foreignDesc, language string) (float64, string, error) {
	model := defaultModel()

	// Step 1: back-translate
	backPrompt := fmt.Sprintf(`Translate the following %s text to English. Produce a faithful translation — do not paraphrase or improve it.
Return ONLY the translated text.

%s text:
%s`, language, language, foreignDesc)

	backTranslated, err := chat(ctx, client, model, backPrompt)
	if err != nil {
		return 0, "", fmt.Errorf("back-translation: %w", err)
	}

	// Step 2: score semantic equivalence
	scorePrompt := fmt.Sprintf(`Compare these two English hotel descriptions for semantic equivalence.
Score them from 0 to 100:
  100 — identical meaning, all facts match, same level of detail
  80  — nearly identical, minor wording differences but all facts present
  60  — same general content but some facts are softened, vague, or missing nuance
  40  — significant meaning drift; some facts missing or changed
  0   — very different

Return ONLY valid JSON:
{"score": <0-100>, "notes": "<brief explanation of differences>"}

Original English:
%s

Back-translated English:
%s`, englishDesc, backTranslated)

	var result struct {
		Score float64 `json:"score"`
		Notes string  `json:"notes"`
	}
	if err := chatJSON(ctx, client, model, scorePrompt, &result); err != nil {
		return 0, "", fmt.Errorf("equivalence scoring: %w", err)
	}
	return result.Score / 100.0, result.Notes, nil
}

// EvaluateTier1 runs all three Tier 1 sub-metrics for one language.
func EvaluateTier1(ctx context.Context, client *openai.Client, d Descriptions, language string) (Tier1Result, error) {
	var foreignDesc string
	switch language {
	case "de":
		foreignDesc = d.DE
	case "fr":
		foreignDesc = d.FR
	default:
		return Tier1Result{}, fmt.Errorf("unsupported language: %s", language)
	}

	coverage, _, err := scoreClaimCoverage(ctx, client, d.EN, foreignDesc, language)
	if err != nil {
		return Tier1Result{}, fmt.Errorf("1a: %w", err)
	}

	precision, _, err := scoreHallucinationPrecision(ctx, client, foreignDesc, language, d.Hotel)
	if err != nil {
		return Tier1Result{}, fmt.Errorf("1b: %w", err)
	}

	backTranslation, _, err := scoreBackTranslation(ctx, client, d.EN, foreignDesc, language)
	if err != nil {
		return Tier1Result{}, fmt.Errorf("1c: %w", err)
	}

	r := Tier1Result{
		ClaimCoverage:          coverage,
		HallucinationPrecision: precision,
		BackTranslation:        backTranslation,
	}
	r.Combined = tier1Combined(r)
	return r, nil
}

// --- Tier 2 ---

// EvaluateTier2 runs all four Tier 2 sub-metrics in a single LLM call.
func EvaluateTier2(ctx context.Context, client *openai.Client, description, language string) (Tier2Result, error) {
	model := defaultModel()

	langNames := map[string]string{"de": "German", "fr": "French"}
	langName := langNames[language]

	prompt := fmt.Sprintf(`You are an expert in %s hotel marketing copywriting. Evaluate the following %s description on four dimensions. Score each 1–10.

Dimensions:
1. register: Is the formality level appropriate for luxury hotel marketing in %s? (%s uses formal "%s" address; sophisticated vocabulary; no contractions)
2. idiom: Does it use natural, idiomatic %s expressions, or does it contain calques (stilted literal translations from English)?
3. flow: Does sentence structure follow natural %s patterns (word order, compound nouns, article use, rhythm)?
4. cultural_resonance: Does the description appeal to what %s-speaking travellers value and find relevant?

Return ONLY valid JSON in this exact shape:
{
  "register":           {"score": <1-10>, "notes": "<specific observations>"},
  "idiom":              {"score": <1-10>, "notes": "<specific observations>"},
  "flow":               {"score": <1-10>, "notes": "<specific observations>"},
  "cultural_resonance": {"score": <1-10>, "notes": "<specific observations>"}
}

%s description to evaluate:
%s`,
		langName, langName, langName,
		langName, map[string]string{"de": "Sie", "fr": "vous"}[language],
		langName, langName, langName,
		langName, description)

	var scores struct {
		Register          SubScore `json:"register"`
		Idiom             SubScore `json:"idiom"`
		Flow              SubScore `json:"flow"`
		CulturalResonance SubScore `json:"cultural_resonance"`
	}
	if err := chatJSON(ctx, client, model, prompt, &scores); err != nil {
		return Tier2Result{}, fmt.Errorf("tier2 scoring: %w", err)
	}

	r := Tier2Result{
		Register:          scores.Register,
		Idiom:             scores.Idiom,
		Flow:              scores.Flow,
		CulturalResonance: scores.CulturalResonance,
	}
	r.Combined = tier2Combined(r)
	return r, nil
}

// Evaluate runs both tiers for a single hotel+language pair.
func Evaluate(ctx context.Context, client *openai.Client, d Descriptions, language string) (EvaluationResult, error) {
	var foreignDesc string
	switch language {
	case "de":
		foreignDesc = d.DE
	case "fr":
		foreignDesc = d.FR
	default:
		return EvaluationResult{}, fmt.Errorf("unsupported language: %s", language)
	}

	t1, err := EvaluateTier1(ctx, client, d, language)
	if err != nil {
		return EvaluationResult{}, fmt.Errorf("tier1: %w", err)
	}

	t2, err := EvaluateTier2(ctx, client, foreignDesc, language)
	if err != nil {
		return EvaluationResult{}, fmt.Errorf("tier2: %w", err)
	}

	combined := combinedScore(t1.Combined, t2.Combined)
	passed := t1.Combined >= Tier1PassThreshold &&
		t2.Combined >= Tier2PassThreshold &&
		combined >= CombinedPassThreshold

	return EvaluationResult{
		HotelName: d.Hotel.Name,
		Language:  language,
		Tier1:     t1,
		Tier2:     t2,
		Combined:  combined,
		Passed:    passed,
	}, nil
}
