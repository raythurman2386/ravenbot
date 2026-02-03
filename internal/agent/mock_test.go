package agent

import (
	"context"
	"iter"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type MockLLM struct {
	QueuedResponses [][]*model.LLMResponse
	CallCount       int
}

func (m *MockLLM) Name() string {
	return "mock-model"
}

func (m *MockLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	var responses []*model.LLMResponse
	if m.CallCount < len(m.QueuedResponses) {
		responses = m.QueuedResponses[m.CallCount]
	}
	m.CallCount++

	return func(yield func(*model.LLMResponse, error) bool) {
		for _, resp := range responses {
			if !yield(resp, nil) {
				return
			}
		}
	}
}

// Helper to create a text response
func NewTextResponse(text string) *model.LLMResponse {
	return &model.LLMResponse{
		Content: &genai.Content{
			Parts: []*genai.Part{{Text: text}},
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			TotalTokenCount: 100,
		},
	}
}
