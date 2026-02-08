// Package ollama provides a model.LLM implementation for Ollama's OpenAI-compatible API.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

const (
	DefaultBaseURL = "http://localhost:11434/v1"
	DefaultModel   = "qwen3:1.7b"
)

// Model implements model.LLM for Ollama's OpenAI-compatible API.
type Model struct {
	baseURL    string
	modelName  string
	httpClient *http.Client
}

// Option configures a Model.
type Option func(*Model)

// WithBaseURL sets the Ollama API base URL.
func WithBaseURL(url string) Option {
	return func(m *Model) {
		m.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithModel sets the model name to use.
func WithModel(name string) Option {
	return func(m *Model) {
		m.modelName = name
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(m *Model) {
		m.httpClient = client
	}
}

// New creates a new Ollama model adapter.
func New(opts ...Option) *Model {
	m := &Model{
		baseURL:    DefaultBaseURL,
		modelName:  DefaultModel,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Name returns the model identifier.
func (m *Model) Name() string {
	return "ollama/" + m.modelName
}

// OpenAI-compatible API request/response types

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type toolDef struct {
	Type     string      `json:"type"`
	Function functionDef `json:"function"`
}

type functionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []toolDef     `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      chatMessage `json:"message"`
		Delta        chatMessage `json:"delta"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// GenerateContent implements model.LLM.
func (m *Model) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// Convert LLMRequest to OpenAI chat format
		chatReq, err := m.buildChatRequest(req, stream)
		if err != nil {
			yield(nil, fmt.Errorf("building chat request: %w", err))
			return
		}

		body, err := json.Marshal(chatReq)
		if err != nil {
			yield(nil, fmt.Errorf("marshaling request: %w", err))
			return
		}

		slog.Debug("Ollama request", "body", string(body))

		// Use a custom context with no timeout if not provided, allowing long inference
		// But usually ctx from ADK has no timeout for the whole generation
		httpReq, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			yield(nil, fmt.Errorf("creating HTTP request: %w", err))
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := m.httpClient.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("sending request: %w", err))
			return
		}
		defer func() { _ = resp.Body.Close() }()

		// Check for specific error status codes
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			yield(nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes)))
			return
		}

		if stream {
			m.handleStreamResponse(resp.Body, yield)
		} else {
			m.handleSyncResponse(resp.Body, yield)
		}
	}
}

