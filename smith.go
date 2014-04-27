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
	"time"

	"github.com/mgutz/ansi"
	"github.com/pivotal-golang/archiver/compressor"
)

type Build struct {
	Guid   string `json:"guid"`
	Image  string `json:"image"`
	Script string `json:"script"`
}

type BuildResult struct {
	Status string `json:"status"`
}

var redgreenURL = flag.String(
	"redgreenAddr",
	"http://127.0.0.1:5637",
	"address denoting the redgreen service",
)

func main() {
	flag.Parse()

	compressor := compressor.NewTgz()

	src, err := os.Getwd()
	if err != nil {
		fmt.Println("Couldn't get current directory...")
		os.Exit(1)
	}

	dest, err := ioutil.TempFile("", "smith")
	if err != nil {
		fmt.Println("Couldn't create temporary file...")
		os.Exit(1)
	}
	dest.Close()

	err = compressor.Compress(src, dest.Name())
	if err != nil {
		fmt.Printf("Couldn't create archive: %s\n", err.Error())
		os.Exit(1)
	}

	build := Build{
		Image:  "mischief/docker-golang",
		Script: "find .",
	}

	buffer := &bytes.Buffer{}

	err = json.NewEncoder(buffer).Encode(build)
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
		log.Fatalln("bad response:", response)
	}

	err = json.NewDecoder(response.Body).Decode(&build)
	if err != nil {
		log.Fatalln("response decoding failed:", err)
	}

	archive, err := os.Open(dest.Name())
	if err != nil {
		log.Fatalln("could not open archive")
	}

	response, err = http.Post(
		*redgreenURL+"/builds/"+build.Guid+"/bits",
		"application/octet-stream",
		archive,
	)
	if err != nil {
		log.Fatalln("request failed:", err)
	}

	if response.StatusCode != http.StatusCreated {
		log.Fatalln("bad response:", response)
	}

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

		if result.Status != "" {
			exitCode := 1
			if result.Status == "succeeded" {
				exitCode = 0
			}

			if exitCode == 0 {
				fmt.Println(ansi.Color(result.Status, "green"))
			} else {
				fmt.Println(ansi.Color(result.Status, "red"))
			}

			os.Exit(exitCode)
		}

		time.Sleep(time.Second)
	}
}
