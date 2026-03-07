// Package main is the entry point for the swaggerjack CLI tool.
package main

import (
	"fmt"
	"os"

	"github.com/queso/swagger-jack/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
