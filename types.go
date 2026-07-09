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

// ClaimAccuracyResult holds scores for the three factual accuracy sub-metrics.
type ClaimAccuracyResult struct {
	ClaimCoverage          float64 // 1a
	HallucinationPrecision float64 // 1b
	BackTranslation        float64 // 1c
	Combined               float64
	// detail for feedback
	MissedClaims         []string
	HallucinatedClaims   []string
	BackTranslationNotes string
}

// LanguageNativenessResult holds scores for the four linguistic quality sub-metrics.
type LanguageNativenessResult struct {
	Register          SubScore `json:"register"`           // 2a
	Idiom             SubScore `json:"idiom"`              // 2b
	Flow              SubScore `json:"flow"`               // 2c
	CulturalResonance SubScore `json:"cultural_resonance"` // 2d
	Combined          float64
}

// EvaluationResult is the full evaluation for one hotel+language pair.
type EvaluationResult struct {
	HotelName          string
	Language           string
	ClaimAccuracy      ClaimAccuracyResult
	LanguageNativeness LanguageNativenessResult
	Combined           float64
	Passed             bool
}

const (
	ClaimAccuracyThreshold      = 0.90
	LanguageNativenessThreshold = 0.70
	CombinedPassThreshold       = 0.85
)

func claimAccuracyCombined(r ClaimAccuracyResult) float64 {
	return r.ClaimCoverage*0.40 + r.HallucinationPrecision*0.35 + r.BackTranslation*0.25
}

func languageNativenessCombined(r LanguageNativenessResult) float64 {
	return (r.Register.Score*0.25 + r.Idiom.Score*0.35 + r.Flow.Score*0.25 + r.CulturalResonance.Score*0.15) / 10.0
}

func combinedScore(t1, t2 float64) float64 {
	return t1*0.70 + t2*0.30
}
