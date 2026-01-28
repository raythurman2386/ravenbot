# RavenBot 🦅

RavenBot is a self-hosted, autonomous AI Research Agent built in Go (1.25+). It monitors technical news across various domains and generates a daily briefing using Gemini 3.0 Flash.

## 🚀 Features

- **Autonomous Research**: Uses Gemini 3.0 Flash with native function calling to research topics.
- **Smart Tools**: Equipped with RSS fetchers and web scrapers to gather real-time data.
- **Scheduled Briefings**: Uses `CronLib` to generate and save a daily technical newsletter at 6:00 AM.
- **Dockerized**: Optimized for deployment on Raspberry Pi 5 (ARM64) using multi-stage builds.

## 🛠 Tech Stack

- **Language**: Go 1.25
- **AI SDK**: `google.golang.org/genai` (Gemini 3.0 Flash)
- **Scheduler**: `github.com/raythurman2386/cronlib`
- **Data Source**: Custom RSS and Web Scraping tools
- **Infrastructure**: Docker & Docker Compose (ARM64 support)

## 📋 Prerequisites

- Go 1.25+
- Docker & Docker Compose
- Google Gemini API Key

## 🛠 Setup

### 1. Clone the repository
```bash
git clone https://github.com/raythurman2386/RavenBot.git
cd RavenBot
```

### 2. Configure Environment Variables
Create a `.env` file based on the example:
```bash
cp .env.example .env
```
Edit `.env` and add your `GEMINI_API_KEY`.

### 3. Run Locally
```bash
export GEMINI_API_KEY=your_key_here
go run cmd/bot/main.go
```

### 4. Deploy with Docker
```bash
docker-compose up -d
```

## 📁 Project Structure

- `cmd/bot/`: Application entry point.
- `internal/agent/`: Gemini agent logic and function calling loop.
- `internal/tools/`: Scraper and RSS utilities.
- `internal/config/`: Environment variable management.
- `daily_logs/`: Directory where daily Markdown reports are saved.

## 📜 License
MIT
