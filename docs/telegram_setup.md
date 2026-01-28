# Telegram Setup Guide for RavenBot

This guide explains how to set up the Telegram integration for RavenBot. To use RavenBot with Telegram, you need to create a bot via BotFather and configure the necessary environment variables.

## 1. Create a New Bot

1.  Open Telegram and search for **@BotFather**.
2.  Start a chat with BotFather and send the command `/newbot`.
3.  Follow the prompts:
    *   **Name:** Choose a display name for your bot (e.g., "RavenBot").
    *   **Username:** Choose a unique username ending in `bot` (e.g., `MyRavenBot`).
4.  Once created, BotFather will send you a message with your **HTTP API Token**.

## 2. Configure the Bot Token

Copy the token provided by BotFather and add it to your `.env` file as `TELEGRAM_BOT_TOKEN`.

```bash
TELEGRAM_BOT_TOKEN=123456789:ABCdefGHIjklMNOpqrSTUvwxyz
```

## 3. Get Your Chat ID

RavenBot requires a specific Chat ID to send messages to (and to accept commands from). This ensures the bot doesn't respond to unauthorized users.

1.  Open a chat with your new bot in Telegram (search for its username).
2.  Send a message (e.g., "Hello") to the bot.
3.  Visit the following URL in your browser (replace `<YOUR_BOT_TOKEN>` with your actual token):

    ```
    https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates
    ```

4.  Look for the JSON response. Find the `chat` object inside the `result` array.
5.  Copy the `id` value. It is usually a large integer (positive for private chats, negative for groups).
    *   Example: `"chat":{"id": 123456789, ...}`

6.  Add this ID to your `.env` file as `TELEGRAM_CHAT_ID`.

```bash
TELEGRAM_CHAT_ID=123456789
```

## 4. Final Configuration

Ensure your `.env` file has both variables set:

```ini
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
TELEGRAM_CHAT_ID=your_telegram_chat_id
```

Once configured, restart RavenBot. The bot will now send daily briefings to this chat and accept commands (like `/research` or `/jules`) from it.
