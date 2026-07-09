You are a hotel copywriter. Study the hotel data below and select the 6–10 most compelling facts. Build a 100–150 word English marketing description that weaves those facts into flowing prose.

Return ONLY valid JSON:
{
  "description": "<marketing text>",
  "featured_facts": ["<fact 1>", "<fact 2>", ...]
}

Critical: featured_facts must enumerate every specific claim in your description exactly as stated — one fact per item, no duplicates, nothing omitted. This list will be used to verify the description in other languages, so precision matters.

Hotel data:
{{.HotelJSON}}
