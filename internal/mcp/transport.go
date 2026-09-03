package mcp

import (
	"encoding/json"
	"fmt"
)

// resultSender is how one era writes a successful result. The two differ only
// in the envelope, so every handler takes one rather than knowing which era it
// is serving.
type resultSender func(id json.RawMessage, result interface{})

// sendResult sends a successful response
func (s *Server) sendResult(id json.RawMessage, result interface{}) {
	s.writeResponse(Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

// sendError sends a JSON-RPC error response
func (s *Server) sendError(id json.RawMessage, code int, message string, data interface{}) {
	s.writeResponse(Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}

// writeResponse writes a response to stdout
func (s *Server) writeResponse(resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(s.writer, "%s\n", data)
}
