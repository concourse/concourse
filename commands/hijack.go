package commands

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/pty"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/gorilla/websocket"
	"github.com/mgutz/ansi"
	"github.com/tedsuo/rata"
	"github.com/vito/go-interact/interact"
)

type HijackCommand struct {
	Job      flaghelpers.JobFlag      `short:"j" long:"job"   value-name:"PIPELINE/JOB"   description:"Name of a job to hijack"`
	Check    flaghelpers.ResourceFlag `short:"c" long:"check" value-name:"PIPELINE/CHECK" description:"Name of a resource's checking container to hijack"`
	Build    string                   `short:"b" long:"build"                             description:"Build number within the job, or global build ID"`
	StepName string                   `short:"s" long:"step"                              description:"Name of step to hijack (e.g. build, unit, resource name)"`
	Attempt  []int                    `short:"a" long:"attempt" description:"Attempt number of step to hijack. Can be specified multiple times for nested retries"`
}

func remoteCommand(argv []string) (string, []string) {
	var path string
	var args []string

	switch len(argv) {
	case 0:
		path = "bash"
	case 1:
		path = argv[0]
	default:
		path = argv[0]
		args = argv[1:]
	}

	return path, args
}

type containerLocator interface {
	locate(containerFingerprint) (map[string]string, error)
}

type stepContainerLocator struct {
	client concourse.Client
}

func (locator stepContainerLocator) locate(fingerprint containerFingerprint) (map[string]string, error) {
	reqValues := map[string]string{}

	if fingerprint.jobName != "" {
		reqValues["pipeline_name"] = fingerprint.pipelineName
		reqValues["job_name"] = fingerprint.jobName
		if fingerprint.buildNameOrID != "" {
			reqValues["build_name"] = fingerprint.buildNameOrID
		}
	} else if fingerprint.buildNameOrID != "" {
		reqValues["build-id"] = fingerprint.buildNameOrID
	} else {
		build, err := GetBuild(locator.client, "", "", "")
		if err != nil {
			return reqValues, err
		}
		reqValues["build-id"] = strconv.Itoa(build.ID)
	}
	if fingerprint.stepName != "" {
		reqValues["step_name"] = fingerprint.stepName
	}

	if len(fingerprint.attempt) > 0 {
		attemptBlob, err := json.Marshal(fingerprint.attempt)
		if err != nil {
			return nil, err
		}
		reqValues["attempt"] = string(attemptBlob)
	}

	return reqValues, nil
}

type checkContainerLocator struct{}

func (locator checkContainerLocator) locate(fingerprint containerFingerprint) (map[string]string, error) {
	reqValues := map[string]string{}

	reqValues["type"] = "check"
	if fingerprint.checkName != "" {
		reqValues["resource_name"] = fingerprint.checkName
	}
	if fingerprint.pipelineName != "" {
		reqValues["pipeline_name"] = fingerprint.pipelineName
	}

	return reqValues, nil
}

type containerFingerprint struct {
	pipelineName  string
	jobName       string
	buildNameOrID string

	stepName string

	checkName string
	attempt   []int
}

func locateContainer(client concourse.Client, fingerprint containerFingerprint) (map[string]string, error) {
	var locator containerLocator

	if fingerprint.checkName == "" {
		locator = stepContainerLocator{
			client: client,
		}
	} else {
		locator = checkContainerLocator{}
	}

	return locator.locate(fingerprint)
}

func getContainerIDs(c *HijackCommand) ([]atc.Container, error) {
	var pipelineName string
	if c.Job.PipelineName != "" {
		pipelineName = c.Job.PipelineName
	} else {
		pipelineName = c.Check.PipelineName
	}

	buildNameOrID := c.Build
	stepName := c.StepName
	jobName := c.Job.JobName
	check := c.Check.ResourceName
	attempt := c.Attempt

	fingerprint := containerFingerprint{
		pipelineName:  pipelineName,
		jobName:       jobName,
		buildNameOrID: buildNameOrID,
		stepName:      stepName,
		checkName:     check,
		attempt:       attempt,
	}

	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return nil, err
	}

	reqValues, err := locateContainer(client, fingerprint)
	if err != nil {
		return nil, err
	}

	containers, err := client.ListContainers(reqValues)
	if err != nil {
		return nil, err
	}

	return containers, nil
}

