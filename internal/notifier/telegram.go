package notifier

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramNotifier struct {
	bot      *tgbotapi.BotAPI
	chatID   int64
	username string
}

func NewTelegramNotifier(token string, chatID int64) (*TelegramNotifier, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize telegram bot: %w", err)
	}
	return &TelegramNotifier{
		bot:      bot,
		chatID:   chatID,
		username: bot.Self.UserName,
	}, nil
}

func (t *TelegramNotifier) Send(ctx context.Context, message string) error {
	// Telegram has a 4096 character limit
	const limit = 4000

	chunks := splitMessage(message, limit)
	for _, chunk := range chunks {
		msg := tgbotapi.NewMessage(t.chatID, chunk)
		msg.ParseMode = tgbotapi.ModeMarkdown

		if _, err := t.bot.Send(msg); err != nil {
			// Fallback to plain text if Markdown fails
			msg.ParseMode = ""
			if _, err := t.bot.Send(msg); err != nil {
				return fmt.Errorf("failed to send telegram message (even without markdown): %w", err)
			}
		}
	}

	return nil
}

func (t *TelegramNotifier) Name() string {
	return "Telegram"
}

// StartTyping triggers the typing indicator and returns a function to stop it.
func (t *TelegramNotifier) StartTyping(ctx context.Context) func() {
	childCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Send initial typing indicator
		action := tgbotapi.NewChatAction(t.chatID, tgbotapi.ChatTyping)
		_, _ = t.bot.Request(action)

		for {
			select {
			case <-ticker.C:
				action := tgbotapi.NewChatAction(t.chatID, tgbotapi.ChatTyping)
				_, _ = t.bot.Request(action)
			case <-childCtx.Done():
				return
			}
		}
	}()
	return cancel
}

// StartListener begins listening for messages on Telegram.
func (t *TelegramNotifier) StartListener(ctx context.Context, handler func(chatID int64, text string)) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := t.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			// Security: Only respond to the configured ChatID
			if update.Message.Chat.ID != t.chatID {
				continue
			}

			text := update.Message.Text
			if update.Message.IsCommand() {
				// Strip bot username from command (e.g., /status@botname -> /status)
				if i := strings.Index(text, "@"); i != -1 {
					spaceIdx := strings.Index(text, " ")
					if spaceIdx == -1 || i < spaceIdx {
						cmdPart := text[:i]
						usernamePart := ""
						if spaceIdx == -1 {
							usernamePart = text[i+1:]
							if usernamePart == t.username {
								text = cmdPart
							}
						} else {
							usernamePart = text[i+1 : spaceIdx]
							if usernamePart == t.username {
								text = cmdPart + text[spaceIdx:]
							}
						}
					}
				}
			}

			handler(update.Message.Chat.ID, text)
		}
	}
}
