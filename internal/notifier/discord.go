package notifier

import (
	"context"
	"fmt"

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
