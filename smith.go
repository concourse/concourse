package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fraenkel/candiedyaml"
	"github.com/gorilla/websocket"
	"github.com/mgutz/ansi"
	"github.com/pivotal-golang/archiver/compressor"
)

type Build struct {
	Guid   string `json:"guid,omitempty"`
	Image  string `json:"image"`
	Path   string `json:"path"`
	Script string `json:"script"`
}

type BuildResult struct {
	Status string `json:"status"`
}

type BuildConfig struct {
	Image  string `yaml:"image"`
	Path   string `yaml:"path"`
	Script string `yaml:"script"`
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

var redgreenAddr = flag.String(
	"redgreenAddr",
	"127.0.0.1:5637",
	"address denoting the redgreen service",
)

func main() {
	flag.Parse()

	build := create(loadConfig())

	buildLog := fmt.Sprintf("ws://%s/builds/%s/log/output", *redgreenAddr, build.Guid)

	conn, res, err := websocket.DefaultDialer.Dial(buildLog, nil)
	if err != nil {
		log.Println("failed to stream output:", err, res)
		return
	}

	streaming := new(sync.WaitGroup)
	streaming.Add(1)

	go stream(conn, streaming)

	upload(build)

	exitCode := poll(build)

	res.Body.Close()
	conn.Close()

	streaming.Wait()

	os.Exit(exitCode)
}

func loadConfig() BuildConfig {
	configFile, err := os.Open(*buildConfig)
	if err != nil {
		log.Fatalln("could not open config file:", err)
	}

	var config BuildConfig

	err = candiedyaml.NewDecoder(configFile).Decode(&config)
	if err != nil {
		log.Fatalln("could not parse config file:", err)
	}

	return config
}

func create(config BuildConfig) Build {
	buffer := &bytes.Buffer{}

	build := Build{
		Image:  config.Image,
		Path:   config.Path,
		Script: config.Script,
	}

	if build.Path == "" {
		build.Path = "."
	}

	err := json.NewEncoder(buffer).Encode(build)
	if err != nil {
		log.Fatalln("encoding build failed:", err)
	}

	response, err := http.Post(
		"http://"+*redgreenAddr+"/builds",
		"application/json",
		buffer,
	)
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

func upload(build Build) {
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

	response, err := http.Post(
		"http://"+*redgreenAddr+"/builds/"+build.Guid+"/bits",
		"application/octet-stream",
		archive,
	)
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

func poll(build Build) int {
	for {
		var result BuildResult

		response, err := http.Get("http://" + *redgreenAddr + "/builds/" + build.Guid + "/result")
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
