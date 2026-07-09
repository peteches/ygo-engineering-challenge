package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// testState holds shared state generated once for the entire test run.
var testState struct {
	sync.Once
	client       *openai.Client
	hotels       []Hotel
	descriptions []Descriptions
	err          error
}

func setup(t *testing.T) (*openai.Client, []Descriptions) {
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
		if err := json.Unmarshal(data, &testState.hotels); err != nil {
			testState.err = fmt.Errorf("parse hotels.json: %w", err)
			return
		}

		ctx := context.Background()
		for _, h := range testState.hotels {
			d, err := GenerateDescriptions(ctx, client, h)
			if err != nil {
				testState.err = fmt.Errorf("generate %s: %w", h.Name, err)
				return
			}
			testState.descriptions = append(testState.descriptions, d)
		}
	})

	if testState.err != nil {
		t.Fatalf("test setup failed: %v", testState.err)
	}
	return testState.client, testState.descriptions
}

// --- Tier 1 tests ---

func TestTier1ClaimCoverage(t *testing.T) {
	client, descs := setup(t)
	ctx := context.Background()

	for _, d := range descs {
		for _, lang := range []string{"de", "fr"} {
			d, lang := d, lang
			t.Run(d.Hotel.Name+"/"+lang, func(t *testing.T) {
				var foreignDesc string
				if lang == "de" {
					foreignDesc = d.DE
				} else {
					foreignDesc = d.FR
				}
				score, missed, err := scoreClaimCoverage(ctx, client, d.EN, foreignDesc, lang)
				if err != nil {
					t.Fatalf("scoreClaimCoverage: %v", err)
				}
				t.Logf("claim coverage = %.2f; missed: %v", score, missed)
				if score < Tier1PassThreshold {
					t.Errorf("claim coverage %.2f < threshold %.2f; missed facts: %v",
						score, Tier1PassThreshold, missed)
				}
			})
		}
	}
}

func TestTier1HallucinationPrecision(t *testing.T) {
	client, descs := setup(t)
	ctx := context.Background()

	for _, d := range descs {
		for _, lang := range []string{"de", "fr"} {
			d, lang := d, lang
			t.Run(d.Hotel.Name+"/"+lang, func(t *testing.T) {
				var foreignDesc string
				if lang == "de" {
					foreignDesc = d.DE
				} else {
					foreignDesc = d.FR
				}
				score, hallucinated, err := scoreHallucinationPrecision(ctx, client, foreignDesc, lang, d.Hotel)
				if err != nil {
					t.Fatalf("scoreHallucinationPrecision: %v", err)
				}
				t.Logf("hallucination precision = %.2f; invented: %v", score, hallucinated)
				if score < Tier1PassThreshold {
					t.Errorf("hallucination precision %.2f < threshold %.2f; invented claims: %v",
						score, Tier1PassThreshold, hallucinated)
				}
			})
		}
	}
}

func TestTier1BackTranslation(t *testing.T) {
	client, descs := setup(t)
	ctx := context.Background()

	for _, d := range descs {
		for _, lang := range []string{"de", "fr"} {
			d, lang := d, lang
			t.Run(d.Hotel.Name+"/"+lang, func(t *testing.T) {
				var foreignDesc string
				if lang == "de" {
					foreignDesc = d.DE
				} else {
					foreignDesc = d.FR
				}
				score, notes, err := scoreBackTranslation(ctx, client, d.EN, foreignDesc, lang)
				if err != nil {
					t.Fatalf("scoreBackTranslation: %v", err)
				}
				t.Logf("back-translation score = %.2f; notes: %s", score, notes)
				if score < Tier1PassThreshold {
					t.Errorf("back-translation score %.2f < threshold %.2f; notes: %s",
						score, Tier1PassThreshold, notes)
				}
			})
		}
	}
}

// --- Tier 2 tests ---

func TestTier2Register(t *testing.T) {
	client, descs := setup(t)
	ctx := context.Background()

	for _, d := range descs {
		for _, lang := range []string{"de", "fr"} {
			d, lang := d, lang
			t.Run(d.Hotel.Name+"/"+lang, func(t *testing.T) {
				result, err := EvaluateTier2(ctx, client, foreignText(d, lang), lang)
				if err != nil {
					t.Fatalf("EvaluateTier2: %v", err)
				}
				s := result.Register
				t.Logf("register score = %.0f/10; notes: %s", s.Score, s.Notes)
				if s.Score/10.0 < Tier2PassThreshold {
					t.Errorf("register %.0f/10 below threshold (%.0f); notes: %s",
						s.Score, Tier2PassThreshold*10, s.Notes)
				}
			})
		}
	}
}

