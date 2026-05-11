package main

import (
	"fmt"
	"os"

	"github.com/ht/multi_agent/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version.String())
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println(`agentctl is a CLI for running multi-agent workflows.

Usage:
  agentctl version
  agentctl help`)
}
