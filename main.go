package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/concourse/fly/commands"
)

var executeFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "config, c",
		Value: "build.yml",
		Usage: "build configuration file",
	},
	cli.StringSliceFlag{
		Name:  "input, i",
		Value: &cli.StringSlice{},
	},
	cli.BoolFlag{
		Name:  "insecure, k",
		Usage: "allow insecure SSL connections and transfers",
	},
	cli.BoolFlag{
		Name:  "exclude-ignored, x",
		Usage: "exclude vcs-ignored files from the build's inputs",
	},
	cli.BoolFlag{
		Name:  "privileged, p",
		Usage: "run the build with root privileges",
	},
}

func main() {
	app := cli.NewApp()
	app.Name = "fly"
	app.Usage = "Concourse CLI"
	app.Version = "0.0.1"
	app.Action = commands.Execute

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
			Action:    commands.Execute,
		},
		{
			Name:      "hijack",
			ShortName: "i",
			Usage:     "Execute an interactive command in a build's container",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "job, j",
					Usage: "if specified, hijacks builds of the given job",
				},
				cli.StringFlag{
					Name:  "build, b",
					Usage: "hijack a specific build of a job",
				},
			},
			Action: commands.Hijack,
		},
		{
			Name:      "configure",
			ShortName: "c",
			Usage:     "Update configuration",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config, c",
					Usage: "pipeline configuration file",
				},
				cli.BoolFlag{
					Name:  "json, j",
					Usage: "print config as json instead of yaml",
				},
			},
			Action: commands.Configure,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}
