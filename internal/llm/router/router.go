// Package router selects between the ollama and openrouter adapter
// families at call time, based on the provider recorded in Settings.
// It satisfies the meme.ImageGenerator and meme.CaptionWriter ports so
// the Pipeline can remain unaware that more than one provider exists.
package router

import (
	"log"

	orsdk "github.com/OpenRouterTeam/go-sdk"

	"meme-generator/internal/llm/openrouter"
	"meme-generator/internal/meme"
)

// Settings is the subset of the store the router needs to pick a
// provider and model for each call.
type Settings interface {
	GetSetting(key string) (string, error)
}

// provider reads the current provider setting, defaulting to Ollama
// when the setting is missing or unreadable.
func provider(s Settings) string {
	p, err := s.GetSetting(meme.ProviderKey)
	if err != nil || p == "" {
		return meme.ProviderOllama
	}
	return p
}

// modelOrDefault reads a model setting, falling back to def when the
// setting is missing, empty, or unreadable. The store migration seeds
// every model setting with the same default, so this fallback is only
// ever exercised against an unexpectedly missing row.
func modelOrDefault(s Settings, key, def string) string {
	model, err := s.GetSetting(key)
	if err != nil || model == "" {
		log.Printf("router: %s setting unavailable (%v), falling back to default %s", key, err, def)
		return def
	}
	return model
}

// CaptionWriter dispatches Write to the ollama adapter or an
// OpenRouter chat request, depending on the current provider setting.
type CaptionWriter struct {
	Settings   Settings
	Ollama     meme.CaptionWriter
	OpenRouter *orsdk.OpenRouter
}

func (r *CaptionWriter) Write(prompt, systemPrompt string) (meme.Caption, error) {
	if provider(r.Settings) != meme.ProviderOpenRouter {
		log.Printf("router: caption via ollama")
		return r.Ollama.Write(prompt, systemPrompt)
	}
	model := modelOrDefault(r.Settings, meme.OpenRouterModelKey, meme.DefaultOpenRouterModel)
	log.Printf("router: caption via openrouter (model=%s)", model)
	adapter := &openrouter.CaptionAdapter{Client: r.OpenRouter, Model: model}
	return adapter.Write(prompt, systemPrompt)
}

// ImageGenerator dispatches Generate to the ollama adapter or an
// OpenRouter image request, depending on the current provider setting.
type ImageGenerator struct {
	Settings   Settings
	Ollama     meme.ImageGenerator
	OpenRouter *orsdk.OpenRouter
	OutputDir  string
}

func (r *ImageGenerator) Generate(prompt, systemPrompt string) (string, error) {
	if provider(r.Settings) != meme.ProviderOpenRouter {
		log.Printf("router: image via ollama")
		return r.Ollama.Generate(prompt, systemPrompt)
	}
	model := modelOrDefault(r.Settings, meme.OpenRouterImageModelKey, meme.DefaultOpenRouterImageModel)
	log.Printf("router: image via openrouter (model=%s)", model)
	adapter := &openrouter.ImageAdapter{Client: r.OpenRouter, Model: model, OutputDir: r.OutputDir}
	return adapter.Generate(prompt, systemPrompt)
}
