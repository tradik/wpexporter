package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	server := NewServer("test-server", "1.0.0")
	assert.NotNil(t, server)
	assert.Equal(t, "test-server", server.name)
	assert.Equal(t, "1.0.0", server.version)
}

func TestNewServerWithIO(t *testing.T) {
	input := strings.NewReader("")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)
	assert.NotNil(t, server)
}

func TestRegisterTool(t *testing.T) {
	server := NewServer("test", "1.0")
	tool := Tool{
		Name:        "test_tool",
		Description: "Test tool",
		InputSchema: InputSchema{Type: "object"},
	}
	handler := func(args map[string]interface{}) (*CallToolResult, error) {
		return &CallToolResult{Content: []Content{TextContent("ok")}}, nil
	}

	server.RegisterTool(tool, handler)

	server.mu.RLock()
	_, exists := server.tools["test_tool"]
	server.mu.RUnlock()

	assert.True(t, exists)
}

func TestHandleInitialize(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("wpexporter", "1.0.0", input, output)

	// Run server (will process one request and exit on EOF)
	err := server.Run()
	require.NoError(t, err)

	// Parse response
	var resp Response
	err = json.Unmarshal(output.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	// The handshake answers with the revision the client asked for, not with
	// whichever one this build prefers.
	assert.Equal(t, Version20241105, result["protocolVersion"])
}

func TestHandlePing(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	err := server.Run()
	require.NoError(t, err)

	var resp Response
	err = json.Unmarshal(output.Bytes(), &resp)
	require.NoError(t, err)

	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestHandleListTools(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)
	RegisterAllTools(server)

	err := server.Run()
	require.NoError(t, err)

	var resp Response
	err = json.Unmarshal(output.Bytes(), &resp)
	require.NoError(t, err)

	assert.Nil(t, resp.Error)
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	tools, ok := result["tools"].([]interface{})
	require.True(t, ok)
	assert.Greater(t, len(tools), 0)
}

func TestHandleCallToolUnknown(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"unknown_tool"}}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	err := server.Run()
	require.NoError(t, err)

	var resp Response
	err = json.Unmarshal(output.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeInvalidParams, resp.Error.Code)
}

func TestHandleMethodNotFound(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"unknown/method"}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	err := server.Run()
	require.NoError(t, err)

	var resp Response
	err = json.Unmarshal(output.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeMethodNotFound, resp.Error.Code)
}

func TestHandleParseError(t *testing.T) {
	input := bytes.NewBufferString(`{invalid json}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	err := server.Run()
	require.NoError(t, err)

	var resp Response
	err = json.Unmarshal(output.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeParseError, resp.Error.Code)
}

func TestHandleInvalidParams(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"invalid"}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	err := server.Run()
	require.NoError(t, err)

	var resp Response
	err = json.Unmarshal(output.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeInvalidParams, resp.Error.Code)
}

func TestHandleToolCall(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool","arguments":{"value":"hello"}}}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	tool := Tool{
		Name:        "test_tool",
		Description: "Test tool",
		InputSchema: InputSchema{Type: "object"},
	}
	handler := func(args map[string]interface{}) (*CallToolResult, error) {
		val, _ := args["value"].(string)
		return &CallToolResult{
			Content: []Content{TextContent("received: " + val)},
		}, nil
	}
	server.RegisterTool(tool, handler)

	err := server.Run()
	require.NoError(t, err)

	var resp Response
	err = json.Unmarshal(output.Bytes(), &resp)
	require.NoError(t, err)

	assert.Nil(t, resp.Error)
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	content, ok := result["content"].([]interface{})
	require.True(t, ok)
	assert.Len(t, content, 1)
}

func TestHandleToolCallWithError(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"error_tool"}}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	tool := Tool{
		Name:        "error_tool",
		Description: "Tool that returns error",
		InputSchema: InputSchema{Type: "object"},
	}
	handler := func(args map[string]interface{}) (*CallToolResult, error) {
		return nil, assert.AnError
	}
	server.RegisterTool(tool, handler)

	err := server.Run()
	require.NoError(t, err)

	var resp Response
	err = json.Unmarshal(output.Bytes(), &resp)
	require.NoError(t, err)

	assert.Nil(t, resp.Error)
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	isError, ok := result["isError"].(bool)
	require.True(t, ok)
	assert.True(t, isError)
}

func TestHandleInitializedNotification(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","method":"initialized"}` + "\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	err := server.Run()
	require.NoError(t, err)

	// Notifications don't get a response
	assert.Empty(t, output.String())
	assert.True(t, server.initialized)
}

func TestMultipleRequests(t *testing.T) {
	requests := `{"jsonrpc":"2.0","id":1,"method":"ping"}
{"jsonrpc":"2.0","id":2,"method":"ping"}
{"jsonrpc":"2.0","id":3,"method":"ping"}
`
	input := bytes.NewBufferString(requests)
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	err := server.Run()
	require.NoError(t, err)

	// Should have 3 response lines
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	assert.Len(t, lines, 3)
}

func TestEmptyLines(t *testing.T) {
	input := bytes.NewBufferString("\n\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n\n")
	output := &bytes.Buffer{}
	server := NewServerWithIO("test", "1.0", input, output)

	err := server.Run()
	require.NoError(t, err)

	// Should still get exactly one response
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	assert.Len(t, lines, 1)
}

// TestVersion: the preferred revision is the newest one implemented, and the
// oldest is still spelled out so a client pinned to it keeps working.
func TestVersion(t *testing.T) {
	assert.Equal(t, "2026-07-28", Version)
	assert.Equal(t, "2024-11-05", Version20241105)
}
