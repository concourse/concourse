package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/config"
	"github.com/concourse/fly/eventstream"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

type ExecuteCommand struct {
	Privileged     bool            `short:"p" long:"privileged"`
	ExcludeIgnored bool            `short:"x" long:"exclude-ignored"`
	Inputs         []InputPairFlag `short:"i" long:"input"`
	InputsFrom     JobFlag         `short:"j" long:"inputs-from"`
	TaskConfig     PathFlag        `short:"c" long:"config" required:"true"`
}

var executeCommand ExecuteCommand

func init() {
	execute, err := Parser.AddCommand(
		"execute",
		"Execute a one-off build",
		"Blah blah blah",
		&executeCommand,
	)
	if err != nil {
		panic(err)
	}

	execute.Aliases = []string{"e"}
}

func (command *ExecuteCommand) Execute(args []string) error {
	target := returnTarget(globalOptions.Target)
	insecure := globalOptions.Insecure
	taskConfigFile := command.TaskConfig
	excludeIgnored := command.ExcludeIgnored

	atcRequester := newAtcRequester(target, insecure)

	taskConfig := config.LoadTaskConfig(string(taskConfigFile), args)

	inputs, err := determineInputs(
		atcRequester,
		taskConfig.Inputs,
		command.Inputs,
		command.InputsFrom,
	)
	if err != nil {
		return err
	}

	build, err := createBuild(
		atcRequester,
		command.Privileged,
		inputs,
		taskConfig,
	)
	if err != nil {
		return err
	}

	fmt.Println("executing build", build.ID)

	terminate := make(chan os.Signal, 1)

	go abortOnSignal(atcRequester, terminate, build)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

	eventSource, err := sse.Connect(atcRequester.httpClient, time.Second, func() *http.Request {
		logOutput, _ := atcRequester.CreateRequest(
			atc.BuildEvents,
			rata.Params{"build_id": strconv.Itoa(build.ID)},
			nil,
		)

		return logOutput
	})
	if err != nil {
		return fmt.Errorf("failed to connect to event stream: %s", err)
	}

	go func() {
		for _, i := range inputs {
			if i.Path != "" {
				upload(i, excludeIgnored, atcRequester)
			}
		}
	}()

	exitCode, err := eventstream.RenderStream(eventSource)
	if err != nil {
		return fmt.Errorf("failed to render stream: %s", err)
	}

	eventSource.Close()

	os.Exit(exitCode)

	return nil
}

type Input struct {
	Name string

	Path string
	Pipe atc.Pipe

	BuildInput atc.BuildInput
}

func createPipe(atcRequester *atcRequester) (atc.Pipe, error) {
	cPipe, err := atcRequester.CreateRequest(atc.CreatePipe, nil, nil)
	if err != nil {
		return atc.Pipe{}, err
	}

	response, err := atcRequester.httpClient.Do(cPipe)
	if err != nil {
		return atc.Pipe{}, fmt.Errorf("failed to create pipe: %s", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return atc.Pipe{}, badResponseError("creating pipe", response)
	}

	var pipe atc.Pipe
	err = json.NewDecoder(response.Body).Decode(&pipe)
	if err != nil {
		return atc.Pipe{}, fmt.Errorf("malformed pipe response: %s", err)
	}

	return pipe, nil
}

func determineInputs(
	atcRequester *atcRequester,
	taskInputs []atc.TaskInputConfig,
	inputMappings []InputPairFlag,
	inputsFrom JobFlag,
) ([]Input, error) {
	if len(inputMappings) == 0 && inputsFrom.PipelineName == "" && inputsFrom.JobName == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		inputMappings = append(inputMappings, InputPairFlag{
			Name: filepath.Base(wd),
			Path: wd,
		})
	}

	inputsFromLocal, err := generateLocalInputs(atcRequester, inputMappings)
	if err != nil {
		return nil, err
	}

	inputsFromJob, err := fetchInputsFromJob(atcRequester, inputsFrom)
	if err != nil {
		return nil, err
	}

	inputs := []Input{}
	for _, taskInput := range taskInputs {
		input, found := inputsFromLocal[taskInput.Name]
		if !found {
			input, found = inputsFromJob[taskInput.Name]
			if !found {
				continue
			}
		}

		inputs = append(inputs, input)
	}

	return inputs, nil
}

func generateLocalInputs(
	atcRequester *atcRequester,
	inputMappings []InputPairFlag,
) (map[string]Input, error) {
	kvMap := map[string]Input{}

	for _, i := range inputMappings {
		inputName := i.Name
		absPath := i.Path

		pipe, err := createPipe(atcRequester)
		if err != nil {
			return nil, err
		}

		kvMap[inputName] = Input{
			Name: inputName,
			Path: absPath,
			Pipe: pipe,
		}
	}

	return kvMap, nil
}

func fetchInputsFromJob(
	atcRequester *atcRequester,
	inputsFrom JobFlag,
) (map[string]Input, error) {
	kvMap := map[string]Input{}
	if inputsFrom.PipelineName == "" && inputsFrom.JobName == "" {
		return kvMap, nil
	}

	listJobInputsRequest, err := atcRequester.CreateRequest(
		atc.ListJobInputs,
		rata.Params{
			"pipeline_name": inputsFrom.PipelineName,
			"job_name":      inputsFrom.JobName,
		},
		nil,
	)
	if err != nil {
		return nil, err
	}

	response, err := atcRequester.httpClient.Do(listJobInputsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job inputs: %s", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, badResponseError("getting job inputs", response)
	}

	var buildInputs []atc.BuildInput
	err = json.NewDecoder(response.Body).Decode(&buildInputs)
	if err != nil {
		return nil, fmt.Errorf("malformed job inputs response: %s", err)
	}

	for _, buildInput := range buildInputs {
		kvMap[buildInput.Name] = Input{
			Name:       buildInput.Name,
			BuildInput: buildInput,
		}
	}

	return kvMap, nil
}

