// Package http contains the HTTP adapter into the meme Pipeline. It is
// deliberately thin: parse the request, call the Pipeline, render a
// template.
package http

import (
	"fmt"
	"html/template"
	"log"
	nethttp "net/http"
	"path/filepath"
	"strconv"

	"meme-generator/internal/meme"
	"meme-generator/internal/store"
)

type Handler struct {
	pipeline *meme.Generator
	store    *store.Store
	tmpl     *template.Template
	imageDir string
}

func New(pipeline *meme.Generator, st *store.Store, templatesDir, imageDir string) (*Handler, error) {
	tmpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	partials, err := template.ParseGlob(filepath.Join(templatesDir, "partials", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("parse partial templates: %w", err)
	}
	for _, t := range partials.Templates() {
		tmpl.AddParseTree(t.Name(), t.Tree)
	}

	return &Handler{
		pipeline: pipeline,
		store:    st,
		tmpl:     tmpl,
		imageDir: imageDir,
	}, nil
}

func (h *Handler) Register(mux *nethttp.ServeMux, staticDir string) {
	mux.HandleFunc("/", h.home)
	mux.HandleFunc("/generate", h.generate)
	mux.HandleFunc("/generation", h.getGeneration)
	mux.HandleFunc("/history", h.history)
	mux.HandleFunc("/settings", h.getSettings)
	mux.HandleFunc("/settings/update", h.updateSettings)
	mux.Handle("/images/", nethttp.StripPrefix("/images/", nethttp.HandlerFunc(h.serveImage)))
	mux.Handle("/static/", nethttp.StripPrefix("/static/", nethttp.FileServer(nethttp.Dir(staticDir))))
}

func (h *Handler) home(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.URL.Path != "/" {
		nethttp.NotFound(w, r)
		return
	}
	gens, err := h.store.ListGenerations(10)
	if err != nil {
		log.Printf("home: list generations: %v", err)
		gens = nil
	}
	h.render(w, "index.html", map[string]interface{}{"Generations": gens})
}

func (h *Handler) generate(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodPost {
		nethttp.Error(w, "Method not allowed", nethttp.StatusMethodNotAllowed)
		return
	}
	prompt := r.FormValue("prompt")
	if prompt == "" {
		nethttp.Error(w, "Prompt is required", nethttp.StatusBadRequest)
		return
	}

	id, err := h.pipeline.Enqueue(prompt)
	if err != nil {
		log.Printf("generate: enqueue: %v", err)
		nethttp.Error(w, "Failed to enqueue generation", nethttp.StatusInternalServerError)
		return
	}

	gen, err := h.pipeline.Get(id)
	if err != nil {
		log.Printf("generate: load enqueued generation %d: %v", id, err)
		nethttp.Error(w, "Failed to load generation", nethttp.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	h.render(w, "image.html", map[string]interface{}{"Generation": gen})
}

func (h *Handler) getGeneration(w nethttp.ResponseWriter, r *nethttp.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		nethttp.Error(w, "ID is required", nethttp.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		nethttp.Error(w, "Invalid ID", nethttp.StatusBadRequest)
		return
	}
	gen, err := h.pipeline.Get(id)
	if err != nil {
		log.Printf("getGeneration: %v", err)
		nethttp.Error(w, "Generation not found", nethttp.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	h.render(w, "image.html", map[string]interface{}{"Generation": gen})
}

func (h *Handler) history(w nethttp.ResponseWriter, r *nethttp.Request) {
	gens, err := h.store.ListGenerations(10)
	if err != nil {
		log.Printf("history: list generations: %v", err)
		nethttp.Error(w, "Failed to fetch history", nethttp.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	h.render(w, "history.html", map[string]interface{}{"Generations": gens})
}

func (h *Handler) serveImage(w nethttp.ResponseWriter, r *nethttp.Request) {
	filename := filepath.Base(r.URL.Path)
	nethttp.ServeFile(w, r, filepath.Join(h.imageDir, filename))
}

func (h *Handler) getSettings(w nethttp.ResponseWriter, r *nethttp.Request) {
	systemPrompt, err := h.store.GetSetting(meme.SystemPromptKey)
	if err != nil {
		log.Printf("getSettings: %v", err)
		systemPrompt = ""
	}
	w.Header().Set("Content-Type", "text/html")
	h.render(w, "settings.html", map[string]interface{}{"SystemPrompt": systemPrompt})
}

func (h *Handler) updateSettings(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodPost {
		nethttp.Error(w, "Method not allowed", nethttp.StatusMethodNotAllowed)
		return
	}
	systemPrompt := r.FormValue("system_prompt")
	if err := h.store.SetSetting(meme.SystemPromptKey, systemPrompt); err != nil {
		log.Printf("updateSettings: %v", err)
		nethttp.Error(w, "Failed to update settings", nethttp.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	h.render(w, "settings.html", map[string]interface{}{
		"SystemPrompt": systemPrompt,
		"Success":      true,
	})
}

func (h *Handler) render(w nethttp.ResponseWriter, name string, data interface{}) {
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		nethttp.Error(w, "Internal Server Error", nethttp.StatusInternalServerError)
	}
}
