package mcp

import "encoding/json"

// handleLegacy answers a request under the handshake era: the client stated its
// protocol version once, in initialize, and every later request is read in that
// light.
func (s *Server) handleLegacy(req *Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleListTools(req, s.sendResult)
	case "tools/call":
		s.handleCallTool(req, s.sendResult)
	case "ping":
		s.handlePing(req, s.sendResult)
	default:
		s.sendError(req.ID, ErrCodeMethodNotFound, "Method not found", req.Method)
	}
}

// handleInitialize answers the handshake with the revision the client asked for
// whenever this server implements it.
//
// It used to answer with one hardcoded revision whatever was asked, which left
// every client pinned to the oldest MCP ever published. A legacy client has no
// way to ask a second time, so an unimplemented request is answered with the
// newest revision this server does implement rather than with an error.
func (s *Server) handleInitialize(req *Request) {
	var params InitializeParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
			return
		}
	}

	s.sendResult(req.ID, InitializeResult{
		ProtocolVersion: s.versions.NegotiateLegacy(params.ProtocolVersion),
		Capabilities:    s.capabilities(),
		ServerInfo:      s.serverInfo(),
		Instructions:    instructions,
	})
}
