package commands

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/executehelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/config"
	"github.com/concourse/fly/eventstream"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
)

type ExecuteCommand struct {
	TaskConfig     atc.PathFlag                 `short:"c" long:"config" required:"true"                description:"The task config to execute"`
	Privileged     bool                         `short:"p" long:"privileged"                            description:"Run the task with full privileges"`
	IncludeIgnored bool                         `          long:"include-ignored"                       description:"Including .gitignored paths. Disregards .gitignore entries and uploads everything"`
	Inputs         []flaghelpers.InputPairFlag  `short:"i" long:"input"       value-name:"NAME=PATH"    description:"An input to provide to the task (can be specified multiple times)"`
	InputsFrom     flaghelpers.JobFlag          `short:"j" long:"inputs-from" value-name:"PIPELINE/JOB" description:"A job to base the inputs on"`
	Outputs        []flaghelpers.OutputPairFlag `short:"o" long:"output"      value-name:"NAME=PATH"    description:"An output to fetch from the task (can be specified multiple times)"`
	Tags           []string                     `          long:"tag"         value-name:"TAG"          description:"A tag for a specific environment (can be specified multiple times)"`
}

func (command *ExecuteCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	taskConfigFile := command.TaskConfig
	includeIgnored := command.IncludeIgnored

	taskConfig, err := config.LoadTaskConfig(string(taskConfigFile), args)
	if err != nil {
		return err
	}

	client := target.Client()

	fact := atc.NewPlanFactory(time.Now().Unix())

	inputs, err := executehelpers.DetermineInputs(
		fact,
		target.Team(),
		taskConfig.Inputs,
		command.Inputs,
		command.InputsFrom,
	)
	if err != nil {
		return err
	}

	outputs, err := executehelpers.DetermineOutputs(
		fact,
		taskConfig.Outputs,
		command.Outputs,
	)
	if err != nil {
		return err
	}

	plan, err := executehelpers.CreateBuildPlan(
		fact,
		target,
		command.Privileged,
		inputs,
		outputs,
		taskConfig,
		command.Tags,
	)

	if err != nil {
		return err
	}

	clientURL, err := url.Parse(client.URL())
	if err != nil {
		return err
	}

	var build atc.Build
	var buildURL *url.URL

	if command.InputsFrom.PipelineName != "" {
		build, err = target.Team().CreatePipelineBuild(command.InputsFrom.PipelineName, plan)
		if err != nil {
			return err
		}

		buildURL, err = url.Parse(fmt.Sprintf("/teams/%s/pipelines/%s/builds/%s", build.TeamName, build.PipelineName, build.Name))
		if err != nil {
			return err
		}

	} else {
		build, err = target.Team().CreateBuild(plan)
		if err != nil {
			return err
		}

		buildURL, err = url.Parse(fmt.Sprintf("/builds/%d", build.ID))
		if err != nil {
			return err
		}
	}

	fmt.Printf("executing build %d at %s \n", build.ID, clientURL.ResolveReference(buildURL))

	terminate := make(chan os.Signal, 1)

	go abortOnSignal(client, terminate, build)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

	inputChan := make(chan interface{})
	go func() {
		for _, i := range inputs {
			if i.Path != "" {
				executehelpers.Upload(client, build.ID, i, includeIgnored)
			}
		}
		close(inputChan)
	}()

	var outputChans []chan (interface{})
	if len(outputs) > 0 {
		for i, output := range outputs {
			outputChans = append(outputChans, make(chan interface{}, 1))
			go func(o executehelpers.Output, outputChan chan<- interface{}) {
				if o.Path != "" {
					executehelpers.Download(client, build.ID, o)
				}

				close(outputChan)
			}(output, outputChans[i])
		}
	}

	eventSource, err := client.BuildEvents(fmt.Sprintf("%d", build.ID))
	if err != nil {
		return err
	}

	exitCode := eventstream.Render(os.Stdout, eventSource)
	eventSource.Close()

	<-inputChan

	if len(outputs) > 0 {
		for _, outputChan := range outputChans {
			<-outputChan
		}
	}

	os.Exit(exitCode)

	return nil
}

func abortOnSignal(
	client concourse.Client,
	terminate <-chan os.Signal,
	build atc.Build,
) {
	<-terminate

	fmt.Fprintf(ui.Stderr, "\naborting...\n")

	err := client.AbortBuild(strconv.Itoa(build.ID))
	if err != nil {
		fmt.Fprintln(ui.Stderr, "failed to abort:", err)
		return
	}

	// if told to terminate again, exit immediately
	<-terminate
	fmt.Fprintln(ui.Stderr, "exiting immediately")
	os.Exit(2)
}
