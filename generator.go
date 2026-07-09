package main

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

type englishResult struct {
	Description   string   `json:"description"`
	FeaturedFacts []string `json:"featured_facts"`
}

// GenerateDescriptions produces EN/DE/FR descriptions for a hotel.
// Stage A: English base + featured fact list.
// Stage B: DE and FR from the same fact list (not from the English text).
func GenerateDescriptions(ctx context.Context, client *openai.Client, hotel Hotel) (Descriptions, error) {
	model := defaultModel()
	hotelJSON, err := json.MarshalIndent(hotel, "", "  ")
	if err != nil {
		return Descriptions{}, err
	}

	// Stage A: English base
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

	// Stage B: German
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

	// Stage B: French
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
