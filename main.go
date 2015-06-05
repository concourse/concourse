package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/concourse/fly/commands"
)

var targetFlag = cli.StringFlag{
	Name:  "target, t",
	Value: "http://192.168.100.4:8080",
	Usage: "named target you have saved to your .flyrc file",
}

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

var pipelineFlag = cli.StringFlag{
	Name:  "pipeline, p",
	Usage: "the name of the pipeline to act upon",
}

var checkFlag = cli.StringFlag{
	Name:  "check, c",
	Usage: "name of a resource's checking container to hijack",
}

var stepTypeFlag = cli.StringFlag{
	Name:  "step-type, t",
	Usage: "type of step to hijack. one of get, put, or task.",
}

var stepNameFlag = cli.StringFlag{
	Name:  "step-name, n",
	Value: "build",
	Usage: "name of step to hijack (e.g. build, unit, resource name)",
}

var varFlag = cli.StringSliceFlag{
	Name:  "var, v",
	Value: &cli.StringSlice{},
	Usage: "variable flag that can be used for filling in template values in configuration (i.e. -var secret=key)",
}

var varFileFlag = cli.StringSliceFlag{
	Name:  "vars-from, vf",
	Value: &cli.StringSlice{},
	Usage: "variable flag that can be used for filling in template values in configuration from a YAML file",
}

var executeFlags = []cli.Flag{
	buildConfigFlag,
	inputFlag,
	excludeIgnoredFlag,
	privilegedFlag,
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

var pausedFlag = cli.BoolFlag{
	Name:  "paused",
	Usage: "should the pipeline start out as paused or unpaused (true/false)",
}

var apiFlag = cli.StringFlag{
	Name:  "api",
	Usage: "api url to target",
}

var usernameFlag = cli.StringFlag{
	Name:  "username, user",
	Usage: "username for the api",
}

var passwordFlag = cli.StringFlag{
	Name:  "password, pass",
	Usage: "password for the api",
}

var certFlag = cli.StringFlag{
	Name:  "cert",
	Usage: "directory to your cert",
}

func main() {
	app := cli.NewApp()
	app.Name = "fly"
	app.Usage = "Concourse CLI"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		insecureFlag,
		targetFlag,
	}
	app.Commands = []cli.Command{
		{
			Name:      "execute",
			ShortName: "e",
			Usage:     "Execute a build",
			Flags:     executeFlags,
			Action:    commands.Execute,
		},
		{
			Name:      "destroy-pipeline",
			ShortName: "d",
			Usage:     "destroy a pipeline",
			Action:    commands.DestroyPipeline,
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
				pipelineFlag,
			},
			Action: commands.Watch,
		},
		{
			Name:        "configure",
			ShortName:   "c",
			Usage:       "Update configuration",
			Description: "Specify a pipeline name to configure via `fly configure your-pipeline-name-here`",
			Flags: []cli.Flag{
				pipelineConfigFlag,
				jsonFlag,
				varFlag,
				varFileFlag,
				pausedFlag,
			},
			Action: commands.Configure,
		},
		{
			Name:      "sync",
			ShortName: "s",
			Usage:     "download and replace the current fly from the target",
			Action:    commands.Sync,
		},
		{
			Name:   "save-target",
			Usage:  "save a fly target to the .flyrc",
			Action: commands.SaveTarget,
			Flags: []cli.Flag{
				apiFlag,
				usernameFlag,
				passwordFlag,
				certFlag,
			},
		},
		{
			Name:      "checklist",
			ShortName: "l",
			Usage:     "print a Checkman checkfile for the pipeline configuration",
			Action:    commands.Checklist,
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
			pipelineFlag,
			stepTypeFlag,
			stepNameFlag,
			checkFlag,
		},
		Action: commands.Hijack,
	}
}
