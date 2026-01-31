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
	session   *discordgo.Session
	channelID string
}

func NewDiscordNotifier(token string, channelID string) (*DiscordNotifier, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize discord session: %w", err)
	}

	// Set intents to receive messages and message content
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	return &DiscordNotifier{session: dg, channelID: channelID}, nil
}

func (d *DiscordNotifier) Send(ctx context.Context, message string) error {
	// Discord has a 2000 character limit
	const limit = 1900

	chunks := splitMessage(message, limit)
	for _, chunk := range chunks {
		if _, err := d.session.ChannelMessageSend(d.channelID, chunk); err != nil {
			return fmt.Errorf("failed to send discord message: %w", err)
		}
	}

	return nil
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

		// Send initial typing indicator
		if err := d.session.ChannelTyping(d.channelID); err != nil {
			slog.Error("Failed to send initial discord typing indicator", "error", err)
		}

		for {
			select {
			case <-ticker.C:
				if err := d.session.ChannelTyping(d.channelID); err != nil {
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
	d.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
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

	if err := d.session.Open(); err != nil {
		slog.Error("Failed to open discord session", "error", err)
		return
	}

	<-ctx.Done()
	d.session.Close()
}
