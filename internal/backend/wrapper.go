// SystemRoleWrapper wraps a model.LLM to ensure compatibility with Gemini's strict
// role requirements. It normalizes non-standard roles (e.g. "assistant" from Ollama)
// and merges consecutive same-role messages to maintain the required user/model alternation.
package backend

import (
	"context"
	"iter"
	"log/slog"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type SystemRoleWrapper struct {
	model model.LLM
}

func NewSystemRoleWrapper(m model.LLM) model.LLM {
	return &SystemRoleWrapper{model: m}
}

func (w *SystemRoleWrapper) Name() string {
	return w.model.Name()
}

// normalizeRole maps non-standard roles to Gemini-compatible ones.
// Gemini only accepts "user" and "model" roles in the contents array.
func normalizeRole(role string) string {
	switch strings.ToLower(role) {
	case "system":
		return "user"
	case "assistant":
		return "model"
	default:
		return role
	}
}

func (w *SystemRoleWrapper) GenerateContent(ctx context.Context, req *model.LLMRequest, streaming bool) iter.Seq2[*model.LLMResponse, error] {
	if req != nil && len(req.Contents) > 0 {
		originalLen := len(req.Contents)

		// 1. Normalize all roles
		for _, content := range req.Contents {
			content.Role = normalizeRole(content.Role)
		}

		// 2. Merge consecutive same-role messages, but preserve FunctionCall and
		//    FunctionResponse entries as separate Content items.
		merged := make([]*genai.Content, 0, originalLen)
		for _, content := range req.Contents {
			hasFuncParts := containsFunctionParts(content)
			prevHasFuncParts := len(merged) > 0 && containsFunctionParts(merged[len(merged)-1])

			if len(merged) > 0 && merged[len(merged)-1].Role == content.Role && !hasFuncParts && !prevHasFuncParts {
				merged[len(merged)-1].Parts = append(merged[len(merged)-1].Parts, content.Parts...)
			} else {
				merged = append(merged, content)
			}
		}

		// 3. Remove any Content entries with nil/empty parts
		cleaned := make([]*genai.Content, 0, len(merged))
		for _, content := range merged {
			if len(content.Parts) > 0 {
				cleaned = append(cleaned, content)
			}
		}

		if len(cleaned) != originalLen {
			slog.Info("SystemRoleWrapper: sanitized content list",
				"original", originalLen, "final", len(cleaned))
		}

		req.Contents = cleaned
	}

	return w.model.GenerateContent(ctx, req, streaming)
}

// containsFunctionParts returns true if any Part is a FunctionCall or FunctionResponse.
func containsFunctionParts(c *genai.Content) bool {
	for _, p := range c.Parts {
		if p.FunctionCall != nil || p.FunctionResponse != nil {
			return true
		}
	}
	return false
}
