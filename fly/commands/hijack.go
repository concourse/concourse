package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/commands/internal/hijacker"
	"github.com/concourse/concourse/fly/commands/internal/hijackhelpers"
	"github.com/concourse/concourse/fly/pty"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/tedsuo/rata"
	"github.com/vito/go-interact/interact"
)

type HijackCommand struct {
	Job            flaghelpers.JobFlag      `short:"j" long:"job"   value-name:"PIPELINE/JOB"   description:"Name of a job to hijack"`
	Handle         string                   `          long:"handle"                            description:"Handle id of a job to hijack"`
	Check          flaghelpers.ResourceFlag `short:"c" long:"check" value-name:"PIPELINE/CHECK" description:"Name of a resource's checking container to hijack"`
	Url            string                   `short:"u" long:"url"                               description:"URL for the build, job, or check container to hijack"`
	Build          string                   `short:"b" long:"build"                             description:"Build number within the job, or global build ID"`
	StepName       string                   `short:"s" long:"step"                              description:"Name of step to hijack (e.g. build, unit, resource name)"`
	StepType       string                   `          long:"step-type"                         description:"Type of step to hijack (e.g. get, put, task)"`
	Attempt        string                   `short:"a" long:"attempt" value-name:"N[,N,...]"    description:"Attempt number of step to hijack."`
	PositionalArgs struct {
		Command []string `positional-arg-name:"command" description:"The command to run in the container (default: bash)"`
	} `positional-args:"yes"`
	Team string `long:"team" description:"Name of the team to which the container belongs, if different from the target default"`
}

