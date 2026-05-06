package main

import (
	"fmt"
	"os"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/gui"
)

var (
	Version   = "dev"
	BuildDate = ""
)

func main() {
	err := gui.Run(Version, BuildDate)
	shared.CloseLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
