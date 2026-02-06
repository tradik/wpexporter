// Package main implements the MCP server for wpexporter.
// This server enables AI assistants like Claude to interact with WordPress export functionality.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tradik/wpexporter/internal/mcp"
)

// Version information - set during build
var (
	Version   = "1.5.0"
	BuildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "wpmcp",
	Short:   "MCP server for WordPress content export",
	Version: Version,
	Long: `MCP (Model Context Protocol) server for wpexporter.

This server enables AI assistants like Claude to interact with WordPress
export functionality through the Model Context Protocol.

The server communicates over stdio using JSON-RPC 2.0.

Available tools:
  - list_formats     List available export formats
  - get_site_info    Get WordPress site information
  - list_posts       List posts from a WordPress site
  - list_pages       List pages from a WordPress site
  - list_categories  List categories from a WordPress site
  - list_media       List media files from a WordPress site
  - get_post         Get a specific post by ID
  - export_site      Export WordPress site content

Usage with Claude Desktop:
  Add to claude_desktop_config.json:
  {
    "mcpServers": {
      "wpexporter": {
        "command": "wpmcp",
        "args": ["serve"]
      }
    }
  }

Usage with Claude Code:
  Add to .claude/mcp.json:
  {
    "mcpServers": {
      "wpexporter": {
        "type": "stdio",
        "command": "wpmcp",
        "args": ["serve"]
      }
    }
  }

Docs: https://github.com/tradik/wpexporter`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long:  "Start the MCP server and listen for requests on stdio.",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	server := mcp.NewServer("wpexporter", Version)
	mcp.RegisterAllTools(server)
	return server.Run()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
