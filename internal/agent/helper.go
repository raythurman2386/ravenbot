package agent

import (
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// runSubAgent is a helper to run a one-off session for a sub-agent
func (a *Agent) runSubAgent(ctx tool.Context, sessionService session.Service, ag agent.Agent, appName, request string) (map[string]any, error) {
	// 1. Create sub-session for the agent
	stateMap := make(map[string]any)
	for k, v := range ctx.State().All() {
		if !strings.HasPrefix(k, "_adk") {
			stateMap[k] = v
		}
	}

	subSession, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: appName,
		UserID:  ctx.UserID(),
		State:   stateMap,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sub-session: %w", err)
	}

	// 2. Create a one-off runner for this call
	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          ag,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sub-runner: %w", err)
	}

	// 3. Execute and accumulate text output
	var fullOutput strings.Builder
	events := r.Run(ctx, ctx.UserID(), subSession.Session.ID(), &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: request}},
	}, agent.RunConfig{
		StreamingMode: agent.StreamingModeSSE,
	})

	for event, err := range events {
		if err != nil {
			return nil, err
		}
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					fullOutput.WriteString(part.Text)
				}
			}
		}
	}

	result := fullOutput.String()
	if result == "" {
		return nil, fmt.Errorf("%s returned no output", appName)
	}

	return map[string]any{"result": result}, nil
}
