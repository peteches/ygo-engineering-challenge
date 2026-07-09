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
		fmt.Printf("%s — %s, %s\n", hotel.Name, hotel.City, hotel.Country)
		fmt.Printf("════════════════════════════════════════\n\n")

		d, err := GenerateDescriptions(ctx, client, hotel, Descriptions{}, nil, DefaultGeneratorPrompts())
		if err != nil {
			log.Fatalf("generate %s: %v", hotel.Name, err)
		}

		fmt.Printf("[EN]\n%s\n\n[DE]\n%s\n\n[FR]\n%s\n", d.EN, d.DE, d.FR)
	}
}
