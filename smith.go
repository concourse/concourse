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
	"time"

	"github.com/fraenkel/candiedyaml"
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

var redgreenURL = flag.String(
	"redgreenURL",
	"http://127.0.0.1:5637",
	"address denoting the redgreen service",
)

func main() {
	flag.Parse()

	build := create(loadConfig())

	upload(build)

	poll(build)
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
		*redgreenURL+"/builds",
		"application/json",
		buffer,
	)
	if err != nil {
		log.Fatalln("request failed:", err)
	}

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
		*redgreenURL+"/builds/"+build.Guid+"/bits",
		"application/octet-stream",
		archive,
	)
	if err != nil {
		log.Fatalln("request failed:", err)
	}

	if response.StatusCode != http.StatusCreated {
		log.Println("bad response when uploading bits:", response)
		response.Write(os.Stderr)
		os.Exit(1)
	}
}

func poll(build Build) {
	for {
		var result BuildResult

		response, err := http.Get(*redgreenURL + "/builds/" + build.Guid + "/result")
		if err != nil {
			log.Fatalln("error polling for result:", err)
		}

		err = json.NewDecoder(response.Body).Decode(&result)
		if err != nil {
			log.Fatalln("error decoding result:", err)
		}

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
		os.Exit(exitCode)
	}
}
