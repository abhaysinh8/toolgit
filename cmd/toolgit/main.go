package main

import (
	"fmt"
	"os"

	"toolgit/internal/app"
)

func main() {
	if err := app.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running toolgit: %v\n", err)
		os.Exit(1)
	}
}
