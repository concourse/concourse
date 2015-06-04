package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strconv"

	"github.com/codegangsta/cli"
	"github.com/concourse/atc"
	atcroutes "github.com/concourse/atc/web/routes"
	"github.com/concourse/fly/template"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"
)

func Configure(c *cli.Context) {
	target := returnTarget(c.GlobalString("target"))
	insecure := c.GlobalBool("insecure")
	configPath := c.String("config")
	paused := c.String("paused")
	asJSON := c.Bool("json")
	templateVariables := c.StringSlice("var")
	templateVariablesFile := c.StringSlice("vars-from")
	pipelineName := c.Args().First()

	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}

	apiRequester := newAtcRequester(target, insecure)
	webRequestGenerator := rata.NewRequestGenerator(target, atcroutes.Routes)

	atcConfig := ATCConfig{
		pipelineName:        pipelineName,
		apiRequester:        apiRequester,
		webRequestGenerator: webRequestGenerator,
	}

	if configPath == "" {
		atcConfig.DumpConfig(asJSON)
	} else {
		atcConfig.SetConfig(paused, configPath, templateVariables, templateVariablesFile)
	}
}

type ATCConfig struct {
	pipelineName        string
	apiRequester        *atcRequester
	webRequestGenerator *rata.RequestGenerator
}

func (atcConfig ATCConfig) DumpConfig(asJSON bool) {
	config := getConfig(atcConfig.pipelineName, atcConfig.apiRequester)

	var payload []byte
	var err error
	if asJSON {
		payload, err = json.Marshal(config)
	} else {
		payload, err = yaml.Marshal(config)
	}

	if err != nil {
		log.Println("failed to marshal config to YAML:", err)
		os.Exit(1)
	}

	fmt.Printf("%s", payload)
}

type PipelineAction int

const (
	PausePipeline PipelineAction = iota
	UnpausePipeline
	DoNotChangePipeline
)

func (atcConfig ATCConfig) shouldPausePipeline(pausedFlag string) PipelineAction {
	if pausedFlag == "" {
		return DoNotChangePipeline
	}

	p, err := strconv.ParseBool(pausedFlag)
	if err != nil {
		failf("paused value '%s' is not a boolean\n", pausedFlag)
	}

	if p {
		return PausePipeline
	} else {
		return UnpausePipeline
	}
}

func failf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message+"\n", args...)
	os.Exit(1)
}

func failWithErrorf(message string, err error) {
	failf(message + ": " + err.Error())
}

func (atcConfig ATCConfig) SetConfig(pausedFlag string, configPath string, templateVariables []string, templateVariablesFile []string) {
	paused := atcConfig.shouldPausePipeline(pausedFlag)

	newConfig, newRawConfig := atcConfig.newConfig(configPath, templateVariablesFile, templateVariables)
	existingConfig, existingConfigVersion := atcConfig.existingConfig()

	diff(existingConfig, newConfig)

	resp := atcConfig.submitConfig(newRawConfig, paused, existingConfigVersion)
	atcConfig.showHelpfulMessage(resp, paused)
}

func (atcConfig ATCConfig) newConfig(configPath string, templateVariablesFiles []string, templateVariables []string) (atc.Config, []byte) {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		failWithErrorf("could not read config file", err)
	}

	var resultVars template.Variables

	for _, path := range templateVariablesFiles {
		fileVars, err := template.LoadVariablesFromFile(path)
		if err != nil {
			log.Fatalln(err)
		}

		resultVars = resultVars.Merge(fileVars)
	}

	vars, err := template.LoadVariables(templateVariables)
	if err != nil {
		log.Fatalln(err)
	}

	resultVars = resultVars.Merge(vars)

	configFile, err = template.Evaluate(configFile, resultVars)
	if err != nil {
		log.Fatalln(err)
	}

	var newConfig atc.Config
	err = yaml.Unmarshal(configFile, &newConfig)
	if err != nil {
		log.Fatalln(err)
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
		log.Fatalln(err)
	}

	resp, err := atcConfig.apiRequester.httpClient.Do(getConfig)
	if err != nil {
		log.Println("failed to get config:", err, resp)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		log.Println("bad response when getting config:", resp.Status)
		os.Exit(1)
	}

	version := resp.Header.Get(atc.ConfigVersionHeader)

	var existingConfig atc.Config
	err = json.NewDecoder(resp.Body).Decode(&existingConfig)
	if err != nil {
		log.Println("invalid config from server:", err)
		os.Exit(1)
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
		log.Println("error building request:", err)
		os.Exit(1)
	}

	_, err = yamlWriter.Write(configFile)
	if err != nil {
		log.Println("error building request:", err)
		os.Exit(1)
	}

	switch paused {
	case PausePipeline:
		err = writer.WriteField("paused", "true")
	case UnpausePipeline:
		err = writer.WriteField("paused", "false")
	}

	if err != nil {
		log.Fatalln("failed to write field:", err)
	}

	writer.Close()

	setConfig, err := atcConfig.apiRequester.CreateRequest(
		atc.SaveConfig,
		rata.Params{"pipeline_name": atcConfig.pipelineName},
		body,
	)
	if err != nil {
		log.Fatalln("failed to set config:", err)
	}

	setConfig.Header.Set("Content-Type", writer.FormDataContentType())
	setConfig.Header.Set(atc.ConfigVersionHeader, existingConfigVersion)

	resp, err := atcConfig.apiRequester.httpClient.Do(setConfig)
	if err != nil {
		println("failed to update configuration: " + err.Error())
		os.Exit(1)
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
