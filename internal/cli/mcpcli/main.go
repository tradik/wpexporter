// Package main implements the MCP server for wpexporter.
// This server enables AI assistants like Claude to interact with WordPress export functionality.
package mcpcli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/tradik/wpexporter/internal/mcp"
	"github.com/tradik/wpexporter/internal/version"
)

var rootCmd = &cobra.Command{
	Use:     "wpmcp",
	Short:   "MCP server for WordPress content export",
	Version: version.String(),
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
	server := mcp.NewServer("wpexporter", version.Version)
	mcp.RegisterAllTools(server)
	return server.Run()
}

// Execute runs this tool's command tree as a standalone binary.
func Execute() error {
	return rootCmd.Execute()
}

// RootCommand returns the tool's own root, for mounting as a group under the
// umbrella command. The caller renames Use to the group it should answer to.
func RootCommand() *cobra.Command {
	return rootCmd
}

// ServeCommand returns the tool's working subcommand, for mounting directly at
// the umbrella's top level.
func ServeCommand() *cobra.Command {
	return serveCmd
}

// PersistentFlags returns the tree's global flags, bound to this package's
// variables. Mounting a subcommand without them would leave those variables
// unset.
func PersistentFlags() *pflag.FlagSet {
	return rootCmd.PersistentFlags()
}
