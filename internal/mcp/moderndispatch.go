package mcp

import "encoding/json"

// handleModern answers a request under the per-request-metadata era: no
// handshake, every request stating its own version, and a mandatory
// server/discover a client can open with instead of guessing.
func (s *Server) handleModern(req *Request) {
	meta, _, err := readModernMeta(req.Params)
	if err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	if meta.ProtocolVersion == "" {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid params",
			"_meta is missing "+metaProtocolVersion)
		return
	}

	if !s.versions.SupportsModern(meta.ProtocolVersion) {
		s.sendUnsupportedVersion(req.ID, meta.ProtocolVersion)
		return
	}

	switch req.Method {
	case methodDiscover:
		s.sendModernResult(req.ID, s.discoverResult())
	case "tools/list":
		s.handleListTools(req, s.sendModernResult)
	case "tools/call":
		s.handleCallTool(req, s.sendModernResult)
	case "ping":
		s.handlePing(req, s.sendModernResult)
	default:
		s.sendError(req.ID, ErrCodeMethodNotFound, "Method not found", req.Method)
	}
}

// discoverResult is what a client learns in one call instead of a handshake
// followed by probing for each kind of thing the server might hold.
func (s *Server) discoverResult() DiscoverResult {
	return DiscoverResult{
		SupportedVersions: s.versions.Modern(),
		Capabilities:      s.capabilities(),
		Instructions:      instructions,
	}
}

// sendUnsupportedVersion names the versions the client can retry with. It is
// also the signal that identifies this server as modern, so a dual-era client
// retries here rather than falling back to the handshake.
func (s *Server) sendUnsupportedVersion(id json.RawMessage, requested string) {
	s.sendError(id, ErrCodeUnsupportedProtocolVersion, "Unsupported protocol version",
		UnsupportedVersionData{
			Supported: s.versions.Modern(),
			Requested: requested,
		})
}

// sendModernResult sends a result in this era's envelope.
func (s *Server) sendModernResult(id json.RawMessage, result interface{}) {
	decorated, err := decorateModernResult(result, s.serverInfo())
	if err != nil {
		s.sendError(id, ErrCodeInternal, "Internal error", err.Error())
		return
	}

	s.sendResult(id, decorated)
}
