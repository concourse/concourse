package commands

import (
	"fmt"
	"os"

	"github.com/concourse/concourse"
)

func init() {
	Fly.Version = func() {
		fmt.Println(concourse.Version)
		os.Exit(0)
	}
}
