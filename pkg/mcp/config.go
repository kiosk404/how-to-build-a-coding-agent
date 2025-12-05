package mcp

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the structure of the ~/.claude.json file.
type Config struct {
	MCPServers map[string]MCPServer `json:"mcpServers"`
}

// MCPServer represents a single MCP server configuration.
type MCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	Type    string            `json:"type,omitempty"`    // "stdio" (default) or "sse"
	URL     string            `json:"url,omitempty"`     // For SSE
	Headers map[string]string `json:"headers,omitempty"` // For SSE
}

// LoadConfig loads the MCP configuration from the specified path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}
