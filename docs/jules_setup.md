# Jules Agent Setup for ravenbot

ravenbot integrates with the **Jules Agent API** to allow you to delegate complex coding and repository tasks directly from your chat interface. This guide explains how to set up and use this feature.

## 1. Prerequisites

To use the Jules Agent, you must have:
1.  Access to the [Jules Agent](https://jules.google).
2.  A Google Cloud Project with the necessary APIs enabled (if applicable, though typically managed via the Jules interface).

## 2. Connect Your Repository

Before ravenbot can delegate tasks for a specific repository, that repository must be connected to Jules.

1.  Visit [https://jules.google](https://jules.google).
2.  Connect the GitHub repository you wish to manage.
3.  Ensure Jules has the necessary permissions to read code and create Pull Requests.

## 3. Get Your API Key

You need a valid API key to authenticate requests to the Jules API.

1.  Obtain your API Key from the Jules platform or your Google Cloud Console (associated with the project using Jules).
2.  Add this key to your `.env` file as `JULES_API_KEY`.

```bash
JULES_API_KEY=your_jules_api_key_here
```

## 4. Usage

Once configured, you can use the `/jules` command in your connected Discord channel or Telegram chat.

**Syntax:**
```
/jules <owner/repo> <task description>
```

**Example:**
```
/jules raythurman2386/ravenbot Add a new documentation file for Discord setup
```

### How it Works

1.  **Request:** ravenbot sends your task and repository context to the Jules API (`v1alpha`).
2.  **Session:** A new Jules session is created with the title "ravenbot Task: ...".
3.  **Automation:** The request is sent with `AutomationMode: "AUTO_CREATE_PR"`, meaning Jules will attempt to implement the requested change and automatically open a Pull Request on the target repository.
4.  **Feedback:** ravenbot will reply with the Session Name/ID confirming the task has been initiated.

## 5. Troubleshooting

*   **"Jules api error: ... not found":** This usually means the repository hasn't been connected to Jules yet. Visit `https://jules.google` to connect it.
*   **"JULES_API_KEY is not set":** Check your `.env` file and ensure the key is present and the bot has been restarted.
*   **Repo Format:** Ensure you are using the `owner/repo` format (e.g., `google/go-genai`, not just `go-genai`).
