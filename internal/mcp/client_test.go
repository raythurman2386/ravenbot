package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// Helper to run the mock server if env var is set
func init() {
	if os.Getenv("GO_TEST_MCP_SERVER") == "1" {
		runMockServer()
		os.Exit(0)
	}
}

func runMockServer() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		// Don't respond to notifications (ID=0)
		if req.ID == 0 {
			continue
		}

		var result any
		switch req.Method {
		case "initialize":
			result = InitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities:    map[string]any{},
				ServerInfo: ServerInfo{
					Name:    "mock-server",
					Version: "0.1.0",
				},
			}
		case "tools/list":
			result = ListToolsResult{
				Tools: []Tool{
					{
						Name:        "echo",
						Description: "Echoes back the input",
						InputSchema: json.RawMessage(`{"type":"object"}`),
					},
				},
			}
		case "tools/call":
			result = CallToolResult{
				Content: []Content{
					{Type: "text", Text: "echo result"},
				},
			}
		case "debug/sendNotification":
			notif := Notification{
				JSONRPC: "2.0",
				Method:  "test/notification",
				Params:  json.RawMessage(`{"message": "hello"}`),
			}
			out, _ := json.Marshal(notif)
			fmt.Println(string(out))
			result = map[string]string{"status": "sent"}
		}

		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage{}, // Placeholder
		}

		if result != nil {
			resBytes, _ := json.Marshal(result)
			resp.Result = resBytes
		}

		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}

func TestStdioClient(t *testing.T) {
	// Re-exec this test binary as the server
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get executable: %v", err)
	}

	client := NewStdioClient(exe, []string{})
	// Set the env var for the subprocess
	client.transport.(*StdioTransport).cmd.Env = append(os.Environ(), "GO_TEST_MCP_SERVER=1")

	if err := client.Start(); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close()

	// 1. Test Initialize
	if err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if client.serverInfo.Name != "mock-server" {
		t.Errorf("Expected server name mock-server, got %s", client.serverInfo.Name)
	}

	// 2. Test ListTools
	tools, err := client.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Errorf("Expected 1 tool 'echo', got %v", tools)
	}

	// 3. Test CallTool
	res, err := client.CallTool("echo", map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if len(res.Content) == 0 || res.Content[0].Text != "echo result" {
		t.Errorf("Unexpected result: %v", res)
	}
}

func TestClientNotification(t *testing.T) {
	// Re-exec this test binary as the server
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get executable: %v", err)
	}

	client := NewStdioClient(exe, []string{})
	// Set the env var for the subprocess
	client.transport.(*StdioTransport).cmd.Env = append(os.Environ(), "GO_TEST_MCP_SERVER=1")

	if err := client.Start(); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close()

	if err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Setup notification handler
	received := make(chan Notification, 1)
	client.OnNotification("test/notification", func(n Notification) {
		received <- n
	})

	// Trigger notification
	_, err = client.SendRequest("debug/sendNotification", nil)
	if err != nil {
		t.Fatalf("Failed to trigger notification: %v", err)
	}

	// Wait for notification
	select {
	case n := <-received:
		if n.Method != "test/notification" {
			t.Errorf("Unexpected method: %s", n.Method)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for notification")
	}
}
