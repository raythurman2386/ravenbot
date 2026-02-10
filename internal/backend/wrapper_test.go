package backend

import (
	"context"
	"iter"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
	"github.com/stretchr/testify/assert"
)

type mockLLM struct {
	lastReq *model.LLMRequest
}

func (m *mockLLM) Name() string { return "mock" }
func (m *mockLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, streaming bool) iter.Seq2[*model.LLMResponse, error] {
	m.lastReq = req
	return func(yield func(*model.LLMResponse, error) bool) {}
}

func TestSystemRoleWrapper_GenerateContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    []*genai.Content
		expected []*genai.Content
	}{
		{
			name: "Role Normalization",
			input: []*genai.Content{
				{Role: "system", Parts: []*genai.Part{{Text: "sys"}}},
				{Role: "assistant", Parts: []*genai.Part{{Text: "ast"}}},
			},
			expected: []*genai.Content{
				{Role: "user", Parts: []*genai.Part{{Text: "sys"}}},
				{Role: "model", Parts: []*genai.Part{{Text: "ast"}}},
			},
		},
		{
			name: "Message Merging",
			input: []*genai.Content{
				{Role: "user", Parts: []*genai.Part{{Text: "part1"}}},
				{Role: "user", Parts: []*genai.Part{{Text: "part2"}}},
				{Role: "model", Parts: []*genai.Part{{Text: "resp1"}}},
				{Role: "model", Parts: []*genai.Part{{Text: "resp2"}}},
			},
			expected: []*genai.Content{
				{Role: "user", Parts: []*genai.Part{{Text: "part1"}, {Text: "part2"}}},
				{Role: "model", Parts: []*genai.Part{{Text: "resp1"}, {Text: "resp2"}}},
			},
		},
		{
			name: "Function Parts Handling",
			input: []*genai.Content{
				{Role: "model", Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "fn1"}}}},
				{Role: "model", Parts: []*genai.Part{{Text: "normal text"}}},
			},
			expected: []*genai.Content{
				{Role: "user", Parts: []*genai.Part{{Text: "Continue"}}}, // Prepended because it starts with model
				{Role: "model", Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "fn1"}}}},
				{Role: "model", Parts: []*genai.Part{{Text: "normal text"}}},
			},
		},
		{
			name: "Initial Role Correction",
			input: []*genai.Content{
				{Role: "model", Parts: []*genai.Part{{Text: "hello"}}},
			},
			expected: []*genai.Content{
				{Role: "user", Parts: []*genai.Part{{Text: "Continue"}}},
				{Role: "model", Parts: []*genai.Part{{Text: "hello"}}},
			},
		},
		{
			name: "Complex Mixed Scenario",
			input: []*genai.Content{
				{Role: "system", Parts: []*genai.Part{{Text: "s1"}}},
				{Role: "system", Parts: []*genai.Part{{Text: "s2"}}},
				{Role: "user", Parts: []*genai.Part{{Text: "u1"}}},
				{Role: "assistant", Parts: []*genai.Part{{Text: "a1"}}},
				{Role: "model", Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "fc1"}}}},
				{Role: "model", Parts: []*genai.Part{{Text: "a2"}}},
			},
			expected: []*genai.Content{
				{Role: "user", Parts: []*genai.Part{{Text: "s1"}, {Text: "s2"}, {Text: "u1"}}},
				{Role: "model", Parts: []*genai.Part{{Text: "a1"}}},
				{Role: "model", Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "fc1"}}}},
				{Role: "model", Parts: []*genai.Part{{Text: "a2"}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mock := &mockLLM{}
			wrapper := NewSystemRoleWrapper(mock)
			req := &model.LLMRequest{Contents: tt.input}

			// GenerateContent returns a sequence, we need to invoke it or at least call it
			// to trigger the transformation.
			wrapper.GenerateContent(context.Background(), req, false)

			assert.Equal(t, tt.expected, mock.lastReq.Contents)
		})
	}
}

func TestSystemRoleWrapper_EmptyRequest(t *testing.T) {
	t.Parallel()

	t.Run("nil request", func(t *testing.T) {
		t.Parallel()
		mock := &mockLLM{}
		wrapper := NewSystemRoleWrapper(mock)
		wrapper.GenerateContent(context.Background(), nil, false)
		assert.Nil(t, mock.lastReq)
	})

	t.Run("empty contents", func(t *testing.T) {
		t.Parallel()
		mock := &mockLLM{}
		wrapper := NewSystemRoleWrapper(mock)
		req := &model.LLMRequest{Contents: []*genai.Content{}}
		wrapper.GenerateContent(context.Background(), req, false)
		assert.Equal(t, req, mock.lastReq)
		assert.Empty(t, mock.lastReq.Contents)
	})

	t.Run("contents with no parts", func(t *testing.T) {
		t.Parallel()
		mock := &mockLLM{}
		wrapper := NewSystemRoleWrapper(mock)
		req := &model.LLMRequest{Contents: []*genai.Content{{Role: "user", Parts: nil}}}
		wrapper.GenerateContent(context.Background(), req, false)
		assert.Equal(t, req, mock.lastReq)
	})
}

func TestSystemRoleWrapper_Name(t *testing.T) {
	t.Parallel()
	mock := &mockLLM{}
	wrapper := NewSystemRoleWrapper(mock)
	assert.Equal(t, "mock", wrapper.Name())
}
