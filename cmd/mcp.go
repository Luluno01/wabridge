package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"wabridge/internal/api"
	"wabridge/internal/feature"
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
	localCfg, err := feature.NewConfig(accessLevel, features)
	if err != nil {
		return err
	}

	appStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open app store: %w", err)
	}
	defer appStore.Close()

	apiClient := api.NewAPIClient(bridgeURL)

	// Pull feature config from the bridge and intersect with local config.
	// On failure (bridge unreachable), fall back to local config only.
	remoteCfg, err := apiClient.GetFeatures()
	effectiveCfg := localCfg
	if err == nil {
		effectiveCfg = feature.Intersect(remoteCfg, localCfg)
	}

	mcpServer := mcp.NewServer(appStore, apiClient, effectiveCfg)
	return mcpServer.ServeStdio()
}
