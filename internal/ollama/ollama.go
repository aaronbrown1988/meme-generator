package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/gomonobold"
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

// GenerateText calls Ollama with gemma3:270m to generate meme text
func (c *Client) GenerateText(userPrompt string) (topText, bottomText string, err error) {
	// Construct prompt asking for JSON meme text
	fullPrompt := fmt.Sprintf(
		"Generate meme text for: %s\n\nRespond ONLY with valid JSON in this exact format: {\"topText\":\"text here\",\"bottomText\":\"text here\"}. Keep text SHORT and FUNNY.",
		userPrompt,
	)

	cmd := exec.Command("ollama", "run", "gemma3:270m", fullPrompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("ollama text generation failed: %w, stderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", "", fmt.Errorf("ollama produced no text output")
	}

	// Parse JSON, handling different field name variations
	topText, bottomText, err = parseTextJSON(output)
	if err != nil {
		// Return empty strings but log error - graceful degradation
		return "", "", fmt.Errorf("failed to parse text JSON: %w (output: %s)", err, output)
	}

	return topText, bottomText, nil
}

// parseTextJSON extracts top and bottom text from JSON, handling field name variations
func parseTextJSON(output string) (string, string, error) {
	// Try to extract JSON from output (model might include extra text)
	jsonStart := strings.Index(output, "{")
	jsonEnd := strings.LastIndex(output, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return "", "", fmt.Errorf("no JSON object found in output")
	}

	jsonStr := output[jsonStart : jsonEnd+1]

	// Try multiple field name variations
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return "", "", fmt.Errorf("invalid JSON: %w", err)
	}

	// Look for topText variations
	topText := getStringField(result, "topText", "top_text", "TopText", "top")
	bottomText := getStringField(result, "bottomText", "bottom_text", "BottomText", "bottom")

	return topText, bottomText, nil
}

// getStringField tries multiple field name variations and returns the first match
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

// OverlayMemeText adds top and bottom text to an image using classic meme styling
func (c *Client) OverlayMemeText(imagePath, topText, bottomText string) error {
	// Skip if no text to overlay
	if topText == "" && bottomText == "" {
		return nil
	}

	// Load the image
	file, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Create drawing context
	dc := gg.NewContextForImage(img)
	width := float64(dc.Width())
	height := float64(dc.Height())

	// Draw top text
	if topText != "" {
		fontSize := c.calculateOptimalFontSize(dc, strings.ToUpper(topText), width, height)
		if err := c.loadFont(dc, fontSize); err != nil {
			return fmt.Errorf("failed to load font for top text: %w", err)
		}
		c.drawTextWithOutline(dc, strings.ToUpper(topText), width/2, height*0.1)
	}

	// Draw bottom text (recalculate font size for different text length)
	if bottomText != "" {
		fontSize := c.calculateOptimalFontSize(dc, strings.ToUpper(bottomText), width, height)
		if err := c.loadFont(dc, fontSize); err != nil {
			return fmt.Errorf("failed to load font for bottom text: %w", err)
		}
		c.drawTextWithOutline(dc, strings.ToUpper(bottomText), width/2, height*0.9)
	}

	// Save the modified image
	outFile, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, dc.Image()); err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	return nil
}

// calculateOptimalFontSize calculates font size that fits text within image width
func (c *Client) calculateOptimalFontSize(dc *gg.Context, text string, width, height float64) float64 {
	// Start with height-based calculation
	maxFontSize := height / 10
	if maxFontSize < 20 {
		maxFontSize = 20
	} else if maxFontSize > 120 {
		maxFontSize = 120
	}

	// Target width is 90% of image width (5% padding on each side)
	targetWidth := width * 0.9

	// Binary search for optimal font size that fits within target width
	minSize := 12.0
	fontSize := maxFontSize

	for iteration := 0; iteration < 10; iteration++ {
		if err := c.loadFont(dc, fontSize); err != nil {
			// If font loading fails, return the current size
			break
		}

		textWidth, _ := dc.MeasureString(text)

		if textWidth <= targetWidth {
			// Text fits, try slightly larger
			if fontSize >= maxFontSize {
				break
			}
			minSize = fontSize
			fontSize = (fontSize + maxFontSize) / 2
		} else {
			// Text too wide, reduce size
			maxFontSize = fontSize
			fontSize = (minSize + fontSize) / 2
		}

		// If we've converged, stop iterating
		if maxFontSize-minSize < 1 {
			break
		}
	}

	// Ensure minimum readable size
	if fontSize < 16 {
		fontSize = 16
	}

	return fontSize
}

// loadFont tries to load Impact.ttf, falls back to embedded gomonobold
func (c *Client) loadFont(dc *gg.Context, size float64) error {
	// Try to load Impact.ttf from assets/fonts
	impactPath := "assets/fonts/Impact.ttf"
	if _, err := os.Stat(impactPath); err == nil {
		if err := dc.LoadFontFace(impactPath, size); err == nil {
			return nil
		}
	}

	// Fallback to embedded gomonobold font
	font, err := truetype.Parse(gomonobold.TTF)
	if err != nil {
		return fmt.Errorf("failed to parse fallback font: %w", err)
	}

	face := truetype.NewFace(font, &truetype.Options{
		Size: size,
	})
	dc.SetFontFace(face)
	return nil
}

// drawTextWithOutline draws white text with black outline (classic meme style)
func (c *Client) drawTextWithOutline(dc *gg.Context, text string, x, y float64) {
	// Draw black outline (stroke)
	outlineSize := 3.0
	dc.SetRGB(0, 0, 0) // Black
	for dx := -outlineSize; dx <= outlineSize; dx++ {
		for dy := -outlineSize; dy <= outlineSize; dy++ {
			if dx != 0 || dy != 0 {
				dc.DrawStringAnchored(text, x+dx, y+dy, 0.5, 0.5)
			}
		}
	}

	// Draw white text on top
	dc.SetRGB(1, 1, 1) // White
	dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
}
