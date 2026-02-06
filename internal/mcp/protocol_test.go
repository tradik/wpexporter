package mcp

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextContent(t *testing.T) {
	content := TextContent("hello world")
	assert.Equal(t, "text", content.Type)
	assert.Equal(t, "hello world", content.Text)
	assert.Empty(t, content.MimeType)
	assert.Empty(t, content.Data)
}

func TestErrorContent(t *testing.T) {
	err := errors.New("test error")
	content := ErrorContent(err)
	assert.Equal(t, "text", content.Type)
	assert.Equal(t, "Error: test error", content.Text)
}

func TestRequestSerialization(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "test_method",
		Params:  json.RawMessage(`{"key": "value"}`),
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded Request
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Equal(t, "test_method", decoded.Method)
}

func TestResponseSerialization(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  map[string]string{"status": "ok"},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded Response
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.NotNil(t, decoded.Result)
	assert.Nil(t, decoded.Error)
}

func TestErrorSerialization(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Error: &Error{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
			Data:    "missing required field",
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded Response
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Nil(t, decoded.Result)
	assert.NotNil(t, decoded.Error)
	assert.Equal(t, ErrCodeInvalidParams, decoded.Error.Code)
	assert.Equal(t, "Invalid params", decoded.Error.Message)
}

func TestInitializeResult(t *testing.T) {
	result := InitializeResult{
		ProtocolVersion: Version,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
		Instructions: "Test instructions",
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded InitializeResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, Version, decoded.ProtocolVersion)
	assert.Equal(t, "test-server", decoded.ServerInfo.Name)
	assert.NotNil(t, decoded.Capabilities.Tools)
}

func TestToolDefinition(t *testing.T) {
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"url": {
					Type:        "string",
					Description: "A URL parameter",
				},
				"limit": {
					Type:    "integer",
					Default: 10,
				},
			},
			Required: []string{"url"},
		},
	}

	data, err := json.Marshal(tool)
	require.NoError(t, err)

	var decoded Tool
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "test_tool", decoded.Name)
	assert.Equal(t, "object", decoded.InputSchema.Type)
	assert.Len(t, decoded.InputSchema.Properties, 2)
	assert.Len(t, decoded.InputSchema.Required, 1)
}

func TestCallToolParams(t *testing.T) {
	params := CallToolParams{
		Name: "export_site",
		Arguments: map[string]interface{}{
			"url":    "https://example.com",
			"format": "json",
		},
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	var decoded CallToolParams
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "export_site", decoded.Name)
	assert.Equal(t, "https://example.com", decoded.Arguments["url"])
}

func TestCallToolResult(t *testing.T) {
	result := CallToolResult{
		Content: []Content{
			TextContent("Operation successful"),
		},
		IsError: false,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded CallToolResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Content, 1)
	assert.False(t, decoded.IsError)
}

func TestCallToolResultWithError(t *testing.T) {
	result := CallToolResult{
		Content: []Content{
			ErrorContent(errors.New("something went wrong")),
		},
		IsError: true,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded CallToolResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Content, 1)
	assert.True(t, decoded.IsError)
	assert.Contains(t, decoded.Content[0].Text, "Error:")
}

func TestErrorCodes(t *testing.T) {
	assert.Equal(t, -32700, ErrCodeParseError)
	assert.Equal(t, -32600, ErrCodeInvalidRequest)
	assert.Equal(t, -32601, ErrCodeMethodNotFound)
	assert.Equal(t, -32602, ErrCodeInvalidParams)
	assert.Equal(t, -32603, ErrCodeInternal)
}

func TestListToolsResult(t *testing.T) {
	result := ListToolsResult{
		Tools: []Tool{
			{
				Name:        "tool1",
				Description: "First tool",
				InputSchema: InputSchema{Type: "object"},
			},
			{
				Name:        "tool2",
				Description: "Second tool",
				InputSchema: InputSchema{Type: "object"},
			},
		},
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded ListToolsResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Tools, 2)
}

func TestPropertyWithEnum(t *testing.T) {
	prop := Property{
		Type:        "string",
		Description: "Export format",
		Enum:        []string{"json", "markdown", "shopify"},
		Default:     "json",
	}

	data, err := json.Marshal(prop)
	require.NoError(t, err)

	var decoded Property
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "string", decoded.Type)
	assert.Len(t, decoded.Enum, 3)
	assert.Equal(t, "json", decoded.Default)
}

func TestClientCapabilities(t *testing.T) {
	caps := ClientCapabilities{
		Roots: &RootsCapability{
			ListChanged: true,
		},
		Sampling: &SamplingCapability{},
	}

	data, err := json.Marshal(caps)
	require.NoError(t, err)

	var decoded ClientCapabilities
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.NotNil(t, decoded.Roots)
	assert.True(t, decoded.Roots.ListChanged)
}

func TestInitializeParams(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: Version,
		Capabilities: ClientCapabilities{
			Roots: &RootsCapability{ListChanged: true},
		},
		ClientInfo: ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	var decoded InitializeParams
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, Version, decoded.ProtocolVersion)
	assert.Equal(t, "test-client", decoded.ClientInfo.Name)
}
