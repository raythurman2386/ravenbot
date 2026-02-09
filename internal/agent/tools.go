package agent

import (
	"google.golang.org/adk/tool"
)

// GetTechnicalTools returns the list of tools intended for the ResearchAssistant sub-agent.
// (Native research is now handled via SearchAssistant sub-agent delegation)
func (a *Agent) GetTechnicalTools() []tool.Tool {
	return nil
}

// GetCoreTools returns the tools for the root conversational agent.
func (a *Agent) GetCoreTools() []tool.Tool {
	return nil
}
