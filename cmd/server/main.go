package main

import (
	"log"
	"meme-generator/internal/db"
	"meme-generator/internal/handlers"
	"meme-generator/internal/ollama"
	"net/http"
	"os"
)

func main() {
	const (
		dbPath       = "meme_generator.db"
		generatedDir = "generated"
		templatesDir = "web/templates"
		staticDir    = "static"
		port         = ":8080"
	)

	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		log.Fatalf("Failed to create generated directory: %v", err)
	}

	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	ollamaClient := ollama.NewClient(generatedDir)

	handler, err := handlers.New(database, ollamaClient, templatesDir, generatedDir)
	if err != nil {
		log.Fatalf("Failed to initialize handlers: %v", err)
	}

	http.HandleFunc("/", handler.Home)
	http.HandleFunc("/generate", handler.Generate)
	http.HandleFunc("/generation", handler.GetGeneration)
	http.HandleFunc("/history", handler.History)
	http.HandleFunc("/settings", handler.GetSettings)
	http.HandleFunc("/settings/update", handler.UpdateSettings)
	http.Handle("/images/", http.StripPrefix("/images/", http.HandlerFunc(handler.ServeImage)))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	log.Printf("Server starting on http://localhost%s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
