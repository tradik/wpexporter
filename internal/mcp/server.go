package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Version is the MCP protocol version supported
const Version = "2024-11-05"

// Server implements an MCP server over stdio
type Server struct {
	name        string
	version     string
	tools       map[string]ToolHandler
	mu          sync.RWMutex
	initialized bool
	reader      *bufio.Reader
	writer      io.Writer
}

// ToolHandler is a function that handles a tool call
type ToolHandler func(args map[string]interface{}) (*CallToolResult, error)

// NewServer creates a new MCP server bound to standard input/output.
func NewServer(name, version string) *Server {
	return NewServerWithIO(name, version, os.Stdin, os.Stdout)
}

// NewServerWithIO creates a new MCP server with custom IO. NewServer delegates
// here so both share a single construction path.
func NewServerWithIO(name, version string, reader io.Reader, writer io.Writer) *Server {
	return &Server{
		name:    name,
		version: version,
		tools:   make(map[string]ToolHandler),
		reader:  bufio.NewReader(reader),
		writer:  writer,
	}
}

// RegisterTool registers a tool with the server
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = handler
}

// Run starts the MCP server and processes requests
func (s *Server) Run() error {
	for {
		line, err := s.reader.ReadBytes('\n')
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read request: %w", err)
		}

		if len(line) == 0 || (len(line) == 1 && line[0] == '\n') {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, ErrCodeParseError, "Parse error", err.Error())
			continue
		}

		s.handleRequest(&req)
	}
}

// handleRequest processes a single request
func (s *Server) handleRequest(req *Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
		s.initialized = true
	case "tools/list":
		s.handleListTools(req)
	case "tools/call":
		s.handleCallTool(req)
	case "ping":
		s.handlePing(req)
	default:
		s.sendError(req.ID, ErrCodeMethodNotFound, "Method not found", req.Method)
	}
}

// handleInitialize processes the initialize request
func (s *Server) handleInitialize(req *Request) {
	result := InitializeResult{
		ProtocolVersion: Version,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
		Instructions: "wpexporter MCP server provides WordPress content export functionality. " +
			"Use list_formats to see available export formats, export_site to start an export, " +
			"and get_export_status to check progress.",
	}
	s.sendResult(req.ID, result)
}

// handlePing responds to ping requests
func (s *Server) handlePing(req *Request) {
	s.sendResult(req.ID, map[string]interface{}{})
}

// handleListTools returns the list of available tools
func (s *Server) handleListTools(req *Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := GetTools()
	result := ListToolsResult{
		Tools: tools,
	}
	s.sendResult(req.ID, result)
}

// handleCallTool executes a tool
func (s *Server) handleCallTool(req *Request) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	s.mu.RLock()
	handler, exists := s.tools[params.Name]
	s.mu.RUnlock()

	if !exists {
		s.sendError(req.ID, ErrCodeInvalidParams, "Unknown tool", params.Name)
		return
	}

	result, err := handler(params.Arguments)
	if err != nil {
		s.sendResult(req.ID, &CallToolResult{
			Content: []Content{ErrorContent(err)},
			IsError: true,
		})
		return
	}

	s.sendResult(req.ID, result)
}

// sendResult sends a successful response
func (s *Server) sendResult(id json.RawMessage, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.writeResponse(resp)
}

// sendError sends an error response
func (s *Server) sendError(id json.RawMessage, code int, message string, data interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.writeResponse(resp)
}

// writeResponse writes a response to stdout
func (s *Server) writeResponse(resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(s.writer, "%s\n", data)
}
