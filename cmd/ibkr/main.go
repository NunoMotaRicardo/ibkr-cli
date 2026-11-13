// Command ibkr is a token-lean CLI for Interactive Brokers' MCP connector.
package main

import (
	"fmt"
	"os"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/cli"
)

func main() {
	app, err := cli.NewDefault()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	os.Exit(app.Execute(os.Args[1:]))
}
