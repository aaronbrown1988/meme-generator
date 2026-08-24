package openrouter

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	orsdk "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"

	"meme-generator/internal/meme"
)

// imageTimeout bounds a single OpenRouter image request. Image models
// are slower than chat models but still must not be allowed to hang a
// generation forever with no feedback to the user.
const imageTimeout = 120 * time.Second

// ImageAdapter satisfies meme.ImageGenerator by requesting an image from
// OpenRouter's image generation router and writing the decoded PNG
// bytes into OutputDir.
type ImageAdapter struct {
	Client    *orsdk.OpenRouter
	Model     string
	OutputDir string
}

func (a *ImageAdapter) Generate(prompt, systemPrompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), imageTimeout)
	defer cancel()

	outputFormat := components.ImageGenerationRequestOutputFormatPng
	res, err := a.Client.Images.Generate(ctx, components.ImageGenerationRequest{
		Model:        a.Model,
		Prompt:       meme.BuildImagePrompt(prompt, systemPrompt),
		OutputFormat: &outputFormat,
	})
	if err != nil {
		return "", wrapRequestError("image", a.Model, imageTimeout, err)
	}
	if res.ImageGenerationResponse == nil || len(res.ImageGenerationResponse.Data) == 0 {
		return "", fmt.Errorf("openrouter returned no image data (model=%s)", a.Model)
	}

	raw, err := base64.StdEncoding.DecodeString(res.ImageGenerationResponse.Data[0].B64JSON)
	if err != nil {
		return "", fmt.Errorf("decode image data (model=%s): %w", a.Model, err)
	}

	filename := fmt.Sprintf("openrouter-%d.png", time.Now().UnixNano())
	if err := os.WriteFile(filepath.Join(a.OutputDir, filename), raw, 0644); err != nil {
		return "", fmt.Errorf("write image file: %w", err)
	}
	return filename, nil
}
