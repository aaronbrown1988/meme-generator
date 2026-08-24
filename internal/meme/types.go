// Package meme is the domain centre of the application: the types and
// the Pipeline that turns a user prompt into a finished Generation.
//
// Other internal packages depend on meme; meme depends on no other
// internal package.
package meme

import "time"

// Status is the lifecycle state of a Generation. Aliased to string so
// HTML templates can compare it against string literals without
// explicit conversion.
type Status = string

const (
	StatusProcessing Status = "processing"
	StatusSuccess    Status = "success"
	StatusFailed     Status = "failed"
)

// Caption is the top/bottom text rendered onto a meme image. Either half
// may be empty; an entirely empty Caption means "no text overlay."
type Caption struct {
	Top    string
	Bottom string
}

// Empty reports whether the Caption has no text at all.
func (c Caption) Empty() bool {
	return c.Top == "" && c.Bottom == ""
}

// Generation is one end-to-end attempt to produce a meme. The unit of
// persistence and the unit of work for the Pipeline.
type Generation struct {
	ID           int64
	Prompt       string
	ImagePath    string
	Caption      Caption
	Status       Status
	ErrorMessage string
	CreatedAt    time.Time
}

// ImageGenerator produces a meme image from a prompt. The returned
// filename is the basename of a PNG written into the Pipeline's
// configured image directory.
//
// Failure is terminal for a Generation — without an image there is no
// meme.
type ImageGenerator interface {
	Generate(prompt, systemPrompt string) (filename string, err error)
}

// CaptionWriter produces top/bottom meme text from a prompt.
//
// Failure is non-fatal — the Pipeline degrades to an un-captioned meme
// when this returns an error.
type CaptionWriter interface {
	Write(prompt, systemPrompt string) (Caption, error)
}
