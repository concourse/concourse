package commands

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/fly/version"
)

func init() {
	Fly.Version = func() {
		fmt.Println(version.Version)
		os.Exit(0)
	}
}
