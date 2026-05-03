// Package fake provides in-memory adapters for the llm ports, used in
// tests of the meme Pipeline.
package fake

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"

	"meme-generator/internal/meme"
)

// ImageAdapter is an in-memory llm.ImageGenerator.
//
// Each Generate call writes a small solid-colour PNG into OutputDir and
// returns its filename. If Err is non-nil, Generate returns it without
// touching the filesystem.
type ImageAdapter struct {
	OutputDir string
	Err       error

	count atomic.Int64
}

func (a *ImageAdapter) Generate(prompt, systemPrompt string) (string, error) {
	if a.Err != nil {
		return "", a.Err
	}
	n := a.count.Add(1)
	filename := fmt.Sprintf("fake-%d.png", n)

	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}

	f, err := os.Create(filepath.Join(a.OutputDir, filename))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return filename, nil
}

// CaptionAdapter is an in-memory llm.CaptionWriter that returns Caption
// (or Err) regardless of the input prompt.
type CaptionAdapter struct {
	Caption meme.Caption
	Err     error
}

func (a *CaptionAdapter) Write(prompt string) (meme.Caption, error) {
	if a.Err != nil {
		return meme.Caption{}, a.Err
	}
	return a.Caption, nil
}
