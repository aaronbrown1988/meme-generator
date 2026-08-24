package meme

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BuildCaptionPrompt assembles the instruction sent to a Caption Writer
// model: the user's concept plus a directive to respond with the exact
// JSON shape ParseCaptionJSON expects. systemPrompt, when set, is
// prepended as an additional preamble.
func BuildCaptionPrompt(userPrompt, systemPrompt string) string {
	prompt := fmt.Sprintf(
		"Generate meme text for: %s\n\nRespond ONLY with valid JSON in this exact format: {\"topText\":\"text here\",\"bottomText\":\"text here\"}. Keep text SHORT and FUNNY.",
		userPrompt,
	)
	if systemPrompt != "" {
		prompt = systemPrompt + "\n\n" + prompt
	}
	return prompt
}

// BuildImagePrompt assembles the instruction sent to an Image
// Generator model: the user's concept, preceded by systemPrompt when
// set.
func BuildImagePrompt(userPrompt, systemPrompt string) string {
	if systemPrompt == "" {
		return userPrompt
	}
	return systemPrompt + "\n\n" + userPrompt
}

// ParseCaptionJSON extracts a Caption from a Caption Writer model's raw
// text output. Models are inconsistent about surrounding the JSON
// object with commentary or code fences, so this scans for the
// outermost {...} span rather than requiring the output to be pure
// JSON, and accepts a few common key spellings.
func ParseCaptionJSON(output string) (Caption, error) {
	output = strings.TrimSpace(output)
	jsonStart := strings.Index(output, "{")
	jsonEnd := strings.LastIndex(output, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return Caption{}, fmt.Errorf("no JSON object found in output")
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output[jsonStart:jsonEnd+1]), &result); err != nil {
		return Caption{}, fmt.Errorf("invalid JSON: %w", err)
	}

	return Caption{
		Top:    stringField(result, "topText", "top_text", "TopText", "top"),
		Bottom: stringField(result, "bottomText", "bottom_text", "BottomText", "bottom"),
	}, nil
}

func stringField(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if str, ok := m[key].(string); ok {
			return str
		}
	}
	return ""
}
