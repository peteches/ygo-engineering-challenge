package main

// Hotel is the source data for a single property.
type Hotel struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	City      string   `json:"city"`
	Country   string   `json:"country"`
	Setting   string   `json:"setting"`
	Amenities []string `json:"amenities"`
	Rooms     []string `json:"rooms"`
	Nearby    []string `json:"nearby"`
	Policies  []string `json:"policies"`
	PriceBand string   `json:"price_band"`
}

// Descriptions holds all three language versions for a hotel.
type Descriptions struct {
	Hotel         Hotel
	FeaturedFacts []string // the fact set selected for the English base
	EN            string
	DE            string
	FR            string
}

// SubScore is a numeric score with LLM-generated notes.
type SubScore struct {
	Score float64 `json:"score"`
	Notes string  `json:"notes"`
}

// Tier1Result holds scores for the three Tier 1 sub-metrics.
type Tier1Result struct {
	ClaimCoverage          float64 // 1a
	HallucinationPrecision float64 // 1b
	BackTranslation        float64 // 1c
	Combined               float64
}

// Tier2Result holds scores for the four Tier 2 sub-metrics.
type Tier2Result struct {
	Register          SubScore `json:"register"`           // 2a
	Idiom             SubScore `json:"idiom"`              // 2b
	Flow              SubScore `json:"flow"`               // 2c
	CulturalResonance SubScore `json:"cultural_resonance"` // 2d
	Combined          float64
}

// EvaluationResult is the full evaluation for one hotel+language pair.
type EvaluationResult struct {
	HotelName string
	Language  string
	Tier1     Tier1Result
	Tier2     Tier2Result
	Combined  float64
	Passed    bool
}

const (
	Tier1PassThreshold    = 0.90
	Tier2PassThreshold    = 0.70
	CombinedPassThreshold = 0.85
)

func tier1Combined(r Tier1Result) float64 {
	return r.ClaimCoverage*0.40 + r.HallucinationPrecision*0.35 + r.BackTranslation*0.25
}

func tier2Combined(r Tier2Result) float64 {
	return (r.Register.Score*0.25 + r.Idiom.Score*0.35 + r.Flow.Score*0.25 + r.CulturalResonance.Score*0.15) / 10.0
}

func combinedScore(t1, t2 float64) float64 {
	return t1*0.70 + t2*0.30
}
