package commands

import (
	"fmt"
	"sort"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
)

type ChecklistCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"The pipeline from which to generate the Checkfile"`
}

func (command *ChecklistCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *ChecklistCommand) Execute([]string) error {
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

	pipelineName := string(command.Pipeline)

	config, _, _, _, err := target.Team().PipelineConfig(pipelineName)
	if err != nil {
		return err
	}

	printCheckfile(target.Team().Name(), pipelineName, config, target.Client().URL())

	return nil
}

func printCheckfile(teamName, pipelineName string, config atc.Config, url string) {
	orphanHeaderName := "misc"
	if len(config.Groups) == 0 {
		orphanHeaderName = pipelineName
	}

	for _, group := range config.Groups {
		printGroup(teamName, pipelineName, group, url)
	}

	miscJobs := orphanedJobs(config)
	if len(miscJobs) > 0 {
		printGroup(teamName, pipelineName, atc.GroupConfig{Name: orphanHeaderName, Jobs: miscJobs}, url)
	}
}

func printGroup(teamName, pipelineName string, group atc.GroupConfig, url string) {
	fmt.Printf("#- %s\n", group.Name)
	for _, job := range group.Jobs {
		fmt.Printf("%s: concourse.check %s %s %s %s\n", job, url, teamName, pipelineName, job)
	}
	fmt.Println("")
}

func orphanedJobs(config atc.Config) []string {
	allJobNames := map[string]struct{}{}
	for _, jobConfig := range config.Jobs {
		allJobNames[jobConfig.Name] = struct{}{}
	}

	for _, group := range config.Groups {
		for _, job := range group.Jobs {
			delete(allJobNames, job)
		}
	}

	result := make([]string, 0, len(config.Jobs))
	for job := range allJobNames {
		result = append(result, job)
	}

	sort.Sort(sort.StringSlice(result))
	return result
}
