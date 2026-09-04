package commands

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/v8/fly/rc"
)

func init() {
	Fly.Version = func() {
		fmt.Println(rc.LocalVersion)
		os.Exit(0)
	}
}
