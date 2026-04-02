package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	dbPath   string
	logLevel string
)

var rootCmd = &cobra.Command{
	Use:   "wabridge",
	Short: "WhatsApp MCP bridge",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "messages.db", "path to SQLite database")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
