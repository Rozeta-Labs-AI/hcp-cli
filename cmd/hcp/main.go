package main

import (
	"fmt"
	"os"

	"github.com/Rozeta-Labs-AI/hcp-cli/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitCode(err))
	}
}
