package main

import (
	"fmt"
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
		Value:  "http://192.168.100.4:8080",
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
		takeControl("hijack"),
		takeControl("intercept"),
		{
			Name:      "watch",
			ShortName: "w",
			Usage:     "Stream a build's log",
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
			Action: commands.Watch,
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
		{
			Name:      "sync",
			ShortName: "s",
			Usage:     "download and replace the current fly from the target",
			Action:    commands.Sync,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}

func takeControl(commandName string) cli.Command {
	return cli.Command{
		Name:      commandName,
		ShortName: "i",
		Usage:     "Execute an interactive command in a build's container",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "job, j",
				Usage: fmt.Sprintf("if specified, %ss builds of the given job", commandName),
			},
			cli.StringFlag{
				Name:  "build, b",
				Usage: fmt.Sprintf("%s a specific build of a job", commandName),
			},
			cli.BoolFlag{
				Name:  "privileged, p",
				Usage: "run the build with root privileges",
			},
		},
		Action: commands.Hijack,
	}
}
