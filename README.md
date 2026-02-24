# 🎨 AI Meme Generator

A web application that generates memes using AI-powered image generation via Ollama. Built with Go, HTMX, and SQLite for a minimal-JavaScript experience.

## Features

- 🤖 AI-powered image generation using Ollama (flux2-klein model)
- 📝 AI-powered meme text generation using Ollama (gemma3:270m model)
- ✨ Automatic text overlay with dynamic sizing to fit image width
- ⚡ Real-time updates with HTMX (no page reloads)
- 📊 Generation history tracking
- 💾 SQLite database for persistent storage
- 🎨 Clean, responsive UI with Pico.css
- 🚀 Minimal JavaScript footprint

## Prerequisites

Before running this application, make sure you have:

1. **Go** (version 1.21 or higher)
   ```bash
   go version
   ```

2. **Ollama** installed and running
   ```bash
   # Install Ollama (if not already installed)
   # Visit: https://ollama.ai/download
   
   # Pull the required models
   ollama pull x/flux2-klein    # For image generation
   ollama pull gemma3:270m      # For meme text generation
   ```

3. **Impact Font** (optional, for classic meme styling)
   - Download Impact.ttf and place in `assets/fonts/Impact.ttf`
   - Falls back to embedded Go Mono Bold if not present

## Installation

1. Clone or navigate to the project directory:
   ```bash
   cd Meme_Generator
   ```

2. Install Go dependencies:
   ```bash
   go mod tidy
   ```

3. Ensure the Ollama service is running:
   ```bash
   ollama serve
   ```

## Running the Application

1. Start the server:
   ```bash
   go run cmd/server/main.go
   ```

2. Open your browser and navigate to:
   ```
   http://localhost:8080
   ```

3. Enter a prompt describing the meme you want to generate and click "Generate Meme"

## Project Structure

```
meme-generator/
├── cmd/
│   └── server/
│       └── main.go           # Application entry point
├── internal/
│   ├── db/
│   │   ├── db.go            # Database operations
│   │   └── models.go        # Data models
│   ├── handlers/
│   │   └── handlers.go      # HTTP handlers
│   └── ollama/
│       └── ollama.go        # Ollama client integration
├── web/
│   └── templates/
│       ├── index.html       # Main page template
│       └── partials/
│           ├── image.html   # Generated image display
│           └── history.html # History list
├── static/
│   └── style.css            # Custom styles
├── generated/               # Generated images storage
├── go.mod
├── go.sum
└── README.md
```

## How It Works

1. **User Input**: User enters a text prompt describing their desired meme
2. **Text Generation**: The server calls `ollama run gemma3:270m` to generate top and bottom meme text in JSON format
3. **Image Generation**: The server calls `ollama run x/flux2-klein` with the prompt to generate the base image
4. **Text Overlay**: The app overlays the generated text on the image with:
   - Dynamic font sizing based on text length and image width
   - Classic meme styling (white text with black outline, uppercase)
   - Automatic scaling to ensure text fits within 90% of image width
5. **Detection**: The app parses Ollama's output to find "Image saved to: <filename>" and extracts the filename
6. **Storage**: Image is moved from CWD to the `generated/` directory, metadata stored in SQLite
7. **Display**: HTMX updates the page and displays the meme with text overlay
8. **Graceful Degradation**: If text generation fails, displays the image without text overlay

## Technology Stack

- **Backend**: Go (net/http, html/template, database/sql)
- **Database**: SQLite (via modernc.org/sqlite)
- **Frontend**: HTMX for dynamic updates
- **Styling**: Pico.css for clean, classless CSS
- **AI**: Ollama with flux2-klein (image generation) and gemma3:270m (text generation)
- **Image Processing**: fogleman/gg for text overlay and rendering

## Configuration

Default configuration values in `cmd/server/main.go`:

- Port: `8080`
- Database: `meme_generator.db`
- Generated images directory: `generated/`
- Templates directory: `web/templates/`
- Static files directory: `static/`

## API Endpoints

- `GET /` - Main page
- `POST /generate` - Generate new meme (accepts `prompt` form data)
- `GET /generation?id={id}` - Get generation status
- `GET /history` - Get recent generations
- `GET /images/{filename}` - Serve generated images
- `GET /static/*` - Serve static files

## Troubleshooting

### Ollama command fails
- Ensure Ollama is installed and running: `ollama serve`
- Verify the models are available: `ollama list`
- Pull the models if needed:
  ```bash
  ollama pull x/flux2-klein
  ollama pull gemma3:270m
  ```

### Text not appearing on images
- Check that gemma3:270m model is installed: `ollama list`
- Text generation failures are logged but don't prevent image generation
- Check server logs for "Warning: Text generation failed" messages

### Port already in use
- Change the port in `cmd/server/main.go` (default: 8080)

### Database errors
- Delete `meme_generator.db` and restart to recreate the database

## Development

Build the application:
```bash
go build -o meme-server cmd/server/main.go
./meme-server
```

Run with live reload (using tools like `air`):
```bash
go install github.com/cosmtrek/air@latest
air
```

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
