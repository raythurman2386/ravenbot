package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

// Transport defines the interface for MCP message transport
type Transport interface {
	Start() error
	Close() error
	WriteRequest(req []byte) error
	ReadLoop(handler func(line []byte))
}

// Client represents an MCP client connected to a server
type Client struct {
	transport Transport

	requestID int64
	pending   map[int64]chan Response
	pendingMu sync.Mutex

	serverInfo   *ServerInfo
	capabilities map[string]any
}

// StdioTransport implements Transport using stdin/stdout
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// NewStdioClient creates a new client using stdio transport
func NewStdioClient(command string, args []string) *Client {
	t := &StdioTransport{
		cmd: exec.Command(command, args...),
	}
	return &Client{
		transport: t,
		pending:   make(map[int64]chan Response),
	}
}

func (t *StdioTransport) Start() error {
	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return err
	}
	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	t.stderr, err = t.cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := t.cmd.Start(); err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(t.stderr)
		for scanner.Scan() {
			slog.Debug("MCP Server Stderr", "msg", scanner.Text())
		}
	}()

	return nil
}

func (t *StdioTransport) Close() error {
	if t.cmd.Process != nil {
		return t.cmd.Process.Kill()
	}
	return nil
}

func (t *StdioTransport) WriteRequest(req []byte) error {
	_, err := t.stdin.Write(append(req, '\n'))
	return err
}

func (t *StdioTransport) ReadLoop(handler func(line []byte)) {
	scanner := bufio.NewScanner(t.stdout)
	for scanner.Scan() {
		handler(scanner.Bytes())
	}
}

// SSETransport implements Transport using Server-Sent Events / Streamable HTTP
type SSETransport struct {
	url             string
	messageEndpoint string
	client          *http.Client
	closer          chan struct{}
}

// NewSSEClient creates a new client using SSE transport
func NewSSEClient(url string) *Client {
	t := &SSETransport{
		url:    url,
		client: &http.Client{},
		closer: make(chan struct{}),
	}
	return &Client{
		transport: t,
		pending:   make(map[int64]chan Response),
	}
}

func (t *SSETransport) Start() error {
	req, err := http.NewRequest("GET", t.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// We'll read from resp.Body in ReadLoop.
	// We need the session ID or message endpoint if provided by the server.
	// For now, assume message endpoint is just the URL + "/messages" or similar
	// if not specified by a specialized handshake.
	t.messageEndpoint = t.url + "/messages"

	return nil
}

func (t *SSETransport) Close() error {
	close(t.closer)
	return nil
}

func (t *SSETransport) WriteRequest(req []byte) error {
	httpReq, err := http.NewRequest("POST", t.messageEndpoint, bytes.NewReader(req))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send message: %d", resp.StatusCode)
	}

	return nil
}

func (t *SSETransport) ReadLoop(handler func(line []byte)) {
	// Re-get connection since it was opened in Start()
	// Actually, Start() should probably store the body or we should re-open here.
	// For simplicity in this implementation, let's re-open.
	req, _ := http.NewRequest("GET", t.url, nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := t.client.Do(req)
	if err != nil {
		slog.Error("SSE ReadLoop failed to connect", "error", err)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			handler([]byte(strings.TrimPrefix(line, "data: ")))
		}
		select {
		case <-t.closer:
			return
		default:
		}
	}
}

// Start starts the server transport and begins listening for messages
func (c *Client) Start() error {
	if err := c.transport.Start(); err != nil {
		return err
	}

	// Start reading messages in background
	go c.transport.ReadLoop(c.handleMessage)

	return nil
}

// handleMessage processes a message from the transport
func (c *Client) handleMessage(line []byte) {
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
		return
	}
	// TODO: Handle notifications
}

// Close terminates the connection
func (c *Client) Close() error {
	return c.transport.Close()
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

	// Write request
	if err := c.transport.WriteRequest(reqBytes); err != nil {
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
	if err := c.transport.WriteRequest(notifBytes); err != nil {
		slog.Warn("Failed to send initialized notification", "error", err)
	}

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

// ListResources fetches available resources
func (c *Client) ListResources() ([]Resource, error) {
	resultRaw, err := c.SendRequest("resources/list", map[string]string{})
	if err != nil {
		return nil, err
	}

	var result ListResourcesResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return nil, err
	}

	return result.Resources, nil
}

// ReadResource reads a specific resource
func (c *Client) ReadResource(uri string) ([]ResourceContent, error) {
	params := ReadResourceParams{
		URI: uri,
	}

	resultRaw, err := c.SendRequest("resources/read", params)
	if err != nil {
		return nil, err
	}

	var result ReadResourceResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return nil, err
	}

	return result.Contents, nil
}
