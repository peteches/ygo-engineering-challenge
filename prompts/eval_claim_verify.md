You are checking whether a {{.Language}} hotel description covers a set of facts.

For each claim in the list below, decide:
  "yes"     — the {{.Language}} description clearly states this fact
  "partial" — the {{.Language}} description mentions it but with less detail or a weaker form
  "no"      — the {{.Language}} description does not mention this fact

Return ONLY a JSON array of objects in this exact shape, one per claim, in the same order:
[{"claim": "...", "result": "yes|partial|no"}, ...]

Claims to check:
{{.ClaimsJSON}}

{{.Language}} description to check:
{{.ForeignDesc}}