func createBuild(
	atcRequester *atcRequester,
	privileged bool,
	inputs []Input,
	config atc.TaskConfig,
) (atc.Build, error) {
	if err := config.Validate(); err != nil {
		return atc.Build{}, err
	}

	buffer := &bytes.Buffer{}

	buildInputs := atc.AggregatePlan{}
	for i, input := range inputs {
		var getPlan atc.GetPlan
		if input.Path != "" {
			readPipe, err := atcRequester.CreateRequest(
				atc.ReadPipe,
				rata.Params{"pipe_id": input.Pipe.ID},
				nil,
			)
			if err != nil {
				return atc.Build{}, err
			}

			getPlan = atc.GetPlan{
				Name: input.Name,
				Type: "archive",
				Source: atc.Source{
					"uri": readPipe.URL.String(),
				},
			}
		} else {
			getPlan = atc.GetPlan{
				Name:    input.Name,
				Type:    input.BuildInput.Type,
				Source:  input.BuildInput.Source,
				Version: input.BuildInput.Version,
				Params:  input.BuildInput.Params,
				Tags:    input.BuildInput.Tags,
			}
		}

		buildInputs = append(buildInputs, atc.Plan{
			Location: &atc.Location{
				// offset by 2 because aggregate gets parallelgroup ID 1
				ID:            uint(i) + 2,
				ParentID:      0,
				ParallelGroup: 1,
			},
			Get: &getPlan,
		})
	}

	plan := atc.Plan{
		OnSuccess: &atc.OnSuccessPlan{
			Step: atc.Plan{
				Aggregate: &buildInputs,
			},
			Next: atc.Plan{
				Location: &atc.Location{
					// offset by 1 because aggregate gets parallelgroup ID 1
					ID:       uint(len(inputs)) + 2,
					ParentID: 0,
				},
				Task: &atc.TaskPlan{
					Name:       "one-off",
					Privileged: privileged,
					Config:     &config,
				},
			},
		},
	}

	err := json.NewEncoder(buffer).Encode(plan)
	if err != nil {
		return atc.Build{}, err
	}

	createBuild, err := atcRequester.CreateRequest(atc.CreateBuild, nil, buffer)
	if err != nil {
		return atc.Build{}, err
	}

	createBuild.Header.Set("Content-Type", "application/json")

	response, err := atcRequester.httpClient.Do(createBuild)
	if err != nil {
		return atc.Build{}, fmt.Errorf("failed to create build: %s", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return atc.Build{}, badResponseError("creating build", response)
	}

	var build atc.Build
	err = json.NewDecoder(response.Body).Decode(&build)
	if err != nil {
		return atc.Build{}, fmt.Errorf("malformed build response: %s", err)
	}

	return build, nil
}

func abortOnSignal(
	atcRequester *atcRequester,
	terminate <-chan os.Signal,
	build atc.Build,
) {
	<-terminate

	fmt.Fprintf(os.Stderr, "\naborting...\n")

	abortReq, err := atcRequester.CreateRequest(
		atc.AbortBuild,
		rata.Params{"build_id": strconv.Itoa(build.ID)},
		nil,
	)
	if err != nil {
		panic(err)
	}

	resp, err := atcRequester.httpClient.Do(abortReq)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to abort:", err)
		return
	}

	resp.Body.Close()

	// if told to terminate again, exit immediately
	<-terminate
	fmt.Fprintln(os.Stderr, "exiting immediately")
	os.Exit(2)
}

func upload(input Input, excludeIgnored bool, atcRequester *atcRequester) {
	path := input.Path
	pipe := input.Pipe

	var files []string
	var err error

	if excludeIgnored {
		files, err = getGitFiles(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "could not determine ignored files:", err)
			return
		}
	} else {
		files = []string{"."}
	}

	archive, err := tarStreamFrom(path, files)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could create tar stream:", err)
		return
	}

	defer archive.Close()

	uploadBits, err := atcRequester.CreateRequest(
		atc.WritePipe,
		rata.Params{"pipe_id": pipe.ID},
		archive,
	)
	if err != nil {
		panic(err)
	}

	response, err := atcRequester.httpClient.Do(uploadBits)
	if err != nil {
		fmt.Fprintln(os.Stderr, "upload request failed:", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, badResponseError("uploading bits", response))
	}
}

func getGitFiles(dir string) ([]string, error) {
	tracked, err := gitLS(dir)
	if err != nil {
		return nil, err
	}

	untracked, err := gitLS(dir, "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	return append(tracked, untracked...), nil
}

func gitLS(dir string, flags ...string) ([]string, error) {
	files := []string{}

	gitLS := exec.Command("git", append([]string{"ls-files", "-z"}, flags...)...)
	gitLS.Dir = dir

	gitOut, err := gitLS.StdoutPipe()
	if err != nil {
		return nil, err
	}

	outScan := bufio.NewScanner(gitOut)
	outScan.Split(scanNull)

	err = gitLS.Start()
	if err != nil {
		return nil, err
	}

	for outScan.Scan() {
		files = append(files, outScan.Text())
	}

	err = gitLS.Wait()
	if err != nil {
		return nil, err
	}

	return files, nil
}

func scanNull(data []byte, atEOF bool) (int, []byte, error) {
	// eof, no more data; terminate
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// look for terminating null byte
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[0:i], nil
	}

	// no final terminator; return what's left
	if atEOF {
		return len(data), data, nil
	}

	// request more data
	return 0, nil, nil
}
