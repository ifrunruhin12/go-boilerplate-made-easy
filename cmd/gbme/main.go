package main

import (
	"fmt"
	"go-boiletplate-made-easy/internal/cli"
	"os"
)

var version = "0.1.0"

func main() {
	if err := cli.New(version, os.Stdin, os.Stdout).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gbme:", err)
		os.Exit(1)
	}
}
