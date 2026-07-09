package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
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
	client       *openai.Client
	hotels       []Hotel
	results      map[evalKey]EvaluationResult
	descriptions map[string]Descriptions // keyed by hotel name — reused by TestPromptVariants
	err          error
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
		testState.hotels = hotels

		ctx := context.Background()
		testState.results = make(map[evalKey]EvaluationResult)
		testState.descriptions = make(map[string]Descriptions)
		langs := []string{"de", "fr"}
		gp := DefaultGeneratorPrompts()
		ep := DefaultEvaluatorPrompts()

		for _, h := range hotels {
			d, err := GenerateDescriptions(ctx, client, h, Descriptions{}, nil, gp)
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
					d, err = GenerateDescriptions(ctx, client, h, d, failingFeedback, gp)
					if err != nil {
						testState.err = fmt.Errorf("regenerate %s attempt %d: %w", h.Name, attempt, err)
						return
					}
				}

				allPassed := true
				for _, lang := range langs {
					r, err := Evaluate(ctx, client, d, lang, ep)
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

			testState.descriptions[h.Name] = d
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

// --- Prompt variant comparison ---

// promptSlot describes one prompt parameter slot and how to apply a variant path to it.
type promptSlot struct {
	name      string
	dir       string
	isGen     bool // true = generator prompt; false = evaluator prompt
	applyGen  func(*GeneratorPrompts, string)
	applyEval func(*EvaluatorPrompts, string)
}

var promptSlots = []promptSlot{
	{name: "gen_english", dir: "prompts/gen_english", isGen: true,
		applyGen: func(gp *GeneratorPrompts, p string) { gp.English = p }},
	{name: "gen_de", dir: "prompts/gen_de", isGen: true,
		applyGen: func(gp *GeneratorPrompts, p string) { gp.German = p }},
	{name: "gen_fr", dir: "prompts/gen_fr", isGen: true,
		applyGen: func(gp *GeneratorPrompts, p string) { gp.French = p }},
	{name: "refine_de", dir: "prompts/refine_de", isGen: true,
		applyGen: func(gp *GeneratorPrompts, p string) { gp.RefineDE = p }},
	{name: "refine_fr", dir: "prompts/refine_fr", isGen: true,
		applyGen: func(gp *GeneratorPrompts, p string) { gp.RefineFR = p }},
	{name: "eval_claim_extract", dir: "prompts/eval_claim_extract", isGen: false,
		applyEval: func(ep *EvaluatorPrompts, p string) { ep.ClaimExtract = p }},
	{name: "eval_claim_verify", dir: "prompts/eval_claim_verify", isGen: false,
		applyEval: func(ep *EvaluatorPrompts, p string) { ep.ClaimVerify = p }},
	{name: "eval_hallucination_extract", dir: "prompts/eval_hallucination_extract", isGen: false,
		applyEval: func(ep *EvaluatorPrompts, p string) { ep.HallucinationExtract = p }},
	{name: "eval_hallucination_verify", dir: "prompts/eval_hallucination_verify", isGen: false,
		applyEval: func(ep *EvaluatorPrompts, p string) { ep.HallucinationVerify = p }},
	{name: "eval_backtrans_translate", dir: "prompts/eval_backtrans_translate", isGen: false,
		applyEval: func(ep *EvaluatorPrompts, p string) { ep.BackTranslate = p }},
	{name: "eval_backtrans_score", dir: "prompts/eval_backtrans_score", isGen: false,
		applyEval: func(ep *EvaluatorPrompts, p string) { ep.BackScore = p }},
	{name: "eval_language_nativeness", dir: "prompts/eval_language_nativeness", isGen: false,
		applyEval: func(ep *EvaluatorPrompts, p string) { ep.LanguageNativeness = p }},
}

// TestPromptVariants discovers every .md file in each prompt slot directory and runs
// a sub-test per variant. Generator variants regenerate from scratch; evaluator variants
// reuse descriptions cached by setup(). Only slots with more than one variant file are
// interesting, but default.md is always included for baseline comparison.
func TestPromptVariants(t *testing.T) {
	setup(t) // ensure testState is populated
	ctx := context.Background()
	langs := []string{"de", "fr"}

	for _, slot := range promptSlots {
		slot := slot
		variants := listPromptVariants(slot.dir)
		if len(variants) <= 1 {
			continue // only default.md — nothing to compare
		}

		t.Run(slot.name, func(t *testing.T) {
			for _, variantPath := range variants {
				variantPath := variantPath
				variantName := strings.TrimSuffix(path.Base(variantPath), ".md")

				t.Run(variantName, func(t *testing.T) {
					for _, h := range testState.hotels {
						h := h
						var d Descriptions
						var err error

						if slot.isGen {
							gp := DefaultGeneratorPrompts()
							slot.applyGen(&gp, variantPath)
							d, err = GenerateDescriptions(ctx, testState.client, h, Descriptions{}, nil, gp)
							if err != nil {
								t.Fatalf("generate %s: %v", h.Name, err)
							}
						} else {
							d = testState.descriptions[h.Name]
						}

						for _, lang := range langs {
							lang := lang
							ep := DefaultEvaluatorPrompts()
							if !slot.isGen {
								slot.applyEval(&ep, variantPath)
							}

							r, err := Evaluate(ctx, testState.client, d, lang, ep)
							if err != nil {
								t.Fatalf("evaluate %s/%s: %v", h.Name, lang, err)
							}

							t.Run(h.Name+"/"+lang, func(t *testing.T) {
								t.Logf("claimAccuracy=%.2f languageNativeness=%.2f combined=%.2f passed=%v",
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
				})
			}
		})
	}
}
