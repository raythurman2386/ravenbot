# 🦅 RavenBot: MVP Roadmap (2026)

**Goal:** Transform RavenBot into a proactive, multi-channel AI Assistant similar to ClawdBot/Moltbot, but built entirely in Go.

---

## ✅ Completed Foundations
- [x] **Skeleton**: Go 1.25 project structure.
- [x] **Brain**: Gemini 3.0 Flash with Function Calling.
- [x] **Tools**: Scraping & RSS foundations.
- [x] **Heartbeat**: CronLib daily scheduling (06:00 AM).
- [x] **Deployment**: Docker/Compose for ARM64 (Raspberry Pi 5).

---

## 🚀 Phase 7: Multi-Channel Messaging (MVP Core)
*Objective: Get RavenBot out of log files and into your pocket.*

- [x] **Notifier Interface**: Define a generic `Notifier` interface to support multiple platforms.
- [x] **Telegram Integration**:
    - [x] Integrate `go-telegram-bot-api`.
    - [x] Implement `TelegramNotifier` to send daily Markdown briefings.
    - [x] Add `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID` to config.
- [x] **Discord Integration**:
    - [x] Integrate `discordgo`.
    - [x] Implement `DiscordNotifier`.
    - [x] Add `DISCORD_BOT_TOKEN` and `DISCORD_CHANNEL_ID` to config.

---

## 🧠 Phase 8: Persistence & Long-Term Memory
*Objective: Prevent duplicate news and allow for "trend" analysis.*

- [ ] **Memory Store**:
    - [ ] Implement a lightweight persistence layer (SQLite or JSON-based).
    - [ ] Track "Latest Headlines" to ensure the daily briefing only contains new information.
- [ ] **RAG Foundation**:
    - [ ] Store past briefings in a local knowledge base.
    - [ ] Allow the agent to reference "Yesterday's findings" in today's report.

---

## 💬 Phase 9: Interactive Agent (Two-Way Comms)
*Objective: Allow for ad-hoc research requests via messaging apps.*

- [ ] **Command Handler**:
    - [ ] Enable Telegram/Discord message listeners.
    - [ ] Handle `/research <topic>` command to trigger `RunMission` on demand.
- [ ] **Interactive Tools**: 
    - [ ] Allow the agent to ask clarifying questions via message if a search is too broad.

---

## 🛠 Phase 10: System & Web Power (Advanced "Clawd" Skills)
*Objective: Give the agent "hands" to interact with the host and web.*

- [ ] **Executor Tool**: Implement a restricted `ShellExecute` tool for system stats and basic tasks.
- [ ] **Web Pilot**: Integrate `chromedp` to allow the agent to log into sites or handle dynamic JS-heavy research.

---

## 📦 MVP Release Target
- [x] **Daily Briefing** delivered to Telegram/Discord.
- [ ] **One-Shot Research** triggered via chat command.
- [ ] **Persistent Memory** of the last 7 days of news.
