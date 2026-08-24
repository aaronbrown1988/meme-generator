package main

import (
	"context"
	"log"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"

	orsdk "github.com/OpenRouterTeam/go-sdk"

	"meme-generator/internal/http"
	"meme-generator/internal/llm/ollama"
	"meme-generator/internal/llm/router"
	"meme-generator/internal/meme"
	"meme-generator/internal/store"
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
		log.Fatalf("create generated directory: %v", err)
	}

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	orClient := orsdk.New(orsdk.WithSecurity(os.Getenv("OPENROUTER_API_KEY")))

	pipeline := meme.NewGenerator(meme.Config{
		Store: st,
		Images: &router.ImageGenerator{
			Settings:   st,
			Ollama:     ollama.NewImageAdapter(generatedDir),
			OpenRouter: orClient,
			OutputDir:  generatedDir,
		},
		Captions: &router.CaptionWriter{
			Settings:   st,
			Ollama:     ollama.NewCaptionAdapter(),
			OpenRouter: orClient,
		},
		ImageDir: generatedDir,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := pipeline.Start(ctx); err != nil {
			log.Printf("pipeline: %v", err)
			cancel()
		}
	}()

	handler, err := http.New(pipeline, st, templatesDir, generatedDir)
	if err != nil {
		log.Fatalf("init handlers: %v", err)
	}

	mux := nethttp.NewServeMux()
	handler.Register(mux, staticDir)

	server := &nethttp.Server{Addr: port, Handler: mux}

	go func() {
		<-ctx.Done()
		log.Printf("shutting down")
		server.Shutdown(context.Background())
	}()

	log.Printf("server starting on http://localhost%s", port)
	if err := server.ListenAndServe(); err != nil && err != nethttp.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
