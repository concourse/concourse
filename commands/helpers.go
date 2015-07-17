package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"
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

func returnTarget(startingTarget string) string {
	target := lookupURLFromName(startingTarget)
	if target == "" {
		target = startingTarget
	}

	return strings.TrimRight(target, "/")
}

func lookupURLFromName(targetName string) string {
	flyrc := filepath.Join(userHomeDir(), ".flyrc")

	currentTargetsBytes, err := ioutil.ReadFile(flyrc)
	if err != nil {
		return ""
	}

	var current *TargetDetailsYAML
	err = yaml.Unmarshal(currentTargetsBytes, &current)
	if err != nil {
		return ""
	}

	if details, ok := current.Targets[targetName]; ok {
		userInfo := url.UserPassword(details.Username, details.Password)
		targetURL, _ := url.Parse(details.API)
		targetURL.User = userInfo

		return targetURL.String()
	}

	return ""
}

func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func getBuild(client *http.Client, reqGenerator *rata.RequestGenerator, jobName string, buildName string, pipelineName string) atc.Build {
	if pipelineName != "" && jobName == "" {
		fmt.Fprintln(os.Stderr, "job must be specified if pipeline is specified")
		os.Exit(1)
	}

	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}

	if jobName != "" && buildName != "" {
		buildReq, err := reqGenerator.CreateRequest(
			atc.GetJobBuild,
			rata.Params{
				"job_name":      jobName,
				"build_name":    buildName,
				"pipeline_name": pipelineName,
			},
			nil,
		)
		if err != nil {
			log.Fatalln("failed to create request", err)
		}

		buildResp, err := client.Do(buildReq)
		if err != nil {
			log.Fatalln("failed to get builds:", err)
		}

		if buildResp.StatusCode != http.StatusOK {
			log.Println("bad response when getting build:")
			buildResp.Body.Close()
			buildResp.Write(os.Stderr)
			os.Exit(1)
		}

		var build atc.Build
		err = json.NewDecoder(buildResp.Body).Decode(&build)
		if err != nil {
			log.Fatalln("failed to decode job:", err)
		}

		return build
	} else if jobName != "" {
		jobReq, err := reqGenerator.CreateRequest(
			atc.GetJob,
			rata.Params{"job_name": jobName, "pipeline_name": pipelineName},
			nil,
		)
		if err != nil {
			log.Fatalln("failed to create request", err)
		}

		jobResp, err := client.Do(jobReq)
		if err != nil {
			log.Fatalln("failed to get builds:", err)
		}

		if jobResp.StatusCode != http.StatusOK {
			log.Println("bad response when getting job:")
			jobResp.Body.Close()
			jobResp.Write(os.Stderr)
			os.Exit(1)
		}

		var job atc.Job
		err = json.NewDecoder(jobResp.Body).Decode(&job)
		if err != nil {
			log.Fatalln("failed to decode job:", err)
		}

		if job.NextBuild != nil {
			return *job.NextBuild
		} else if job.FinishedBuild != nil {
			return *job.FinishedBuild
		} else {
			println("job has no builds")
			os.Exit(1)
		}
	} else {
		buildsReq, err := reqGenerator.CreateRequest(
			atc.ListBuilds,
			nil,
			nil,
		)
		if err != nil {
			log.Fatalln("failed to create request", err)
		}

		buildsResp, err := client.Do(buildsReq)
		if err != nil {
			log.Fatalln("failed to get builds:", err)
		}

		if buildsResp.StatusCode != http.StatusOK {
			log.Println("bad response when getting builds:")
			buildsResp.Body.Close()
			buildsResp.Write(os.Stderr)
			os.Exit(1)
		}

		var builds []atc.Build
		err = json.NewDecoder(buildsResp.Body).Decode(&builds)
		if err != nil {
			log.Fatalln("failed to decode builds:", err)
		}

		for _, build := range builds {
			if build.JobName == "" {
				return build
			}
		}

		println("no builds")
		os.Exit(1)
	}

	panic("unreachable")
}
