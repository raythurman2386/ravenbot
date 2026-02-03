package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/raythurman2386/ravenbot/internal/config"
	"github.com/raythurman2386/ravenbot/internal/db"
	"github.com/raythurman2386/ravenbot/internal/mcp"
	"github.com/raythurman2386/ravenbot/internal/tools"
	"google.golang.org/adk/session"
)

// SharedResources holds resources shared across all agents
type SharedResources struct {
	Config         *config.Config
	DB             *db.DB
	MCPClients     map[string]*mcp.Client
	BrowserManager *tools.BrowserManager
	SessionService session.Service
}

// Manager handles the lifecycle of multiple agents
type Manager struct {
	SharedResources *SharedResources
	agents          map[string]*Agent
	mu              sync.RWMutex
}

// NewManager initializes shared resources and returns a Manager
func NewManager(ctx context.Context, cfg *config.Config, database *db.DB) (*Manager, error) {
	// 1. Initialize MCP Servers
	mcpClients := make(map[string]*mcp.Client)
	var mu sync.Mutex // Mutex for mcpClients map population
	var wg sync.WaitGroup

	for name, serverCfg := range cfg.MCPServers {
		wg.Add(1)
		go func(name string, serverCfg config.MCPServerConfig) {
			defer wg.Done()
			slog.Info("Initializing MCP Server", "name", name, "command", serverCfg.Command)
			var mcpClient *mcp.Client
			if strings.HasPrefix(serverCfg.Command, "http://") || strings.HasPrefix(serverCfg.Command, "https://") {
				mcpClient = mcp.NewSSEClient(serverCfg.Command)
			} else {
				mcpClient = mcp.NewStdioClient(serverCfg.Command, serverCfg.Args)
			}

			if err := mcpClient.Start(); err != nil {
				slog.Error("Failed to start MCP server", "name", name, "error", err)
				return
			}

			if err := mcpClient.Initialize(); err != nil {
				slog.Error("Failed to initialize MCP server", "name", name, "error", err)
				mcpClient.Close()
				return
			}

			mu.Lock()
			mcpClients[name] = mcpClient
			mu.Unlock()
		}(name, serverCfg)
	}
	wg.Wait()

	// 2. Initialize Browser Manager
	browserManager := tools.NewBrowserManager(ctx)

	// 3. Initialize Session Service
	sessionService := session.InMemoryService()

	resources := &SharedResources{
		Config:         cfg,
		DB:             database,
		MCPClients:     mcpClients,
		BrowserManager: browserManager,
		SessionService: sessionService,
	}

	return &Manager{
		SharedResources: resources,
		agents:          make(map[string]*Agent),
	}, nil
}

// Spawn creates and registers a new agent
func (m *Manager) Spawn(ctx context.Context, name, systemPrompt string) (*Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[name]; exists {
		return nil, fmt.Errorf("agent %s already exists", name)
	}

	// New function will be available after refactoring agent.go
	agent, err := New(ctx, name, systemPrompt, m.SharedResources)
	if err != nil {
		return nil, err
	}

	m.agents[name] = agent
	return agent, nil
}

// Get retrieves an agent by name
func (m *Manager) Get(name string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", name)
	}
	return agent, nil
}

// List returns a list of active agent names
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.agents))
	for name := range m.agents {
		names = append(names, name)
	}
	return names
}

// Close cleans up shared resources
func (m *Manager) Close() {
	if m.SharedResources.BrowserManager != nil {
		m.SharedResources.BrowserManager.Close()
	}
	for _, client := range m.SharedResources.MCPClients {
		client.Close()
	}
}
