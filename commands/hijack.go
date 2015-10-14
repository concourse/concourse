// +build !windows

package commands

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	"github.com/kr/pty"
	"github.com/mgutz/ansi"
	"github.com/pkg/term"
	"github.com/tedsuo/rata"
)

type HijackCommand struct {
	Job      JobFlag      `short:"j" long:"job"   value-name:"[PIPELINE/]JOB"   description:"Name of a job to hijack"`
	Check    ResourceFlag `short:"c" long:"check" value-name:"[PIPELINE/]CHECK" description:"Name of a resource's checking container to hijack"`
	Build    string       `short:"b" long:"build"                               description:"Name of a specific build of a job"`
	StepName string       `short:"s" long:"step"                                description:"Name of step to hijack (e.g. build, unit, resource name)"`
}

var hijackCommand HijackCommand

func init() {

	hijack, err := Parser.AddCommand(
		"hijack",
		"Execute an interactive command in a build's container",
		"",
		&hijackCommand,
	)
	if err != nil {
		panic(err)
	}

	hijack.Aliases = []string{"intercept", "i"}
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
	locate(containerFingerprint) url.Values
}

type stepContainerLocator struct {
	handler atcclient.Handler
}

func (locator stepContainerLocator) locate(fingerprint containerFingerprint) url.Values {
	build := getBuild(
		locator.handler,
		fingerprint.jobName,
		fingerprint.buildName,
		fingerprint.pipelineName,
	)

	reqValues := url.Values{}
	reqValues["build-id"] = []string{strconv.Itoa(build.ID)}
	reqValues["name"] = []string{fingerprint.stepName}

	return reqValues
}

type checkContainerLocator struct{}

func (locator checkContainerLocator) locate(fingerprint containerFingerprint) url.Values {
	reqValues := url.Values{}

	reqValues["type"] = []string{"check"}
	if fingerprint.checkName != "" {
		reqValues["name"] = []string{fingerprint.checkName}
	}
	if fingerprint.pipelineName != "" {
		reqValues["pipeline"] = []string{fingerprint.pipelineName}
	}

	return reqValues
}

type containerFingerprint struct {
	pipelineName string
	jobName      string
	buildName    string

	stepName string

	checkName string
}

func locateContainer(handler atcclient.Handler, fingerprint containerFingerprint) url.Values {
	var locator containerLocator

	if fingerprint.checkName == "" {
		locator = stepContainerLocator{
			handler: handler,
		}
	} else {
		locator = checkContainerLocator{}
	}

	return locator.locate(fingerprint)
}

func constructRequest(reqGenerator *rata.RequestGenerator, spec atc.HijackProcessSpec, id string) *http.Request {
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

	if hijackReq.URL.User != nil {
		hijackReq.Header.Add("Authorization", basicAuth(hijackReq.URL.User))
		hijackReq.URL.User = nil
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

	target, _ := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	client, err := atcclient.NewClient(*target)
	if err != nil {
		log.Fatalln("failed to create client:", err)
	}
	handler := atcclient.NewAtcHandler(client)

	reqValues := locateContainer(handler, fingerprint)

	atcRequester := newAtcRequester(target.URL(), target.Insecure)
	listContainersReq, err := atcRequester.RequestGenerator.CreateRequest(
		atc.ListContainers,
		rata.Params{},
		nil,
	)
	if err != nil {
		log.Fatalln("failed to create containers list request:", err)
	}
	listContainersReq.URL.RawQuery = reqValues.Encode()

	resp, err := atcRequester.httpClient.Do(listContainersReq)
	if err != nil {
		log.Fatalln("failed to get containers:", err)
	}

	var containers []atc.Container
	err = json.NewDecoder(resp.Body).Decode(&containers)
	if err != nil {
		log.Fatalln("failed to decode containers:", err)
	}

	return containers
}

func (command *HijackCommand) Execute(args []string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	containers := getContainerIDs(command)

	var id string
	var selection int
	if len(containers) == 0 {
		fmt.Fprintln(os.Stderr, "no containers matched your search parameters! they may have expired if your build hasn't recently finished")
		os.Exit(1)
	} else if len(containers) > 1 {
		for i, container := range containers {
			fmt.Printf("%d. ", i+1)

			if container.PipelineName != "" {
				fmt.Printf("pipeline: %s, ", container.PipelineName)
			}

			if container.BuildID != 0 {
				fmt.Printf("build id: %d, ", container.BuildID)
			}

			fmt.Printf("type: %s, ", container.Type)
			fmt.Printf("name: %s", container.Name)

			fmt.Printf("\n")
		}

		for {
			fmt.Printf("choose a container: ")

			_, err := fmt.Scanf("%d", &selection)

			if err == io.EOF {
				os.Exit(0)
			}

			if err != nil || selection > len(containers) || selection < 1 {
				fmt.Println("invalid selection", err)
				continue
			}

			break
		}

		id = containers[selection-1].ID
	} else {
		id = containers[0].ID
	}

	path, args := remoteCommand(args)
	privileged := true

	reqGenerator := rata.NewRequestGenerator(target.URL(), atc.Routes)
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

		Privileged: privileged,
		TTY:        ttySpec,
	}

	hijackReq := constructRequest(reqGenerator, spec, id)
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

	term, err := term.Open(os.Stdin.Name())
	if err == nil {
		err = term.SetRaw()
		if err != nil {
			log.Fatalln("failed to set raw:", term)
		}

		defer term.Restore()

		in = term
	} else {
		in = os.Stdin
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(br)

	resized := make(chan os.Signal, 10)
	signal.Notify(resized, syscall.SIGWINCH)

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

func basicAuth(user *url.Userinfo) string {
	username := user.Username()
	password, _ := user.Password()
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

var canonicalPortMap = map[string]string{
	"http":  "80",
	"https": "443",
}

func dialEndpoint(url *url.URL, tlsConfig *tls.Config) (net.Conn, error) {
	addr := canonicalAddr(url)

	if url.Scheme == "https" {
		return tls.Dial("tcp", addr, tlsConfig)
	} else {
		return net.Dial("tcp", addr)
	}
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
