package main

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// --- Claim Accuracy ---

// scoreClaimCoverage implements metric 1a.
func scoreClaimCoverage(ctx context.Context, client *openai.Client, englishDesc, foreignDesc, language, extractPath, verifyPath string) (float64, []string, error) {
	model := defaultModel()

	extractPrompt, err := renderPrompt(extractPath, map[string]any{
		"Description": englishDesc,
	})
	if err != nil {
		return 0, nil, err
	}

	var claims []string
	if err := chatJSON(ctx, client, model, extractPrompt, &claims); err != nil {
		return 0, nil, fmt.Errorf("claim extraction: %w", err)
	}
	if len(claims) == 0 {
		return 0, nil, fmt.Errorf("no claims extracted from English description")
	}

	claimsJSON, _ := json.Marshal(claims)
	verifyPrompt, err := renderPrompt(verifyPath, map[string]any{
		"Language":   language,
		"ClaimsJSON": string(claimsJSON),
		"ForeignDesc": foreignDesc,
	})
	if err != nil {
		return 0, nil, err
	}

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
func scoreHallucinationPrecision(ctx context.Context, client *openai.Client, foreignDesc, language string, hotel Hotel, extractPath, verifyPath string) (float64, []string, error) {
	model := defaultModel()
	hotelJSON, _ := json.MarshalIndent(hotel, "", "  ")

	extractPrompt, err := renderPrompt(extractPath, map[string]any{
		"Language":   language,
		"ForeignDesc": foreignDesc,
	})
	if err != nil {
		return 0, nil, err
	}

	var claims []string
	if err := chatJSON(ctx, client, model, extractPrompt, &claims); err != nil {
		return 0, nil, fmt.Errorf("foreign claim extraction: %w", err)
	}
	if len(claims) == 0 {
		return 1.0, nil, nil
	}

	claimsJSON, _ := json.Marshal(claims)
	verifyPrompt, err := renderPrompt(verifyPath, map[string]any{
		"HotelJSON":  string(hotelJSON),
		"ClaimsJSON": string(claimsJSON),
	})
	if err != nil {
		return 0, nil, err
	}

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
func scoreBackTranslation(ctx context.Context, client *openai.Client, englishDesc, foreignDesc, language, translatePath, scorePath string) (float64, string, error) {
	model := defaultModel()

	backPrompt, err := renderPrompt(translatePath, map[string]any{
		"Language":   language,
		"ForeignDesc": foreignDesc,
	})
	if err != nil {
		return 0, "", err
	}

	backTranslated, err := chat(ctx, client, model, backPrompt)
	if err != nil {
		return 0, "", fmt.Errorf("back-translation: %w", err)
	}

	scorePrompt, err := renderPrompt(scorePath, map[string]any{
		"EnglishDesc":    englishDesc,
		"BackTranslated": backTranslated,
	})
	if err != nil {
		return 0, "", err
	}

	var result struct {
		Score float64 `json:"score"`
		Notes string  `json:"notes"`
	}
	if err := chatJSON(ctx, client, model, scorePrompt, &result); err != nil {
		return 0, "", fmt.Errorf("equivalence scoring: %w", err)
	}
	return result.Score / 100.0, result.Notes, nil
}

// EvaluateClaimAccuracy runs all three factual accuracy sub-metrics for one language.
func EvaluateClaimAccuracy(ctx context.Context, client *openai.Client, d Descriptions, language string, ep EvaluatorPrompts) (ClaimAccuracyResult, error) {
	var foreignDesc string
	switch language {
	case "de":
		foreignDesc = d.DE
	case "fr":
		foreignDesc = d.FR
	default:
		return ClaimAccuracyResult{}, fmt.Errorf("unsupported language: %s", language)
	}

	coverage, missed, err := scoreClaimCoverage(ctx, client, d.EN, foreignDesc, language, ep.ClaimExtract, ep.ClaimVerify)
	if err != nil {
		return ClaimAccuracyResult{}, fmt.Errorf("1a: %w", err)
	}

	precision, hallucinated, err := scoreHallucinationPrecision(ctx, client, foreignDesc, language, d.Hotel, ep.HallucinationExtract, ep.HallucinationVerify)
	if err != nil {
		return ClaimAccuracyResult{}, fmt.Errorf("1b: %w", err)
	}

	backTranslation, btNotes, err := scoreBackTranslation(ctx, client, d.EN, foreignDesc, language, ep.BackTranslate, ep.BackScore)
	if err != nil {
		return ClaimAccuracyResult{}, fmt.Errorf("1c: %w", err)
	}

	r := ClaimAccuracyResult{
		ClaimCoverage:          coverage,
		HallucinationPrecision: precision,
		BackTranslation:        backTranslation,
		MissedClaims:           missed,
		HallucinatedClaims:     hallucinated,
		BackTranslationNotes:   btNotes,
	}
	r.Combined = claimAccuracyCombined(r)
	return r, nil
}

// --- Language Nativeness ---

// EvaluateLanguageNativeness runs all four linguistic quality sub-metrics in a single LLM call.
func EvaluateLanguageNativeness(ctx context.Context, client *openai.Client, description, language string, ep EvaluatorPrompts) (LanguageNativenessResult, error) {
	model := defaultModel()

	langNames := map[string]string{"de": "German", "fr": "French"}
	formalAddr := map[string]string{"de": "Sie", "fr": "vous"}

	prompt, err := renderPrompt(ep.LanguageNativeness, map[string]any{
		"LangName":      langNames[language],
		"FormalAddress": formalAddr[language],
		"Description":   description,
	})
	if err != nil {
		return LanguageNativenessResult{}, err
	}

	var scores struct {
		Register          SubScore `json:"register"`
		Idiom             SubScore `json:"idiom"`
		Flow              SubScore `json:"flow"`
		CulturalResonance SubScore `json:"cultural_resonance"`
	}
	if err := chatJSON(ctx, client, model, prompt, &scores); err != nil {
		return LanguageNativenessResult{}, fmt.Errorf("language nativeness scoring: %w", err)
	}

	r := LanguageNativenessResult{
		Register:          scores.Register,
		Idiom:             scores.Idiom,
		Flow:              scores.Flow,
		CulturalResonance: scores.CulturalResonance,
	}
	r.Combined = languageNativenessCombined(r)
	return r, nil
}

// Evaluate runs both evaluations for a single hotel+language pair.
func Evaluate(ctx context.Context, client *openai.Client, d Descriptions, language string, ep EvaluatorPrompts) (EvaluationResult, error) {
	var foreignDesc string
	switch language {
	case "de":
		foreignDesc = d.DE
	case "fr":
		foreignDesc = d.FR
	default:
		return EvaluationResult{}, fmt.Errorf("unsupported language: %s", language)
	}

	ca, err := EvaluateClaimAccuracy(ctx, client, d, language, ep)
	if err != nil {
		return EvaluationResult{}, fmt.Errorf("claim accuracy: %w", err)
	}

	ln, err := EvaluateLanguageNativeness(ctx, client, foreignDesc, language, ep)
	if err != nil {
		return EvaluationResult{}, fmt.Errorf("language nativeness: %w", err)
	}

	combined := combinedScore(ca.Combined, ln.Combined)
	passed := ca.Combined >= ClaimAccuracyThreshold &&
		ln.Combined >= LanguageNativenessThreshold &&
		combined >= CombinedPassThreshold

	return EvaluationResult{
		HotelName:          d.Hotel.Name,
		Language:           language,
		ClaimAccuracy:      ca,
		LanguageNativeness: ln,
		Combined:           combined,
		Passed:             passed,
	}, nil
}
