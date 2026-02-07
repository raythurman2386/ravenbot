package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raythurman2386/cronlib"
	"github.com/raythurman2386/ravenbot/internal/agent"
	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/handler"
	"github.com/raythurman2386/ravenbot/internal/notifier"
	"github.com/raythurman2386/ravenbot/internal/stats"
)

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
	botStats := stats.New()

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

	// Create handler with all dependencies
	h := handler.New(bot, database, cfg, botStats, notifiers)

	// Start Notifier Listeners
	for _, n := range notifiers {
		switch botNotifier := n.(type) {
		case *notifier.TelegramNotifier:
			go botNotifier.StartListener(ctx, func(chatID int64, text string) {
				sessionID := fmt.Sprintf("telegram-%d", chatID)
				h.HandleMessage(ctx, sessionID, text, botNotifier, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						slog.Error("Failed to send Telegram reply", "error", err)
					}
				})
			})
		case *notifier.DiscordNotifier:
			go botNotifier.StartListener(ctx, func(channelID string, text string) {
				sessionID := fmt.Sprintf("discord-%s", channelID)
				h.HandleMessage(ctx, sessionID, text, botNotifier, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						slog.Error("Failed to send Discord reply", "error", err)
					}
				})
			})
		}
	}

	// Schedule jobs from config
	for _, job := range cfg.Jobs {
		job := job // Capture loop variable
		_, err = scheduler.AddJobWithOptions(job.Schedule, func(ctx context.Context) {
			h.RunJob(ctx, job)
		}, cronlib.JobOptions{
			Overlap: cronlib.OverlapForbid,
		})
		if err != nil {
			slog.Error("Failed to schedule job", "name", job.Name, "error", err)
			continue
		}
		slog.Info("Scheduled job", "name", job.Name, "schedule", job.Schedule)
	}

	// Reminder check — runs every 30 seconds via cronlib
	_, err = scheduler.AddJobWithOptions("*/30 * * * * *", func(ctx context.Context) {
		h.DeliverReminders(ctx)
	}, cronlib.JobOptions{
		Overlap: cronlib.OverlapForbid,
	})
	if err != nil {
		slog.Error("Failed to schedule reminder checker", "error", err)
	} else {
		slog.Info("Scheduled reminder checker", "schedule", "*/30 * * * * *")
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
