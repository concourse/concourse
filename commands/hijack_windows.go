// +build windows

package commands

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
)

func Hijack(c *cli.Context) {
	log.Fatalln("command not supported on windows!")
	os.Exit(1)
}
