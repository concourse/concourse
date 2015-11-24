package commands

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/pty"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/mgutz/ansi"
	"github.com/tedsuo/rata"
	"github.com/vito/go-interact/interact"
)

type HijackCommand struct {
	Job      flaghelpers.JobFlag      `short:"j" long:"job"   value-name:"PIPELINE/JOB"   description:"Name of a job to hijack"`
	Check    flaghelpers.ResourceFlag `short:"c" long:"check" value-name:"PIPELINE/CHECK" description:"Name of a resource's checking container to hijack"`
	Build    string                   `short:"b" long:"build"                               description:"Name of a specific build of a job"`
	StepName string                   `short:"s" long:"step"                                description:"Name of step to hijack (e.g. build, unit, resource name)"`
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

	build, err := GetBuild(
		locator.client,
		fingerprint.jobName,
		fingerprint.buildName,
		fingerprint.pipelineName,
	)
	if err != nil {
		return reqValues, err
	}

	reqValues["build-id"] = strconv.Itoa(build.ID)
	reqValues["name"] = fingerprint.stepName

	return reqValues, nil
}

type checkContainerLocator struct{}

func (locator checkContainerLocator) locate(fingerprint containerFingerprint) (map[string]string, error) {
	reqValues := map[string]string{}

	reqValues["type"] = "check"
	if fingerprint.checkName != "" {
		reqValues["name"] = fingerprint.checkName
	}
	if fingerprint.pipelineName != "" {
		reqValues["pipeline_name"] = fingerprint.pipelineName
	}

	return reqValues, nil
}

type containerFingerprint struct {
	pipelineName string
	jobName      string
	buildName    string

	stepName string

	checkName string
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

func constructRequest(reqGenerator *rata.RequestGenerator, spec atc.HijackProcessSpec, id string, token *rc.TargetToken) *http.Request {
	payload, err := json.Marshal(spec)
	if err != nil {
		log.Fatalln("failed to marshal process spec:", err)
	}

	hijackReq, err := reqGenerator.CreateRequest(
		atc.HijackContainer,
		rata.Params{"id": id},
		bytes.NewBuffer(payload),
	)
	if err != nil {
		log.Fatalln("failed to create hijack request:", err)
	}

	if token != nil {
		hijackReq.Header.Add("Authorization", token.Type+" "+token.Value)
	}

	return hijackReq
}

func getContainerIDs(c *HijackCommand) []atc.Container {
	var pipelineName string
	if c.Job.PipelineName != "" {
		pipelineName = c.Job.PipelineName
	} else {
		pipelineName = c.Check.PipelineName
	}

	buildName := c.Build
	stepName := c.StepName
	jobName := c.Job.JobName
	check := c.Check.ResourceName

	fingerprint := containerFingerprint{
		pipelineName: pipelineName,
		jobName:      jobName,
		buildName:    buildName,
		stepName:     stepName,
		checkName:    check,
	}

	connection, err := rc.TargetConnection(Fly.Target)
	if err != nil {
		log.Fatalln("failed to create client:", err)
	}
	client := concourse.NewClient(connection)

	reqValues, err := locateContainer(client, fingerprint)
	if err != nil {
		log.Fatalln(err)
	}

	containers, err := client.ListContainers(reqValues)
	if err != nil {
		log.Fatalln("failed to get containers:", err)
	}
	return containers
}

func (command *HijackCommand) Execute(args []string) error {
	target, err := rc.SelectTarget(Fly.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	containers := getContainerIDs(command)

	var chosenContainer atc.Container
	if len(containers) == 0 {
		fmt.Fprintln(os.Stderr, "no containers matched your search parameters! they may have expired if your build hasn't recently finished")
		os.Exit(1)
	} else if len(containers) > 1 {
		var choices []interact.Choice
		for _, container := range containers {
			var infos []string
			if container.PipelineName != "" {
				infos = append(infos, fmt.Sprintf("pipeline: %s", container.PipelineName))
			}

			if container.BuildID != 0 {
				infos = append(infos, fmt.Sprintf("build id: %d", container.BuildID))
			}

			infos = append(infos, fmt.Sprintf("type: %s", container.Type))
			infos = append(infos, fmt.Sprintf("name: %s", container.Name))

			choices = append(choices, interact.Choice{
				Display: strings.Join(infos, ", "),
				Value:   container,
			})
		}

		err = interact.NewInteraction("choose a container", choices...).Resolve(&chosenContainer)
		if err == io.EOF {
			os.Exit(0)
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

	spec := atc.HijackProcessSpec{
		Path: path,
		Args: args,
		Env:  []string{"TERM=" + os.Getenv("TERM")},
		User: "root",
		Dir:  chosenContainer.WorkingDirectory,

		Privileged: privileged,
		TTY:        ttySpec,
	}

	hijackReq := constructRequest(reqGenerator, spec, chosenContainer.ID, target.Token)
	hijackResult := performHijack(hijackReq, tlsConfig)
	os.Exit(hijackResult)

	return nil
}

func performHijack(hijackReq *http.Request, tlsConfig *tls.Config) int {
	conn, err := dialEndpoint(hijackReq.URL, tlsConfig)
	if err != nil {
		log.Fatalln("failed to dial hijack endpoint:", err)
	}

	clientConn := httputil.NewClientConn(conn, nil)

	resp, err := clientConn.Do(hijackReq)
	if err != nil {
		log.Fatalln("failed to hijack:", err)
	}

	if resp.StatusCode != http.StatusOK {
		handleBadResponse("hijacking", resp)
	}

	return hijack(clientConn.Hijack())
}

func hijack(conn net.Conn, br *bufio.Reader) int {
	var in io.Reader

	term, err := pty.OpenRawTerm()
	if err == nil {
		defer term.Restore()

		in = term
	} else {
		in = os.Stdin
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(br)

	resized := pty.ResizeNotifier()

	go func() {
		for {
			<-resized
			// TODO json race
			sendSize(encoder)
		}
	}()

	go io.Copy(&stdinWriter{encoder}, in)

	var exitStatus int
	for {
		var output atc.HijackOutput
		err := decoder.Decode(&output)
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

func sendSize(enc *json.Encoder) {
	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		enc.Encode(atc.HijackInput{
			TTYSpec: &atc.HijackTTYSpec{
				WindowSize: atc.HijackWindowSize{
					Columns: cols,
					Rows:    rows,
				},
			},
		})
	}
}

type stdinWriter struct {
	enc *json.Encoder
}

func (w *stdinWriter) Write(d []byte) (int, error) {
	err := w.enc.Encode(atc.HijackInput{
		Stdin: d,
	})
	if err != nil {
		return 0, err
	}

	return len(d), nil
}

var canonicalPortMap = map[string]string{
	"http":  "80",
	"https": "443",
}

func dialEndpoint(url *url.URL, tlsConfig *tls.Config) (net.Conn, error) {
	addr := canonicalAddr(url)

	if url.Scheme == "https" {
		return tls.Dial("tcp", addr, tlsConfig)
	}

	return net.Dial("tcp", addr)
}

func canonicalAddr(url *url.URL) string {
	host, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			host = url.Host
			port = canonicalPortMap[url.Scheme]
		} else {
			log.Fatalln("invalid host:", err)
		}
	}

	return net.JoinHostPort(host, port)
}
