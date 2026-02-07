package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/raythurman2386/ravenbot/internal/agent"
	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/notifier"
	"github.com/raythurman2386/ravenbot/internal/stats"
)

// Handler owns all message routing, command handling, and job execution.
type Handler struct {
	bot       *agent.Agent
	db        *db.DB
	cfg       *config.Config
	stats     *stats.Stats
	notifiers []notifier.Notifier

	// replies maps sessionID → reply function for reminder delivery
	replies map[string]func(string)
	mu      sync.Mutex
}

// New creates a Handler with all required dependencies.
func New(bot *agent.Agent, database *db.DB, cfg *config.Config, s *stats.Stats, notifiers []notifier.Notifier) *Handler {
	return &Handler{
		bot:       bot,
		db:        database,
		cfg:       cfg,
		stats:     s,
		notifiers: notifiers,
		replies:   make(map[string]func(string)),
	}
}

// HandleMessage is the unified entry point for all incoming messages.
// It routes commands and general conversation to the appropriate handler.
func (h *Handler) HandleMessage(ctx context.Context, sessionID, text string, n notifier.Notifier, reply func(string)) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	h.stats.RecordMessage()

	// Register reply function for reminder delivery
	h.mu.Lock()
	h.replies[sessionID] = reply
	h.mu.Unlock()

	// Start typing indicator if notifier is provided
	if n != nil {
		stopTyping := n.StartTyping(ctx)
		defer stopTyping()
	}

	lowerText := strings.ToLower(text)
	switch {
	case lowerText == "/help" || strings.HasPrefix(lowerText, "/help "):
		reply(h.cfg.Bot.HelpMessage)

	case lowerText == "/status" || strings.HasPrefix(lowerText, "/status "):
		h.handleStatus(ctx, sessionID, reply)

	case lowerText == "/reset" || strings.HasPrefix(lowerText, "/reset "):
		h.bot.ClearSession(sessionID)
		reply("🔄 Conversation cleared! Let's start fresh.")

	case lowerText == "/uptime" || strings.HasPrefix(lowerText, "/uptime "):
		reply(h.stats.Summary())

	case strings.HasPrefix(lowerText, "/remind "):
		h.handleRemind(ctx, sessionID, text, reply)

	case strings.HasPrefix(lowerText, "/export"):
		h.handleExport(ctx, text, reply)

	case strings.HasPrefix(lowerText, "/research "):
		h.handleResearch(ctx, text, reply)

	case strings.HasPrefix(lowerText, "/jules "):
		h.handleJules(ctx, sessionID, text, reply)

	default:
		h.handleChat(ctx, sessionID, text, reply)
	}
}

func (h *Handler) handleStatus(ctx context.Context, sessionID string, reply func(string)) {
	reply("🔍 Checking server health...")
	response, err := h.bot.Chat(ctx, sessionID, h.cfg.Bot.StatusPrompt)
	if err != nil {
		slog.Error("Status check failed", "sessionID", sessionID, "error", err)
		reply(fmt.Sprintf("❌ Status check failed. I couldn't retrieve the system health metrics: %v", err))
		return
	}
	reply(response)
}

func (h *Handler) handleRemind(ctx context.Context, sessionID, text string, reply func(string)) {
	args := strings.TrimSpace(text[len("/remind"):])
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		reply("Usage: `/remind <duration> <message>`\nExamples: `/remind 30m Check Docker`, `/remind 2h Review PR`")
		return
	}
	duration, err := time.ParseDuration(parts[0])
	if err != nil {
		reply(fmt.Sprintf("❌ Invalid duration `%s`. Use Go duration format: `30s`, `5m`, `2h`, `1h30m`", parts[0]))
		return
	}
	remindAt := time.Now().Add(duration)
	if err := h.db.AddReminder(ctx, sessionID, parts[1], remindAt); err != nil {
		slog.Error("Failed to add reminder", "error", err)
		reply("❌ Failed to save reminder.")
		return
	}
	reply(fmt.Sprintf("⏰ Reminder set! I'll remind you in **%s**: %s", parts[0], parts[1]))
}

