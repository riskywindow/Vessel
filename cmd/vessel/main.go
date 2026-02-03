// Package main is the entrypoint for the Vessel CLI.
package main

import "github.com/vessel/vessel/internal/cli"

// Version information, set via ldflags during build.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	cli.Execute()
}
