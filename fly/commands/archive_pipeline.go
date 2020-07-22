package commands

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	"github.com/vito/go-interact/interact"
)

type ArchivePipelineCommand struct {
	Pipeline        flaghelpers.PipelineFlag `short:"p"  long:"pipeline"        description:"Pipeline to archive"`
	All             bool                     `short:"a"  long:"all"             description:"Archive all pipelines"`
	SkipInteractive bool                     `short:"n"  long:"non-interactive" description:"Skips interactions, uses default values"`
}

func (command *ArchivePipelineCommand) Validate() error {
	_, err := command.Pipeline.Validate()
	return err
}

func (command *ArchivePipelineCommand) Execute(args []string) error {
	if string(command.Pipeline) == "" && !command.All {
		displayhelpers.Failf("one of the flags '-p, --pipeline' or '-a, --all' is required")
	}

	if string(command.Pipeline) != "" && command.All {
		displayhelpers.Failf("only one of the flags '-p, --pipeline' or '-a, --all' is allowed")
	}

	err := command.Validate()
	if err != nil {
		return err
	}

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var pipelineNames []string
	if string(command.Pipeline) != "" {
		pipelineNames = []string{string(command.Pipeline)}
	}

	if command.All {
		pipelines, err := target.Team().ListPipelines()
		if err != nil {
			return err
		}

		for _, pipeline := range pipelines {
			if !pipeline.Archived {
				pipelineNames = append(pipelineNames, pipeline.Name)
			}
		}
	}

	if len(pipelineNames) == 0 {
		fmt.Println("there are no unarchived pipelines")
		fmt.Println("bailing out")
		return nil
	}

	if !command.confirmArchive(pipelineNames) {
		fmt.Println("bailing out")
		return nil
	}

	for _, pipelineName := range pipelineNames {
		found, err := target.Team().ArchivePipeline(pipelineName)
		if err != nil {
			return err
		}

		if found {
			fmt.Printf("archived '%s'\n", pipelineName)
		} else {
			displayhelpers.Failf("pipeline '%s' not found\n", pipelineName)
		}
	}

	return nil
}

func (command ArchivePipelineCommand) confirmArchive(pipelines []string) bool {
	if command.SkipInteractive {
		return true
	}

	if len(pipelines) > 1 {
		command.printPipelinesTable(pipelines)
	}

	fmt.Printf("!!! archiving the pipeline will remove its configuration. Build history will be retained.\n\n")

	var confirm bool
	err := interact.NewInteraction(command.archivePrompt(pipelines)).Resolve(&confirm)
	if err != nil {
		return false
	}

	return confirm
}

func (ArchivePipelineCommand) printPipelinesTable(pipelines []string) {
	table := ui.Table{Headers: ui.TableRow{{Contents: "pipelines", Color: color.New(color.Bold)}}}
	for _, pipeline := range pipelines {
		table.Data = append(table.Data, ui.TableRow{{Contents: pipeline}})
	}
	table.Render(os.Stdout, true)
	fmt.Println()
}

func (ArchivePipelineCommand) archivePrompt(pipelines []string) string {
	if len(pipelines) == 1 {
		return fmt.Sprintf("archive pipeline '%s'?", pipelines[0])
	}
	return fmt.Sprintf("archive %d pipelines?", len(pipelines))
}
