package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"wabridge/internal/api"
	"wabridge/internal/feature"
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
	featureCfg, err := feature.NewConfig(accessLevel, features)
	if err != nil {
		return err
	}

	rt, err := newRuntime(bridgeSessionDB, bridgeMediaDir)
	if err != nil {
		return err
	}
	defer rt.Store.Close()

	rt.handleShutdown()

	apiServer := api.NewAPIServer(rt.Backend, bridgeAddr, featureCfg)
	fmt.Fprintf(os.Stderr, "Bridge listening on %s\n", bridgeAddr)
	return apiServer.Start()
}
