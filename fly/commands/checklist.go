package commands

import (
	"fmt"
	"sort"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type ChecklistCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"The pipeline from which to generate the Checkfile"`
	Team     string                   `long:"team" description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *ChecklistCommand) Validate() error {
	_, err := command.Pipeline.Validate()
	return err
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

	var team concourse.Team

	if command.Team != "" {
		team, err = target.FindTeam(command.Team)
		if err != nil {
			return err
		}
	} else {
		team = target.Team()
	}

	pipelineRef := command.Pipeline.Ref()
	config, _, _, err := team.PipelineConfig(pipelineRef)
	if err != nil {
		return err
	}

	printCheckfile(team.Name(), pipelineRef.String(), config, target.Client().URL())

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

	sort.Strings(result)
	return result
}
