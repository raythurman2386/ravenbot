# ravenbot Persona & Memory Guide

ravenbot is designed to be more than just a chatbot; it's an autonomous research partner inspired by projects like **OpenClaw (Clawdbot)**. This guide explains how to leverage its memory and persona to make it more useful.

## 1. The OpenClaw Inspiration
Our research into OpenClaw/Clawdbot highlighted several key traits of a "useful" messaging agent:
- **Persistent Context**: Remembering user projects and preferences.
- **Proactivity**: Suggesting next steps and checking interests automatically.
- **Multimodal Action**: Not just talking, but performing real work (Git, GitHub, Shell).
- **Personality**: A consistent, relatable "vibe" that builds trust.

## 2. Training ravenbot's Memory
ravenbot uses the **Graph Memory MCP Server**. You can "teach" it things during conversation, and it will remember them across restarts (if persistence is configured) and sessions.

### How to teach ravenbot:
Just tell it!
- *"My name is Ray and I'm a Go developer."*
- *"I'm currently working on a project called 'ravenbot' and I'm interested in Geospatial AI."*
- *"Remind me that I prefer concise summaries for news but detailed explanations for code."*

### How ravenbot uses memory:
- **Daily Briefings**: ravenbot now checks its memory every morning before the 7:00 AM mission. It will include news about topics you've mentioned interest in.
- **Conversation**: It will try to recall your name and current projects when you start a chat.

## 3. Personality & Tone
ravenbot's persona is defined as:
- **Warm & Professional**: Approachable but technically deep.
- **Proactive Partner**: It will suggest tools or commands that might help you complete a task.
- **Subtly Humorous**: Expect occasional technical puns or dry wit.

## 4. Useful Commands via Messaging
Beyond standard chat, use these strategically:
- `/research <topic>` - Use this for deep dives. ravenbot will browse the web and summarize its findings.
- `/status` - Checks the health of the host server.
- `/jules <repo> <task>` - Delegates coding work to the Jules AI agent.

## 5. Proactive Usage Tips
To get the most out of ravenbot on Discord or Telegram:
1. **GitHub Integration**: Ask "Any updates on my GitHub repo?" – since we've now passed your `GITHUB_PERSONAL_ACCESS_TOKEN` to the MCP server, ravenbot can check your PRs and issues.
2. **Context Checks**: Ask "What do you remember about my current projects?" to see the state of its graph memory.
3. **Drafting**: Tell ravenbot, "I'm about to work on X, keep an eye out for news about Y."
