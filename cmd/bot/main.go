package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/raythurman2386/cronlib"
	"github.com/raythurman2386/ravenbot/internal/agent"
	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/notifier"
)

func runJob(ctx context.Context, job config.JobConfig, bot *agent.Agent, notifiers []notifier.Notifier) {
	slog.Info("Running scheduled job", "name", job.Name, "type", job.Type)
	switch job.Type {
	case "research":
		prompt := job.Params["prompt"]
		today := time.Now().Format("Monday, January 2, 2006")
		fullPrompt := fmt.Sprintf("Today is %s. %s", today, prompt)

		report, err := bot.RunMission(ctx, fullPrompt)
		if err != nil {
			slog.Error("Job failed", "name", job.Name, "error", err)
			return
		}

		path, err := agent.SaveReport("daily_logs", report)
		if err != nil {
			slog.Error("Failed to save report", "name", job.Name, "error", err)
			return
		}

		slog.Info("Job completed", "name", job.Name, "path", path)

		var wg sync.WaitGroup
		for _, n := range notifiers {
			wg.Add(1)
			go func(n notifier.Notifier) {
				defer wg.Done()
				if err := n.Send(ctx, report); err != nil {
					slog.Error("Failed to send report", "job", job.Name, "notifier", n.Name(), "error", err)
				} else {
					slog.Info("Report sent", "job", job.Name, "notifier", n.Name())
				}
			}(n)
		}
		wg.Wait()
	default:
		slog.Warn("Unknown job type", "type", job.Type, "name", job.Name)
	}
}

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
	defer func() {
		if err := database.Close(); err != nil {
			slog.Error("Failed to close database", "error", err)
		}
	}()

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

	// Unified message handler - handles all messages conversationally
	handleMessage := func(sessionID, text string, n notifier.Notifier, reply func(string)) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}

		// Start typing indicator if notifier is provided
		var stopTyping func()
		if n != nil {
			stopTyping = n.StartTyping(ctx)
			defer stopTyping()
		}

		// Handle special commands first
		lowerText := strings.ToLower(text)
		switch {
		case lowerText == "/help" || strings.HasPrefix(lowerText, "/help "):
			reply(cfg.Bot.HelpMessage)
			return

		case lowerText == "/status" || strings.HasPrefix(lowerText, "/status "):
			reply("🔍 Checking server health...")
			statusPrompt := cfg.Bot.StatusPrompt
			response, err := bot.Chat(ctx, sessionID, statusPrompt)
			if err != nil {
				slog.Error("Status check failed", "sessionID", sessionID, "error", err)
				reply(fmt.Sprintf("❌ Status check failed. I couldn't retrieve the system health metrics: %v", err))
				return
			}
			reply(response)
			return

		case lowerText == "/reset" || strings.HasPrefix(lowerText, "/reset "):
			bot.ClearSession(sessionID)
			reply("🔄 Conversation cleared! Let's start fresh.")
			return

		case strings.HasPrefix(lowerText, "/research "):
			topic := strings.TrimSpace(text[len("/research"):])
			if topic == "" {
				reply("Please provide a topic. Usage: `/research <topic>`")
				return
			}
			reply(fmt.Sprintf("🔬 Starting research on: **%s**...", topic))
			prompt := fmt.Sprintf("Research the following topic in depth and provide a technical report: %s", topic)
			report, err := bot.RunMission(ctx, prompt)
			if err != nil {
				slog.Error("Research failed", "topic", topic, "error", err)
				reply(fmt.Sprintf("❌ Research failed. I couldn't complete the research mission: %v", err))
				return
			}
			if err := database.SaveBriefing(ctx, report); err != nil {
				slog.Error("Failed to save briefing", "error", err)
			}
			reply(report)
			return

		case strings.HasPrefix(lowerText, "/jules "):
			parts := strings.Fields(text[len("/jules"):])
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
				slog.Error("Jules delegation failed", "repo", repo, "task", task, "error", err)
				reply(fmt.Sprintf("❌ Jules delegation failed. I couldn't hand off the task to Jules: %v", err))
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
				handleMessage(sessionID, text, botNotifier, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						slog.Error("Failed to send Telegram reply", "error", err)
					}
				})
			})
		case *notifier.DiscordNotifier:
			go botNotifier.StartListener(ctx, func(channelID string, text string) {
				sessionID := fmt.Sprintf("discord-%s", channelID)
				handleMessage(sessionID, text, botNotifier, func(reply string) {
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
			handleMessage(sessionID, text, nil, func(reply string) {
				fmt.Printf("\n%s\n\n> ", reply)
			})
		}
	}()

	// Schedule jobs from config
	for _, job := range cfg.Jobs {
		job := job // Capture loop variable
		_, err = scheduler.AddJobWithOptions(job.Schedule, func(ctx context.Context) {
			runJob(ctx, job, bot, notifiers)
		}, cronlib.JobOptions{
			Overlap: cronlib.OverlapForbid,
		})
		if err != nil {
			slog.Error("Failed to schedule job", "name", job.Name, "error", err)
			continue
		}
		slog.Info("Scheduled job", "name", job.Name, "schedule", job.Schedule)
	}

	scheduler.Start()
	slog.Info("ravenbot started", "time", time.Now().Format("15:04:05"))

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	slog.Info("Shutting down ravenbot...")
	scheduler.Stop()
	bot.Close()
	cancel()
	slog.Info("ravenbot stopped gracefully.")
}
