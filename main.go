package main

import (
	"os"

	"github.com/codegangsta/cli"
)

var executeFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "config",
		Value: "build.yml",
		Usage: "build configuration file",
	},
	cli.StringFlag{
		Name:  "dir, d",
		Value: ".",
		Usage: "source directory to build",
	},
}

func main() {
	app := cli.NewApp()
	app.Name = "fly"
	app.Usage = "Concourse CLI"
	app.Version = "0.0.1"
	app.Action = execute

	app.Flags = append(executeFlags, cli.StringFlag{
		Name:   "atcURL",
		Value:  "http://127.0.0.1:8080",
		Usage:  "address of the ATC to use",
		EnvVar: "ATC_URL",
	})

	app.Commands = []cli.Command{
		{
			Name:      "execute",
			ShortName: "e",
			Usage:     "Execute a build",
			Flags:     executeFlags,
			Action:    execute,
		},
		{
			Name:      "hijack",
			ShortName: "h",
			Usage:     "Execute an interactive command in a build's container",
			Action:    hijack,
		},
	}

	app.Run(os.Args)
}
