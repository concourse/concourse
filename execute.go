package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/concourse/glider/api/builds"
	"github.com/concourse/glider/routes"
	TurbineBuilds "github.com/concourse/turbine/api/builds"
	"github.com/fraenkel/candiedyaml"
	"github.com/gorilla/websocket"
	"github.com/mgutz/ansi"
	"github.com/pivotal-golang/archiver/compressor"
	"github.com/tedsuo/rata"
)

func execute(reqGenerator *rata.RequestGenerator) {
	absConfig, err := filepath.Abs(*buildConfig)
	if err != nil {
		log.Fatalln("could not locate config file:", err)
	}

	build := create(reqGenerator, loadConfig(absConfig), filepath.Base(filepath.Dir(absConfig)))

	logOutput, err := reqGenerator.CreateRequest(
		routes.LogOutput,
		rata.Params{"guid": build.Guid},
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

func loadConfig(configPath string) TurbineBuilds.Config {
	passArgs := false
	args := []string{}
	for _, arg := range os.Args {
		if arg == "--" {
			passArgs = true
			continue
		}

		if passArgs {
			args = append(args, arg)
		}
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		log.Fatalln("could not open config file:", err)
	}

	var config TurbineBuilds.Config

	err = candiedyaml.NewDecoder(configFile).Decode(&config)
	if err != nil {
		log.Fatalln("could not parse config file:", err)
	}

	config.Run.Args = append(config.Run.Args, args...)

	return config
}

func create(reqGenerator *rata.RequestGenerator, config TurbineBuilds.Config, name string) builds.Build {
	buffer := &bytes.Buffer{}

	build := builds.Build{
		Name:   name,
		Config: config,
	}

	err := json.NewEncoder(buffer).Encode(build)
	if err != nil {
		log.Fatalln("encoding build failed:", err)
	}

	createBuild, err := reqGenerator.CreateRequest(routes.CreateBuild, nil, buffer)
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

func upload(reqGenerator *rata.RequestGenerator, build builds.Build) {
	src, err := filepath.Abs(*buildDir)
	if err != nil {
		log.Fatalln("could not locate build config:", err)
	}

	var archive io.ReadCloser

	tarPath, err := exec.LookPath("tar")
	if err != nil {
		compressor := compressor.NewTgz()

		tmpfile, err := ioutil.TempFile("", "fly")
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

	uploadBits, err := reqGenerator.CreateRequest(
		routes.UploadBits,
		rata.Params{"guid": build.Guid},
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

func poll(reqGenerator *rata.RequestGenerator, build builds.Build) int {
	for {
		var result builds.BuildResult

		getResult, err := reqGenerator.CreateRequest(
			routes.GetResult,
			rata.Params{"guid": build.Guid},
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
