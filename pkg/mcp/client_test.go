package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseToolName(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedServer string
		expectedTool   string
		expectError    bool
	}{
		{
			name:           "valid name",
			input:          "server__tool",
			expectedServer: "server",
			expectedTool:   "tool",
			expectError:    false,
		},
		{
			name:        "missing separator",
			input:       "tool",
			expectError: true,
		},
		{
			name:        "too many separators",
			input:       "server__tool__extra",
			expectError: true,
		},
		{
			name:        "empty",
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, tool, err := parseToolName(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedServer, server)
				assert.Equal(t, tt.expectedTool, tool)
			}
		})
	}
}
