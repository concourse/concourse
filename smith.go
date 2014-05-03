package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/fraenkel/candiedyaml"
	"github.com/gorilla/websocket"
	"github.com/mgutz/ansi"
	"github.com/pivotal-golang/archiver/compressor"
	"github.com/tedsuo/router"

	"github.com/winston-ci/redgreen/routes"
)

type Build struct {
	Guid   string      `json:"guid,omitempty"`
	Image  string      `json:"image"`
	Path   string      `json:"path"`
	Script string      `json:"script"`
	Env    [][2]string `json:"env"`
}

type BuildResult struct {
	Status string `json:"status"`
}

type BuildConfig struct {
	Image  string   `yaml:"image"`
	Path   string   `yaml:"path"`
	Script string   `yaml:"script"`
	Env    []string `yaml:"env"`
}

var buildConfig = flag.String(
	"c",
	"build.yml",
	"build configuration file",
)

var buildDir = flag.String(
	"d",
	".",
	"source directory to build",
)

var redgreenURL = flag.String(
	"redgreenURL",
	os.Getenv("REDGREEN_URL"),
	"address denoting the redgreen service",
)

func main() {
	flag.Parse()

	if *redgreenURL == "" {
		println("must specify $REDGREEN_URL. for example:")
		println()
		println("export REDGREEN_URL=http://10.244.8.66:5637")
		os.Exit(1)
	}

	redgreenURL, err := url.Parse(*redgreenURL)
	if err != nil {
		log.Fatalln("could not parse redgreen URL:", err)
	}

	endpoint := &EndpointRoutes{
		URL:    redgreenURL,
		Routes: routes.Routes,
	}

	build := create(endpoint, loadConfig())

	logOutput, err := endpoint.RequestForHandler(
		routes.LogOutput,
		router.Params{"guid": build.Guid},
		nil,
	)
	if err != nil {
		log.Fatalln(err)
	}

	logOutput.URL.Scheme = "ws"

	conn, res, err := websocket.DefaultDialer.Dial(logOutput.URL.String(), nil)
	if err != nil {
		log.Println("failed to stream output:", err, res)
		return
	}

	streaming := new(sync.WaitGroup)
	streaming.Add(1)

	go stream(conn, streaming)

	upload(endpoint, build)

	exitCode := poll(endpoint, build)

	res.Body.Close()
	conn.Close()

	streaming.Wait()

	os.Exit(exitCode)
}

type ConfigContext struct {
	Args string
}

func loadConfig() BuildConfig {
	passArgs := false
	args := []string{}
	for _, arg := range os.Args {
		if arg == "--" {
			passArgs = true
			continue
		}

		if passArgs {
			args = append(args, "\""+strings.Replace(arg, `"`, `\"`, -1)+"\"")
		}
	}

	context := ConfigContext{
		Args: strings.Join(args, " "),
	}

	template, err := template.ParseFiles(*buildConfig)
	if err != nil {
		log.Fatalln("could not open config file:", err)
	}

	rendered := new(bytes.Buffer)

	err = template.Execute(rendered, context)
	if err != nil {
		log.Fatalln("could not render config file:", err)
	}

	var config BuildConfig

	err = candiedyaml.NewDecoder(rendered).Decode(&config)
	if err != nil {
		log.Fatalln("could not parse config file:", err)
	}

	return config
}

func create(endpoint *EndpointRoutes, config BuildConfig) Build {
	buffer := &bytes.Buffer{}

	env := [][2]string{}
	for _, e := range config.Env {
		segs := strings.SplitN(e, "=", 2)
		if len(segs) != 2 {
			log.Fatalln("invalid environment configuration:", e)
		}

		env = append(env, [2]string{segs[0], segs[1]})
	}

	build := Build{
		Image:  config.Image,
		Path:   config.Path,
		Script: config.Script,
		Env:    env,
	}

	if build.Path == "" {
		build.Path = "."
	}

	err := json.NewEncoder(buffer).Encode(build)
	if err != nil {
		log.Fatalln("encoding build failed:", err)
	}

	createBuild, err := endpoint.RequestForHandler(routes.CreateBuild, nil, buffer)
	if err != nil {
		log.Fatalln(err)
	}

	createBuild.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(createBuild)
	if err != nil {
		log.Fatalln("request failed:", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		log.Println("bad response when creating build:", response)
		response.Write(os.Stderr)
		os.Exit(1)
	}

	err = json.NewDecoder(response.Body).Decode(&build)
	if err != nil {
		log.Fatalln("response decoding failed:", err)
	}

	return build
}

func stream(conn *websocket.Conn, streaming *sync.WaitGroup) {
	defer streaming.Done()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		fmt.Print(string(data))
	}
}

func upload(endpoint *EndpointRoutes, build Build) {
	src, err := filepath.Abs(*buildDir)
	if err != nil {
		log.Fatalln("could not locate build config:", err)
	}

	compressor := compressor.NewTgz()

	tmpfile, err := ioutil.TempFile("", "smith")
	if err != nil {
		log.Fatalln("creating tempfile failed:", err)
	}

	tmpfile.Close()

	defer os.Remove(tmpfile.Name())

	err = compressor.Compress(src, tmpfile.Name())
	if err != nil {
		log.Fatalln("creating archive failed:", err)
	}

	archive, err := os.Open(tmpfile.Name())
	if err != nil {
		log.Fatalln("could not open archive:", err)
	}

	info, err := archive.Stat()
	if err != nil {
		log.Fatalln("could not stat archive:", err)
	}

	progress := pb.New64(info.Size())
	progress.SetUnits(pb.U_BYTES)

	progress.Start()
	defer progress.Finish()

	uploadBits, err := endpoint.RequestForHandler(
		routes.UploadBits,
		router.Params{"guid": build.Guid},
		progress.NewProxyReader(archive),
	)
	if err != nil {
		log.Fatalln(err)
	}

	response, err := http.DefaultClient.Do(uploadBits)
	if err != nil {
		log.Fatalln("request failed:", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		log.Println("bad response when uploading bits:", response)
		response.Write(os.Stderr)
		os.Exit(1)
	}
}

func poll(endpoint *EndpointRoutes, build Build) int {
	for {
		var result BuildResult

		getResult, err := endpoint.RequestForHandler(
			routes.GetResult,
			router.Params{"guid": build.Guid},
			nil,
		)
		if err != nil {
			log.Fatalln(err)
		}

		response, err := http.DefaultClient.Do(getResult)
		if err != nil {
			log.Fatalln("error polling for result:", err)
		}

		err = json.NewDecoder(response.Body).Decode(&result)
		if err != nil {
			response.Body.Close()
			log.Fatalln("error decoding result:", err)
		}

		response.Body.Close()

		var color string
		var exitCode int

		switch result.Status {
		case "succeeded":
			color = "green"
			exitCode = 0
		case "failed":
			color = "red"
			exitCode = 1
		case "errored":
			color = "magenta"
			exitCode = 2
		default:
			time.Sleep(time.Second)
			continue
		}

		fmt.Println(ansi.Color(result.Status, color))
		return exitCode
	}
}
