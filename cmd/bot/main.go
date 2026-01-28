package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"ravenbot/internal/agent"
	"ravenbot/internal/config"
	"ravenbot/internal/db"
	"ravenbot/internal/notifier"
	"strings"
	"syscall"
	"time"

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

	// Command Processor
	processCommand := func(text string, reply func(string)) {
		if !strings.HasPrefix(text, "/research") {
			return
		}

		topic := strings.TrimSpace(strings.TrimPrefix(text, "/research"))
		if topic == "" {
			reply("Please provide a topic. Usage: /research <topic>")
			return
		}

		reply(fmt.Sprintf("Starting research mission for: %s...", topic))
		prompt := fmt.Sprintf("Research the following topic in depth and provide a technical report: %s", topic)

		report, err := bot.RunMission(ctx, prompt)
		if err != nil {
			reply(fmt.Sprintf("Research mission failed: %v", err))
			return
		}

		// Save briefing to DB
		if err := database.SaveBriefing(ctx, report); err != nil {
			log.Printf("Failed to save interactive briefing to DB: %v", err)
		}

		reply(report)
	}

	// Jules Command Processor
	processJules := func(text string, reply func(string)) {
		if !strings.HasPrefix(text, "/jules") {
			return
		}

		parts := strings.Fields(strings.TrimPrefix(text, "/jules"))
		if len(parts) < 2 {
			reply("Usage: /jules <owner/repo> <task description>")
			return
		}

		repo := parts[0]
		task := strings.Join(parts[1:], " ")

		reply(fmt.Sprintf("Delegating task to Jules for %s: %s", repo, task))

		// We use the agent's HandleToolCall logic but triggered directly
		// However, it's cleaner to just call the tool directly here or let the agent handle it.
		// For now, let's keep it consistent with the agent's tool set.
		// Actually, let's just use the agent to handle it so it can reason about the response.
		prompt := fmt.Sprintf("Delegate the following task to Jules for the repository %s: %s", repo, task)
		res, err := bot.RunMission(ctx, prompt)
		if err != nil {
			reply(fmt.Sprintf("Jules delegation failed: %v", err))
			return
		}
		reply(res)
	}

	// Start Notifier Listeners
	for _, n := range notifiers {
		switch botNotifier := n.(type) {
		case *notifier.TelegramNotifier:
			go botNotifier.StartListener(ctx, func(chatID int64, text string) {
				processCommand(text, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						log.Printf("Failed to send Telegram reply: %v", err)
					}
				})
				processJules(text, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						log.Printf("Failed to send Telegram reply: %v", err)
					}
				})
			})
		case *notifier.DiscordNotifier:
			go botNotifier.StartListener(ctx, func(channelID string, text string) {
				processCommand(text, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						log.Printf("Failed to send Discord reply: %v", err)
					}
				})
				processJules(text, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						log.Printf("Failed to send Discord reply: %v", err)
					}
				})
			})
		}
	}

	// CLI Listener (stdin)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		log.Println("CLI Listener active. Type /research <topic> to test.")
		for scanner.Scan() {
			text := scanner.Text()
			processCommand(text, func(reply string) {
				log.Printf("\n--- RavenBot Response ---\n%s\n------------------------\n", reply)
			})
			processJules(text, func(reply string) {
				log.Printf("\n--- Jules Delegation Response ---\n%s\n------------------------\n", reply)
			})
		}
	}()

	// Schedule the job: 0 50 20 * * * (20:50:00 Daily)
	_, err = scheduler.AddJobWithOptions("0 50 20 * * *", missionFunc, cronlib.JobOptions{
		Overlap: cronlib.OverlapForbid, // Skip if previous one is still running
	})
	if err != nil {
		log.Fatalf("Failed to schedule mission: %v", err)
	}

	scheduler.Start()
	log.Printf("RavenBot started in %s. Current time: %s", time.Local, time.Now().Format("15:04:05"))
	log.Println("Scheduled mission at 20:50 Daily.")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down RavenBot...")
	scheduler.Stop()
	cancel()
	log.Println("RavenBot stopped gracefully.")
}
