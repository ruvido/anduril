package main

import (
	_ "embed"
	"strings"

	"anduril/cmd"
)

//go:embed VERSION
var embeddedVersion string

func init() {
	v := strings.TrimSpace(embeddedVersion)
	if v != "" && cmd.Version == "dev" {
		cmd.Version = v
	}
	// Re-apply to Cobra command after updating Version.
	cmd.ApplyVersion()
}
