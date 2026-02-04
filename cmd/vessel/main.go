// Package main is the entrypoint for the Vessel CLI.
package main

import (
	"fmt"
	"os"

	"github.com/vessel/vessel/internal/cli"
	"github.com/vessel/vessel/internal/runtime"
)

// Version information, set via ldflags during build.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Check if this is a container init invocation.
	// This MUST happen before Cobra initialization because the init process
	// runs in a namespace where Cobra's setup might fail.
	if len(os.Args) > 1 && os.Args[1] == "__init__" {
		if err := runtime.ContainerInit(); err != nil {
			fmt.Fprintf(os.Stderr, "container init failed: %v\n", err)
			os.Exit(1)
		}
		// ContainerInit calls unix.Exec, so we should never reach here
		os.Exit(0)
	}

	cli.Execute()
}
