// greve — a catalogue of dept44 microservices, served as a CLI and an MCP
// server. It scans locally cloned repos and answers: what does each service
// do, who talks to whom, and what version of what is where.
package main

import (
	"fmt"
	"os"

	"github.com/CheeziCrew/greve/cli"
)

func main() {
	if err := cli.BuildCLI().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
