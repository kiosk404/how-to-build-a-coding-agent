package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "claude.json")

	configContent := `{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        "/Users/test/Documents"
      ],
      "env": {
        "TEST_ENV": "value"
      }
    }
  }
}`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Test loading the config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify the loaded config
	assert.Contains(t, config.MCPServers, "filesystem")
	fsServer := config.MCPServers["filesystem"]
	assert.Equal(t, "npx", fsServer.Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/Users/test/Documents"}, fsServer.Args)
	assert.Equal(t, "value", fsServer.Env["TEST_ENV"])
}

func TestLoadConfig_SSE(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "claude_sse.json")

	configContent := `{
  "mcpServers": {
    "remote": {
      "type": "sse",
      "url": "http://localhost:8080/sse",
      "headers": {
        "Authorization": "Bearer token"
      }
    }
  }
}`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Contains(t, config.MCPServers, "remote")
	remoteServer := config.MCPServers["remote"]
	assert.Equal(t, "sse", remoteServer.Type)
	assert.Equal(t, "http://localhost:8080/sse", remoteServer.URL)
	assert.Equal(t, "Bearer token", remoteServer.Headers["Authorization"])
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/non/existent/path.json")
	assert.Error(t, err)
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")
	err := os.WriteFile(configPath, []byte("{invalid json}"), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(configPath)
	assert.Error(t, err)
}
