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
		// 1. Deep-copy contents to avoid mutating the caller's slice
		//    (ADK may reuse the request for session persistence).
		rawContents := make([]*genai.Content, 0, len(req.Contents))
		for _, orig := range req.Contents {
			role := normalizeRole(orig.Role)
			if len(orig.Parts) > 0 {
				// Shallow-copy Parts slice; individual Parts are not mutated.
				parts := make([]*genai.Part, len(orig.Parts))
				copy(parts, orig.Parts)
				rawContents = append(rawContents, &genai.Content{
					Role:  role,
					Parts: parts,
				})
			}
		}

		if len(rawContents) == 0 {
			return w.model.GenerateContent(ctx, req, streaming)
		}

		// 2. Merge consecutive same-role messages, preserving FunctionCall/Response alternation
		merged := make([]*genai.Content, 0, len(rawContents))
		for _, content := range rawContents {
			if len(merged) == 0 {
				merged = append(merged, content)
				continue
			}

			prev := merged[len(merged)-1]
			hasFuncParts := containsFunctionParts(content)
			prevHasFuncParts := containsFunctionParts(prev)

			// Merge if roles are same AND neither message has function call/response parts.
			// Gemini requires function calls/responses to be in their own turn.
			if prev.Role == content.Role && !hasFuncParts && !prevHasFuncParts {
				prev.Parts = append(prev.Parts, content.Parts...)
			} else {
				merged = append(merged, content)
			}
		}

		// 3. Ensure the sequence starts with "user" if it's for Gemini
		// (ADK usually handles this, but we reinforce it here)
		if len(merged) > 0 && merged[0].Role == "model" {
			slog.Warn("SystemRoleWrapper: first message is 'model', prepending empty 'user' message for compatibility")
			merged = append([]*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: "Continue"}}}}, merged...)
		}

		req.Contents = merged
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
