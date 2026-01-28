package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ravenbot/internal/agent"
	"ravenbot/internal/config"
	"ravenbot/internal/db"
	"ravenbot/internal/notifier"

	"github.com/raythurman2386/cronlib"
)

func main() {
	// Root context for the application
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Database
	database, err := db.InitDB("data/ravenbot.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	bot, err := agent.NewAgent(ctx, cfg, database)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	scheduler := cronlib.NewCron()

	// Initialize Notifiers
	var notifiers []notifier.Notifier

	if cfg.TelegramBotToken != "" && cfg.TelegramChatID != 0 {
		tn, err := notifier.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramChatID)
		if err != nil {
			log.Printf("Warning: Failed to setup Telegram notifier: %v", err)
		} else {
			notifiers = append(notifiers, tn)
		}
	}

	if cfg.DiscordBotToken != "" && cfg.DiscordChannelID != "" {
		dn, err := notifier.NewDiscordNotifier(cfg.DiscordBotToken, cfg.DiscordChannelID)
		if err != nil {
			log.Printf("Warning: Failed to setup Discord notifier: %v", err)
		} else {
			notifiers = append(notifiers, dn)
		}
	}

	// Daily Mission at 6:00 AM
	missionFunc := func(ctx context.Context) {
		log.Println("Starting scheduled mission...")
		prompt := "Research the latest technical news in Golang, Python, Geospatial Engineering, and AI/LLM. Generate a detailed daily briefing in Markdown format."

		report, err := bot.RunMission(ctx, prompt)
		if err != nil {
			log.Printf("Mission failed: %v", err)
			return
		}

		path, err := agent.SaveReport("daily_logs", report)
		if err != nil {
			log.Printf("Failed to save report: %v", err)
			return
		}

		log.Printf("Mission completed. Report saved to: %s", path)

		// Send to all enabled notifiers
		for _, n := range notifiers {
			if err := n.Send(ctx, report); err != nil {
				log.Printf("Failed to send report to %s: %v", n.Name(), err)
			} else {
				log.Printf("Report sent to %s", n.Name())
			}
		}
	}

	// Schedule the job: 0 0 6 * * * (6:00:00 AM Daily)
	_, err = scheduler.AddJobWithOptions("0 0 6 * * *", missionFunc, cronlib.JobOptions{
		Overlap: cronlib.OverlapForbid, // Skip if previous one is still running
	})
	if err != nil {
		log.Fatalf("Failed to schedule mission: %v", err)
	}

	scheduler.Start()
	log.Println("RavenBot started. Scheduled mission at 06:00 Daily.")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down RavenBot...")
	scheduler.Stop()
	cancel()
	log.Println("RavenBot stopped gracefully.")
}
