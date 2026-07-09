You are fact-checking hotel marketing claims against the original hotel data.

For each claim below, decide if it is supported by the hotel data:
  "yes"     — clearly supported by the hotel data
  "partial" — roughly supported but slightly exaggerated or imprecise
  "no"      — not supported or contradicts the hotel data

Return ONLY a JSON array in this shape, one per claim, in order:
[{"claim": "...", "result": "yes|partial|no"}, ...]

Hotel source data:
{{.HotelJSON}}

Claims to fact-check:
{{.ClaimsJSON}}
