// Package ollama is the production adapter family for the llm ports.
// Each adapter shells out to the local `ollama` CLI.
package ollama

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"meme-generator/internal/meme"
)

// ImageAdapter satisfies llm.ImageGenerator by invoking
// `ollama run <model> <prompt>` in a private temp directory and moving
// the resulting PNG into outputDir.
type ImageAdapter struct {
	Model     string
	OutputDir string
}

func NewImageAdapter(outputDir string) *ImageAdapter {
	return &ImageAdapter{
		Model:     "x/flux2-klein",
		OutputDir: outputDir,
	}
}

func (a *ImageAdapter) Generate(prompt, systemPrompt string) (string, error) {
	fullPrompt := meme.BuildImagePrompt(prompt, systemPrompt)

	tmpDir, err := os.MkdirTemp("", "meme-gen-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("ollama", "run", a.Model, fullPrompt)
	cmd.Dir = tmpDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ollama command failed: %w, stderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", fmt.Errorf("ollama produced no output")
	}

	filename, err := extractFilename(output)
	if err != nil {
		return "", fmt.Errorf("extract filename from ollama output: %w", err)
	}

	srcPath := filepath.Join(tmpDir, filename)
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return "", fmt.Errorf("generated image not found at: %s", srcPath)
	}

	destPath := filepath.Join(a.OutputDir, filename)
	if err := os.Rename(srcPath, destPath); err != nil {
		if cpErr := copyFile(srcPath, destPath); cpErr != nil {
			return "", fmt.Errorf("move image to %s: %w (copy fallback: %v)", destPath, err, cpErr)
		}
	}

	return filename, nil
}

var imageSavedRE = regexp.MustCompile(`Image saved to:\s+(.+\.png)`)

func extractFilename(output string) (string, error) {
	matches := imageSavedRE.FindStringSubmatch(output)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find 'Image saved to:' pattern in output")
	}
	return strings.TrimSpace(matches[1]), nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
