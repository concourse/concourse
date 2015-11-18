package commands

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/deprecated"
	"github.com/concourse/fly/commands/internal/executehelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/config"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/go-concourse/concourse/eventstream"
)

type ExecuteCommand struct {
	TaskConfig     flaghelpers.PathFlag         `short:"c" long:"config" required:"true"                description:"The task config to execute"`
	Privileged     bool                         `short:"p" long:"privileged"                            description:"Run the task with full privileges"`
	ExcludeIgnored bool                         `short:"x" long:"exclude-ignored"                       description:"Skip uploading .gitignored paths"`
	Inputs         []flaghelpers.InputPairFlag  `short:"i" long:"input"       value-name:"NAME=PATH"    description:"An input to provide to the task (can be specified multiple times)"`
	InputsFrom     flaghelpers.JobFlag          `short:"j" long:"inputs-from" value-name:"PIPELINE/JOB" description:"A job to base the inputs on"`
	Outputs        []flaghelpers.OutputPairFlag `short:"o" long:"output"      value-name:"NAME=PATH"    description:"An output to fetch from the task (can be specified multiple times)"`
	Tag            string                       `long:"tag"        description:"targets only workers with this tag."`
}

func (command *ExecuteCommand) Execute(args []string) error {
	connection, err := rc.TargetConnection(Fly.Target)

	if err != nil {
		log.Fatalln(err)
		return nil
	}

	client := concourse.NewClient(connection)

	taskConfigFile := command.TaskConfig
	excludeIgnored := command.ExcludeIgnored

	atcRequester := deprecated.NewAtcRequester(connection.URL(), connection.HTTPClient())

	taskConfig := config.LoadTaskConfig(string(taskConfigFile), args)

	inputs, err := executehelpers.DetermineInputs(
		client,
		taskConfig.Inputs,
		command.Inputs,
		command.InputsFrom,
	)
	if err != nil {
		return err
	}

	outputs, err := executehelpers.DetermineOutputs(
		client,
		taskConfig.Outputs,
		command.Outputs,
	)
	if err != nil {
		return err
	}

	tags := []string{}
	if len(command.Tag) != 0 {
		tags = append(tags, command.Tag)
	}

	build, err := executehelpers.CreateBuild(
		atcRequester,
		client,
		command.Privileged,
		inputs,
		outputs,
		taskConfig,
		tags,
		Fly.Target,
	)
	if err != nil {
		return err
	}

	fmt.Println("executing build", build.ID)

	terminate := make(chan os.Signal, 1)

	go abortOnSignal(client, terminate, build)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

	inputChan := make(chan interface{})
	go func() {
		for _, i := range inputs {
			if i.Path != "" {
				executehelpers.Upload(i, excludeIgnored, atcRequester)
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
					executehelpers.Download(o, atcRequester)
				}

				close(outputChan)
			}(output, outputChans[i])
		}
	}

	eventSource, err := client.BuildEvents(fmt.Sprintf("%d", build.ID))

	if err != nil {
		log.Println("failed to attach to stream:", err)
		os.Exit(1)
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

	fmt.Fprintf(os.Stderr, "\naborting...\n")

	err := client.AbortBuild(strconv.Itoa(build.ID))
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to abort:", err)
		return
	}

	// if told to terminate again, exit immediately
	<-terminate
	fmt.Fprintln(os.Stderr, "exiting immediately")
	os.Exit(2)
}
