package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"

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

	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/codegangsta/cli"
	"github.com/concourse/atc/api/resources"
	"github.com/concourse/atc/api/routes"
	"github.com/concourse/fly/eventstream"
	tbuilds "github.com/concourse/turbine/api/builds"
	"github.com/gorilla/websocket"
	"github.com/pivotal-golang/archiver/compressor"
	"github.com/tedsuo/rata"
)

type Input struct {
	Name string
	Path string
	Pipe resources.Pipe
}

func execute(c *cli.Context) {
	atc := c.GlobalString("atcURL")
	buildConfig := c.String("config")
	insecure := c.GlobalBool("insecure")
	excludeIgnored := c.GlobalBool("exclude-ignored")

	atcRequester := newAtcRequester(atc, insecure)

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

	build, cookies := createBuild(
		atcRequester,
		c.Bool("privileged"),
		inputs,
		loadConfig(absConfig, c.Args()),
	)

	terminate := make(chan os.Signal, 1)

	go abortOnSignal(atcRequester, terminate, build)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

	logOutput, err := atcRequester.CreateRequest(
		routes.BuildEvents,
		rata.Params{"build_id": strconv.Itoa(build.ID)},
		nil,
	)
	if err != nil {
		log.Fatalln(err)
	}

	logOutput.URL.Scheme = "ws"
	logOutput.URL.User = nil

	cookieHeaders := []string{}
	for _, cookie := range cookies {
		cookieHeaders = append(cookieHeaders, cookie.String())
	}

	conn, res, err := atcRequester.websocketDialer.Dial(
		logOutput.URL.String(),
		http.Header{"Cookie": cookieHeaders},
	)
	if err != nil {
		log.Println("failed to stream output:", err, res)
		os.Exit(1)
	}

	go func() {
		for _, i := range inputs {
			upload(i, excludeIgnored, atcRequester)
		}
	}()

	exitCode, err := eventstream.RenderStream(conn)
	if err != nil {
		log.Println("failed to render stream:", err)
		os.Exit(1)
	}

	res.Body.Close()
	conn.Close()

	os.Exit(exitCode)
}

func createPipe(atcRequester *atcRequester) resources.Pipe {
	cPipe, err := atcRequester.CreateRequest(routes.CreatePipe, nil, nil)
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

	var pipe resources.Pipe
	err = json.NewDecoder(response.Body).Decode(&pipe)
	if err != nil {
		log.Println("malformed response when creating pipe:", err)
		os.Exit(1)
	}

	return pipe
}

func loadConfig(configPath string, args []string) tbuilds.Config {
	configFile, err := os.Open(configPath)
	if err != nil {
		log.Fatalln("could not open config file:", err)
	}

	var config tbuilds.Config

	err = candiedyaml.NewDecoder(configFile).Decode(&config)
	if err != nil {
		log.Fatalln("could not parse config file:", err)
	}

	config.Run.Args = append(config.Run.Args, args...)

	for k, _ := range config.Params {
		env, found := syscall.Getenv(k)
		if found {
			config.Params[k] = env
		}
	}

	return config
}

func createBuild(
	atcRequester *atcRequester,
	privileged bool,
	inputs []Input,
	config tbuilds.Config,
) (resources.Build, []*http.Cookie) {
	buffer := &bytes.Buffer{}

	buildInputs := make([]tbuilds.Input, len(inputs))
	for idx, i := range inputs {
		readPipe, err := atcRequester.CreateHTTPRequest(
			routes.ReadPipe,
			rata.Params{"pipe_id": i.Pipe.ID},
			nil,
		)
		if err != nil {
			log.Fatalln(err)
		}

		readPipe.URL.Host = i.Pipe.PeerAddr

		buildInputs[idx] = tbuilds.Input{
			Name: i.Name,
			Type: "archive",
			Source: tbuilds.Source{
				"uri": readPipe.URL.String(),
			},
		}
	}

	turbineBuild := tbuilds.Build{
		Privileged: privileged,
		Config:     config,
		Inputs:     buildInputs,
	}

	err := json.NewEncoder(buffer).Encode(turbineBuild)
	if err != nil {
		log.Fatalln("encoding build failed:", err)
	}

	createBuild, err := atcRequester.CreateRequest(routes.CreateBuild, nil, buffer)
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

	var build resources.Build
	err = json.NewDecoder(response.Body).Decode(&build)
	if err != nil {
		log.Fatalln("response decoding failed:", err)
	}

	return build, response.Cookies()
}

func abortOnSignal(
	atcRequester *atcRequester,
	terminate <-chan os.Signal,
	build resources.Build,
) {
	<-terminate

	println("\naborting...")

	abortReq, err := atcRequester.CreateRequest(
		routes.AbortBuild,
		rata.Params{"build_id": strconv.Itoa(build.ID)},
		nil,
	)

	if err != nil {
		log.Fatalln(err)
	}

	resp, err := atcRequester.httpClient.Do(abortReq)
	if err != nil {
		log.Println("failed to abort:", err)
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
	}

	defer archive.Close()

	uploadBits, err := atcRequester.CreateRequest(
		routes.WritePipe,
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
	httpClient      *http.Client
	websocketDialer *websocket.Dialer
}

func newAtcRequester(atcUrl string, insecure bool) *atcRequester {
	tlsClientConfig := &tls.Config{InsecureSkipVerify: insecure}

	return &atcRequester{
		rata.NewRequestGenerator(atcUrl, routes.Routes),
		&http.Client{Transport: &http.Transport{TLSClientConfig: tlsClientConfig}},
		&websocket.Dialer{TLSClientConfig: tlsClientConfig},
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