func (h *Handler) handleExport(ctx context.Context, text string, reply func(string)) {
	limitStr := strings.TrimSpace(text[len("/export"):])
	limit := 5
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
			if limit > 20 {
				limit = 20
			}
		}
	}
	briefings, err := h.db.GetRecentBriefings(ctx, limit)
	if err != nil {
		slog.Error("Export failed", "error", err)
		reply("❌ Failed to retrieve briefings.")
		return
	}
	if len(briefings) == 0 {
		reply("📭 No briefings found. Run `/research <topic>` to generate one!")
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 **Exported %d Briefing(s)**\n\n", len(briefings)))
	for i, b := range briefings {
		sb.WriteString(fmt.Sprintf("---\n### Briefing %d (Created: %s)\n\n", i+1, b.CreatedAt))
		sb.WriteString(b.Content)
		sb.WriteString("\n\n")
	}
	reply(sb.String())
}

func (h *Handler) handleResearch(ctx context.Context, text string, reply func(string)) {
	topic := strings.TrimSpace(text[len("/research"):])
	if topic == "" {
		reply("Please provide a topic. Usage: `/research <topic>`")
		return
	}
	reply(fmt.Sprintf("🔬 Starting research on: **%s**...", topic))
	prompt := fmt.Sprintf("Research the following topic in depth and provide a technical report: %s", topic)
	report, err := h.bot.RunMission(ctx, prompt)
	if err != nil {
		slog.Error("Research failed", "topic", topic, "error", err)
		reply(fmt.Sprintf("❌ Research failed. I couldn't complete the research mission: %v", err))
		return
	}
	h.stats.RecordMission()
	if err := h.db.SaveBriefing(ctx, report); err != nil {
		slog.Error("Failed to save briefing", "error", err)
	}
	reply(report)
}

func (h *Handler) handleJules(ctx context.Context, sessionID, text string, reply func(string)) {
	parts := strings.Fields(text[len("/jules"):])
	if len(parts) < 2 {
		reply("Usage: `/jules <owner/repo> <task description>`")
		return
	}
	repo := parts[0]
	task := strings.Join(parts[1:], " ")
	reply(fmt.Sprintf("🤖 Delegating to Jules for **%s**: %s", repo, task))
	prompt := fmt.Sprintf("Ask the Jules agent to delegate this coding task to the external Jules service for repository %s: %s", repo, task)
	response, err := h.bot.Chat(ctx, sessionID, prompt)
	if err != nil {
		slog.Error("Jules delegation failed", "repo", repo, "task", task, "error", err)
		reply(fmt.Sprintf("❌ Jules delegation failed. I couldn't hand off the task to Jules: %v", err))
		return
	}
	reply(response)
}

func (h *Handler) handleChat(ctx context.Context, sessionID, text string, reply func(string)) {
	response, err := h.bot.Chat(ctx, sessionID, text)
	if err != nil {
		reply(fmt.Sprintf("Sorry, I encountered an error: %v", err))
		return
	}
	reply(response)
}

// RunJob executes a scheduled job (e.g., daily research briefing).
func (h *Handler) RunJob(ctx context.Context, job config.JobConfig) {
	slog.Info("Running scheduled job", "name", job.Name, "type", job.Type)
	switch job.Type {
	case "research":
		prompt := job.Params["prompt"]
		today := time.Now().Format("Monday, January 2, 2006")
		fullPrompt := fmt.Sprintf("Today is %s. %s", today, prompt)

		report, err := h.bot.RunMission(ctx, fullPrompt)
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
		h.stats.RecordMission()

		var wg sync.WaitGroup
		for _, n := range h.notifiers {
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

// DeliverReminders checks for pending reminders and delivers them.
// Intended to be called by a cronlib scheduled job.
func (h *Handler) DeliverReminders(ctx context.Context) {
	pending, err := h.db.GetPendingReminders(ctx, time.Now())
	if err != nil {
		slog.Error("Failed to check reminders", "error", err)
		return
	}
	for _, r := range pending {
		msg := fmt.Sprintf("⏰ **Reminder**: %s", r.Message)
		delivered := false

		// Try session-specific reply function first
		h.mu.Lock()
		if replyFn, ok := h.replies[r.SessionID]; ok {
			replyFn(msg)
			delivered = true
		}
		h.mu.Unlock()

		// Fallback to notifiers if no direct reply function
		if !delivered {
			for _, n := range h.notifiers {
				if err := n.Send(ctx, msg); err != nil {
					slog.Error("Failed to deliver reminder", "error", err)
				}
			}
		}

		if err := h.db.MarkReminderDelivered(ctx, r.ID); err != nil {
			slog.Error("Failed to mark reminder delivered", "id", r.ID, "error", err)
		}
		slog.Info("Reminder delivered", "id", r.ID, "session", r.SessionID)
	}
}
