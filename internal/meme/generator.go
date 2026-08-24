package meme

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"meme-generator/internal/caption"
)

// ImageSystemPromptKey and CaptionSystemPromptKey are the settings keys
// the Pipeline reads to construct the system preamble for image
// generation and caption writing respectively. They are set
// independently since the two steps use different models with
// different concerns (visual description vs. joke writing).
const (
	ImageSystemPromptKey   = "system_prompt"
	CaptionSystemPromptKey = "caption_system_prompt"
)

// Settings keys and values controlling which LLM provider generates
// captions and images. ProviderKey selects between ProviderOllama
// (the local Ollama CLI) and ProviderOpenRouter (the OpenRouter API).
// The two model keys hold the model identifier to use when
// ProviderOpenRouter is selected.
const (
	ProviderKey             = "provider"
	OpenRouterModelKey      = "openrouter_model"
	OpenRouterImageModelKey = "openrouter_image_model"

	ProviderOllama     = "ollama"
	ProviderOpenRouter = "openrouter"
)

// Default OpenRouter model identifiers. These are the single source of
// truth for "what model do we use when none is configured" — the store
// migration seeds settings rows with these values, and the router
// falls back to them if a row is somehow missing or empty, so the two
// can't drift apart.
const (
	DefaultOpenRouterModel      = "openai/gpt-4o-mini"
	DefaultOpenRouterImageModel = "bytedance-seed/seedream-4.5"
)

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
	runStart := time.Now()
	log.Printf("pipeline[%d]: starting", id)

	gen, err := g.store.GetGeneration(id)
	if err != nil {
		log.Printf("pipeline[%d]: load generation: %v", id, err)
		return
	}

	// Step 1: Caption (non-fatal on failure).
	stepStart := time.Now()
	captionSystemPrompt, err := g.store.GetSetting(CaptionSystemPromptKey)
	if err != nil {
		log.Printf("pipeline[%d]: read caption system prompt: %v", id, err)
		captionSystemPrompt = ""
	}
	log.Printf("pipeline[%d]: caption-write starting", id)
	cap, capErr := g.captions.Write(gen.Prompt, captionSystemPrompt)
	if capErr != nil {
		log.Printf("pipeline[%d]: caption-write failed after %s (continuing without caption): %v", id, time.Since(stepStart).Round(time.Millisecond), capErr)
		cap = Caption{}
	} else {
		log.Printf("pipeline[%d]: caption-write succeeded in %s", id, time.Since(stepStart).Round(time.Millisecond))
	}

	// Step 2: System prompt for image generation.
	imageSystemPrompt, err := g.store.GetSetting(ImageSystemPromptKey)
	if err != nil {
		log.Printf("pipeline[%d]: read image system prompt: %v", id, err)
		imageSystemPrompt = ""
	}

	// Step 3: Image (terminal on failure).
	stepStart = time.Now()
	log.Printf("pipeline[%d]: image-generate starting", id)
	filename, err := g.images.Generate(gen.Prompt, imageSystemPrompt)
	if err != nil {
		log.Printf("pipeline[%d]: image-generate failed after %s (terminal): %v", id, time.Since(stepStart).Round(time.Millisecond), err)
		if err := g.store.UpdateGenerationStatus(id, StatusFailed, "", err.Error()); err != nil {
			log.Printf("pipeline[%d]: update status to failed: %v", id, err)
		}
		return
	}
	log.Printf("pipeline[%d]: image-generate succeeded in %s (file=%s)", id, time.Since(stepStart).Round(time.Millisecond), filename)

	// Step 4: Render caption onto the image (non-fatal on failure).
	if !cap.Empty() {
		stepStart = time.Now()
		log.Printf("pipeline[%d]: caption-render starting", id)
		if err := caption.Render(filepath.Join(g.imageDir, filename), cap.Top, cap.Bottom); err != nil {
			log.Printf("pipeline[%d]: caption-render failed after %s (continuing, image keeps no text overlay): %v", id, time.Since(stepStart).Round(time.Millisecond), err)
		} else {
			log.Printf("pipeline[%d]: caption-render succeeded in %s", id, time.Since(stepStart).Round(time.Millisecond))
		}
	}

	if err := g.store.UpdateGenerationStatus(id, StatusSuccess, filename, ""); err != nil {
		log.Printf("pipeline[%d]: update status to success: %v", id, err)
	}
	if capErr == nil && !cap.Empty() {
		if err := g.store.UpdateGenerationCaption(id, cap); err != nil {
			log.Printf("pipeline[%d]: update caption: %v", id, err)
		}
	}

	log.Printf("pipeline[%d]: finished in %s", id, time.Since(runStart).Round(time.Millisecond))
}
