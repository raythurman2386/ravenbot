package notifier

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramNotifier struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

func NewTelegramNotifier(token string, chatID int64) (*TelegramNotifier, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize telegram bot: %w", err)
	}
	return &TelegramNotifier{bot: bot, chatID: chatID}, nil
}

func (t *TelegramNotifier) Send(ctx context.Context, message string) error {
	// Telegram has a 4096 character limit
	const limit = 4000

	msgRunes := []rune(message)
	for i := 0; i < len(msgRunes); i += limit {
		end := i + limit
		if end > len(msgRunes) {
			end = len(msgRunes)
		}

		msg := tgbotapi.NewMessage(t.chatID, string(msgRunes[i:end]))
		msg.ParseMode = tgbotapi.ModeMarkdown

		if _, err := t.bot.Send(msg); err != nil {
			return fmt.Errorf("failed to send telegram message: %w", err)
		}
	}

	return nil
}

func (t *TelegramNotifier) Name() string {
	return "Telegram"
}
