package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ravenbot/internal/agent"
	"ravenbot/internal/config"

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

	bot, err := agent.NewAgent(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	scheduler := cronlib.NewCron()

	// Daily Mission at 6:00 AM
	missionFunc := func(ctx context.Context) {
		log.Println("Starting scheduled mission...")
		prompt := "Research the latest technical news in Golang, Python, Geospatial Engineering, and AI/LLM. Generate a detailed daily briefing in Markdown format."

		report, err := bot.RunMission(ctx, prompt)
		if err != nil {
			log.Printf("Mission failed: %v", err)
			return
		}

		path, err := agent.SaveReport(report)
		if err != nil {
			log.Printf("Failed to save report: %v", err)
			return
		}

		log.Printf("Mission completed. Report saved to: %s", path)
	}

	// Schedule the job: 0 6 * * * (6:00 AM Daily)
	_, err = scheduler.AddJobWithOptions("0 6 * * *", missionFunc, cronlib.JobOptions{
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
