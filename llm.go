package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// newClient builds an OpenAI-compatible client from environment variables.
// LLM_BASE_URL   — base URL of the API (default: https://api.openai.com/v1)
// LLM_API_KEY    — API key (required)
// LLM_MODEL      — model name to use (default: gpt-4o-mini)
func newClient() (*openai.Client, error) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("LLM_API_KEY environment variable not set")
	}

	cfg := openai.DefaultConfig(apiKey)

	if base := os.Getenv("LLM_BASE_URL"); base != "" {
		cfg.BaseURL = base
	}

	return openai.NewClientWithConfig(cfg), nil
}

func defaultModel() string {
	if m := os.Getenv("LLM_MODEL"); m != "" {
		return m
	}
	return "gpt-4o-mini"
}

// chat sends a single user prompt and returns the assistant's text response.
func chat(ctx context.Context, client *openai.Client, model, prompt string) (string, error) {
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from model")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// chatJSON sends a prompt and unmarshals the response as JSON into dest.
func chatJSON(ctx context.Context, client *openai.Client, model, prompt string, dest any) error {
	raw, err := chat(ctx, client, model, prompt)
	if err != nil {
		return err
	}
	// Strip markdown code fences if the model wraps its JSON
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) >= 2 {
			raw = strings.Join(lines[1:], "\n")
		}
		raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	}
	if err := json.Unmarshal([]byte(raw), dest); err != nil {
		return fmt.Errorf("JSON unmarshal failed (raw: %q): %w", raw, err)
	}
	return nil
}
