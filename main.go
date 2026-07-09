package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func main() {
	client, err := newClient()
	if err != nil {
		log.Fatalf("error: %v\nSet LLM_API_KEY (and optionally LLM_BASE_URL, LLM_MODEL)", err)
	}

	data, err := os.ReadFile("hotels.json")
	if err != nil {
		log.Fatalf("read hotels.json: %v", err)
	}
	var hotels []Hotel
	if err := json.Unmarshal(data, &hotels); err != nil {
		log.Fatalf("parse hotels.json: %v", err)
	}

	ctx := context.Background()

	for _, hotel := range hotels {
		fmt.Printf("\n════════════════════════════════════════\n")
		fmt.Printf("Hotel: %s (%s)\n", hotel.Name, hotel.City)
		fmt.Printf("════════════════════════════════════════\n")

		d, err := GenerateDescriptions(ctx, client, hotel)
		if err != nil {
			log.Fatalf("generate %s: %v", hotel.Name, err)
		}

		fmt.Printf("\n[EN]\n%s\n\n[DE]\n%s\n\n[FR]\n%s\n", d.EN, d.DE, d.FR)

		for _, lang := range []string{"de", "fr"} {
			result, err := Evaluate(ctx, client, d, lang)
			if err != nil {
				log.Printf("evaluate %s/%s: %v", hotel.Name, lang, err)
				continue
			}
			status := "PASS"
			if !result.Passed {
				status = "FAIL"
			}
			fmt.Printf("\n[%s] %s\n", lang, status)
			fmt.Printf("  Tier 1: coverage=%.2f  precision=%.2f  back-trans=%.2f  → %.2f\n",
				result.Tier1.ClaimCoverage,
				result.Tier1.HallucinationPrecision,
				result.Tier1.BackTranslation,
				result.Tier1.Combined)
			fmt.Printf("  Tier 2: register=%.0f  idiom=%.0f  flow=%.0f  culture=%.0f  → %.2f\n",
				result.Tier2.Register.Score,
				result.Tier2.Idiom.Score,
				result.Tier2.Flow.Score,
				result.Tier2.CulturalResonance.Score,
				result.Tier2.Combined)
			fmt.Printf("  Combined: %.2f\n", result.Combined)
		}
	}
}
