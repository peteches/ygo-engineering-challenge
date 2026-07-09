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
func GenerateDescriptions(ctx context.Context, client *openai.Client, hotel Hotel, prev Descriptions, feedback []GenerationFeedback) (Descriptions, error) {
	model := defaultModel()

	if feedback == nil {
		return generateFresh(ctx, client, model, hotel)
	}
	return generateRefinement(ctx, client, model, hotel, prev, feedback)
}

func generateFresh(ctx context.Context, client *openai.Client, model string, hotel Hotel) (Descriptions, error) {
	hotelJSON, err := json.MarshalIndent(hotel, "", "  ")
	if err != nil {
		return Descriptions{}, err
	}

	promptEN := fmt.Sprintf(`You are a hotel copywriter. Given the hotel data below, write a compelling English marketing description (100–150 words). Choose the most interesting facts — you do not need to use every field.

Return ONLY valid JSON in this exact shape:
{
  "description": "<the marketing text>",
  "featured_facts": ["<fact 1>", "<fact 2>", ...]
}

The featured_facts list must enumerate, in plain English, every specific claim made in the description (one fact per item, no duplicates).

Hotel data:
%s`, string(hotelJSON))

	var enResult englishResult
	if err := chatJSON(ctx, client, model, promptEN, &enResult); err != nil {
		return Descriptions{}, fmt.Errorf("English generation: %w", err)
	}

	factsJSON, _ := json.Marshal(enResult.FeaturedFacts)

	promptDE := fmt.Sprintf(`You are a native German hotel copywriter. Write a marketing description for %s that reads as if written by a German native speaker for a German-speaking audience.

You MUST convey ALL of the following facts, and NOTHING that is not in this list:
%s

Rules:
- Use formal "Sie" address
- Avoid literal translations and English calques; use natural German idioms
- 100–150 words
- Return ONLY the description text, no JSON, no headings`, hotel.Name, string(factsJSON))

	deText, err := chat(ctx, client, model, promptDE)
	if err != nil {
		return Descriptions{}, fmt.Errorf("German generation: %w", err)
	}

	promptFR := fmt.Sprintf(`You are a native French hotel copywriter. Write a marketing description for %s that reads as if written by a French native speaker for a French-speaking audience.

You MUST convey ALL of the following facts, and NOTHING that is not in this list:
%s

Rules:
- Use formal "vous" address
- Avoid literal translations and English calques; use natural French idioms
- 100–150 words
- Return ONLY the description text, no JSON, no headings`, hotel.Name, string(factsJSON))

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

func generateRefinement(ctx context.Context, client *openai.Client, model string, hotel Hotel, prev Descriptions, feedback []GenerationFeedback) (Descriptions, error) {
	result := Descriptions{
		Hotel:         hotel,
		FeaturedFacts: prev.FeaturedFacts,
		EN:            prev.EN,
		DE:            prev.DE,
		FR:            prev.FR,
	}

	feedbackByLang := make(map[string]GenerationFeedback, len(feedback))
	for _, fb := range feedback {
		feedbackByLang[fb.Language] = fb
	}

	factsJSON, _ := json.Marshal(prev.FeaturedFacts)

	type langConfig struct {
		name      string
		formal    string
		idiomNote string
	}
	langCfg := map[string]langConfig{
		"de": {"German", "Sie", "Avoid English calques; use natural German idioms"},
		"fr": {"French", "vous", "Avoid English calques; use natural French idioms"},
	}

	for lang, fb := range feedbackByLang {
		cfg := langCfg[lang]

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

		prompt := fmt.Sprintf(`You are a native %s hotel copywriter. You previously wrote this %s description:

%s

A quality check found these problems:
%s

Write a corrected %s description for %s that fixes every issue above.
You must still convey exactly these facts (and nothing else):
%s

Rules:
- Use formal "%s" address
- %s
- 100–150 words
- Return ONLY the description text, no JSON, no headings`,
			cfg.name, cfg.name,
			fb.PreviousDescription,
			strings.Join(problems, "\n"),
			cfg.name, hotel.Name,
			string(factsJSON),
			cfg.formal,
			cfg.idiomNote)

		text, err := chat(ctx, client, model, prompt)
		if err != nil {
			return Descriptions{}, fmt.Errorf("%s refinement: %w", lang, err)
		}

		switch lang {
		case "de":
			result.DE = text
		case "fr":
			result.FR = text
		}
	}

	return result, nil
}
