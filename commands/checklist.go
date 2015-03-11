package commands

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/codegangsta/cli"
	"github.com/concourse/atc"
)

func Checklist(c *cli.Context) {
	rawATCURL := c.GlobalString("atcURL")
	insecure := c.GlobalBool("insecure")

	atcRequester := newAtcRequester(rawATCURL, insecure)

	printCheckfile(getConfig(atcRequester), newATCURL(rawATCURL))
}

func printCheckfile(config atc.Config, au atcURL) {
	for _, group := range config.Groups {
		printGroup(group, au)
	}

	miscJobs := orphanedJobs(config)
	if len(miscJobs) > 0 {
		printGroup(atc.GroupConfig{Name: "misc", Jobs: miscJobs}, au)
	}
}

func printGroup(group atc.GroupConfig, au atcURL) {
	fmt.Printf("#- %s\n", group.Name)
	for _, job := range group.Jobs {
		fmt.Printf("%s: concourse.check %s %s %s %s\n", job, au.url, au.username, au.password, job)
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

type atcURL struct {
	url      string
	username string
	password string
}

func newATCURL(rawATCURL string) atcURL {
	u, err := url.Parse(rawATCURL)
	if err != nil {
		log.Printf("invalid atcURL '%s': %s\n", rawATCURL, err.Error())
		os.Exit(1)
	}

	var username, password string
	if user := u.User; user != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	u.User = nil
	url := u.String()

	return atcURL{
		url:      url,
		username: username,
		password: password,
	}
}
