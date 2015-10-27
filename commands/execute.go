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
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/atcclient/eventstream"
	"github.com/concourse/fly/config"
	"github.com/concourse/fly/rc"
	"github.com/tedsuo/rata"
)

type ExecuteCommand struct {
	TaskConfig     PathFlag        `short:"c" long:"config" required:"true"                  description:"The task config to execute"`
	Privileged     bool            `short:"p" long:"privileged"                              description:"Run the task with full privileges"`
	ExcludeIgnored bool            `short:"x" long:"exclude-ignored"                         description:"Skip uploading .gitignored paths"`
	Inputs         []InputPairFlag `short:"i" long:"input"                                   description:"An input to provide to the task (can be specified multiple times)"`
	InputsFrom     JobFlag         `short:"j" long:"inputs-from" value-name:"[PIPELINE/]JOB" description:"A job to base the inputs on"`
}

func (command *ExecuteCommand) Execute(args []string) error {
	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	handler := atcclient.NewAtcHandler(client)

	taskConfigFile := command.TaskConfig
	excludeIgnored := command.ExcludeIgnored

	atcRequester := newAtcRequester(client.URL(), client.HTTPClient())

	taskConfig := config.LoadTaskConfig(string(taskConfigFile), args)

	inputs, err := determineInputs(
		handler,
		taskConfig.Inputs,
		command.Inputs,
		command.InputsFrom,
	)
	if err != nil {
		return err
	}

	build, err := createBuild(
		atcRequester,
		handler,
		command.Privileged,
		inputs,
		taskConfig,
	)
	if err != nil {
		return err
	}

	fmt.Println("executing build", build.ID)

	terminate := make(chan os.Signal, 1)

	go abortOnSignal(handler, terminate, build)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for _, i := range inputs {
			if i.Path != "" {
				upload(i, excludeIgnored, atcRequester)
			}
		}
	}()

	eventSource, err := handler.BuildEvents(fmt.Sprintf("%d", build.ID))

	if err != nil {
		log.Println("failed to attach to stream:", err)
		os.Exit(1)
	}

	exitCode := eventstream.Render(os.Stdout, eventSource)
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

func determineInputs(
	handler atcclient.Handler,
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

	inputsFromLocal, err := generateLocalInputs(handler, inputMappings)
	if err != nil {
		return nil, err
	}

	inputsFromJob, err := fetchInputsFromJob(handler, inputsFrom)
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

func generateLocalInputs(handler atcclient.Handler, inputMappings []InputPairFlag) (map[string]Input, error) {
	kvMap := map[string]Input{}

	for _, i := range inputMappings {
		inputName := i.Name
		absPath := i.Path

		pipe, err := handler.CreatePipe()
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

func fetchInputsFromJob(handler atcclient.Handler, inputsFrom JobFlag) (map[string]Input, error) {
	kvMap := map[string]Input{}
	if inputsFrom.PipelineName == "" && inputsFrom.JobName == "" {
		return kvMap, nil
	}

	buildInputs, found, err := handler.BuildInputsForJob(inputsFrom.PipelineName, inputsFrom.JobName)
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
	handler atcclient.Handler,
	privileged bool,
	inputs []Input,
	config atc.TaskConfig,
) (atc.Build, error) {
	if err := config.Validate(); err != nil {
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

	return handler.CreateBuild(plan)
}

func abortOnSignal(
	handler atcclient.Handler,
	terminate <-chan os.Signal,
	build atc.Build,
) {
	<-terminate

	fmt.Fprintf(os.Stderr, "\naborting...\n")

	err := handler.AbortBuild(strconv.Itoa(build.ID))
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
