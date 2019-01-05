package commands

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/executehelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"github.com/concourse/concourse/fly/config"
	"github.com/concourse/concourse/fly/eventstream"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type ExecuteCommand struct {
	TaskConfig     atc.PathFlag                       `short:"c" long:"config" required:"true"                description:"The task config to execute"`
	Privileged     bool                               `short:"p" long:"privileged"                            description:"Run the task with full privileges"`
	IncludeIgnored bool                               `          long:"include-ignored"                       description:"Including .gitignored paths. Disregards .gitignore entries and uploads everything"`
	Inputs         []flaghelpers.InputPairFlag        `short:"i" long:"input"       value-name:"NAME=PATH"    description:"An input to provide to the task (can be specified multiple times)"`
	InputMappings  []flaghelpers.VariablePairFlag     `short:"m" long:"input-mapping"       value-name:"[NAME=STRING]"    description:"Map a resource to a different name as task input"`
	InputsFrom     flaghelpers.JobFlag                `short:"j" long:"inputs-from" value-name:"PIPELINE/JOB" description:"A job to base the inputs on"`
	Outputs        []flaghelpers.OutputPairFlag       `short:"o" long:"output"      value-name:"NAME=PATH"    description:"An output to fetch from the task (can be specified multiple times)"`
	Image          string                             `long:"image" description:"Image resource for the one-off build"`
	Tags           []string                           `          long:"tag"         value-name:"TAG"          description:"A tag for a specific environment (can be specified multiple times)"`
	Var            []flaghelpers.VariablePairFlag     `short:"v"  long:"var"       value-name:"[NAME=STRING]"  description:"Specify a string value to set for a variable in the pipeline"`
	YAMLVar        []flaghelpers.YAMLVariablePairFlag `short:"y"  long:"yaml-var"  value-name:"[NAME=YAML]"    description:"Specify a YAML value to set for a variable in the pipeline"`
	VarsFrom       []atc.PathFlag                     `short:"l"  long:"load-vars-from"  description:"Variable flag that can be used for filling in template values in configuration from a YAML file"`
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

	includeIgnored := command.IncludeIgnored

	taskTemplate := templatehelpers.NewYamlTemplateWithParams(command.TaskConfig, command.VarsFrom, command.Var, command.YAMLVar)
	taskTemplateEvaluated, err := taskTemplate.Evaluate(false, false)
	if err != nil {
		return err
	}

	taskConfig, err := config.OverrideTaskParams(taskTemplateEvaluated, args)
	if err != nil {
		return err
	}

	client := target.Client()

	fact := atc.NewPlanFactory(time.Now().Unix())

	inputMappings := executehelpers.DetermineInputMappings(command.InputMappings)
	inputs, imageResource, err := executehelpers.DetermineInputs(
		fact,
		target.Team(),
		taskConfig.Inputs,
		command.Inputs,
		inputMappings,
		command.Image,
		command.InputsFrom,
	)
	if err != nil {
		return err
	}

	if imageResource != nil {
		taskConfig.ImageResource = imageResource
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
		inputMappings,
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

	renderOptions := eventstream.RenderOptions{}

	exitCode := eventstream.Render(os.Stdout, eventSource, renderOptions)
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
	}

	// if told to terminate again, exit immediately
	<-terminate
	fmt.Fprintln(ui.Stderr, "exiting immediately")
	os.Exit(2)
}
