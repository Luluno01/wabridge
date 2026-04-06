package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	dbPath      string
	logLevel    string
	accessLevel int
	features    string
)

var rootCmd = &cobra.Command{
	Use:   "wabridge",
	Short: "WhatsApp MCP bridge",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "messages.db", "path to SQLite database")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().IntVar(&accessLevel, "access-level", 3, "access level 0-3 (0=read-only, 3=full)")
	rootCmd.PersistentFlags().StringVar(&features, "features", "", "per-feature overrides (+send,-download,+history-sync)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