func TestTier2Idiom(t *testing.T) {
	client, descs := setup(t)
	ctx := context.Background()

	for _, d := range descs {
		for _, lang := range []string{"de", "fr"} {
			d, lang := d, lang
			t.Run(d.Hotel.Name+"/"+lang, func(t *testing.T) {
				result, err := EvaluateTier2(ctx, client, foreignText(d, lang), lang)
				if err != nil {
					t.Fatalf("EvaluateTier2: %v", err)
				}
				s := result.Idiom
				t.Logf("idiom score = %.0f/10; notes: %s", s.Score, s.Notes)
				if s.Score/10.0 < Tier2PassThreshold {
					t.Errorf("idiom %.0f/10 below threshold (%.0f); notes: %s",
						s.Score, Tier2PassThreshold*10, s.Notes)
				}
			})
		}
	}
}

func TestTier2Flow(t *testing.T) {
	client, descs := setup(t)
	ctx := context.Background()

	for _, d := range descs {
		for _, lang := range []string{"de", "fr"} {
			d, lang := d, lang
			t.Run(d.Hotel.Name+"/"+lang, func(t *testing.T) {
				result, err := EvaluateTier2(ctx, client, foreignText(d, lang), lang)
				if err != nil {
					t.Fatalf("EvaluateTier2: %v", err)
				}
				s := result.Flow
				t.Logf("flow score = %.0f/10; notes: %s", s.Score, s.Notes)
				if s.Score/10.0 < Tier2PassThreshold {
					t.Errorf("flow %.0f/10 below threshold (%.0f); notes: %s",
						s.Score, Tier2PassThreshold*10, s.Notes)
				}
			})
		}
	}
}

func TestTier2CulturalResonance(t *testing.T) {
	client, descs := setup(t)
	ctx := context.Background()

	for _, d := range descs {
		for _, lang := range []string{"de", "fr"} {
			d, lang := d, lang
			t.Run(d.Hotel.Name+"/"+lang, func(t *testing.T) {
				result, err := EvaluateTier2(ctx, client, foreignText(d, lang), lang)
				if err != nil {
					t.Fatalf("EvaluateTier2: %v", err)
				}
				s := result.CulturalResonance
				t.Logf("cultural resonance = %.0f/10; notes: %s", s.Score, s.Notes)
				if s.Score/10.0 < Tier2PassThreshold {
					t.Errorf("cultural resonance %.0f/10 below threshold (%.0f); notes: %s",
						s.Score, Tier2PassThreshold*10, s.Notes)
				}
			})
		}
	}
}

// --- Combined gate ---

func TestCombinedScore(t *testing.T) {
	client, descs := setup(t)
	ctx := context.Background()

	for _, d := range descs {
		for _, lang := range []string{"de", "fr"} {
			d, lang := d, lang
			t.Run(d.Hotel.Name+"/"+lang, func(t *testing.T) {
				result, err := Evaluate(ctx, client, d, lang)
				if err != nil {
					t.Fatalf("Evaluate: %v", err)
				}
				t.Logf("tier1=%.2f  tier2=%.2f  combined=%.2f  passed=%v",
					result.Tier1.Combined, result.Tier2.Combined, result.Combined, result.Passed)
				if result.Tier1.Combined < Tier1PassThreshold {
					t.Errorf("tier1 %.2f < %.2f", result.Tier1.Combined, Tier1PassThreshold)
				}
				if result.Tier2.Combined < Tier2PassThreshold {
					t.Errorf("tier2 %.2f < %.2f", result.Tier2.Combined, Tier2PassThreshold)
				}
				if result.Combined < CombinedPassThreshold {
					t.Errorf("combined %.2f < %.2f", result.Combined, CombinedPassThreshold)
				}
			})
		}
	}
}

func foreignText(d Descriptions, lang string) string {
	if lang == "de" {
		return d.DE
	}
	return d.FR
}
