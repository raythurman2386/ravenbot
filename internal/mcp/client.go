package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client represents an MCP client connected to a server
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	requestID int64
	pending   map[int64]chan Response
	pendingMu sync.Mutex

	serverInfo   *ServerInfo
	capabilities map[string]any
}

// NewStdioClient creates a new client using stdio transport
func NewStdioClient(command string, args []string) *Client {
	cmd := exec.Command(command, args...)
	return &Client{
		cmd:     cmd,
		pending: make(map[int64]chan Response),
	}
}

// Start starts the server process and begins listening for messages
func (c *Client) Start() error {
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Start reading stderr in background for logging
	go func() {
		scanner := bufio.NewScanner(c.stderr)
		for scanner.Scan() {
			slog.Debug("MCP Server Stderr", "msg", scanner.Text())
		}
	}()

	// Start reading stdout in background
	go c.readLoop()

	return nil
}

// readLoop reads JSON-RPC messages from stdout
func (c *Client) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		// slog.Debug("MCP Received", "raw", string(line))

		// Try to parse as Response first (most common for us)
		var resp Response
		if err := json.Unmarshal(line, &resp); err == nil && resp.ID != 0 {
			c.pendingMu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				delete(c.pending, resp.ID)
				ch <- resp
			}
			c.pendingMu.Unlock()
			continue
		}

		// Could be a notification or request from server (not handled yet)
	}
}

// Close terminates the connection
func (c *Client) Close() error {
	return c.cmd.Process.Kill()
}

// SendRequest sends a request and waits for the response
func (c *Client) SendRequest(method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.requestID, 1)

	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req := Request{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  paramBytes,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ch := make(chan Response, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	// Write request (newline delimited)
	if _, err := c.stdin.Write(append(reqBytes, '\n')); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, err
	}

	// Wait for response
	resp := <-ch

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

// Initialize performs the handshake
func (c *Client) Initialize() error {
	params := InitializeParams{
		ProtocolVersion: LATEST_PROTOCOL_VERSION,
		ClientInfo: ClientInfo{
			Name:    "ravenbot",
			Version: "1.0.0",
		},
		Capabilities: Capabilities{},
	}

	resultRaw, err := c.SendRequest("initialize", params)
	if err != nil {
		return err
	}

	var result InitializeResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return err
	}

	c.serverInfo = &result.ServerInfo
	c.capabilities = result.Capabilities

	// Send initialized notification
	notif := Notification{
		JSONRPC: JSONRPCVersion,
		Method:  "notifications/initialized",
	}
	notifBytes, _ := json.Marshal(notif)
	c.stdin.Write(append(notifBytes, '\n'))

	return nil
}

// ListTools fetches available tools
func (c *Client) ListTools() ([]Tool, error) {
	resultRaw, err := c.SendRequest("tools/list", map[string]string{})
	if err != nil {
		return nil, err
	}

	var result ListToolsResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return nil, err
	}

	return result.Tools, nil
}

// CallTool executes a tool
func (c *Client) CallTool(name string, args map[string]any) (*CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}

	resultRaw, err := c.SendRequest("tools/call", params)
	if err != nil {
		return nil, err
	}

	var result CallToolResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
