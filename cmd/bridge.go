package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"wabridge/internal/api"
	"wabridge/internal/mcp"
	"wabridge/internal/store"
	"wabridge/internal/whatsapp"
)

var (
	bridgeAddr      string
	bridgeSessionDB string
	bridgeMediaDir  string
)

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "Persistent WhatsApp bridge daemon with REST API",
	RunE:  runBridge,
}

func init() {
	bridgeCmd.Flags().StringVar(&bridgeAddr, "addr", ":8080", "address to listen on for REST API")
	bridgeCmd.Flags().StringVar(&bridgeSessionDB, "session-db", "whatsapp.db", "path to WhatsApp session database")
	bridgeCmd.Flags().StringVar(&bridgeMediaDir, "media-dir", "media", "directory for downloaded media files")
	rootCmd.AddCommand(bridgeCmd)
}

func runBridge(cmd *cobra.Command, args []string) error {
	// Open application store
	appStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open app store: %w", err)
	}
	defer appStore.Close()

	// Create WhatsApp client
	waClient, err := whatsapp.NewClient(bridgeSessionDB, appStore, logLevel)
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
	absMediaDir, err := filepath.Abs(bridgeMediaDir)
	if err != nil {
		return fmt.Errorf("failed to resolve media dir: %w", err)
	}

	// Create direct backend
	backend := mcp.NewDirectBackend(waClient, appStore, absMediaDir)

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

	// Create and start API server (blocks)
	apiServer := api.NewAPIServer(backend, bridgeAddr)
	fmt.Fprintf(os.Stderr, "Bridge listening on %s\n", bridgeAddr)
	return apiServer.Start()
}
