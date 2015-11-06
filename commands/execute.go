package commands

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/concourse/atc"
	"github.com/concourse/fly/config"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/go-concourse/concourse/eventstream"
	"github.com/tedsuo/rata"
)

type ExecuteCommand struct {
	TaskConfig     PathFlag         `short:"c" long:"config" required:"true"                description:"The task config to execute"`
	Privileged     bool             `short:"p" long:"privileged"                            description:"Run the task with full privileges"`
	ExcludeIgnored bool             `short:"x" long:"exclude-ignored"                       description:"Skip uploading .gitignored paths"`
	Inputs         []InputPairFlag  `short:"i" long:"input"       value-name:"NAME=PATH"    description:"An input to provide to the task (can be specified multiple times)"`
	InputsFrom     JobFlag          `short:"j" long:"inputs-from" value-name:"PIPELINE/JOB" description:"A job to base the inputs on"`
	Outputs        []OutputPairFlag `short:"o" long:"output"      value-name:"NAME=PATH"    description:"An output to fetch from the task (can be specified multiple times)"`
}

func (command *ExecuteCommand) Execute(args []string) error {
	connection, err := rc.TargetConnection(Fly.Target)

	if err != nil {
		log.Fatalln(err)
		return nil
	}

	client := concourse.NewClient(connection)

	taskConfigFile := command.TaskConfig
	excludeIgnored := command.ExcludeIgnored

	atcRequester := newAtcRequester(connection.URL(), connection.HTTPClient())

	taskConfig := config.LoadTaskConfig(string(taskConfigFile), args)

	inputs, err := determineInputs(
		client,
		taskConfig.Inputs,
		command.Inputs,
		command.InputsFrom,
	)
	if err != nil {
		return err
	}

	outputs, err := determineOutputs(
		client,
		taskConfig.Outputs,
		command.Outputs,
	)
	if err != nil {
		return err
	}

	build, err := createBuild(
		atcRequester,
		client,
		command.Privileged,
		inputs,
		outputs,
		taskConfig,
	)
	if err != nil {
		return err
	}

	fmt.Println("executing build", build.ID)

	terminate := make(chan os.Signal, 1)

	go abortOnSignal(client, terminate, build)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for _, i := range inputs {
			if i.Path != "" {
				upload(i, excludeIgnored, atcRequester)
			}
		}
	}()

	var outputChans []chan (interface{})
	if len(outputs) > 0 {
		for i, output := range outputs {
			outputChans = append(outputChans, make(chan interface{}, 1))
			go func(o Output, outputChan chan<- interface{}) {
				if o.Path != "" {
					download(o, atcRequester)
				}

				close(outputChan)
			}(output, outputChans[i])
		}
	}

	eventSource, err := client.BuildEvents(fmt.Sprintf("%d", build.ID))

	if err != nil {
		log.Println("failed to attach to stream:", err)
		os.Exit(1)
	}

	exitCode := eventstream.Render(os.Stdout, eventSource)
	eventSource.Close()

	if len(outputs) > 0 {
		for _, outputChan := range outputChans {
			<-outputChan
		}
	}

	os.Exit(exitCode)

	return nil
}

type Input struct {
	Name string

	Path string
	Pipe atc.Pipe

	BuildInput atc.BuildInput
}
type Output struct {
	Name string
	Path string
	Pipe atc.Pipe
}

func determineOutputs(
	client concourse.Client,
	taskOutputs []atc.TaskOutputConfig,
	outputMappings []OutputPairFlag,
) ([]Output, error) {

	outputs := []Output{}

	for _, i := range outputMappings {
		outputName := i.Name

		notInConfig := true
		for _, configOutput := range taskOutputs {
			if configOutput.Name == outputName {
				notInConfig = false
			}
		}
		if notInConfig {
			return nil, fmt.Errorf("unknown output '%s'", outputName)
		}

		absPath, err := filepath.Abs(i.Path)
		if err != nil {
			return nil, err
		}

		pipe, err := client.CreatePipe()
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, Output{
			Name: outputName,
			Path: absPath,
			Pipe: pipe,
		})
	}

	return outputs, nil
}

func determineInputs(
	client concourse.Client,
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
	} else {
		err := inputValidation(inputMappings, taskInputs)
		if err != nil {
			return nil, err
		}
	}

	inputsFromLocal, err := generateLocalInputs(client, inputMappings)
	if err != nil {
		return nil, err
	}

	inputsFromJob, err := fetchInputsFromJob(client, inputsFrom)
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

