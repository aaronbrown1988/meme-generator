package ollama

import (
	"bytes"
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

func (a *CaptionAdapter) Write(userPrompt, systemPrompt string) (meme.Caption, error) {
	fullPrompt := meme.BuildCaptionPrompt(userPrompt, systemPrompt)

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

	c, err := meme.ParseCaptionJSON(output)
	if err != nil {
		return meme.Caption{}, fmt.Errorf("parse caption JSON: %w (output: %s)", err, output)
	}
	return c, nil
}
