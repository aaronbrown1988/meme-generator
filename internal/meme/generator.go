package meme

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"meme-generator/internal/caption"
)

// SystemPromptKey is the settings key the Pipeline reads to construct
// the image prompt's system preamble.
const SystemPromptKey = "system_prompt"

// Store is the persistence port the Pipeline depends on.
type Store interface {
	InsertGeneration(prompt, imagePath string, status Status, errorMessage string) (int64, error)
	UpdateGenerationStatus(id int64, status Status, imagePath, errorMessage string) error
	UpdateGenerationCaption(id int64, c Caption) error
	GetGeneration(id int64) (Generation, error)
	GetSetting(key string) (string, error)
	MarkProcessingFailed(message string) (int64, error)
}

// Generator is the Pipeline. It accepts prompts via Enqueue, runs them
// asynchronously on a single worker goroutine, and persists the
// resulting Generation. Get exposes the latest persisted state for
// HTMX-style polling.
type Generator struct {
	store     Store
	images    ImageGenerator
	captions  CaptionWriter
	imageDir  string
	queueSize int

	queue chan int64

	startOnce sync.Once
	started   bool
	startErr  error
}

// Config wires a Generator's dependencies.
type Config struct {
	Store         Store
	Images        ImageGenerator
	Captions      CaptionWriter
	ImageDir      string
	QueueCapacity int
}

func NewGenerator(cfg Config) *Generator {
	cap := cfg.QueueCapacity
	if cap <= 0 {
		cap = 64
	}
	return &Generator{
		store:     cfg.Store,
		images:    cfg.Images,
		captions:  cfg.Captions,
		imageDir:  cfg.ImageDir,
		queueSize: cap,
		queue:     make(chan int64, cap),
	}
}

// Start reconciles any stranded processing rows and then runs the
// worker until ctx is cancelled. It blocks for the lifetime of the
// worker. Subsequent calls are no-ops.
func (g *Generator) Start(ctx context.Context) error {
	g.startOnce.Do(func() {
		n, err := g.store.MarkProcessingFailed("interrupted by restart")
		if err != nil {
			g.startErr = fmt.Errorf("reconcile processing rows: %w", err)
			return
		}
		if n > 0 {
			log.Printf("pipeline: reconciled %d stranded processing row(s)", n)
		}
		g.started = true
	})
	if g.startErr != nil {
		return g.startErr
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case id := <-g.queue:
			g.run(id)
		}
	}
}

// Enqueue records a new Generation in the store with status processing
// and returns its id. The actual work happens on the worker.
func (g *Generator) Enqueue(prompt string) (int64, error) {
	if prompt == "" {
		return 0, errors.New("prompt is required")
	}
	id, err := g.store.InsertGeneration(prompt, "", StatusProcessing, "")
	if err != nil {
		return 0, fmt.Errorf("insert generation: %w", err)
	}
	select {
	case g.queue <- id:
	default:
		// Queue full. Mark as failed immediately so the row doesn't
		// linger in processing forever.
		_ = g.store.UpdateGenerationStatus(id, StatusFailed, "", "queue full")
		return id, errors.New("pipeline queue is full")
	}
	return id, nil
}

// Get returns the current persisted state of a Generation.
func (g *Generator) Get(id int64) (Generation, error) {
	return g.store.GetGeneration(id)
}

// run executes the four-step recipe for one queued id. Errors are
// logged and folded into the persisted Generation; run never returns
// them.
func (g *Generator) run(id int64) {
	gen, err := g.store.GetGeneration(id)
	if err != nil {
		log.Printf("pipeline: load generation %d: %v", id, err)
		return
	}

	// Step 1: Caption (non-fatal on failure).
	cap, capErr := g.captions.Write(gen.Prompt)
	if capErr != nil {
		log.Printf("pipeline: caption write for %d failed (continuing): %v", id, capErr)
		cap = Caption{}
	}

	// Step 2: System prompt for image generation.
	systemPrompt, err := g.store.GetSetting(SystemPromptKey)
	if err != nil {
		log.Printf("pipeline: read system prompt: %v", err)
		systemPrompt = ""
	}

	// Step 3: Image (terminal on failure).
	filename, err := g.images.Generate(gen.Prompt, systemPrompt)
	if err != nil {
		log.Printf("pipeline: image generate for %d failed: %v", id, err)
		_ = g.store.UpdateGenerationStatus(id, StatusFailed, "", err.Error())
		return
	}

	// Step 4: Render caption onto the image (non-fatal on failure).
	if !cap.Empty() {
		if err := caption.Render(filepath.Join(g.imageDir, filename), cap.Top, cap.Bottom); err != nil {
			log.Printf("pipeline: caption render for %d failed (continuing): %v", id, err)
		}
	}

	if err := g.store.UpdateGenerationStatus(id, StatusSuccess, filename, ""); err != nil {
		log.Printf("pipeline: update status for %d: %v", id, err)
	}
	if capErr == nil && !cap.Empty() {
		if err := g.store.UpdateGenerationCaption(id, cap); err != nil {
			log.Printf("pipeline: update caption for %d: %v", id, err)
		}
	}
}
