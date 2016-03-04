package commands

import (
	"fmt"
	"os"
)

// overridden via linker flags
var Version = "0.0.0-dev"

func init() {
	Fly.Version = func() {
		fmt.Println(Version)
		os.Exit(0)
	}
}
