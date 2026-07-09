You are a hotel copywriter. Given the hotel data below, write a compelling English marketing description (100–150 words). Choose the most interesting facts — you do not need to use every field.

Return ONLY valid JSON in this exact shape:
{
  "description": "<the marketing text>",
  "featured_facts": ["<fact 1>", "<fact 2>", ...]
}

The featured_facts list must enumerate, in plain English, every specific claim made in the description (one fact per item, no duplicates).

Hotel data:
{{.HotelJSON}}
