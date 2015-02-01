package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
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
	"github.com/pivotal-golang/archiver/compressor"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

type Input struct {
	Name string
	Path string
	Pipe atc.Pipe
}

func Execute(c *cli.Context) {
	atcURL := c.GlobalString("atcURL")
	buildConfig := c.String("config")
	insecure := c.GlobalBool("insecure")
	excludeIgnored := c.GlobalBool("exclude-ignored")

	atcRequester := newAtcRequester(atcURL, insecure)

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
		config.LoadBuildConfig(absConfig, c.Args()),
	)

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
		log.Println("bad response when creating pipe:", response)
		response.Write(os.Stderr)
		os.Exit(1)
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
	config atc.BuildConfig,
) atc.Build {
	buffer := &bytes.Buffer{}

	buildInputs := make([]atc.InputPlan, len(inputs))
	for idx, i := range inputs {
		readPipe, err := atcRequester.CreateHTTPRequest(
			atc.ReadPipe,
			rata.Params{"pipe_id": i.Pipe.ID},
			nil,
		)
		if err != nil {
			log.Fatalln(err)
		}

		readPipe.URL.Host = i.Pipe.PeerAddr

		buildInputs[idx] = atc.InputPlan{
			Name: i.Name,
			Type: "archive",
			Source: atc.Source{
				"uri": readPipe.URL.String(),
			},
		}
	}

	buildPlan := atc.BuildPlan{
		Privileged: privileged,
		Config:     &config,
		Inputs:     buildInputs,
	}

	err := json.NewEncoder(buffer).Encode(buildPlan)
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
		log.Println("bad response when creating build:", response)
		response.Write(os.Stderr)
		os.Exit(1)
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

	var archive io.ReadCloser
	if tarPath, err := exec.LookPath("tar"); err != nil {
		compressor := compressor.NewTgz()

		tmpfile, err := ioutil.TempFile("", "fly")
		if err != nil {
			log.Fatalln("creating tempfile failed:", err)
		}

		tmpfile.Close()

		defer os.Remove(tmpfile.Name())

		err = compressor.Compress(path+"/", tmpfile.Name())
		if err != nil {
			log.Fatalln("creating archive failed:", err)
		}

		archive, err = os.Open(tmpfile.Name())
		if err != nil {
			log.Fatalln("could not open archive:", err)
		}
	} else {
		var files []string

		if excludeIgnored {
			files, err = getGitFiles(path)
			if err != nil {
				log.Fatalln("could not determine ignored files:", err)
			}
		} else {
			files = []string{"."}
		}

		tarCmd := exec.Command(tarPath, append([]string{"-czf", "-"}, files...)...)
		tarCmd.Dir = path
		tarCmd.Stderr = os.Stderr

		archive, err = tarCmd.StdoutPipe()
		if err != nil {
			log.Fatalln("could not create tar pipe:", err)
		}

		err = tarCmd.Start()
		if err != nil {
			log.Fatalln("could not run tar:", err)
		}

		defer tarCmd.Wait()
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
		log.Println("bad response when uploading bits:", response)
		response.Write(os.Stderr)
		os.Exit(1)
	}
}

type atcRequester struct {
	*rata.RequestGenerator
	httpClient *http.Client
}

func newAtcRequester(atcUrl string, insecure bool) *atcRequester {
	tlsClientConfig := &tls.Config{InsecureSkipVerify: insecure}

	return &atcRequester{
		rata.NewRequestGenerator(atcUrl, atc.Routes),
		&http.Client{Transport: &http.Transport{TLSClientConfig: tlsClientConfig}},
	}
}

func (ar *atcRequester) CreateHTTPRequest(
	name string,
	params rata.Params,
	body io.Reader,
) (*http.Request, error) {
	request, err := ar.CreateRequest(name, params, body)
	if err != nil {
		return nil, err
	}

	url := request.URL
	url.Scheme = "http"
	request.URL = url
	return request, nil
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
