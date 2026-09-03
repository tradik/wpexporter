// Package main implements the MCP server for wpexporter.
// This server enables AI assistants like Claude to interact with WordPress export functionality.
package mcpcli

import (
	"strings"

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

The server communicates over stdio using JSON-RPC 2.0, and speaks both MCP
eras: the current per-request-versioned revision (2026-07-28) and the older
initialize handshake (2025-11-25 and earlier). A client of either kind is
served without being told which to use; --protocol narrows that when one has
to be pinned.

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

// protocolFlag holds the operator's --protocol choice. Empty means every
// revision this build implements, in both eras.
var protocolFlag string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Start the MCP server and listen for requests on stdio.

The server answers both MCP eras at once, deciding per request which one it is
being addressed in, so no client has to be configured for it.

--protocol pins that when it has to be pinned:

  all      both eras (default) — every revision below
  modern   ` + mcp.Version20260728 + ` only: per-request versioning, server/discover
  legacy   the initialize handshake only, exactly as a pre-2026 server looks
  <date>   one revision, e.g. ` + mcp.Version20241105 + `, and nothing else

Pinning to one era is a compatibility tool, not a hardening one: a client that
probes for the other era is told plainly that this server does not speak it.`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&protocolFlag, "protocol", "",
		"MCP protocol revisions to answer for: all, modern, legacy, or one of "+
			strings.Join(mcp.KnownVersions(), ", "))
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	versions, err := mcp.ParseVersionSet(protocolFlag)
	if err != nil {
		return err
	}

	server := mcp.NewServer("wpexporter", version.Version)
	server.SetProtocols(versions)
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
