package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/concourse/fly/eventstream"
	"github.com/concourse/glider/api/builds"
	"github.com/concourse/glider/routes"
	TurbineBuilds "github.com/concourse/turbine/api/builds"
	"github.com/fraenkel/candiedyaml"
	"github.com/gorilla/websocket"
	"github.com/pivotal-golang/archiver/compressor"
	"github.com/tedsuo/rata"
)

func execute(reqGenerator *rata.RequestGenerator) {
	absConfig, err := filepath.Abs(*buildConfig)
	if err != nil {
		log.Println("could not locate config file:", err)
		os.Exit(1)
	}

	build := create(reqGenerator, loadConfig(absConfig), filepath.Base(filepath.Dir(absConfig)))

	terminate := make(chan os.Signal, 1)

	go abortOnSignal(reqGenerator, terminate, build)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

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
		os.Exit(1)
	}

	go upload(reqGenerator, build)

	exitCode, err := eventstream.RenderStream(conn)
	if err != nil {
		log.Println("failed to render stream:", err)
		os.Exit(1)
	}

	res.Body.Close()
	conn.Close()

	os.Exit(exitCode)
}

func loadConfig(configPath string) TurbineBuilds.Config {
	configFile, err := os.Open(configPath)
	if err != nil {
		log.Fatalln("could not open config file:", err)
	}

	var config TurbineBuilds.Config

	err = candiedyaml.NewDecoder(configFile).Decode(&config)
	if err != nil {
		log.Fatalln("could not parse config file:", err)
	}

	config.Run.Args = append(config.Run.Args, flag.Args()...)

	for k, _ := range config.Params {
		env, found := syscall.Getenv(k)
		if found {
			config.Params[k] = env
		}
	}

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

func abortOnSignal(
	reqGenerator *rata.RequestGenerator,
	terminate <-chan os.Signal,
	build builds.Build,
) {
	<-terminate

	println("\naborting...")

	abortReq, err := reqGenerator.CreateRequest(
		routes.AbortBuild,
		rata.Params{"guid": build.Guid},
		nil,
	)

	if err != nil {
		log.Fatalln(err)
	}

	resp, err := http.DefaultClient.Do(abortReq)
	if err != nil {
		log.Println("failed to abort:", err)
	}

	resp.Body.Close()

	// if told to terminate again, exit immediately
	<-terminate
	println("exiting immediately")
	os.Exit(2)
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
