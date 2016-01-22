package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
)

func handleBadResponse(process string, resp *http.Response) {
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatalln("failed to read response when %s:", process, err)
	}
	log.Fatalf("bad response when %s:\n%s\n%s", process, resp.Status, b)
}

func GetBuild(client concourse.Client, jobName string, buildNameOrID string, pipelineName string) (atc.Build, error) {
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
			build, found, err = client.JobBuild(pipelineName, jobName, buildNameOrID)
		} else {
			build, found, err = client.Build(buildNameOrID)
		}

		if err != nil {
			return atc.Build{}, fmt.Errorf("failed to get build %s", err)
		}

		if !found {
			return atc.Build{}, errors.New("build not found")
		}

		return build, nil
	} else if jobName != "" {
		job, found, err := client.Job(pipelineName, jobName)

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
		page := &concourse.Page{Limit: 100}

		for page != nil {
			allBuilds, pagination, err := client.Builds(*page)
			if err != nil {
				return atc.Build{}, fmt.Errorf("failed to get builds %s", err)
			}

			for _, build := range allBuilds {
				if build.JobName == "" {
					return build, nil
				}
			}

			page = pagination.Next
		}

		return atc.Build{}, errors.New("no builds match job")
	}
}

func SliceItoa(slice []int) string {
	var strSlice string
	for i, val := range slice {
		if i > 0 {
			strSlice += "."
		}
		strSlice += strconv.Itoa(val)
	}
	return strSlice
}
