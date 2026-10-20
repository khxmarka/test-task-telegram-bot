package agent

import (
	"encoding/json"
	"strings"
)

// StripNullArgs removes null-valued keys from tool argument JSON (Groq rejects nulls).
func StripNullArgs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return "{}"
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return raw
	}
	clean := map[string]any{}
	for k, v := range m {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		clean[k] = v
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return raw
	}
	return string(b)
}