func (command *HijackCommand) Execute([]string) error {
	var (
		chosenContainer atc.Container
		err             error
		name            rc.TargetName
		target          rc.Target
		team            concourse.Team
	)
	if Fly.Target == "" && command.Url != "" {
		u, err := url.Parse(command.Url)
		if err != nil {
			return err
		}
		urlMap := parseUrlPath(u.Path)
		target, name, err = rc.LoadTargetFromURL(fmt.Sprintf("%s://%s", u.Scheme, u.Host), urlMap["teams"], Fly.Verbose)
		if err != nil {
			return err
		}
		Fly.Target = name
	} else {
		target, err = rc.LoadTarget(Fly.Target, Fly.Verbose)
		if err != nil {
			return err
		}
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	if command.Team != "" {
		team, err = target.FindTeam(command.Team)
		if err != nil {
			return err
		}
	} else {
		team = target.Team()
	}

	if command.Handle != "" {
		chosenContainer, err = team.GetContainer(command.Handle)
		if err != nil {
			displayhelpers.Failf("no containers matched the given handle id!\n\nthey may have expired if your build hasn't recently finished.")
		}

	} else {
		fingerprint, err := command.getContainerFingerprint(target, team)
		if err != nil {
			return err
		}

		containers, err := command.getContainerIDs(target, fingerprint, team)
		if err != nil {
			return err
		}

		hijackableContainers := make([]atc.Container, 0)

		for _, container := range containers {
			if container.State == atc.ContainerStateCreated || container.State == atc.ContainerStateFailed {
				hijackableContainers = append(hijackableContainers, container)
			}
		}

		if len(hijackableContainers) == 0 {
			displayhelpers.Failf("no containers matched your search parameters!\n\nthey may have expired if your build hasn't recently finished.")
		} else if len(hijackableContainers) > 1 {
			var choices []interact.Choice
			for _, container := range hijackableContainers {
				var infos []string

				if container.BuildID != 0 {
					if container.JobName != "" {
						infos = append(infos, fmt.Sprintf("build #%s", container.BuildName))
					} else {
						infos = append(infos, fmt.Sprintf("build id: %d", container.BuildID))
					}
				}

				if container.StepName != "" {
					infos = append(infos, fmt.Sprintf("step: %s", container.StepName))
				}

				if container.ResourceName != "" {
					infos = append(infos, fmt.Sprintf("resource: %s", container.ResourceName))
				}

				infos = append(infos, fmt.Sprintf("type: %s", container.Type))

				if container.Type == "check" {
					infos = append(infos, fmt.Sprintf("expires in: %s", container.ExpiresIn))
				}

				if container.Attempt != "" {
					infos = append(infos, fmt.Sprintf("attempt: %s", container.Attempt))
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
			chosenContainer = hijackableContainers[0]
		}
	}

	privileged := true

	reqGenerator := rata.NewRequestGenerator(target.URL(), atc.Routes)

	var ttySpec *atc.HijackTTYSpec
	rows, cols, err := pty.Getsize(os.Stdout)
	if err == nil {
		ttySpec = &atc.HijackTTYSpec{
			WindowSize: atc.HijackWindowSize{
				Columns: cols,
				Rows:    rows,
			},
		}
	}

	path, args := remoteCommand(command.PositionalArgs.Command)

	spec := atc.HijackProcessSpec{
		Path: path,
		Args: args,
		Env:  []string{"TERM=" + os.Getenv("TERM")},
		User: chosenContainer.User,
		Dir:  chosenContainer.WorkingDirectory,

		Privileged: privileged,
		TTY:        ttySpec,
	}

	result, err := func() (int, error) { // so the term.Restore() can run before the os.Exit()
		var in io.Reader

		if pty.IsTerminal() {
			term, err := pty.OpenRawTerm()
			if err != nil {
				return -1, err
			}

			defer func() {
				_ = term.Restore()
			}()

			in = term
		} else {
			in = os.Stdin
		}

		io := hijacker.ProcessIO{
			In:  in,
			Out: os.Stdout,
			Err: os.Stderr,
		}

		h := hijacker.New(target.TLSConfig(), reqGenerator, target.Token())

		return h.Hijack(team.Name(), chosenContainer.ID, spec, io)
	}()

	if err != nil {
		return err
	}

	os.Exit(result)

	return nil
}

func parseUrlPath(urlPath string) map[string]string {
	pathWithoutFirstSlash := strings.Replace(urlPath, "/", "", 1)
	urlComponents := strings.Split(pathWithoutFirstSlash, "/")
	urlMap := make(map[string]string)

	for i := 0; i < len(urlComponents)/2; i++ {
		keyIndex := i * 2
		valueIndex := keyIndex + 1
		urlMap[urlComponents[keyIndex]] = urlComponents[valueIndex]
	}

	return urlMap
}

func (command *HijackCommand) getContainerFingerprintFromUrl(target rc.Target, urlParam string, team concourse.Team) (*containerFingerprint, error) {
	u, err := url.Parse(urlParam)
	if err != nil {
		return nil, err
	}

	urlMap := parseUrlPath(u.Path)

	parsedTargetUrl := url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	host := parsedTargetUrl.String()
	if host != target.URL() {
		err = fmt.Errorf("URL doesn't match that of target")
		return nil, err
	}

	teamFromUrl := urlMap["teams"]

	if teamFromUrl != team.Name() {
		err = fmt.Errorf("Team in URL doesn't match the current team of the target")
		return nil, err
	}

	fingerprint := &containerFingerprint{
		pipelineName:         urlMap["pipelines"],
		pipelineInstanceVars: u.Query().Get("instance_vars"),
		jobName:              urlMap["jobs"],
		buildNameOrID:        urlMap["builds"],
		checkName:            urlMap["resources"],
	}

	return fingerprint, nil
}

func (command *HijackCommand) getContainerFingerprint(target rc.Target, team concourse.Team) (*containerFingerprint, error) {
	var err error
	fingerprint := &containerFingerprint{}

	if command.Url != "" {
		fingerprint, err = command.getContainerFingerprintFromUrl(target, command.Url, team)
		if err != nil {
			return nil, err
		}
	}

	pipelineName := command.Check.PipelineRef.Name
	if command.Job.PipelineRef.Name != "" {
		pipelineName = command.Job.PipelineRef.Name
	}

	var pipelineInstanceVars string
	var instanceVars atc.InstanceVars
	if command.Check.PipelineRef.InstanceVars != nil {
		instanceVars = command.Check.PipelineRef.InstanceVars
	} else {
		instanceVars = command.Job.PipelineRef.InstanceVars
	}
	if instanceVars != nil {
		instanceVarsJSON, _ := json.Marshal(instanceVars)
		pipelineInstanceVars = string(instanceVarsJSON)
	}

	for _, field := range []struct {
		fp  *string
		cmd string
	}{
		{fp: &fingerprint.pipelineName, cmd: pipelineName},
		{fp: &fingerprint.pipelineInstanceVars, cmd: pipelineInstanceVars},
		{fp: &fingerprint.buildNameOrID, cmd: command.Build},
		{fp: &fingerprint.stepName, cmd: command.StepName},
		{fp: &fingerprint.stepType, cmd: command.StepType},
		{fp: &fingerprint.jobName, cmd: command.Job.JobName},
		{fp: &fingerprint.checkName, cmd: command.Check.ResourceName},
		{fp: &fingerprint.attempt, cmd: command.Attempt},
	} {
		if field.cmd != "" {
			*field.fp = field.cmd
		}
	}

	return fingerprint, nil
}

func (command *HijackCommand) getContainerIDs(target rc.Target, fingerprint *containerFingerprint, team concourse.Team) ([]atc.Container, error) {
	reqValues, err := locateContainer(target.Client(), fingerprint)
	if err != nil {
		return nil, err
	}

	containers, err := team.ListContainers(reqValues)
	if err != nil {
		return nil, err
	}
	sort.Sort(hijackhelpers.ContainerSorter(containers))

	return containers, nil
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
	locate(*containerFingerprint) (map[string]string, error)
}

type stepContainerLocator struct {
	client concourse.Client
}

func (locator stepContainerLocator) locate(fingerprint *containerFingerprint) (map[string]string, error) {
	reqValues := map[string]string{}

	if fingerprint.stepType != "" {
		reqValues["type"] = fingerprint.stepType
	}

	if fingerprint.stepName != "" {
		reqValues["step_name"] = fingerprint.stepName
	}

	if fingerprint.attempt != "" {
		reqValues["attempt"] = fingerprint.attempt
	}

	if fingerprint.jobName != "" {
		reqValues["pipeline_name"] = fingerprint.pipelineName
		if fingerprint.pipelineInstanceVars != "" {
			reqValues["instance_vars"] = fingerprint.pipelineInstanceVars
		}
		reqValues["job_name"] = fingerprint.jobName
		if fingerprint.buildNameOrID != "" {
			reqValues["build_name"] = fingerprint.buildNameOrID
		}
	} else if fingerprint.buildNameOrID != "" {
		reqValues["build_id"] = fingerprint.buildNameOrID
	} else {
		build, err := GetBuild(locator.client, nil, "", "", atc.PipelineRef{})
		if err != nil {
			return reqValues, err
		}
		reqValues["build_id"] = strconv.Itoa(build.ID)
	}

	return reqValues, nil
}

type checkContainerLocator struct{}

func (locator checkContainerLocator) locate(fingerprint *containerFingerprint) (map[string]string, error) {
	reqValues := map[string]string{}

	reqValues["type"] = "check"
	if fingerprint.checkName != "" {
		reqValues["resource_name"] = fingerprint.checkName
	}
	if fingerprint.pipelineName != "" {
		reqValues["pipeline_name"] = fingerprint.pipelineName
	}
	if fingerprint.pipelineInstanceVars != "" {
		reqValues["instance_vars"] = fingerprint.pipelineInstanceVars
	}

	return reqValues, nil
}

type containerFingerprint struct {
	pipelineName         string
	pipelineInstanceVars string
	jobName              string
	buildNameOrID        string

	stepName string
	stepType string

	checkName string
	attempt   string
}

func locateContainer(client concourse.Client, fingerprint *containerFingerprint) (map[string]string, error) {
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
