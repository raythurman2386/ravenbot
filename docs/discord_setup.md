# Discord Setup Guide for RavenBot

This guide explains how to set up the Discord integration for RavenBot. To use RavenBot with Discord, you need to create a bot application, invite it to your server, and configure the necessary environment variables.

## 1. Create a Discord Application

1.  Go to the [Discord Developer Portal](https://discord.com/developers/applications).
2.  Click **New Application** in the top right corner.
3.  Give your application a name (e.g., "RavenBot") and click **Create**.

## 2. Create a Bot User and Get Token

1.  In the menu on the left, click **Bot**.
2.  Click **Reset Token** (or create one if it's the first time) to generate your bot token.
3.  **Copy this token immediately.** You will not be able to see it again.
4.  This token goes into your `.env` file as `DISCORD_BOT_TOKEN`.

    ```bash
    DISCORD_BOT_TOKEN=your_token_here
    ```

5.  (Optional) Disable "Public Bot" if you want to prevent others from adding your bot to their servers.
6.  Ensure **Message Content Intent** is enabled if the bot needs to read message content (RavenBot requires this to read commands).

## 3. Invite the Bot to Your Server

1.  In the menu on the left, click **OAuth2** -> **URL Generator**.
2.  Under **Scopes**, check `bot`.
3.  Under **Bot Permissions**, check the following permissions:
    *   Read Messages / View Channels
    *   Send Messages
    *   Embed Links (useful for rich responses)
    *   Attach Files (if needed for logs/reports)
4.  Copy the generated URL at the bottom.
5.  Paste the URL into your browser, select the server you want to add the bot to, and click **Authorize**.

## 4. Get the Channel ID

RavenBot is designed to listen to a specific channel for security reasons.

1.  Open your Discord User Settings (gear icon).
2.  Go to **Advanced** and enable **Developer Mode**.
3.  Right-click the channel where you want RavenBot to be active.
4.  Click **Copy Channel ID** (or **Copy ID**).
5.  This ID goes into your `.env` file as `DISCORD_CHANNEL_ID`.

    ```bash
    DISCORD_CHANNEL_ID=123456789012345678
    ```

## 5. Final Configuration

Ensure your `.env` file has both variables set:

```ini
DISCORD_BOT_TOKEN=your_discord_bot_token
DISCORD_CHANNEL_ID=your_discord_channel_id
```

Once configured, restart RavenBot. The bot will now listen for commands (like `/research` or `/jules`) in that specific channel.
