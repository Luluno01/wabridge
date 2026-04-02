package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"wabridge/internal/mcp"
	"wabridge/internal/store"
	"wabridge/internal/whatsapp"
)

var (
	standaloneSessionDB string
	standaloneMediaDir  string
)

var standaloneCmd = &cobra.Command{
	Use:   "standalone",
	Short: "All-in-one mode: WhatsApp connection + MCP server in one process",
	RunE:  runStandalone,
}

func init() {
	standaloneCmd.Flags().StringVar(&standaloneSessionDB, "session-db", "whatsapp.db", "path to WhatsApp session database")
	standaloneCmd.Flags().StringVar(&standaloneMediaDir, "media-dir", "media", "directory for downloaded media files")
	rootCmd.AddCommand(standaloneCmd)
}

func runStandalone(cmd *cobra.Command, args []string) error {
	// Open application store
	appStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open app store: %w", err)
	}
	defer appStore.Close()

	// Create WhatsApp client
	waClient, err := whatsapp.NewClient(standaloneSessionDB, appStore, logLevel)
	if err != nil {
		return fmt.Errorf("failed to create WhatsApp client: %w", err)
	}

	// Register event handlers
	waClient.RegisterHandlers()

	// Connect to WhatsApp
	if err := waClient.Connect(context.Background()); err != nil {
		return fmt.Errorf("failed to connect to WhatsApp: %w", err)
	}

	// Resolve media dir to absolute path
	absMediaDir, err := filepath.Abs(standaloneMediaDir)
	if err != nil {
		return fmt.Errorf("failed to resolve media dir: %w", err)
	}

	// Create direct backend for MCP server
	backend := mcp.NewDirectBackend(waClient, appStore, absMediaDir)

	// Create MCP server
	mcpServer := mcp.NewServer(appStore, backend)

	// Handle SIGINT/SIGTERM for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		waClient.Disconnect()
		appStore.Close()
		os.Exit(0)
	}()

	// Serve MCP over stdio (blocks)
	return mcpServer.ServeStdio()
}
