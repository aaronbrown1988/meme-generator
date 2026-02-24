# Copilot Instructions for AI Meme Generator

## Running the Application

**Start server:**
```bash
go run cmd/server/main.go
```

**Build:**
```bash
go build -o meme-server cmd/server/main.go
./meme-server
```

**Dependencies:**
```bash
go mod tidy
```

## Prerequisites

- **Ollama** must be running (`ollama serve`)
- **flux2-klein model** must be installed (`ollama pull x/flux2-klein`)
- Server runs on `localhost:8080`

## Architecture

### Request Flow
1. User submits prompt → `POST /generate` creates DB record with `processing` status
2. Handler calls `ollama.Client.GenerateImage()` which shells out to `ollama run x/flux2-klein`
3. Ollama saves image to CWD with descriptive filename (e.g., `a-cat-wearing-a-hat-20260224.png`)
4. Ollama output parser extracts filename from `"Image saved to: <filename>"` line
5. Image moved from CWD → `generated/` directory
6. DB updated with final status (`success`/`failed`) and filename
7. HTMX renders response without page reload

### Key Components
- **cmd/server/main.go**: Entry point, wires dependencies, defines routes
- **internal/handlers**: HTTP handlers, template rendering
- **internal/ollama**: Ollama CLI wrapper, filename extraction, file management
- **internal/db**: SQLite operations for generations + settings
- **web/templates**: HTML templates (index.html + partials/)
- **generated/**: Runtime directory for AI-generated images

### Database Schema
**generations table:**
- `id`, `prompt`, `image_path`, `status` (processing/success/failed), `error_message`, `created_at`

**settings table:**
- Key-value store for `system_prompt` (prepended to user prompts)

## Code Conventions

### Error Handling
- Database/Ollama errors are logged but don't crash the app
- Failed generations update DB with error message and return partial HTML with error state
- HTTP handlers return 500s for unexpected errors after logging

### Template Pattern
- Main template: `index.html`
- Partials: `image.html` (single generation), `history.html` (list), `settings.html`
- Handlers execute templates with map[string]interface{} data
- HTMX swaps partial HTML responses into DOM

### File Organization
- Never put generated images in `static/` (that's for CSS only)
- Images go to `generated/` and served via `/images/` route
- Ollama generates to CWD, then moved to `generated/` immediately

### Ollama Integration
- Command: `ollama run x/flux2-klein "<prompt>"`
- System prompt (if set) prepended to user prompt with double newline
- Output parsing is brittle: depends on exact "Image saved to:" format
- Generated filename format: `<descriptive-name>-YYYYMMDD-HHMMSS.png`
