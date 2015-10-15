package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"

	"github.com/concourse/atc"
	atcroutes "github.com/concourse/atc/web/routes"
	"github.com/concourse/fly/template"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"
)

type ATCConfig struct {
	pipelineName        string
	apiRequester        *atcRequester
	webRequestGenerator *rata.RequestGenerator
}

func (atcConfig ATCConfig) Set(paused PipelineAction, configPath PathFlag, templateVariables template.Variables, templateVariablesFiles []PathFlag) {
	newConfig, newRawConfig := atcConfig.newConfig(configPath, templateVariablesFiles, templateVariables)
	existingConfig, existingConfigVersion := atcConfig.existingConfig()

	diff(existingConfig, newConfig)

	resp := atcConfig.submitConfig(newRawConfig, paused, existingConfigVersion)
	atcConfig.showHelpfulMessage(resp, paused)
}

func (atcConfig ATCConfig) newConfig(configPath PathFlag, templateVariablesFiles []PathFlag, templateVariables template.Variables) (atc.Config, []byte) {
	configFile, err := ioutil.ReadFile(string(configPath))
	if err != nil {
		failWithErrorf("could not read config file", err)
	}

	var resultVars template.Variables

	for _, path := range templateVariablesFiles {
		fileVars, err := template.LoadVariablesFromFile(string(path))
		if err != nil {
			failWithErrorf("failed to load variables from file (%s)", err, string(path))
		}

		resultVars = resultVars.Merge(fileVars)
	}

	resultVars = resultVars.Merge(templateVariables)

	configFile, err = template.Evaluate(configFile, resultVars)
	if err != nil {
		failWithErrorf("failed to evaluate variables into template", err)
	}

	var newConfig atc.Config
	err = yaml.Unmarshal(configFile, &newConfig)
	if err != nil {
		failWithErrorf("failed to parse configuration file", err)
	}

	return newConfig, configFile
}

func (atcConfig ATCConfig) existingConfig() (atc.Config, string) {
	getConfig, err := atcConfig.apiRequester.CreateRequest(
		atc.GetConfig,
		rata.Params{"pipeline_name": atcConfig.pipelineName},
		nil,
	)
	if err != nil {
		failWithErrorf("failed to build request", err)
	}

	resp, err := atcConfig.apiRequester.httpClient.Do(getConfig)
	if err != nil {
		failWithErrorf("failed to retrieve current configuration", err)
	}

	if resp.StatusCode != http.StatusOK {
		failWithErrorf("bad response when getting config", errors.New(resp.Status))
	}

	version := resp.Header.Get(atc.ConfigVersionHeader)

	var existingConfig atc.Config
	err = json.NewDecoder(resp.Body).Decode(&existingConfig)
	if err != nil {
		failWithErrorf("invalid configuration from server", err)
	}

	return existingConfig, version
}

func (atcConfig ATCConfig) submitConfig(configFile []byte, paused PipelineAction, existingConfigVersion string) *http.Response {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	yamlWriter, err := writer.CreatePart(
		textproto.MIMEHeader{
			"Content-type": {"application/x-yaml"},
		},
	)
	if err != nil {
		failWithErrorf("error building request", err)
	}

	_, err = yamlWriter.Write(configFile)
	if err != nil {
		failWithErrorf("error building request", err)
	}

	switch paused {
	case PausePipeline:
		err = writer.WriteField("paused", "true")
	case UnpausePipeline:
		err = writer.WriteField("paused", "false")
	}
	if err != nil {
		failWithErrorf("error building request", err)
	}

	writer.Close()

	setConfig, err := atcConfig.apiRequester.CreateRequest(
		atc.SaveConfig,
		rata.Params{"pipeline_name": atcConfig.pipelineName},
		body,
	)
	if err != nil {
		failWithErrorf("failed to build set config request", err)
	}

	setConfig.Header.Set("Content-Type", writer.FormDataContentType())
	setConfig.Header.Set(atc.ConfigVersionHeader, existingConfigVersion)

	resp, err := atcConfig.apiRequester.httpClient.Do(setConfig)
	if err != nil {
		failWithErrorf("failed to update configuration", err)
	}

	return resp
}

func (atcConfig ATCConfig) showHelpfulMessage(resp *http.Response, paused PipelineAction) {
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Println("configuration updated")
	case http.StatusCreated:
		pipelineWebReq, _ := atcConfig.webRequestGenerator.CreateRequest(
			atcroutes.Pipeline,
			rata.Params{"pipeline_name": atcConfig.pipelineName},
			nil,
		)

		fmt.Println("pipeline created!")

		pipelineURL := pipelineWebReq.URL
		// don't show username and password
		pipelineURL.User = nil

		fmt.Printf("you can view your pipeline here: %s\n", pipelineURL.String())

		if paused == DoNotChangePipeline || paused == PausePipeline {
			fmt.Println("")
			fmt.Println("the pipeline is currently paused. to unpause, either:")
			fmt.Println("  - run again with --paused=false")
			fmt.Println("  - click play next to the pipeline in the web ui")
		}
	default:
		fmt.Fprintln(os.Stderr, "failed to update configuration.")

		indent := gexec.NewPrefixedWriter("  ", os.Stderr)
		fmt.Fprintf(indent, "response code: %s\n", resp.Status)
		fmt.Fprintf(indent, "response body:\n")

		indentAgain := gexec.NewPrefixedWriter("  ", indent)
		io.Copy(indentAgain, resp.Body)
		os.Exit(1)
	}
}

func diff(existingConfig atc.Config, newConfig atc.Config) {
	indent := gexec.NewPrefixedWriter("  ", os.Stdout)

	groupDiffs := diffIndices(GroupIndex(existingConfig.Groups), GroupIndex(newConfig.Groups))
	if len(groupDiffs) > 0 {
		fmt.Println("groups:")

		for _, diff := range groupDiffs {
			diff.WriteTo(indent, "group")
		}
	}

	resourceDiffs := diffIndices(ResourceIndex(existingConfig.Resources), ResourceIndex(newConfig.Resources))
	if len(resourceDiffs) > 0 {
		fmt.Println("resources:")

		for _, diff := range resourceDiffs {
			diff.WriteTo(indent, "resource")
		}
	}

	jobDiffs := diffIndices(JobIndex(existingConfig.Jobs), JobIndex(newConfig.Jobs))
	if len(jobDiffs) > 0 {
		fmt.Println("jobs:")

		for _, diff := range jobDiffs {
			diff.WriteTo(indent, "job")
		}
	}

	if !askToConfirm("apply configuration?") {
		println("bailing out")
		os.Exit(1)
	}
}
