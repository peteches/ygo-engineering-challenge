package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

var maxIterations = flag.Int("max-iterations", 3, "max generation+evaluation attempts per hotel")

type evalKey struct {
	hotelName string
	lang      string
}

// testState holds everything generated once for the entire test run.
// Descriptions are generated, then all evaluations run with a feedback loop
// so individual test functions just read from the cache — no redundant LLM calls.
var testState struct {
	sync.Once
	client  *openai.Client
	results map[evalKey]EvaluationResult
	err     error
}

func toGenerationFeedback(lang string, d Descriptions, r EvaluationResult) GenerationFeedback {
	var prev string
	switch lang {
	case "de":
		prev = d.DE
	case "fr":
		prev = d.FR
	}
	return GenerationFeedback{
		Language:             lang,
		PreviousDescription:  prev,
		MissedClaims:         r.ClaimAccuracy.MissedClaims,
		HallucinatedClaims:   r.ClaimAccuracy.HallucinatedClaims,
		BackTranslationNotes: r.ClaimAccuracy.BackTranslationNotes,
		LinguisticNotes: map[string]string{
			"register":           r.LanguageNativeness.Register.Notes,
			"idiom":              r.LanguageNativeness.Idiom.Notes,
			"flow":               r.LanguageNativeness.Flow.Notes,
			"cultural_resonance": r.LanguageNativeness.CulturalResonance.Notes,
		},
	}
}

func setup(t *testing.T) map[evalKey]EvaluationResult {
	t.Helper()
	testState.Do(func() {
		client, err := newClient()
		if err != nil {
			testState.err = err
			return
		}
		testState.client = client

		data, err := os.ReadFile("hotels.json")
		if err != nil {
			testState.err = fmt.Errorf("read hotels.json: %w", err)
			return
		}
		var hotels []Hotel
		if err := json.Unmarshal(data, &hotels); err != nil {
			testState.err = fmt.Errorf("parse hotels.json: %w", err)
			return
		}

		ctx := context.Background()
		testState.results = make(map[evalKey]EvaluationResult)
		langs := []string{"de", "fr"}

		for _, h := range hotels {
			d, err := GenerateDescriptions(ctx, client, h, Descriptions{}, nil)
			if err != nil {
				testState.err = fmt.Errorf("generate %s: %w", h.Name, err)
				return
			}

			langResults := make(map[string]EvaluationResult)

			for attempt := 1; attempt <= *maxIterations; attempt++ {
				if attempt > 1 {
					var failingFeedback []GenerationFeedback
					for _, lang := range langs {
						if r, ok := langResults[lang]; ok && !r.Passed {
							failingFeedback = append(failingFeedback, toGenerationFeedback(lang, d, r))
						}
					}
					if len(failingFeedback) == 0 {
						break
					}
					d, err = GenerateDescriptions(ctx, client, h, d, failingFeedback)
					if err != nil {
						testState.err = fmt.Errorf("regenerate %s attempt %d: %w", h.Name, attempt, err)
						return
					}
				}

				allPassed := true
				for _, lang := range langs {
					r, err := Evaluate(ctx, client, d, lang)
					if err != nil {
						testState.err = fmt.Errorf("evaluate %s/%s attempt %d: %w", h.Name, lang, attempt, err)
						return
					}
					log.Printf("attempt %d | %s/%s | claimAccuracy=%.2f languageNativeness=%.2f combined=%.2f passed=%v",
						attempt, h.Name, lang, r.ClaimAccuracy.Combined, r.LanguageNativeness.Combined, r.Combined, r.Passed)
					langResults[lang] = r
					if !r.Passed {
						allPassed = false
					}
				}
				if allPassed {
					break
				}
			}

			for _, lang := range langs {
				testState.results[evalKey{h.Name, lang}] = langResults[lang]
			}
		}
	})

	if testState.err != nil {
		t.Fatalf("test setup failed: %v", testState.err)
	}
	return testState.results
}

// --- Claim Accuracy tests ---

func TestClaimAccuracyCoverage(t *testing.T) {
	results := setup(t)
	for k, r := range results {
		k, r := k, r
		t.Run(k.hotelName+"/"+k.lang, func(t *testing.T) {
			score := r.ClaimAccuracy.ClaimCoverage
			t.Logf("claim coverage = %.2f", score)
			if score < ClaimAccuracyThreshold {
				t.Errorf("claim coverage %.2f < threshold %.2f", score, ClaimAccuracyThreshold)
			}
		})
	}
}