func inputValidation(inputs []InputPairFlag, validInputs []atc.TaskInputConfig) error {

	for _, input := range inputs {
		name := input.Name
		if !containsInput(validInputs, name) {
			return fmt.Errorf("unknown input `%s`", name)
		}
	}
	return nil
}

func containsInput(inputs []atc.TaskInputConfig, name string) bool {
	for _, input := range inputs {
		if input.Name == name {
			return true
		}
	}
	return false
}

func generateLocalInputs(client concourse.Client, inputMappings []InputPairFlag) (map[string]Input, error) {
	kvMap := map[string]Input{}

	for _, i := range inputMappings {
		inputName := i.Name
		absPath := i.Path

		pipe, err := client.CreatePipe()
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

func fetchInputsFromJob(client concourse.Client, inputsFrom JobFlag) (map[string]Input, error) {
	kvMap := map[string]Input{}
	if inputsFrom.PipelineName == "" && inputsFrom.JobName == "" {
		return kvMap, nil
	}

	buildInputs, found, err := client.BuildInputsForJob(inputsFrom.PipelineName, inputsFrom.JobName)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, errors.New("build inputs not found")
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
	client concourse.Client,
	privileged bool,
	inputs []Input,
	outputs []Output,
	config atc.TaskConfig,
) (atc.Build, error) {
	if err := config.Validate(); err != nil {
		return atc.Build{}, err
	}

	targetProps, err := rc.SelectTarget(Fly.Target)
	if err != nil {
		return atc.Build{}, err
	}

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

			source := atc.Source{
				"uri": readPipe.URL.String(),
			}

			if targetProps.Token != nil {
				source["authorization"] = targetProps.Token.Type + " " + targetProps.Token.Value
			}
			getPlan = atc.GetPlan{
				Name:   input.Name,
				Type:   "archive",
				Source: source,
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

	taskPlan := atc.Plan{
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
	}

	buildOutputs := atc.AggregatePlan{}
	for i, output := range outputs {
		writePipe, err := atcRequester.CreateRequest(
			atc.WritePipe,
			rata.Params{"pipe_id": output.Pipe.ID},
			nil,
		)
		if err != nil {
			return atc.Build{}, err
		}
		source := atc.Source{
			"uri": writePipe.URL.String(),
		}

		params := atc.Params{
			"directory": output.Name,
		}

		if targetProps.Token != nil {
			source["authorization"] = targetProps.Token.Type + " " + targetProps.Token.Value
		}

		buildOutputs = append(buildOutputs, atc.Plan{
			Location: &atc.Location{
				ID:            taskPlan.Location.ID + 2 + uint(i),
				ParentID:      0,
				ParallelGroup: taskPlan.Location.ID + 1,
			},
			Put: &atc.PutPlan{
				Name:   output.Name,
				Type:   "archive",
				Source: source,
				Params: params,
			},
		})
	}

	var plan atc.Plan
	if len(buildOutputs) == 0 {
		plan = atc.Plan{
			OnSuccess: &atc.OnSuccessPlan{
				Step: atc.Plan{
					Aggregate: &buildInputs,
				},
				Next: taskPlan,
			},
		}
	} else {
		plan = atc.Plan{
			OnSuccess: &atc.OnSuccessPlan{
				Step: atc.Plan{
					Aggregate: &buildInputs,
				},
				Next: atc.Plan{
					Ensure: &atc.EnsurePlan{
						Step: taskPlan,
						Next: atc.Plan{
							Aggregate: &buildOutputs,
						},
					},
				},
			},
		}
	}

	return client.CreateBuild(plan)
}

func abortOnSignal(
	client concourse.Client,
	terminate <-chan os.Signal,
	build atc.Build,
) {
	<-terminate

	fmt.Fprintf(os.Stderr, "\naborting...\n")

	err := client.AbortBuild(strconv.Itoa(build.ID))
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to abort:", err)
		return
	}

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

func download(output Output, atcRequester *atcRequester) {
	path := output.Path
	pipe := output.Pipe

	downloadBits, err := atcRequester.CreateRequest(
		atc.ReadPipe,
		rata.Params{"pipe_id": pipe.ID},
		nil,
	)
	if err != nil {
		panic(err)
	}

	response, err := atcRequester.httpClient.Do(downloadBits)
	if err != nil {
		fmt.Fprintln(os.Stderr, "download request failed:", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, badResponseError("downloading bits", response))
		panic("unexpected-response-code")
	}

	err = os.MkdirAll(path, 0755)
	if err != nil {
		panic(err)
	}

	err = tarStreamTo(path, response.Body)
	if err != nil {
		panic(err)
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
