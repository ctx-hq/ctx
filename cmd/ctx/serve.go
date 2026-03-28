package main

import (
	"github.com/ctx-hq/ctx/internal/mcpserver"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run ctx as an MCP server",
	Long: `Start ctx as a Model Context Protocol (MCP) server over stdio.

This allows AI agents to use ctx tools directly:
  - ctx_search: Search for packages
  - ctx_install: Install packages
  - ctx_info: Get package details
  - ctx_list: List installed packages

Add to your agent's MCP config:
  {
    "mcpServers": {
      "ctx": {
        "command": "ctx",
        "args": ["serve"]
      }
    }
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server := mcpserver.New()
		return server.Serve()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
