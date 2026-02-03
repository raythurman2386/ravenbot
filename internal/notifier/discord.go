package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type DiscordNotifier struct {
	sessions  []*discordgo.Session
	channelID string
}

func NewDiscordNotifier(tokens []string, channelID string) (*DiscordNotifier, error) {
	var sessions []*discordgo.Session

	for _, token := range tokens {
		dg, err := discordgo.New("Bot " + token)
		if err != nil {
			slog.Warn("Failed to initialize discord session for a token", "error", err)
			continue
		}

		// Set intents to receive messages and message content
		dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent
		sessions = append(sessions, dg)
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("failed to initialize any discord sessions")
	}

	return &DiscordNotifier{sessions: sessions, channelID: channelID}, nil
}

func (d *DiscordNotifier) Send(ctx context.Context, message string) error {
	// Discord has a 2000 character limit
	const limit = 1900

	chunks := splitMessage(message, limit)

	var lastErr error
	for _, session := range d.sessions {
		success := true
		for i, chunk := range chunks {
			if _, err := session.ChannelMessageSend(d.channelID, chunk); err != nil {
				slog.Warn("Failed to send discord message chunk, trying next key", "chunk", i+1, "error", err)
				lastErr = err
				success = false
				break
			}
		}

		if success {
			return nil
		}
	}

	return fmt.Errorf("failed to send discord message with any available key: %w", lastErr)
}

func (d *DiscordNotifier) Name() string {
	return "Discord"
}

// StartTyping triggers the typing indicator and returns a function to stop it.
func (d *DiscordNotifier) StartTyping(ctx context.Context) func() {
	childCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		session := d.sessions[0]

		// Send initial typing indicator
		if err := session.ChannelTyping(d.channelID); err != nil {
			slog.Error("Failed to send initial discord typing indicator", "error", err)
		}

		for {
			select {
			case <-ticker.C:
				if err := session.ChannelTyping(d.channelID); err != nil {
					slog.Error("Failed to send discord typing indicator", "error", err)
				}
			case <-childCtx.Done():
				return
			}
		}
	}()
	return cancel
}

// StartListener begins listening for messages on Discord.
func (d *DiscordNotifier) StartListener(ctx context.Context, handler func(channelID string, text string)) {
	session := d.sessions[0]
	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Ignore all messages created by the bot itself
		if m.Author.ID == s.State.User.ID {
			return
		}

		// Security: Only respond to the configured ChannelID
		if m.ChannelID != d.channelID {
			return
		}

		// Clean up the message: strip bot mentions and trim space
		content := m.Content
		botMention := fmt.Sprintf("<@%s>", s.State.User.ID)
		botMentionNick := fmt.Sprintf("<@!%s>", s.State.User.ID)
		content = strings.ReplaceAll(content, botMention, "")
		content = strings.ReplaceAll(content, botMentionNick, "")
		content = strings.TrimSpace(content)

		if content != "" {
			handler(m.ChannelID, content)
		}
	})

	if err := session.Open(); err != nil {
		slog.Error("Failed to open discord session", "error", err)
		return
	}

	<-ctx.Done()
	session.Close()
}
