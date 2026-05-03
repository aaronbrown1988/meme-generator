package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"meme-generator/internal/meme"
)

// CaptionAdapter satisfies llm.CaptionWriter by invoking the local
// `ollama` CLI with a JSON-formatted text model.
type CaptionAdapter struct {
	Model string
}

func NewCaptionAdapter() *CaptionAdapter {
	return &CaptionAdapter{Model: "gemma4:latest"}
}

func (a *CaptionAdapter) Write(userPrompt string) (meme.Caption, error) {
	fullPrompt := fmt.Sprintf(
		"Generate meme text for: %s\n\nRespond ONLY with valid JSON in this exact format: {\"topText\":\"text here\",\"bottomText\":\"text here\"}. Keep text SHORT and FUNNY. ",
		userPrompt,
	)

	cmd := exec.Command("ollama", "run", a.Model, "--hidethinking", "--nowordwrap", "--format", "json", fullPrompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return meme.Caption{}, fmt.Errorf("ollama caption generation failed: %w, stderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return meme.Caption{}, fmt.Errorf("ollama produced no caption output")
	}

	c, err := parseCaptionJSON(output)
	if err != nil {
		return meme.Caption{}, fmt.Errorf("parse caption JSON: %w (output: %s)", err, output)
	}
	return c, nil
}

func parseCaptionJSON(output string) (meme.Caption, error) {
	jsonStart := strings.Index(output, "{")
	jsonEnd := strings.LastIndex(output, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return meme.Caption{}, fmt.Errorf("no JSON object found in output")
	}

	jsonStr := output[jsonStart : jsonEnd+1]

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return meme.Caption{}, fmt.Errorf("invalid JSON: %w", err)
	}

	return meme.Caption{
		Top:    getStringField(result, "topText", "top_text", "TopText", "top"),
		Bottom: getStringField(result, "bottomText", "bottom_text", "BottomText", "bottom"),
	}, nil
}

func getStringField(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}
	return ""
}