func (command *HijackCommand) Execute(args []string) error {
	target, err := rc.SelectTarget(Fly.Target)
	if err != nil {
		return err
	}

	containers, err := getContainerIDs(command)
	if err != nil {
		return err
	}

	var chosenContainer atc.Container
	if len(containers) == 0 {
		displayhelpers.Failf("no containers matched your search parameters!\n\nthey may have expired if your build hasn't recently finished.")
	} else if len(containers) > 1 {
		var choices []interact.Choice
		for _, container := range containers {
			var infos []string

			if container.JobName != "" {
				infos = append(infos, fmt.Sprintf("build #%s", container.BuildName))
			} else {
				infos = append(infos, fmt.Sprintf("build id: %d", container.BuildID))
			}

			if container.StepType != "" {
				infos = append(infos, fmt.Sprintf("step: %s", container.StepName))
				infos = append(infos, fmt.Sprintf("type: %s", container.StepType))
			} else {
				infos = append(infos, fmt.Sprintf("resource: %s", container.ResourceName))
				infos = append(infos, "type: check")
			}

			if len(container.Attempts) != 0 {
				attempt := SliceItoa(container.Attempts)
				infos = append(infos, fmt.Sprintf("attempt: %s", attempt))
			}

			choices = append(choices, interact.Choice{
				Display: strings.Join(infos, ", "),
				Value:   container,
			})
		}

		err = interact.NewInteraction("choose a container", choices...).Resolve(&chosenContainer)
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}
	} else {
		chosenContainer = containers[0]
	}

	path, args := remoteCommand(args)
	privileged := true

	reqGenerator := rata.NewRequestGenerator(target.API, atc.Routes)
	tlsConfig := &tls.Config{InsecureSkipVerify: target.Insecure}

	var ttySpec *atc.HijackTTYSpec
	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		ttySpec = &atc.HijackTTYSpec{
			WindowSize: atc.HijackWindowSize{
				Columns: cols,
				Rows:    rows,
			},
		}
	}

	envVariables := append(chosenContainer.EnvironmentVariables, "TERM="+os.Getenv("TERM"))

	spec := atc.HijackProcessSpec{
		Path: path,
		Args: args,
		Env:  envVariables,
		User: chosenContainer.User,
		Dir:  chosenContainer.WorkingDirectory,

		Privileged: privileged,
		TTY:        ttySpec,
	}

	result, err := hijack(tlsConfig, reqGenerator, target, chosenContainer.ID, spec)
	if err != nil {
		return err
	}

	os.Exit(result)

	return nil
}

func hijack(tlsConfig *tls.Config, reqGenerator *rata.RequestGenerator, target rc.TargetProps, containerID string, spec atc.HijackProcessSpec) (int, error) {
	hijackReq, err := reqGenerator.CreateRequest(
		atc.HijackContainer,
		rata.Params{"id": containerID},
		nil,
	)
	if err != nil {
		return -1, fmt.Errorf("failed to create hijack request: %s", err)
	}

	if target.Token != nil {
		hijackReq.Header.Add("Authorization", target.Token.Type+" "+target.Token.Value)
	}

	wsUrl := hijackReq.URL

	var found bool
	wsUrl.Scheme, found = websocketSchemeMap[wsUrl.Scheme]
	if !found {
		return -1, fmt.Errorf("unknown target scheme: %s", wsUrl.Scheme)
	}
	dialer := websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}

	conn, _, err := dialer.Dial(wsUrl.String(), hijackReq.Header)
	if err != nil {
		return -1, err
	}

	defer conn.Close()

	err = conn.WriteJSON(spec)
	if err != nil {
		return -1, err
	}

	var in io.Reader

	term, err := pty.OpenRawTerm()
	if err == nil {
		defer term.Restore()

		in = term
	} else {
		in = os.Stdin
	}

	inputs := make(chan atc.HijackInput, 1)
	finished := make(chan struct{}, 1)

	go monitorTTYSize(inputs, finished)
	go io.Copy(&stdinWriter{inputs}, in)
	go handleInput(conn, inputs, finished)

	exitStatus := handleOutput(conn)

	close(finished)

	return exitStatus, nil
}

func handleOutput(conn *websocket.Conn) int {
	var exitStatus int
	for {
		var output atc.HijackOutput
		err := conn.ReadJSON(&output)
		if err != nil {
			break
		}

		if output.ExitStatus != nil {
			exitStatus = *output.ExitStatus
		} else if len(output.Error) > 0 {
			fmt.Fprintf(os.Stderr, "%s\n", ansi.Color(output.Error, "red+b"))
			exitStatus = 255
		} else if len(output.Stdout) > 0 {
			os.Stdout.Write(output.Stdout)
		} else if len(output.Stderr) > 0 {
			os.Stderr.Write(output.Stderr)
		}
	}

	return exitStatus
}

func handleInput(conn *websocket.Conn, inputs <-chan atc.HijackInput, finished chan struct{}) {
	for {
		select {
		case input := <-inputs:
			err := conn.WriteJSON(input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to send input:", err)
				return
			}
		case <-finished:
			return
		}
	}
}

func monitorTTYSize(inputs chan<- atc.HijackInput, finished chan struct{}) {
	resized := pty.ResizeNotifier()

	for {
		select {
		case <-resized:
			rows, cols, err := pty.Getsize(os.Stdin)
			if err == nil {
				inputs <- atc.HijackInput{
					TTYSpec: &atc.HijackTTYSpec{
						WindowSize: atc.HijackWindowSize{
							Columns: cols,
							Rows:    rows,
						},
					},
				}
			}
		case <-finished:
			return
		}
	}
}

type stdinWriter struct {
	inputs chan<- atc.HijackInput
}

func (w *stdinWriter) Write(d []byte) (int, error) {
	w.inputs <- atc.HijackInput{
		Stdin: d,
	}

	return len(d), nil
}

var websocketSchemeMap = map[string]string{
	"http":  "ws",
	"https": "wss",
}
