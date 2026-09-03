package mcp

import "encoding/json"

// handlePing answers the liveness check, whose result is empty in both eras.
func (s *Server) handlePing(req *Request, send resultSender) {
	send(req.ID, map[string]interface{}{})
}

// handleListTools returns the list of available tools
func (s *Server) handleListTools(req *Request, send resultSender) {
	send(req.ID, ListToolsResult{Tools: GetTools()})
}

// handleCallTool executes a tool
func (s *Server) handleCallTool(req *Request, send resultSender) {
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
		send(req.ID, &CallToolResult{
			Content: []Content{ErrorContent(err)},
			IsError: true,
		})
		return
	}

	send(req.ID, result)
}

// serverInfo is this build's identity, as both eras report it.
func (s *Server) serverInfo() ServerInfo {
	return ServerInfo{Name: s.name, Version: s.version}
}

// capabilities is what this server can do, as both eras report it.
func (s *Server) capabilities() ServerCapabilities {
	return ServerCapabilities{Tools: &ToolsCapability{ListChanged: false}}
}
