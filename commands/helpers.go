package commands

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/tedsuo/rata"
)

func getConfig(pipelineName string, atcRequester *atcRequester) atc.Config {
	getConfigRequest, err := atcRequester.CreateRequest(
		atc.GetConfig,
		rata.Params{"pipeline_name": pipelineName},
		nil,
	)
	if err != nil {
		log.Fatalln(err)
	}

	resp, err := atcRequester.httpClient.Do(getConfigRequest)
	if err != nil {
		log.Println("failed to get config:", err, resp)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		log.Println("bad response when getting config:", resp.Status)
		os.Exit(1)
	}

	var config atc.Config
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		log.Println("invalid config from server:", err)
		os.Exit(1)
	}

	return config
}

func handleBadResponse(process string, resp *http.Response) {
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatalln("failed to read response when %s:", process, err)
	}
	log.Fatalf("bad response when %s:\n%s\n%s", process, resp.Status, b)
}

func getBuild(handler atcclient.Handler, jobName string, buildNameOrId string, pipelineName string) atc.Build {
	if pipelineName != "" && jobName == "" {
		log.Fatalln("job must be specified if pipeline is specified")
	}

	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}

	if buildNameOrId != "" {
		var build atc.Build
		var err error

		if jobName != "" {
			build, err = handler.JobBuild(pipelineName, jobName, buildNameOrId)
		} else {
			build, err = handler.Build(buildNameOrId)
		}
		if err != nil {
			log.Fatalln("failed to get build", err)
		}
		return build
	} else if jobName != "" {
		job, err := handler.Job(pipelineName, jobName)
		if err != nil {
			log.Fatalln("failed to get job", err)
		}

		if job.NextBuild != nil {
			return *job.NextBuild
		} else if job.FinishedBuild != nil {
			return *job.FinishedBuild
		} else {
			log.Fatalln("job has no builds")
		}
	} else {
		allBuilds, err := handler.AllBuilds()
		if err != nil {
			log.Fatalln("failed to get builds", err)
		}

		for _, build := range allBuilds {
			if build.JobName == "" {
				return build
			}
		}

		log.Fatalln("no builds", err)
	}

	panic("unreachable")
}
