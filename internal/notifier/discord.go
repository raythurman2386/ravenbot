package notifier

import (
	"context"
	"fmt"
	"log/slog"

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

	msgRunes := []rune(message)
	for i := 0; i < len(msgRunes); i += limit {
		end := i + limit
		if end > len(msgRunes) {
			end = len(msgRunes)
		}

		if _, err := d.session.ChannelMessageSend(d.channelID, string(msgRunes[i:end])); err != nil {
			return fmt.Errorf("failed to send discord message: %w", err)
		}
	}

	return nil
}

func (d *DiscordNotifier) Name() string {
	return "Discord"
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

		handler(m.ChannelID, m.Content)
	})

	if err := d.session.Open(); err != nil {
		slog.Error("Failed to open discord session", "error", err)
		return
	}

	<-ctx.Done()
	d.session.Close()
}
