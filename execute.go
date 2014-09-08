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
	"strconv"
	"syscall"

	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/pipes"
	"github.com/concourse/atc/builds"
	"github.com/concourse/fly/eventstream"
	tbuilds "github.com/concourse/turbine/api/builds"
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

	pipe := createPipe(reqGenerator)

	build, cookies := createBuild(
		reqGenerator,
		pipe,
		loadConfig(absConfig),
		filepath.Base(filepath.Dir(absConfig)),
	)

	terminate := make(chan os.Signal, 1)

	go abortOnSignal(reqGenerator, terminate, build)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

	logOutput, err := reqGenerator.CreateRequest(
		api.BuildEvents,
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

	conn, res, err := websocket.DefaultDialer.Dial(
		logOutput.URL.String(),
		http.Header{"Cookie": cookieHeaders},
	)
	if err != nil {
		log.Println("failed to stream output:", err, res)
		os.Exit(1)
	}

	go upload(reqGenerator, pipe)

	exitCode, err := eventstream.RenderStream(conn)
	if err != nil {
		log.Println("failed to render stream:", err)
		os.Exit(1)
	}

	res.Body.Close()
	conn.Close()

	os.Exit(exitCode)
}

func createPipe(reqGenerator *rata.RequestGenerator) pipes.Pipe {
	cPipe, err := reqGenerator.CreateRequest(api.CreatePipe, nil, nil)
	if err != nil {
		log.Fatalln(err)
	}

	response, err := http.DefaultClient.Do(cPipe)
	if err != nil {
		log.Fatalln("request failed:", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		log.Println("bad response when creating pipe:", response)
		response.Write(os.Stderr)
		os.Exit(1)
	}

	var pipe pipes.Pipe
	err = json.NewDecoder(response.Body).Decode(&pipe)
	if err != nil {
		log.Println("malformed response when creating pipe:", err)
		os.Exit(1)
	}

	return pipe
}

func loadConfig(configPath string) tbuilds.Config {
	configFile, err := os.Open(configPath)
	if err != nil {
		log.Fatalln("could not open config file:", err)
	}

	var config tbuilds.Config

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

func createBuild(
	reqGenerator *rata.RequestGenerator,
	pipe pipes.Pipe,
	config tbuilds.Config,
	name string,
) (builds.Build, []*http.Cookie) {
	readPipe, err := reqGenerator.CreateRequest(
		api.ReadPipe,
		rata.Params{"pipe_id": pipe.ID},
		nil,
	)
	if err != nil {
		log.Fatalln(err)
	}

	readPipe.URL.Host = pipe.PeerAddr

	buffer := &bytes.Buffer{}

	turbineBuild := tbuilds.Build{
		Config: config,
		Inputs: []tbuilds.Input{
			{
				Name: name,
				Type: "archive",
				Source: tbuilds.Source{
					"uri": readPipe.URL.String(),
				},
			},
		},
	}

	err = json.NewEncoder(buffer).Encode(turbineBuild)
	if err != nil {
		log.Fatalln("encoding build failed:", err)
	}

	createBuild, err := reqGenerator.CreateRequest(api.CreateBuild, nil, buffer)
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

	var build builds.Build
	err = json.NewDecoder(response.Body).Decode(&build)
	if err != nil {
		log.Fatalln("response decoding failed:", err)
	}

	return build, response.Cookies()
}

func abortOnSignal(
	reqGenerator *rata.RequestGenerator,
	terminate <-chan os.Signal,
	build builds.Build,
) {
	<-terminate

	println("\naborting...")

	abortReq, err := reqGenerator.CreateRequest(
		api.AbortBuild,
		rata.Params{"build_id": strconv.Itoa(build.ID)},
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

func upload(reqGenerator *rata.RequestGenerator, pipe pipes.Pipe) {
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
		api.WritePipe,
		rata.Params{"pipe_id": pipe.ID},
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

	if response.StatusCode != http.StatusOK {
		log.Println("bad response when uploading bits:", response)
		response.Write(os.Stderr)
		os.Exit(1)
	}
}
