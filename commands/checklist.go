package commands

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
)

type ChecklistCommand struct {
	Pipeline string `short:"p" long:"pipeline" required:"true" description:"The pipeline from which to generate the Checkfile"`
}

func (command *ChecklistCommand) Execute([]string) error {
	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}

	pipelineName := command.Pipeline

	config, _, _, err := client.PipelineConfig(pipelineName)
	if err != nil {
		return err
	}

	printCheckfile(pipelineName, config, client.URL())

	return nil
}

func printCheckfile(pipelineName string, config atc.Config, url string) {
	for _, group := range config.Groups {
		printGroup(pipelineName, group, url)
	}

	miscJobs := orphanedJobs(config)
	if len(miscJobs) > 0 {
		printGroup(pipelineName, atc.GroupConfig{Name: "misc", Jobs: miscJobs}, url)
	}
}

func printGroup(pipelineName string, group atc.GroupConfig, url string) {
	fmt.Printf("#- %s\n", group.Name)
	for _, job := range group.Jobs {
		fmt.Printf("%s: concourse.check %s %s %s\n", job, url, pipelineName, job)
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

	return result
}
