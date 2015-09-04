package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/concourse/atc"
	"github.com/concourse/fly/config"
	"github.com/concourse/fly/eventstream"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

type Input struct {
	Name string
	Path string
	Pipe atc.Pipe
}

func Execute(c *cli.Context) {
	target := returnTarget(c.GlobalString("target"))
	buildConfig := c.String("config")
	insecure := c.GlobalBool("insecure")
	excludeIgnored := c.Bool("exclude-ignored")

	atcRequester := newAtcRequester(target, insecure)

	inputMappings := c.StringSlice("input")
	if len(inputMappings) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalln(err)
		}

		inputMappings = append(inputMappings, filepath.Base(wd)+"="+wd)
	}

	inputs := []Input{}
	for _, i := range inputMappings {
		segs := strings.SplitN(i, "=", 2)
		if len(segs) < 2 {
			log.Println("malformed input:", i)
			os.Exit(1)
		}

		inputName := segs[0]

		absPath, err := filepath.Abs(segs[1])
		if err != nil {
			log.Printf("could not locate input %s: %s\n", inputName, err)
			os.Exit(1)
		}

		pipe := createPipe(atcRequester)

		inputs = append(inputs, Input{
			Name: inputName,
			Path: absPath,
			Pipe: pipe,
		})
	}

	absConfig, err := filepath.Abs(buildConfig)
	if err != nil {
		log.Println("could not locate config file:", err)
		os.Exit(1)
	}

	build := createBuild(
		atcRequester,
		c.Bool("privileged"),
		inputs,
		config.LoadTaskConfig(absConfig, c.Args()),
	)

	fmt.Fprintf(os.Stdout, "executing build %d\n", build.ID)

	terminate := make(chan os.Signal, 1)

	go abortOnSignal(atcRequester, terminate, build)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

	eventSource, err := sse.Connect(atcRequester.httpClient, time.Second, func() *http.Request {
		logOutput, err := atcRequester.CreateRequest(
			atc.BuildEvents,
			rata.Params{"build_id": strconv.Itoa(build.ID)},
			nil,
		)
		if err != nil {
			log.Fatalln(err)
		}

		return logOutput
	})
	if err != nil {
		log.Println("failed to connect to event stream:", err)
		os.Exit(1)
	}

	go func() {
		for _, i := range inputs {
			upload(i, excludeIgnored, atcRequester)
		}
	}()

	exitCode, err := eventstream.RenderStream(eventSource)
	if err != nil {
		log.Println("failed to render stream:", err)
		os.Exit(1)
	}

	eventSource.Close()

	os.Exit(exitCode)
}

func createPipe(atcRequester *atcRequester) atc.Pipe {
	cPipe, err := atcRequester.CreateRequest(atc.CreatePipe, nil, nil)
	if err != nil {
		log.Fatalln(err)
	}

	response, err := atcRequester.httpClient.Do(cPipe)
	if err != nil {
		log.Fatalln("request failed:", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		handleBadResponse("creating pipe", response)
	}

	var pipe atc.Pipe
	err = json.NewDecoder(response.Body).Decode(&pipe)
	if err != nil {
		log.Println("malformed response when creating pipe:", err)
		os.Exit(1)
	}

	return pipe
}

func createBuild(
	atcRequester *atcRequester,
	privileged bool,
	inputs []Input,
	config atc.TaskConfig,
) atc.Build {
	if err := config.Validate(); err != nil {
		println(err.Error())
		os.Exit(1)
	}

	buffer := &bytes.Buffer{}

	buildInputs := atc.AggregatePlan{}
	for i, input := range inputs {
		readPipe, err := atcRequester.CreateRequest(
			atc.ReadPipe,
			rata.Params{"pipe_id": input.Pipe.ID},
			nil,
		)
		if err != nil {
			log.Fatalln(err)
		}

		buildInputs = append(buildInputs, atc.Plan{
			Location: &atc.Location{
				// offset by 2 because aggregate gets parallelgroup ID 1
				ID:            uint(i) + 2,
				ParentID:      0,
				ParallelGroup: 1,
			},
			Get: &atc.GetPlan{
				Name: input.Name,
				Type: "archive",
				Source: atc.Source{
					"uri": readPipe.URL.String(),
				},
			},
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
					Name:       "build",
					Privileged: privileged,
					Config:     &config,
				},
			},
		},
	}

	err := json.NewEncoder(buffer).Encode(plan)
	if err != nil {
		log.Fatalln("encoding build failed:", err)
	}

	createBuild, err := atcRequester.CreateRequest(atc.CreateBuild, nil, buffer)
	if err != nil {
		log.Fatalln(err)
	}

	createBuild.Header.Set("Content-Type", "application/json")

	response, err := atcRequester.httpClient.Do(createBuild)
	if err != nil {
		log.Fatalln("request failed:", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		handleBadResponse("creating build", response)
	}

	var build atc.Build
	err = json.NewDecoder(response.Body).Decode(&build)
	if err != nil {
		log.Fatalln("response decoding failed:", err)
	}

	return build
}

func abortOnSignal(
	atcRequester *atcRequester,
	terminate <-chan os.Signal,
	build atc.Build,
) {
	<-terminate

	println("\naborting...")

	abortReq, err := atcRequester.CreateRequest(
		atc.AbortBuild,
		rata.Params{"build_id": strconv.Itoa(build.ID)},
		nil,
	)
	if err != nil {
		log.Fatalln(err)
	}

	resp, err := atcRequester.httpClient.Do(abortReq)
	if err != nil {
		log.Println("failed to abort:", err)
		os.Exit(255)
	}

	resp.Body.Close()

	// if told to terminate again, exit immediately
	<-terminate
	println("exiting immediately")
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
			log.Fatalln("could not determine ignored files:", err)
		}
	} else {
		files = []string{"."}
	}

	archive, err := tarStreamFrom(path, files)
	if err != nil {
		log.Fatalln("failed to create tar stream:", err)
	}

	defer archive.Close()

	uploadBits, err := atcRequester.CreateRequest(
		atc.WritePipe,
		rata.Params{"pipe_id": pipe.ID},
		archive,
	)
	if err != nil {
		log.Fatalln(err)
	}

	response, err := atcRequester.httpClient.Do(uploadBits)
	if err != nil {
		log.Fatalln("request failed:", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		handleBadResponse("uploading bits", response)
	}
}

type atcRequester struct {
	*rata.RequestGenerator
	httpClient *http.Client
}

func newAtcRequester(target string, insecure bool) *atcRequester {
	tlsClientConfig := &tls.Config{InsecureSkipVerify: insecure}

	return &atcRequester{
		rata.NewRequestGenerator(target, atc.Routes),
		&http.Client{Transport: &http.Transport{TLSClientConfig: tlsClientConfig}},
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
