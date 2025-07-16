package cmd

import (
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "anduril",
    Short: "Anduril media organizer",
}

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    // No args for root command, only subcommands
}
