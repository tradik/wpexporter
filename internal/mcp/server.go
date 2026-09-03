package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// instructions is the guidance both eras hand a client: server/discover carries
// it in the modern era, initialize in the legacy one.
const instructions = "wpexporter MCP server provides WordPress content export functionality. " +
	"Use list_formats to see available export formats, export_site to start an export, " +
	"and get_export_status to check progress."

// Server implements an MCP server over stdio, in either protocol era.
//
// Which era a request is answered under is decided per request, from the
// request itself: one carrying `_meta.io.modelcontextprotocol/protocolVersion`
// is modern, and `initialize` opens the legacy handshake. The spec allows a
// dual-era server to serve both on one process, and that is the default here —
// narrow it with SetProtocols when an operator wants only one.
type Server struct {
	name        string
	version     string
	versions    VersionSet
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
		name:     name,
		version:  version,
		versions: AllVersions(),
		tools:    make(map[string]ToolHandler),
		reader:   bufio.NewReader(reader),
		writer:   writer,
	}
}

// SetProtocols narrows the protocol revisions this server answers for. The
// default is every revision it implements, in both eras.
func (s *Server) SetProtocols(versions VersionSet) {
	s.versions = versions
}

// Protocols reports the revisions this server answers for.
func (s *Server) Protocols() VersionSet {
	return s.versions
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

// handleRequest routes one request to the era it was written in.
//
// The order matters. A request that declares a protocol version is modern and
// is answered under those rules, including the version error a dual-era client
// reads as proof that the server is modern. Everything else falls to the
// handshake era — which is also how a server pinned to --protocol legacy stays
// indistinguishable from one built before the split, so a probing client falls
// back instead of trying to negotiate a version it will never get.
func (s *Server) handleRequest(req *Request) {
	if s.handleNotification(req.Method) {
		return
	}

	if s.versions.SpeaksModern() && s.isModernRequest(req) {
		s.handleModern(req)
		return
	}

	if s.versions.SpeaksLegacy() {
		s.handleLegacy(req)
		return
	}

	// A legacy client reached a server that has no handshake to offer it. It
	// has no way to ask again, so the one diagnostic it can show a user names
	// what this server does speak.
	s.sendError(req.ID, ErrCodeMethodNotFound,
		"This server speaks only per-request-versioned MCP ("+s.versions.Describe()+"); "+
			"initialize is not part of it",
		map[string]interface{}{"supportedVersions": s.versions.Modern()})
}

// isModernRequest decides which era a request was written in.
//
// A request that declares a protocol version is modern, and so is
// server/discover, which exists only in that era. `initialize` never is: it is
// the handshake by definition, and answering it with an era error is how a
// modern-only server tells a legacy client what it does speak. Anything else on
// a server with no legacy era left is read as modern, so a request missing the
// metadata this era requires is answered as malformed rather than as a method
// that does not exist.
func (s *Server) isModernRequest(req *Request) bool {
	switch req.Method {
	case "initialize":
		return false
	case methodDiscover:
		return true
	}

	if _, isModern, _ := readModernMeta(req.Params); isModern {
		return true
	}

	return !s.versions.SpeaksLegacy()
}

// handleNotification consumes the one-way messages, which are never answered.
// It reports whether the method was one.
func (s *Server) handleNotification(method string) bool {
	switch method {
	case "initialized", "notifications/initialized":
		s.initialized = true
		return true
	case methodCancelled:
		return true
	default:
		return false
	}
}
