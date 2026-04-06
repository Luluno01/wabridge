package mcp

import (
	"wabridge/internal/action"
	"wabridge/internal/feature"
	appstore "wabridge/internal/store"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Server wraps an MCP stdio server with access to the application store
// (for query tools) and an action.Backend (for mutation tools).
type Server struct {
	mcp      *mcpserver.MCPServer
	store    *appstore.Store
	backend  action.Backend
	features feature.Config
}

// NewServer creates a new MCP server with all tools registered.
func NewServer(store *appstore.Store, backend action.Backend, features feature.Config) *Server {
	s := &Server{
		mcp:      mcpserver.NewMCPServer("wabridge", "1.0.0"),
		store:    store,
		backend:  backend,
		features: features,
	}

	s.registerTools()

	return s
}

// ServeStdio starts the MCP server on stdin/stdout.
func (s *Server) ServeStdio() error {
	return mcpserver.ServeStdio(s.mcp)
}
