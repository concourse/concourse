package commands

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
)

type ChecklistCommand struct {
	Pipeline string `short:"p" long:"pipeline" required:"true" description:"The pipeline from which to generate the Checkfile"`
}

var checklistCommand ChecklistCommand

func init() {
	command, err := Parser.AddCommand(
		"checklist",
		"Print a Checkfile of the given pipeline",
		"",
		&checklistCommand,
	)
	if err != nil {
		panic(err)
	}

	command.Aliases = []string{"l"}
}

func (command *ChecklistCommand) Execute([]string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	pipelineName := command.Pipeline

	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}

	atcRequester := newAtcRequester(target.URL(), target.Insecure)

	printCheckfile(pipelineName, getConfig(pipelineName, atcRequester), newTarget(target.URL()))

	return nil
}

func printCheckfile(pipelineName string, config atc.Config, au target) {
	for _, group := range config.Groups {
		printGroup(pipelineName, group, au)
	}

	miscJobs := orphanedJobs(config)
	if len(miscJobs) > 0 {
		printGroup(pipelineName, atc.GroupConfig{Name: "misc", Jobs: miscJobs}, au)
	}
}

func printGroup(pipelineName string, group atc.GroupConfig, au target) {
	fmt.Printf("#- %s\n", group.Name)
	for _, job := range group.Jobs {
		fmt.Printf("%s: concourse.check %s %s %s %s %s\n", job, au.url, au.username, au.password, pipelineName, job)
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

type target struct {
	url      string
	username string
	password string
}

func newTarget(rawTarget string) target {
	u, err := url.Parse(rawTarget)
	if err != nil {
		log.Printf("invalid target '%s': %s\n", rawTarget, err.Error())
		os.Exit(1)
	}

	var username, password string
	if user := u.User; user != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	u.User = nil
	url := u.String()

	return target{
		url:      url,
		username: username,
		password: password,
	}
}
