// +build windows

package commands

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
)

func Hijack(c *cli.Context) {
	log.Fatalln("Command not supported on Windows!")
	os.Exit(1)
}
