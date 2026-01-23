// Package main provides the entry point for the agent CLI.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/felixgeelhaar/agent-go/interfaces/cli"
)

func main() {
	app := cli.New()

	if err := app.Execute(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
