package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
)

func handleBadResponse(process string, resp *http.Response) {
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatalln("failed to read response when %s:", process, err)
	}
	log.Fatalf("bad response when %s:\n%s\n%s", process, resp.Status, b)
}

func GetBuild(handler atcclient.Handler, jobName string, buildNameOrID string, pipelineName string) (atc.Build, error) {
	if pipelineName != "" && jobName == "" {
		log.Fatalln("job must be specified if pipeline is specified")
	}
	if pipelineName == "" && jobName != "" {
		log.Fatalln("pipeline must be specified if job is specified")
	}

	if buildNameOrID != "" {
		var build atc.Build
		var err error
		var found bool

		if jobName != "" {
			build, found, err = handler.JobBuild(pipelineName, jobName, buildNameOrID)
		} else {
			build, found, err = handler.Build(buildNameOrID)
		}

		if err != nil {
			return atc.Build{}, fmt.Errorf("failed to get build %s", err)
		}

		if !found {
			return atc.Build{}, errors.New("build not found")
		}

		return build, nil
	} else if jobName != "" {
		job, found, err := handler.Job(pipelineName, jobName)

		if err != nil {
			return atc.Build{}, fmt.Errorf("failed to get job %s", err)
		}

		if !found {
			return atc.Build{}, errors.New("job not found")
		}

		if job.NextBuild != nil {
			return *job.NextBuild, nil
		} else if job.FinishedBuild != nil {
			return *job.FinishedBuild, nil
		} else {
			return atc.Build{}, errors.New("job has no builds")
		}
	} else {
		allBuilds, err := handler.AllBuilds()
		if err != nil {
			return atc.Build{}, fmt.Errorf("failed to get builds %s", err)
		}

		for _, build := range allBuilds {
			if build.JobName == "" {
				return build, nil
			}
		}

		return atc.Build{}, errors.New("no builds match job")
	}
}

func failf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message+"\n", args...)
	os.Exit(1)
}

func failWithErrorf(message string, err error, args ...interface{}) {
	templatedMessage := fmt.Sprintf(message, args...)
	failf("%s: %s", templatedMessage, err.Error())
}
