package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/raythurman2386/ravenbot/internal/agent"
	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/notifier"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/raythurman2386/cronlib"
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

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

	// Daily Mission at 7:00 AM
	missionFunc := func(ctx context.Context) {
		log.Println("Starting scheduled mission...")
		today := time.Now().Format("Monday, January 2, 2006")
		prompt := fmt.Sprintf("Today is %s. Research the latest technical news in Golang, Python, Geospatial Engineering, and AI/LLM. "+
			"IMPORTANT: potentially use search queries that include the date or 'last 24 hours' to ensure you find news from today or yesterday. "+
			"Do not report on old news. Generate a detailed daily briefing in Markdown format.", today)

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

		for _, n := range notifiers {
			if err := n.Send(ctx, report); err != nil {
				log.Printf("Failed to send report to %s: %v", n.Name(), err)
			} else {
				log.Printf("Report sent to %s", n.Name())
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
				log.Printf("Failed to save briefing: %v", err)
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
						log.Printf("Failed to send Telegram reply: %v", err)
					}
				})
			})
		case *notifier.DiscordNotifier:
			go botNotifier.StartListener(ctx, func(channelID string, text string) {
				sessionID := fmt.Sprintf("discord-%s", channelID)
				handleMessage(sessionID, text, func(reply string) {
					if err := botNotifier.Send(ctx, reply); err != nil {
						log.Printf("Failed to send Discord reply: %v", err)
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
		log.Fatalf("Failed to schedule mission: %v", err)
	}

	scheduler.Start()
	log.Printf("ravenbot started. Time: %s", time.Now().Format("15:04:05"))
	log.Println("Scheduled daily briefing at 07:00")

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down ravenbot...")
	scheduler.Stop()
	cancel()
	log.Println("ravenbot stopped gracefully.")
}
