package main

import (
	"os"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], cli.Stdio{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}))
}
