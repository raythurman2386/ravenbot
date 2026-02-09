package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestNew_Defaults(t *testing.T) {
	m := New()

	if m.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %v, want %v", m.baseURL, DefaultBaseURL)
	}
	if m.modelName != DefaultModel {
		t.Errorf("modelName = %v, want %v", m.modelName, DefaultModel)
	}
	if m.httpClient != http.DefaultClient {
		t.Error("httpClient should be http.DefaultClient")
	}
}

func TestNew_WithOptions(t *testing.T) {
	customClient := &http.Client{}

	m := New(
		WithBaseURL("http://custom:8080/v1"),
		WithModel("custom-model"),
		WithHTTPClient(customClient),
	)

	if m.baseURL != "http://custom:8080/v1" {
		t.Errorf("baseURL = %v, want http://custom:8080/v1", m.baseURL)
	}
	if m.modelName != "custom-model" {
		t.Errorf("modelName = %v, want custom-model", m.modelName)
	}
	if m.httpClient != customClient {
		t.Error("httpClient should be custom client")
	}
}

func TestWithBaseURL_TrimsTrailingSlash(t *testing.T) {
	m := New(WithBaseURL("http://localhost:8080/v1/"))

	if m.baseURL != "http://localhost:8080/v1" {
		t.Errorf("baseURL = %v, want http://localhost:8080/v1 (no trailing slash)", m.baseURL)
	}
}

func TestModel_Name(t *testing.T) {
	m := New(WithModel("llama3.2"))

	name := m.Name()
	if name != "ollama/llama3.2" {
		t.Errorf("Name() = %v, want ollama/llama3.2", name)
	}
}

func TestModel_GenerateContent_Success(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Expected /chat/completions, got %s", r.URL.Path)
		}

		// Check content type
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", ct)
		}

		// Parse request
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "llama3.2" {
			t.Errorf("Expected model llama3.2, got %s", req.Model)
		}

		// Send response
		resp := chatResponse{
			ID:     "test-id",
			Object: "chat.completion",
			Model:  "llama3.2",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				Delta        chatMessage `json:"delta"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	m := New(
		WithBaseURL(server.URL),
		WithModel("llama3.2"),
	)

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					genai.NewPartFromText("Hello"),
				},
			},
		},
	}

	var responses []*model.LLMResponse
	var errors []error

	for resp, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			errors = append(errors, err)
		}
		if resp != nil {
			responses = append(responses, resp)
		}
	}

	if len(errors) > 0 {
		t.Fatalf("GenerateContent returned errors: %v", errors)
	}

	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Content == nil {
		t.Fatal("Response content is nil")
	}
	if len(resp.Content.Parts) == 0 {
		t.Fatal("Response has no parts")
	}
	if resp.Content.Parts[0].Text != "Hello! How can I help you?" {
		t.Errorf("Response text = %v, want 'Hello! How can I help you?'", resp.Content.Parts[0].Text)
	}
}

func TestModel_GenerateContent_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	m := New(WithBaseURL(server.URL))

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					genai.NewPartFromText("Hello"),
				},
			},
		},
	}

	var errors []error
	for _, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) == 0 {
		t.Error("Expected error for 500 response")
	}
}

func TestModel_GenerateContent_WithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Return a tool call response
		resp := chatResponse{
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				Delta        chatMessage `json:"delta"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role: "assistant",
						ToolCalls: []toolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: functionCall{
									Name:      "get_temperature",
									Arguments: `{"unit":"celsius"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	m := New(WithBaseURL(server.URL))

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					genai.NewPartFromText("What's the temperature?"),
				},
			},
		},
		Tools: map[string]any{
			"get_temperature": &genai.FunctionDeclaration{
				Name:        "get_temperature",
				Description: "Get current temperature",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"unit": {Type: genai.TypeString},
					},
				},
			},
		},
	}

	var responses []*model.LLMResponse
	for resp, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatalf("GenerateContent error: %v", err)
		}
		if resp != nil {
			responses = append(responses, resp)
		}
	}

	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	// Check for function call
	foundFunctionCall := false
	for _, part := range resp.Content.Parts {
		if part.FunctionCall != nil {
			foundFunctionCall = true
			if part.FunctionCall.Name != "get_temperature" {
				t.Errorf("FunctionCall.Name = %v, want get_temperature", part.FunctionCall.Name)
			}
		}
	}

	if !foundFunctionCall {
		t.Error("Expected function call in response")
	}
}

func TestModel_GenerateContent_Streaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.ResponseWriter to be an http.Flusher")
		}

		chunks := []string{"Hello", " world"}
		for _, chunk := range chunks {
			resp := chatResponse{
				Choices: []struct {
					Index        int         `json:"index"`
					Message      chatMessage `json:"message"`
					Delta        chatMessage `json:"delta"`
					FinishReason string      `json:"finish_reason"`
				}{
					{
						Delta: chatMessage{
							Role:    "assistant",
							Content: chunk,
						},
					},
				},
			}
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	m := New(WithBaseURL(server.URL))

	req := &model.LLMRequest{
		Contents: []*genai.Content{{Role: "user", Parts: []*genai.Part{genai.NewPartFromText("Hi")}}},
	}

	var responses []*model.LLMResponse
	for resp, err := range m.GenerateContent(context.Background(), req, true) {
		if err != nil {
			t.Fatalf("GenerateContent error: %v", err)
		}
		if resp != nil {
			responses = append(responses, resp)
		}
	}

	if len(responses) == 0 {
		t.Fatal("Expected at least one streaming response")
	}

	var fullText strings.Builder
	for _, resp := range responses {
		if !resp.Partial {
			t.Error("Streaming response should be marked as partial")
		}
		if resp.Content != nil {
			for _, p := range resp.Content.Parts {
				fullText.WriteString(p.Text)
			}
		}
	}

	if fullText.String() != "Hello world" {
		t.Errorf("Full text = %q, want 'Hello world'", fullText.String())
	}
}

func TestSSEReader(t *testing.T) {
	input := "data: hello\n\ndata: world\n\n"
	reader := newSSEReader(strings.NewReader(input))

	event1, err := reader.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent() error = %v", err)
	}
	if event1 != "hello" {
		t.Errorf("event1 = %v, want hello", event1)
	}

	event2, err := reader.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent() error = %v", err)
	}
	if event2 != "world" {
		t.Errorf("event2 = %v, want world", event2)
	}

	_, err = reader.ReadEvent()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

func TestBufioReader_ReadLine(t *testing.T) {
	input := "line1\nline2\r\nline3"
	reader := &bufioReader{r: strings.NewReader(input), buf: make([]byte, 0, 4096)}

	line1, err := reader.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if line1 != "line1" {
		t.Errorf("line1 = %v, want line1", line1)
	}

	line2, err := reader.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if line2 != "line2" {
		t.Errorf("line2 = %v, want line2", line2)
	}

	line3, err := reader.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if line3 != "line3" {
		t.Errorf("line3 = %v, want line3", line3)
	}
}
