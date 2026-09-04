package main

import (
	"flag"
	"fmt"
	"os"

	"toolgit/internal/app"
)

func main() {
	branchFlag := flag.String("branch", "", "Target branch to open (skips interactive branch selector)")
	flag.StringVar(branchFlag, "b", "", "Target branch to open (shorthand)")
	flag.Parse()

	if err := app.StartWithBranch(*branchFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error running toolgit: %v\n", err)
		os.Exit(1)
	}
}
