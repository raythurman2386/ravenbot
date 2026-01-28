# ravenbot

ravenbot is an autonomous technical research agent built in Go 1.25+. It leverages Google's Gemini 3 Pro model to proactively research technical topics, scrape the web, and deliver daily briefings via Telegram and Discord. It is designed to be self-hosted and persistent, capable of remembering past findings and executing complex tasks.

## Project Overview

-   **Language:** Go (1.25+)
-   **AI Engine:** Google Gemini 3 Pro (`google.golang.org/genai`)
-   **Architecture:** Modular agent with tool use, scheduling, and persistence.
-   **Deployment:** Docker & Docker Compose (Optimized for ARM64/Raspberry Pi 5).

## Key Features

-   **Autonomous Research:** Browses the web using `chromedp` and scrapes content with `goquery`.
-   **Multi-Channel Notifications:** Sends briefings to Telegram and Discord.
-   **Interactive:** Responds to `/research` and `/jules` commands via chat or CLI.
-   **Persistence:** Uses SQLite to store headlines and prevent duplicate reporting.
-   **Scheduling:** Automated daily briefings via `cronlib`.

## Building and Running

### Prerequisites

-   Go 1.25+
-   Docker & Docker Compose
-   Google Gemini API Key
-   (Optional) Telegram/Discord Bot Tokens

### Common Commands (Makefile)

-   **Build:** `make build` - Compiles the `ravenbot` binary.
-   **Test:** `make test` - Runs all tests.
-   **Lint:** `make lint` - Runs `golangci-lint`.
-   **Format:** `make fmt` - Formats code.
-   **Clean:** `make clean` - Removes build artifacts.

### Docker Deployment

To start the agent in a container:

```bash
docker compose up -d --build
```

To interact with the running agent via CLI:

```bash
docker attach ravenbot-ravenbot-1
```

## Project Structure

-   `cmd/bot/`: Application entry point (`main.go`).
-   `internal/agent/`: Core agent logic, including the Gemini client and persona.
-   `internal/config/`: Configuration loading (env vars).
-   `internal/db/`: SQLite database interactions for persistence.
-   `internal/notifier/`: Telegram and Discord integration.
-   `internal/tools/`: Tool implementations:
    -   `browser.go`: Headless browser control (`chromedp`).
    -   `rss.go`: RSS feed fetching (`gofeed`).
    -   `scraper.go`: HTML content extraction (`goquery`).
    -   `shell.go`: Safe system command execution.
    -   `jules.go`: Integration with the Jules Agent API.
-   `daily_logs/`: Directory where generated markdown reports are saved.

## Development Conventions

-   **Code Style:** Follows standard Go conventions. Use `make fmt` and `make lint` before committing.
-   **Configuration:** All configuration is managed via environment variables (see `.env.example`).
-   **Testing:** Unit tests are located alongside the code (e.g., `_test.go` files). Use `testify` for assertions.
-   **Dependencies:** Managed via `go.mod`.
