package commands

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	"github.com/vito/go-interact/interact"
)

type ArchivePipelineCommand struct {
	Pipeline        *flaghelpers.PipelineFlag `short:"p"  long:"pipeline"        description:"Pipeline to archive"`
	All             bool                      `short:"a"  long:"all"             description:"Archive all pipelines"`
	SkipInteractive bool                      `short:"n"  long:"non-interactive" description:"Skips interactions, uses default values"`
}

func (command *ArchivePipelineCommand) Validate() error {
	_, err := command.Pipeline.Validate()
	return err
}

func (command *ArchivePipelineCommand) Execute(args []string) error {
	if command.Pipeline == nil && !command.All {
		displayhelpers.Failf("one of the flags '-p, --pipeline' or '-a, --all' is required")
	}

	if command.Pipeline != nil && command.All {
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

	var pipelineRefs []atc.PipelineRef
	if command.Pipeline != nil {
		pipelineRefs = []atc.PipelineRef{command.Pipeline.Ref()}
	}

	if command.All {
		pipelines, err := target.Team().ListPipelines()
		if err != nil {
			return err
		}

		for _, pipeline := range pipelines {
			if !pipeline.Archived {
				pipelineRefs = append(pipelineRefs, pipeline.Ref())
			}
		}
	}

	if len(pipelineRefs) == 0 {
		fmt.Println("there are no unarchived pipelines")
		fmt.Println("bailing out")
		return nil
	}

	if !command.confirmArchive(pipelineRefs) {
		fmt.Println("bailing out")
		return nil
	}

	for _, pipelineRef := range pipelineRefs {
		found, err := target.Team().ArchivePipeline(pipelineRef)
		if err != nil {
			return err
		}

		if found {
			fmt.Printf("archived '%s'\n", pipelineRef.String())
		} else {
			displayhelpers.Failf("pipeline '%s' not found\n", pipelineRef.String())
		}
	}

	return nil
}

func (command ArchivePipelineCommand) confirmArchive(pipelines []atc.PipelineRef) bool {
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

func (ArchivePipelineCommand) printPipelinesTable(pipelines []atc.PipelineRef) {
	table := ui.Table{Headers: ui.TableRow{{Contents: "pipelines", Color: color.New(color.Bold)}}}
	for _, pipeline := range pipelines {
		table.Data = append(table.Data, ui.TableRow{{Contents: pipeline.String()}})
	}
	table.Render(os.Stdout, true)
	fmt.Println()
}

func (ArchivePipelineCommand) archivePrompt(pipelines []atc.PipelineRef) string {
	if len(pipelines) == 1 {
		return fmt.Sprintf("archive pipeline '%s'?", pipelines[0].String())
	}
	return fmt.Sprintf("archive %d pipelines?", len(pipelines))
}
