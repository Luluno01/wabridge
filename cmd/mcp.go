package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"wabridge/internal/api"
	"wabridge/internal/mcp"
	"wabridge/internal/store"
)

var bridgeURL string

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Ephemeral MCP stdio server (reads SQLite, calls bridge REST API for actions)",
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().StringVar(&bridgeURL, "bridge-url", "http://localhost:8080", "URL of the bridge REST API")
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	// Open application store (read-only queries)
	appStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open app store: %w", err)
	}
	defer appStore.Close()

	// Create API client as the action backend
	apiClient := api.NewAPIClient(bridgeURL)

	// Create MCP server
	mcpServer := mcp.NewServer(appStore, apiClient)

	// Serve MCP over stdio (blocks)
	return mcpServer.ServeStdio()
}
