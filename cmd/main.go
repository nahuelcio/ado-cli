package main

import (
	"github.com/nahuelcio/ado-cli/internal/cli"
)

var version = "dev"

func main() {
	cli.SetVersion(version)
	cli.Execute()
}
