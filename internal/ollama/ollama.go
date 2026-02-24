package ollama

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Client struct {
	outputDir string
}

func NewClient(outputDir string) *Client {
	return &Client{
		outputDir: outputDir,
	}
}

func (c *Client) GenerateImage(prompt, systemPrompt string) (string, error) {
	fullPrompt := prompt
	if systemPrompt != "" {
		fullPrompt = systemPrompt + "\n\n" + prompt
	}

	cmd := exec.Command("ollama", "run", "x/flux2-klein", fullPrompt)
	
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
		return "", fmt.Errorf("failed to extract filename from output: %w", err)
	}

	cwdPath := filepath.Join(".", filename)
	if _, err := os.Stat(cwdPath); os.IsNotExist(err) {
		return "", fmt.Errorf("generated image not found at: %s", cwdPath)
	}

	destPath := filepath.Join(c.outputDir, filename)
	if err := os.Rename(cwdPath, destPath); err != nil {
		return "", fmt.Errorf("failed to move image to %s: %w", destPath, err)
	}

	return filename, nil
}

func extractFilename(output string) (string, error) {
	re := regexp.MustCompile(`Image saved to:\s+(.+\.png)`)
	matches := re.FindStringSubmatch(output)
	
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find 'Image saved to:' pattern in output")
	}
	
	filename := strings.TrimSpace(matches[1])
	return filename, nil
}
