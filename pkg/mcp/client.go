package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ollama/ollama/api"
)

const ToolTypeFunction = "function"

// Client manages connections to multiple MCP servers.
type Client struct {
	sessions map[string]*mcp.ClientSession
}

// NewClient creates a new MCP client and connects to the servers defined in the config.
func NewClient(ctx context.Context, config *Config) (*Client, error) {
	c := &Client{
		sessions: make(map[string]*mcp.ClientSession),
	}

	for name, server := range config.MCPServers {
		if err := c.connectToServer(ctx, name, server); err != nil {
			// Log error but continue connecting to other servers
			fmt.Fprintf(os.Stderr, "Failed to connect to MCP server %s: %v\n", name, err)
		}
	}

	return c, nil
}

func (c *Client) connectToServer(ctx context.Context, name string, server MCPServer) error {
	var transport mcp.Transport

	if server.Type == "sse" {
		sseTransport := &mcp.SSEClientTransport{
			Endpoint: server.URL,
		}
		if len(server.Headers) > 0 {
			sseTransport.HTTPClient = &http.Client{
				Transport: &headerTransport{
					Transport: http.DefaultTransport,
					Headers:   server.Headers,
				},
			}
		}
		transport = sseTransport
	} else {
		// Default to stdio
		cmd := exec.Command(server.Command, server.Args...)
		cmd.Env = os.Environ()
		for k, v := range server.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		// Capture stderr for debugging
		cmd.Stderr = os.Stderr

		transport = &mcp.CommandTransport{
			Command: cmd,
		}
	}

	mcpClient := mcp.NewClient(&mcp.Implementation{
		Name:    "goskills",
		Version: "0.1.0",
	}, nil)

	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.sessions[name] = session
	return nil
}

type headerTransport struct {
	Transport http.RoundTripper
	Headers   map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}
	return t.Transport.RoundTrip(req)
}

// Close closes all connections.
func (c *Client) Close() error {
	var errs []error
	for _, session := range c.sessions {
		if err := session.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to close some connections: %v", errs)
	}
	return nil
}

// GetTools fetches tools from all connected servers and converts them to OpenAI tools.
func (c *Client) GetTools(ctx context.Context) ([]api.Tool, error) {
	var allTools []api.Tool

	for serverName, session := range c.sessions {
		listToolsResult, err := session.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list tools from server %s: %v\n", serverName, err)
			continue
		}

		for _, tool := range listToolsResult.Tools {
			openaiTool := api.Tool{
				Type: ToolTypeFunction,
				Function: api.ToolFunction{
					Name:        fmt.Sprintf("%s__%s", serverName, tool.Name),
					Description: tool.Description,
					Parameters:  convertToOllamaParameters(tool.InputSchema),
				},
			}
			allTools = append(allTools, openaiTool)
		}
	}

	return allTools, nil
}

// CallTool calls a tool on the appropriate server.
// The tool name is expected to be in the format "serverName__toolName".
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	serverName, toolName, err := parseToolName(name)
	if err != nil {
		return nil, err
	}

	session, ok := c.sessions[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}

	return result, nil
}

func parseToolName(name string) (string, string, error) {
	parts := strings.Split(name, "__")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tool name format: %s", name)
	}
	return parts[0], parts[1], nil
}

func convertToOllamaParameters(inputScheme interface{}) api.ToolFunctionParameters {
	var params api.ToolFunctionParameters

	data, err := json.Marshal(inputScheme)
	if err != nil {
		return api.ToolFunctionParameters{
			Type:       "object",
			Properties: make(map[string]api.ToolProperty),
		}
	}

	if err := json.Unmarshal(data, &params); err != nil {
		return api.ToolFunctionParameters{
			Type:       "object",
			Properties: make(map[string]api.ToolProperty),
		}
	}

	return params
}
