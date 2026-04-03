package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"wabridge/internal/action"
	"wabridge/internal/mcp"
	"wabridge/internal/store"
	"wabridge/internal/whatsapp"
)

// runtime holds the shared resources for both bridge and standalone modes.
type runtime struct {
	Store    *store.Store
	WAClient *whatsapp.Client
	Backend  action.Backend
}

// newRuntime creates and connects shared resources used by bridge and standalone.
func newRuntime(sessionDB, mediaDir string) (*runtime, error) {
	appStore, err := store.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open app store: %w", err)
	}

	waClient, err := whatsapp.NewClient(sessionDB, appStore, logLevel)
	if err != nil {
		appStore.Close()
		return nil, fmt.Errorf("failed to create WhatsApp client: %w", err)
	}

	waClient.RegisterHandlers()

	if err := waClient.Connect(context.Background()); err != nil {
		appStore.Close()
		return nil, fmt.Errorf("failed to connect to WhatsApp: %w", err)
	}

	absMediaDir, err := filepath.Abs(mediaDir)
	if err != nil {
		appStore.Close()
		return nil, fmt.Errorf("failed to resolve media dir: %w", err)
	}

	backend := mcp.NewDirectBackend(waClient, appStore, absMediaDir)

	return &runtime{
		Store:    appStore,
		WAClient: waClient,
		Backend:  backend,
	}, nil
}

// handleShutdown sets up a goroutine that listens for SIGINT/SIGTERM and
// cleanly shuts down the runtime.
func (rt *runtime) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		rt.WAClient.Disconnect()
		rt.Store.Close()
		os.Exit(0)
	}()
}
