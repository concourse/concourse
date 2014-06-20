package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/fraenkel/candiedyaml"
	"github.com/gorilla/websocket"
	"github.com/mgutz/ansi"
	"github.com/pivotal-golang/archiver/compressor"
	"github.com/tedsuo/router"

	"github.com/winston-ci/redgreen/api/builds"
	"github.com/winston-ci/redgreen/routes"
)

type BuildConfig struct {
	Image  string              `yaml:"image"`
	Script string              `yaml:"script"`
	Env    []map[string]string `yaml:"env"`
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
		println("export REDGREEN_URL=http://10.244.8.2:5637")
		os.Exit(1)
	}

	reqGenerator := router.NewRequestGenerator(*redgreenURL, routes.Routes)

	absConfig, err := filepath.Abs(*buildConfig)
	if err != nil {
		log.Fatalln("could not locate config file:", err)
	}

	build := create(reqGenerator, loadConfig(absConfig), filepath.Base(filepath.Dir(absConfig)))

	logOutput, err := reqGenerator.RequestForHandler(
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

	upload(reqGenerator, build)

	exitCode := poll(reqGenerator, build)

	res.Body.Close()
	conn.Close()

	streaming.Wait()

	os.Exit(exitCode)
}

type ConfigContext struct {
	Args string
}

func loadConfig(configPath string) BuildConfig {
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

	template, err := template.ParseFiles(configPath)
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

func create(reqGenerator *router.RequestGenerator, config BuildConfig, path string) builds.Build {
	buffer := &bytes.Buffer{}

	build := builds.Build{
		Image:  config.Image,
		Path:   path,
		Script: config.Script,
		Env:    config.Env,
	}

	err := json.NewEncoder(buffer).Encode(build)
	if err != nil {
		log.Fatalln("encoding build failed:", err)
	}

	createBuild, err := reqGenerator.RequestForHandler(routes.CreateBuild, nil, buffer)
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

func upload(reqGenerator *router.RequestGenerator, build builds.Build) {
	src, err := filepath.Abs(*buildDir)
	if err != nil {
		log.Fatalln("could not locate build config:", err)
	}

	var archive io.ReadCloser

	tarPath, err := exec.LookPath("tar")
	if err != nil {
		compressor := compressor.NewTgz()

		tmpfile, err := ioutil.TempFile("", "smith")
		if err != nil {
			log.Fatalln("creating tempfile failed:", err)
		}

		tmpfile.Close()

		defer os.Remove(tmpfile.Name())

		err = compressor.Compress(src+"/", tmpfile.Name())
		if err != nil {
			log.Fatalln("creating archive failed:", err)
		}

		archive, err = os.Open(tmpfile.Name())
		if err != nil {
			log.Fatalln("could not open archive:", err)
		}
	} else {
		tarCmd := exec.Command(tarPath, "--exclude", ".git", "-czf", "-", ".")
		tarCmd.Dir = src
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

	uploadBits, err := reqGenerator.RequestForHandler(
		routes.UploadBits,
		router.Params{"guid": build.Guid},
		archive,
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

func poll(reqGenerator *router.RequestGenerator, build builds.Build) int {
	for {
		var result builds.BuildResult

		getResult, err := reqGenerator.RequestForHandler(
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
