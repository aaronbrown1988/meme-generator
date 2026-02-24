# 🎨 AI Meme Generator

A web application that generates memes using AI-powered image generation via Ollama. Built with Go, HTMX, and SQLite for a minimal-JavaScript experience.

## Features

- 🤖 AI-powered image generation using Ollama (flux-klein model)
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
   
   # Pull the flux2-klein model
   ollama pull x/flux2-klein
   ```

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
2. **Processing**: The server calls `ollama run x/flux-klein` with the prompt
3. **Generation**: Ollama generates the image and saves it to the current directory with a descriptive filename
4. **Detection**: The app parses Ollama's output to find "Image saved to: <filename>" and extracts the filename
5. **Storage**: Image is moved from CWD to the `generated/` directory, metadata stored in SQLite
6. **Display**: HTMX updates the page and displays the image when ready

## Technology Stack

- **Backend**: Go (net/http, html/template, database/sql)
- **Database**: SQLite (via modernc.org/sqlite)
- **Frontend**: HTMX for dynamic updates
- **Styling**: Pico.css for clean, classless CSS
- **AI**: Ollama with flux-klein model

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
- Verify the model is available: `ollama list`
- Pull the model if needed: `ollama pull x/flux-klein`

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
