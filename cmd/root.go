package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version info can be overridden at build time with:
// go build -ldflags "-X anduril/cmd.Version=1.2.3 -X anduril/cmd.Commit=abc123 -X 'anduril/cmd.Date=2025-01-01'"
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

var rootCmd = &cobra.Command{
	Use:   "anduril",
	Short: "Anduril media organizer",
}

// printVersionLine writes a single-line version header.
func printVersionLine(cmd *cobra.Command) {
	v := cmd.Version
	if v == "" {
		v = Version
	}
	line := "Anduril version " + v
	if commit := cmd.Annotations["commit"]; commit != "" {
		line += " (commit " + commit + ")"
	}
	if date := cmd.Annotations["date"]; date != "" {
		line += " built " + date
	}
	line += "\n"
	fmt.Fprint(cmd.OutOrStdout(), line)
}

// skipVersionPrint returns true when the --version flag is explicitly requested.
func skipVersionPrint(cmd *cobra.Command) bool {
	if f := cmd.Flags().Lookup("version"); f != nil && f.Changed {
		return true
	}
	if f := cmd.InheritedFlags().Lookup("version"); f != nil && f.Changed {
		return true
	}
	return false
}

// ApplyVersion sets the version and annotation metadata on the root command.
func ApplyVersion() {
	rootCmd.Version = Version
	if rootCmd.Annotations == nil {
		rootCmd.Annotations = map[string]string{}
	}
	rootCmd.Annotations["commit"] = Commit
	rootCmd.Annotations["date"] = Date
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// No args for root command, only subcommands
	ApplyVersion()
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if skipVersionPrint(cmd) {
			return
		}
		printVersionLine(cmd)
	}
	rootCmd.SetVersionTemplate("Anduril version {{.Version}}{{if .Annotations.commit}} (commit {{.Annotations.commit}}){{end}}{{if .Annotations.date}} built {{.Annotations.date}}{{end}}\n")
}
