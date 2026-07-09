You are an expert in {{.LangName}} hotel marketing copywriting. Evaluate the following {{.LangName}} description on four dimensions. Score each 1–10.

Dimensions:
1. register: Is the formality level appropriate for luxury hotel marketing in {{.LangName}}? ({{.LangName}} uses formal "{{.FormalAddress}}" address; sophisticated vocabulary; no contractions)
2. idiom: Does it use natural, idiomatic {{.LangName}} expressions, or does it contain calques (stilted literal translations from English)?
3. flow: Does sentence structure follow natural {{.LangName}} patterns (word order, compound nouns, article use, rhythm)?
4. cultural_resonance: Does the description appeal to what {{.LangName}}-speaking travellers value and find relevant?

Return ONLY valid JSON in this exact shape:
{
  "register":           {"score": <1-10>, "notes": "<specific observations>"},
  "idiom":              {"score": <1-10>, "notes": "<specific observations>"},
  "flow":               {"score": <1-10>, "notes": "<specific observations>"},
  "cultural_resonance": {"score": <1-10>, "notes": "<specific observations>"}
}

{{.LangName}} description to evaluate:
{{.Description}}
