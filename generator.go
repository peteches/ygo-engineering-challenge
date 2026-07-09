package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type englishResult struct {
	Description   string   `json:"description"`
	FeaturedFacts []string `json:"featured_facts"`
}

// GenerationFeedback carries evaluation results from a previous attempt
// so the model can be guided to fix specific problems in refinement mode.
type GenerationFeedback struct {
	Language             string
	PreviousDescription  string
	MissedClaims         []string
	HallucinatedClaims   []string
	BackTranslationNotes string
	LinguisticNotes      map[string]string // "register","idiom","flow","cultural_resonance" -> notes
}

// GenerateDescriptions produces EN/DE/FR descriptions for a hotel.
//
// When feedback is nil (first attempt), both languages are generated fresh.
// When feedback is non-nil (refinement), only languages listed in feedback
// are regenerated; the rest are copied unchanged from prev.
func GenerateDescriptions(ctx context.Context, client *openai.Client, hotel Hotel, prev Descriptions, feedback []GenerationFeedback, prompts GeneratorPrompts) (Descriptions, error) {
	model := defaultModel()

	if feedback == nil {
		return generateFresh(ctx, client, model, hotel, prompts)
	}
	return generateRefinement(ctx, client, model, hotel, prev, feedback, prompts)
}

func generateFresh(ctx context.Context, client *openai.Client, model string, hotel Hotel, prompts GeneratorPrompts) (Descriptions, error) {
	hotelJSON, err := json.MarshalIndent(hotel, "", "  ")
	if err != nil {
		return Descriptions{}, err
	}

	promptEN, err := renderPrompt(prompts.English, map[string]any{
		"HotelJSON": string(hotelJSON),
	})
	if err != nil {
		return Descriptions{}, err
	}

	var enResult englishResult
	if err := chatJSON(ctx, client, model, promptEN, &enResult); err != nil {
		return Descriptions{}, fmt.Errorf("English generation: %w", err)
	}

	factsJSON, _ := json.Marshal(enResult.FeaturedFacts)

	promptDE, err := renderPrompt(prompts.German, map[string]any{
		"HotelName": hotel.Name,
		"FactsJSON": string(factsJSON),
	})
	if err != nil {
		return Descriptions{}, err
	}

	deText, err := chat(ctx, client, model, promptDE)
	if err != nil {
		return Descriptions{}, fmt.Errorf("German generation: %w", err)
	}

	promptFR, err := renderPrompt(prompts.French, map[string]any{
		"HotelName": hotel.Name,
		"FactsJSON": string(factsJSON),
	})
	if err != nil {
		return Descriptions{}, err
	}

	frText, err := chat(ctx, client, model, promptFR)
	if err != nil {
		return Descriptions{}, fmt.Errorf("French generation: %w", err)
	}

	return Descriptions{
		Hotel:         hotel,
		FeaturedFacts: enResult.FeaturedFacts,
		EN:            enResult.Description,
		DE:            deText,
		FR:            frText,
	}, nil
}

func generateRefinement(ctx context.Context, client *openai.Client, model string, hotel Hotel, prev Descriptions, feedback []GenerationFeedback, prompts GeneratorPrompts) (Descriptions, error) {
	result := Descriptions{
		Hotel:         hotel,
		FeaturedFacts: prev.FeaturedFacts,
		EN:            prev.EN,
		DE:            prev.DE,
		FR:            prev.FR,
	}

	factsJSON, _ := json.Marshal(prev.FeaturedFacts)

	refinePaths := map[string]string{
		"de": prompts.RefineDE,
		"fr": prompts.RefineFR,
	}

	for _, fb := range feedback {
		refinePath, ok := refinePaths[fb.Language]
		if !ok {
			return Descriptions{}, fmt.Errorf("unsupported language for refinement: %s", fb.Language)
		}

		var problems []string
		if len(fb.MissedClaims) > 0 {
			problems = append(problems, fmt.Sprintf("Facts you missed (must include): %v", fb.MissedClaims))
		}
		if len(fb.HallucinatedClaims) > 0 {
			problems = append(problems, fmt.Sprintf("Facts you invented (must remove): %v", fb.HallucinatedClaims))
		}
		if fb.BackTranslationNotes != "" {
			problems = append(problems, fmt.Sprintf("Meaning drift detected: %s", fb.BackTranslationNotes))
		}
		for dim, notes := range fb.LinguisticNotes {
			if notes != "" {
				problems = append(problems, fmt.Sprintf("Linguistic issue (%s): %s", dim, notes))
			}
		}

		prompt, err := renderPrompt(refinePath, map[string]any{
			"HotelName":           hotel.Name,
			"PreviousDescription": fb.PreviousDescription,
			"Problems":            strings.Join(problems, "\n"),
			"FactsJSON":           string(factsJSON),
		})
		if err != nil {
			return Descriptions{}, err
		}

		text, err := chat(ctx, client, model, prompt)
		if err != nil {
			return Descriptions{}, fmt.Errorf("%s refinement: %w", fb.Language, err)
		}

		switch fb.Language {
		case "de":
			result.DE = text
		case "fr":
			result.FR = text
		}
	}

	return result, nil
}