func TestClaimAccuracyHallucinationPrecision(t *testing.T) {
	results := setup(t)
	for k, r := range results {
		k, r := k, r
		t.Run(k.hotelName+"/"+k.lang, func(t *testing.T) {
			score := r.ClaimAccuracy.HallucinationPrecision
			t.Logf("hallucination precision = %.2f", score)
			if score < ClaimAccuracyThreshold {
				t.Errorf("hallucination precision %.2f < threshold %.2f", score, ClaimAccuracyThreshold)
			}
		})
	}
}

func TestClaimAccuracyBackTranslation(t *testing.T) {
	results := setup(t)
	for k, r := range results {
		k, r := k, r
		t.Run(k.hotelName+"/"+k.lang, func(t *testing.T) {
			score := r.ClaimAccuracy.BackTranslation
			t.Logf("back-translation = %.2f", score)
			if score < ClaimAccuracyThreshold {
				t.Errorf("back-translation score %.2f < threshold %.2f", score, ClaimAccuracyThreshold)
			}
		})
	}
}

// --- Language Nativeness tests ---

func TestLanguageNativenessRegister(t *testing.T) {
	results := setup(t)
	for k, r := range results {
		k, r := k, r
		t.Run(k.hotelName+"/"+k.lang, func(t *testing.T) {
			s := r.LanguageNativeness.Register
			t.Logf("register = %.0f/10: %s", s.Score, s.Notes)
			if s.Score/10.0 < LanguageNativenessThreshold {
				t.Errorf("register %.0f/10 below threshold (%.0f): %s", s.Score, LanguageNativenessThreshold*10, s.Notes)
			}
		})
	}
}

func TestLanguageNativenessIdiom(t *testing.T) {
	results := setup(t)
	for k, r := range results {
		k, r := k, r
		t.Run(k.hotelName+"/"+k.lang, func(t *testing.T) {
			s := r.LanguageNativeness.Idiom
			t.Logf("idiom = %.0f/10: %s", s.Score, s.Notes)
			if s.Score/10.0 < LanguageNativenessThreshold {
				t.Errorf("idiom %.0f/10 below threshold (%.0f): %s", s.Score, LanguageNativenessThreshold*10, s.Notes)
			}
		})
	}
}

func TestLanguageNativenessFlow(t *testing.T) {
	results := setup(t)
	for k, r := range results {
		k, r := k, r
		t.Run(k.hotelName+"/"+k.lang, func(t *testing.T) {
			s := r.LanguageNativeness.Flow
			t.Logf("flow = %.0f/10: %s", s.Score, s.Notes)
			if s.Score/10.0 < LanguageNativenessThreshold {
				t.Errorf("flow %.0f/10 below threshold (%.0f): %s", s.Score, LanguageNativenessThreshold*10, s.Notes)
			}
		})
	}
}

func TestLanguageNativenessCulturalResonance(t *testing.T) {
	results := setup(t)
	for k, r := range results {
		k, r := k, r
		t.Run(k.hotelName+"/"+k.lang, func(t *testing.T) {
			s := r.LanguageNativeness.CulturalResonance
			t.Logf("cultural resonance = %.0f/10: %s", s.Score, s.Notes)
			if s.Score/10.0 < LanguageNativenessThreshold {
				t.Errorf("cultural resonance %.0f/10 below threshold (%.0f): %s", s.Score, LanguageNativenessThreshold*10, s.Notes)
			}
		})
	}
}

// --- Combined gate ---

func TestCombinedScore(t *testing.T) {
	results := setup(t)
	for k, r := range results {
		k, r := k, r
		t.Run(k.hotelName+"/"+k.lang, func(t *testing.T) {
			t.Logf("claimAccuracy=%.2f  languageNativeness=%.2f  combined=%.2f  passed=%v",
				r.ClaimAccuracy.Combined, r.LanguageNativeness.Combined, r.Combined, r.Passed)
			if r.ClaimAccuracy.Combined < ClaimAccuracyThreshold {
				t.Errorf("claimAccuracy %.2f < %.2f", r.ClaimAccuracy.Combined, ClaimAccuracyThreshold)
			}
			if r.LanguageNativeness.Combined < LanguageNativenessThreshold {
				t.Errorf("languageNativeness %.2f < %.2f", r.LanguageNativeness.Combined, LanguageNativenessThreshold)
			}
			if r.Combined < CombinedPassThreshold {
				t.Errorf("combined %.2f < %.2f", r.Combined, CombinedPassThreshold)
			}
		})
	}
}
