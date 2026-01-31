package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/raythurman2386/cronlib"
	"github.com/raythurman2386/ravenbot/internal/agent"
	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/notifier"
)

const helpMessage = `🐦 **ravenbot Commands**

**Conversation:**
Just type naturally! I can chat about anything.

**Commands:**
• **/research <topic>** - Deep dive research on any topic
• **/jules <owner/repo> <task>** - Delegate coding task to Jules AI
• **/status** - Check server health
• **/reset** - Clear conversation history
• **/help** - Show this message

**Examples:**
• "What's new in Go 1.25?"
• "/research kubernetes best practices"
• "/jules raythurman2386/ravenbot add unit tests"
• "/status"
`

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	database, err := db.InitDB("data/ravenbot.db")
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	bot, err := agent.NewAgent(ctx, cfg, database)
	if err != nil {
		slog.Error("Failed to create agent", "error", err)
		os.Exit(1)
	}

	scheduler := cronlib.NewCron()

	// Initialize Notifiers
	var notifiers []notifier.Notifier

	if cfg.TelegramBotToken != "" && cfg.TelegramChatID != 0 {
		tn, err := notifier.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramChatID)
		if err != nil {
			slog.Warn("Failed to setup Telegram notifier", "error", err)
		} else {
			notifiers = append(notifiers, tn)
		}
	}

	if cfg.DiscordBotToken != "" && cfg.DiscordChannelID != "" {
		dn, err := notifier.NewDiscordNotifier(cfg.DiscordBotToken, cfg.DiscordChannelID)
		if err != nil {
			slog.Warn("Failed to setup Discord notifier", "error", err)
		} else {
			notifiers = append(notifiers, dn)
		}
	}

	// Daily Mission at 7:00 AM
	missionFunc := func(ctx context.Context) {
		slog.Info("Starting scheduled mission")
		today := time.Now().Format("Monday, January 2, 2006")
		prompt := fmt.Sprintf("Today is %s. Access your memory to see if there are any specific technologies, projects, or interests the user has previously mentioned. "+
			"Then, conduct research to find the most important technical news from the **past 24 hours** in Golang, Python, Geospatial Engineering, AI/LLM, and any other topics found in memory. "+
			"Use your **SearchWeb** tool with date-specific queries (e.g., 'Golang news Jan 31 2026') and check **RSS feeds** to ensure you only report on brand new developments. "+
			"Do not report on old news. Generate a detailed, personalized daily briefing in Markdown format.", today)

		report, err := bot.RunMission(ctx, prompt)
		if err != nil {
			slog.Error("Mission failed", "error", err)
			return
		}

		path, err := agent.SaveReport("daily_logs", report)
		if err != nil {
			slog.Error("Failed to save report", "error", err)
			return
		}

		slog.Info("Mission completed", "path", path)

		for _, n := range notifiers {
			if err := n.Send(ctx, report); err != nil {
				slog.Error("Failed to send report", "notifier", n.Name(), "error", err)
			} else {
				slog.Info("Report sent", "notifier", n.Name())
			}
		}
	}

	// Unified message handler - handles all messages conversationally
	handleMessage := func(sessionID, text string, reply func(string)) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}

		// Handle special commands first
		switch {
		case text == "/help":
			reply(helpMessage)
			return

		case text == "/status":
			reply("🔍 Checking server health...")
			statusPrompt := "Run system health checks using the ShellExecute tool. Check disk space (df -h), memory (free -h), and uptime. Provide a brief, friendly summary."
			response, err := bot.Chat(ctx, sessionID, statusPrompt)
			if err != nil {
				reply(fmt.Sprintf("❌ Status check failed: %v", err))
				return
			}
			reply(response)
			return

		case text == "/reset":
			bot.ClearSession(sessionID)
			reply("🔄 Conversation cleared! Let's start fresh.")
			return

		case strings.HasPrefix(text, "/research "):
			topic := strings.TrimSpace(strings.TrimPrefix(text, "/research"))
			if topic == "" {
				reply("Please provide a topic. Usage: `/research <topic>`")
				return
			}
			reply(fmt.Sprintf("🔬 Starting research on: **%s**...", topic))
			prompt := fmt.Sprintf("Research the following topic in depth and provide a technical report: %s", topic)
			report, err := bot.RunMission(ctx, prompt)
			if err != nil {
				reply(fmt.Sprintf("❌ Research failed: %v", err))
				return
			}
			if err := database.SaveBriefing(ctx, report); err != nil {
				slog.Error("Failed to save briefing", "error", err)
			}
			reply(report)
			return

		case strings.HasPrefix(text, "/jules "):
			parts := strings.Fields(strings.TrimPrefix(text, "/jules"))
			if len(parts) < 2 {
				reply("Usage: `/jules <owner/repo> <task description>`")
				return
			}
			repo := parts[0]
			task := strings.Join(parts[1:], " ")
			reply(fmt.Sprintf("🤖 Delegating to Jules for **%s**: %s", repo, task))
			prompt := fmt.Sprintf("Use the JulesTask tool to delegate this coding task to Jules for the repository %s: %s", repo, task)
			response, err := bot.Chat(ctx, sessionID, prompt)
			if err != nil {
				reply(fmt.Sprintf("❌ Jules delegation failed: %v", err))
				return
			}
			reply(response)
			return

		default:
			// General conversation - use persistent session
			response, err := bot.Chat(ctx, sessionID, text)
			if err != nil {
				reply(fmt.Sprintf("Sorry, I encountered an error: %v", err))
				return
			}
			reply(response)
		}
	}

	// Start Notifier Listeners
	for _, n := range notifiers {
		switch botNotifier := n.(type) {
		case *notifier.TelegramNotifier:
			go botNotifier.StartListener(ctx, func(chatID int64, text string) {
				sessionID := fmt.Sprintf("telegram-%d", chatID)
				handleMessage(sessionID, text, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						slog.Error("Failed to send Telegram reply", "error", err)
					}
				})
			})
		case *notifier.DiscordNotifier:
			go botNotifier.StartListener(ctx, func(channelID string, text string) {
				sessionID := fmt.Sprintf("discord-%s", channelID)
				handleMessage(sessionID, text, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						slog.Error("Failed to send Discord reply", "error", err)
					}
				})
			})
		}
	}

	// CLI Listener
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		sessionID := "cli-local"
		fmt.Println("\n🐦 ravenbot ready! Type anything to chat, or /help for commands.")
		fmt.Print("> ")
		for scanner.Scan() {
			text := scanner.Text()
			handleMessage(sessionID, text, func(reply string) {
				fmt.Printf("\n%s\n\n> ", reply)
			})
		}
	}()

	// Schedule daily mission at 07:00
	_, err = scheduler.AddJobWithOptions("0 0 7 * * *", missionFunc, cronlib.JobOptions{
		Overlap: cronlib.OverlapForbid,
	})
	if err != nil {
		slog.Error("Failed to schedule mission", "error", err)
		os.Exit(1)
	}

	scheduler.Start()
	slog.Info("ravenbot started", "time", time.Now().Format("15:04:05"))
	slog.Info("Scheduled daily briefing at 07:00")

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	slog.Info("Shutting down ravenbot...")
	scheduler.Stop()
	cancel()
	slog.Info("ravenbot stopped gracefully.")
}
