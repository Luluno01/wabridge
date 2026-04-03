package cmd

import (
	"github.com/spf13/cobra"

	"wabridge/internal/mcp"
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
	rt, err := newRuntime(standaloneSessionDB, standaloneMediaDir)
	if err != nil {
		return err
	}
	defer rt.Store.Close()

	rt.handleShutdown()

	mcpServer := mcp.NewServer(rt.Store, rt.Backend)
	return mcpServer.ServeStdio()
}