func (m *Model) buildChatRequest(req *model.LLMRequest, stream bool) (*chatRequest, error) {
	slog.Debug("Building chat request", "num_contents", len(req.Contents), "num_tools", len(req.Tools))

	chatReq := &chatRequest{
		Model:    m.modelName,
		Messages: make([]chatMessage, 0, len(req.Contents)),
		Stream:   stream,
	}

	// Convert Contents to chat messages
	for _, content := range req.Contents {
		// Handle tool responses separately (must be individual messages)
		isToolResponse := false
		for _, part := range content.Parts {
			if part.FunctionResponse != nil {
				isToolResponse = true
				break
			}
		}

		if isToolResponse {
			for _, part := range content.Parts {
				if part.FunctionResponse != nil {
					respJSON, err := json.Marshal(part.FunctionResponse.Response)
					if err != nil {
						return nil, fmt.Errorf("marshaling function response: %w", err)
					}
					chatReq.Messages = append(chatReq.Messages, chatMessage{
						Role:       "tool",
						Content:    string(respJSON),
						ToolCallID: part.FunctionResponse.ID,
					})
				}
			}
			continue
		}

		// Handle normal messages (user, assistant/model)
		msg := chatMessage{
			Role: content.Role,
		}
		if msg.Role == "" || msg.Role == "model" {
			// Map 'model' to 'assistant' for OpenAI compatibility
			if msg.Role == "model" {
				msg.Role = "assistant"
			} else {
				msg.Role = "user"
			}
		}

		// Aggregate text parts
		var textParts []string
		for _, part := range content.Parts {
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
			if part.FunctionCall != nil {
				// Model's function call
				argsJSON, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					return nil, fmt.Errorf("marshaling function args: %w", err)
				}
				msg.ToolCalls = append(msg.ToolCalls, toolCall{
					ID:   part.FunctionCall.ID,
					Type: "function",
					Function: functionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}
		if len(textParts) > 0 {
			msg.Content = strings.Join(textParts, "\n")
		}
		chatReq.Messages = append(chatReq.Messages, msg)
	}

	// Convert Tools from LLMRequest.Tools map to OpenAI format
	for name, td := range req.Tools {
		var toolName, toolDesc string
		var toolParams map[string]any

		if t, ok := td.(tool.Tool); ok {
			toolName = t.Name()
			toolDesc = t.Description()

			// Check if the tool has a Config() method that returns a struct with InputSchema
			// This matches google.golang.org/adk/tool/functiontool
			method := reflect.ValueOf(td).MethodByName("Config")
			if method.IsValid() {
				results := method.Call(nil)
				if len(results) > 0 {
					cfg := results[0]
					if cfg.Kind() == reflect.Struct {
						schemaField := cfg.FieldByName("InputSchema")
						if schemaField.IsValid() && !schemaField.IsNil() {
							// Marshal the schema to JSON to convert it to map[string]any
							if data, err := json.Marshal(schemaField.Interface()); err == nil {
								_ = json.Unmarshal(data, &toolParams)
							}
						}
					}
				}
			}
		}

		if toolName == "" {
			toolName = name
		}

		// Ensure toolParams is not nil (Ollama/OpenAI requirement)
		if toolParams == nil {
			toolParams = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}

		chatReq.Tools = append(chatReq.Tools, toolDef{
			Type: "function",
			Function: functionDef{
				Name:        toolName,
				Description: toolDesc,
				Parameters:  toolParams,
			},
		})
	}

	return chatReq, nil
}

func (m *Model) handleSyncResponse(body io.Reader, yield func(*model.LLMResponse, error) bool) {
	data, err := io.ReadAll(body)
	if err != nil {
		yield(nil, fmt.Errorf("reading response: %w", err))
		return
	}
	slog.Debug("Ollama response", "body", string(data))

	var chatResp chatResponse
	if err := json.Unmarshal(data, &chatResp); err != nil {
		yield(nil, fmt.Errorf("decoding response: %w", err))
		return
	}

	if len(chatResp.Choices) == 0 {
		yield(nil, fmt.Errorf("no choices in response"))
		return
	}

	llmResp := m.convertToLLMResponse(&chatResp.Choices[0].Message, chatResp.Usage.TotalTokens)
	yield(llmResp, nil)
}

func (m *Model) handleStreamResponse(body io.Reader, yield func(*model.LLMResponse, error) bool) {
	reader := newSSEReader(body)

	for {
		data, err := reader.ReadEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			yield(nil, fmt.Errorf("reading SSE: %w", err))
			return
		}

		if data == "" || data == "[DONE]" {
			continue
		}

		var chatResp chatResponse
		if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
			continue // Skip malformed chunks
		}

		if len(chatResp.Choices) > 0 {
			// In streaming, we look at Delta, not Message
			msg := &chatResp.Choices[0].Delta

			// If both content and tool calls are empty, skip (unless it's a finish reason update)
			if msg.Content == "" && msg.ToolCalls == nil && msg.Role == "" {
				continue
			}

			llmResp := m.convertToLLMResponse(msg, 0)
			llmResp.Partial = true
			if !yield(llmResp, nil) {
				return
			}
		}
	}
}

// sseReader reads server-sent events.
type sseReader struct {
	r *bufioReader
}

type bufioReader struct {
	r   io.Reader
	buf []byte
}

func newSSEReader(r io.Reader) *sseReader {
	return &sseReader{r: &bufioReader{r: r, buf: make([]byte, 0, 4096)}}
}

func (r *bufioReader) ReadLine() (string, error) {
	for {
		// Look for newline in buffer
		for i, b := range r.buf {
			if b == '\n' {
				line := string(r.buf[:i])
				r.buf = r.buf[i+1:]
				return strings.TrimSuffix(line, "\r"), nil
			}
		}

		// Read more data
		tmp := make([]byte, 1024)
		n, err := r.r.Read(tmp)
		if n > 0 {
			r.buf = append(r.buf, tmp[:n]...)
		}
		if err != nil {
			if len(r.buf) > 0 {
				line := string(r.buf)
				r.buf = r.buf[:0]
				return line, nil
			}
			return "", err
		}
	}
}

func (s *sseReader) ReadEvent() (string, error) {
	for {
		line, err := s.r.ReadLine()
		if err != nil {
			return "", err
		}

		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: "), nil
		}
	}
}

func (m *Model) convertToLLMResponse(msg *chatMessage, tokens int) *model.LLMResponse {
	resp := &model.LLMResponse{
		Content: &genai.Content{
			Role:  msg.Role,
			Parts: make([]*genai.Part, 0),
		},
	}

	if tokens > 0 {
		resp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			TotalTokenCount: int32(tokens),
		}
	}

	// Add text content
	if msg.Content != "" {
		resp.Content.Parts = append(resp.Content.Parts, genai.NewPartFromText(msg.Content))
	}

	// Add function calls
	for _, tc := range msg.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			args = map[string]any{"raw": tc.Function.Arguments}
		}
		resp.Content.Parts = append(resp.Content.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			},
		})
	}

	return resp
}
