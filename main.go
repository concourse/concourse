package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/concourse/fly/commands"
)

var buildConfigFlag = cli.StringFlag{
	Name:  "config, c",
	Value: "build.yml",
	Usage: "build configuration file",
}

var inputFlag = cli.StringSliceFlag{
	Name:  "input, i",
	Value: &cli.StringSlice{},
}

var insecureFlag = cli.BoolFlag{
	Name:  "insecure, k",
	Usage: "allow insecure SSL connections and transfers",
}

var excludeIgnoredFlag = cli.BoolFlag{
	Name:  "exclude-ignored, x",
	Usage: "exclude vcs-ignored files from the build's inputs",
}

var privilegedFlag = cli.BoolFlag{
	Name:  "privileged, p",
	Usage: "run the build or command with root privileges",
}

var targetFlag = cli.StringFlag{
	Name:   "target",
	Value:  "http://192.168.100.4:8080",
	Usage:  "address of the Concourse API server to use",
	EnvVar: "ATC_URL",
}

var executeFlags = []cli.Flag{
	buildConfigFlag,
	inputFlag,
	insecureFlag,
	excludeIgnoredFlag,
	privilegedFlag,
	targetFlag,
}

func jobFlag(verb string) cli.StringFlag {
	return cli.StringFlag{
		Name:  "job, j",
		Usage: fmt.Sprintf("if specified, %s builds of the given job", verb),
	}
}

func buildFlag(verb string) cli.StringFlag {
	return cli.StringFlag{
		Name:  "build, b",
		Usage: fmt.Sprintf("%s a specific build of a job", verb),
	}
}

var pipelineConfigFlag = cli.StringFlag{
	Name:  "config, c",
	Usage: "pipeline configuration file",
}

var jsonFlag = cli.BoolFlag{
	Name:  "json, j",
	Usage: "print config as json instead of yaml",
}

func main() {
	app := cli.NewApp()
	app.Name = "fly"
	app.Usage = "Concourse CLI"
	app.Version = "0.0.1"
	app.Action = commands.Execute

	app.Flags = executeFlags

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
				buildFlag("watches"),
				jobFlag("watches"),
				insecureFlag,
				targetFlag,
			},
			Action: commands.Watch,
		},
		{
			Name:      "configure",
			ShortName: "c",
			Usage:     "Update configuration",
			Flags: []cli.Flag{
				pipelineConfigFlag,
				jsonFlag,
				insecureFlag,
				targetFlag,
			},
			Action: commands.Configure,
		},
		{
			Name:      "sync",
			ShortName: "s",
			Usage:     "download and replace the current fly from the target",
			Action:    commands.Sync,
			Flags: []cli.Flag{
				insecureFlag,
				targetFlag,
			},
		},
		{
			Name:      "checklist",
			ShortName: "l",
			Usage:     "print a Checkman checkfile for the pipeline configuration",
			Action:    commands.Checklist,
			Flags: []cli.Flag{
				insecureFlag,
				targetFlag,
			},
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
			jobFlag(commandName + "s"),
			buildFlag(commandName + "s"),
			privilegedFlag,
			insecureFlag,
			targetFlag,
		},
		Action: commands.Hijack,
	}
}
