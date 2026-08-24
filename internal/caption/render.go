// Package caption renders top/bottom meme text onto an image file.
// Pure in-process work — no LLM, no domain types, no I/O beyond
// reading and rewriting the target image.
package caption

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/gomonobold"
)

// FontPath is the optional path to an Impact-style TTF. If absent or
// unloadable, Render falls back to the embedded gomonobold font.
var FontPath = "assets/fonts/Impact.ttf"

// Render draws top and bottom onto the image at imagePath in classic
// meme style (white text, black outline, all-caps). Either string may
// be empty; if both are, Render is a no-op. The image is overwritten
// in place.
func Render(imagePath, top, bottom string) error {
	if top == "" && bottom == "" {
		return nil
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	img, _, err := image.Decode(file)
	file.Close()
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	dc := gg.NewContextForImage(img)
	width := float64(dc.Width())
	height := float64(dc.Height())

	if top != "" {
		text := strings.ToUpper(top)
		fontSize := optimalFontSize(dc, text, width, height)
		if err := loadFont(dc, fontSize); err != nil {
			return fmt.Errorf("load font for top: %w", err)
		}
		drawTextWithOutline(dc, text, width/2, height*0.1)
	}

	if bottom != "" {
		text := strings.ToUpper(bottom)
		fontSize := optimalFontSize(dc, text, width, height)
		if err := loadFont(dc, fontSize); err != nil {
			return fmt.Errorf("load font for bottom: %w", err)
		}
		drawTextWithOutline(dc, text, width/2, height*0.9)
	}

	out, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer out.Close()
	if err := png.Encode(out, dc.Image()); err != nil {
		return fmt.Errorf("encode image: %w", err)
	}
	return nil
}

func optimalFontSize(dc *gg.Context, text string, width, height float64) float64 {
	maxFontSize := height / 10
	if maxFontSize < 20 {
		maxFontSize = 20
	} else if maxFontSize > 120 {
		maxFontSize = 120
	}

	targetWidth := width * 0.9
	minSize := 12.0
	fontSize := maxFontSize

	for i := 0; i < 10; i++ {
		if err := loadFont(dc, fontSize); err != nil {
			break
		}
		textWidth, _ := dc.MeasureString(text)
		if textWidth <= targetWidth {
			if fontSize >= maxFontSize {
				break
			}
			minSize = fontSize
			fontSize = (fontSize + maxFontSize) / 2
		} else {
			maxFontSize = fontSize
			fontSize = (minSize + fontSize) / 2
		}
		if maxFontSize-minSize < 1 {
			break
		}
	}

	if fontSize < 16 {
		fontSize = 16
	}
	return fontSize
}

func loadFont(dc *gg.Context, size float64) error {
	if _, err := os.Stat(FontPath); err == nil {
		if err := dc.LoadFontFace(FontPath, size); err == nil {
			return nil
		}
	}

	font, err := truetype.Parse(gomonobold.TTF)
	if err != nil {
		return fmt.Errorf("parse fallback font: %w", err)
	}
	dc.SetFontFace(truetype.NewFace(font, &truetype.Options{Size: size}))
	return nil
}

func drawTextWithOutline(dc *gg.Context, text string, x, y float64) {
	const outlineSize = 3.0
	dc.SetRGB(0, 0, 0)
	for dx := -outlineSize; dx <= outlineSize; dx++ {
		for dy := -outlineSize; dy <= outlineSize; dy++ {
			if dx != 0 || dy != 0 {
				dc.DrawStringAnchored(text, x+dx, y+dy, 0.5, 0.5)
			}
		}
	}
	dc.SetRGB(1, 1, 1)
	dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
}
