package handlers

import (
	"fmt"
	"html/template"
	"log"
	"meme-generator/internal/db"
	"meme-generator/internal/ollama"
	"net/http"
	"path/filepath"
	"strconv"
)

type Handler struct {
	db       *db.DB
	ollama   *ollama.Client
	tmpl     *template.Template
	imageDir string
}

func New(database *db.DB, ollamaClient *ollama.Client, templatesDir, imageDir string) (*Handler, error) {
	tmpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	partials, err := template.ParseGlob(filepath.Join(templatesDir, "partials", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse partial templates: %w", err)
	}

	for _, t := range partials.Templates() {
		tmpl.AddParseTree(t.Name(), t.Tree)
	}

	return &Handler{
		db:       database,
		ollama:   ollamaClient,
		tmpl:     tmpl,
		imageDir: imageDir,
	}, nil
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	generations, err := h.db.ListGenerations(10)
	if err != nil {
		log.Printf("Error fetching generations: %v", err)
		generations = []db.Generation{}
	}

	data := map[string]interface{}{
		"Generations": generations,
	}

	if err := h.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prompt := r.FormValue("prompt")
	if prompt == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	id, err := h.db.InsertGeneration(prompt, "", db.StatusProcessing, "")
	if err != nil {
		log.Printf("Error inserting generation: %v", err)
		http.Error(w, "Failed to create generation", http.StatusInternalServerError)
		return
	}

	systemPrompt, err := h.db.GetSetting("system_prompt")
	if err != nil {
		log.Printf("Error fetching system prompt: %v", err)
		systemPrompt = ""
	}

	filename, err := h.ollama.GenerateImage(prompt, systemPrompt)
	if err != nil {
		log.Printf("Error generating image: %v", err)
		h.db.UpdateGenerationStatus(id, db.StatusFailed, "", err.Error())
		
		gen, _ := h.db.GetGeneration(id)
		data := map[string]interface{}{
			"Generation": gen,
		}
		
		w.Header().Set("Content-Type", "text/html")
		if err := h.tmpl.ExecuteTemplate(w, "image.html", data); err != nil {
			log.Printf("Error executing template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if err := h.db.UpdateGenerationStatus(id, db.StatusSuccess, filename, ""); err != nil {
		log.Printf("Error updating generation status: %v", err)
	}

	gen, err := h.db.GetGeneration(id)
	if err != nil {
		log.Printf("Error fetching generation: %v", err)
		http.Error(w, "Failed to fetch generation", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Generation": gen,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "image.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) GetGeneration(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	gen, err := h.db.GetGeneration(id)
	if err != nil {
		log.Printf("Error fetching generation: %v", err)
		http.Error(w, "Generation not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Generation": gen,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "image.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	generations, err := h.db.ListGenerations(10)
	if err != nil {
		log.Printf("Error fetching generations: %v", err)
		http.Error(w, "Failed to fetch history", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Generations": generations,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "history.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) ServeImage(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(r.URL.Path)
	imagePath := filepath.Join(h.imageDir, filename)

	http.ServeFile(w, r, imagePath)
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	systemPrompt, err := h.db.GetSetting("system_prompt")
	if err != nil {
		log.Printf("Error fetching system prompt: %v", err)
		systemPrompt = ""
	}

	data := map[string]interface{}{
		"SystemPrompt": systemPrompt,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "settings.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	systemPrompt := r.FormValue("system_prompt")

	if err := h.db.SetSetting("system_prompt", systemPrompt); err != nil {
		log.Printf("Error updating system prompt: %v", err)
		http.Error(w, "Failed to update settings", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"SystemPrompt": systemPrompt,
		"Success":      true,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.ExecuteTemplate(w, "settings.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
