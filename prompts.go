package main

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed prompts
var promptFS embed.FS

// GeneratorPrompts holds embedded-FS paths for all generation prompts.
type GeneratorPrompts struct {
	English  string
	German   string
	French   string
	RefineDE string
	RefineFR string
}

// DefaultGeneratorPrompts returns the standard prompt file paths.
func DefaultGeneratorPrompts() GeneratorPrompts {
	return GeneratorPrompts{
		English:  "prompts/gen_english/default.md",
		German:   "prompts/gen_de/default.md",
		French:   "prompts/gen_fr/default.md",
		RefineDE: "prompts/refine_de/default.md",
		RefineFR: "prompts/refine_fr/default.md",
	}
}

// EvaluatorPrompts holds embedded-FS paths for all evaluator prompts.
type EvaluatorPrompts struct {
	ClaimExtract         string
	ClaimVerify          string
	HallucinationExtract string
	HallucinationVerify  string
	BackTranslate        string
	BackScore            string
	LanguageNativeness   string
}

// DefaultEvaluatorPrompts returns the standard evaluator prompt file paths.
func DefaultEvaluatorPrompts() EvaluatorPrompts {
	return EvaluatorPrompts{
		ClaimExtract:         "prompts/eval_claim_extract/default.md",
		ClaimVerify:          "prompts/eval_claim_verify/default.md",
		HallucinationExtract: "prompts/eval_hallucination_extract/default.md",
		HallucinationVerify:  "prompts/eval_hallucination_verify/default.md",
		BackTranslate:        "prompts/eval_backtrans_translate/default.md",
		BackScore:            "prompts/eval_backtrans_score/default.md",
		LanguageNativeness:   "prompts/eval_language_nativeness/default.md",
	}
}

// listPromptVariants returns all .md file paths within a prompt slot directory.
func listPromptVariants(dir string) []string {
	entries, _ := promptFS.ReadDir(dir)
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			paths = append(paths, dir+"/"+e.Name())
		}
	}
	return paths
}

// renderPrompt loads a prompt template from the embedded FS and executes it with data.
func renderPrompt(path string, data any) (string, error) {
	raw, err := promptFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("load prompt %s: %w", path, err)
	}
	tmpl, err := template.New(path).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse prompt %s: %w", path, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render prompt %s: %w", path, err)
	}
	return buf.String(), nil
}
