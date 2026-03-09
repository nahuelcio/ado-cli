package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nahuelcio/ado-cli/internal/cli"
)

var version = "dev"

func main() {
	// Check for version flag before cobra
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("ado version %s\n", version)
		os.Exit(0)
	}

	// Handle -version flag
	flagVersion := flag.Bool("version", false, "Show version")
	flag.Parse()
	if *flagVersion {
		fmt.Printf("ado version %s\n", version)
		os.Exit(0)
	}

	cli.Execute()
}
