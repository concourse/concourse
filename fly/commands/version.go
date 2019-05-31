package commands

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/v5"
)

func init() {
	Fly.Version = func() {
		fmt.Println(concourse.Version)
		os.Exit(0)
	}
}
