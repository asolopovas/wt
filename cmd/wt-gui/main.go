package main

import (
	"fmt"
	"os"

	"github.com/asolopovas/wt/internal/gui"
)

var Version = "dev"

func main() {
	if err := gui.Run(Version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
