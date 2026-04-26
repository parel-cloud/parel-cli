package main

import (
	"os"

	"github.com/parel-cloud/parel-cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
