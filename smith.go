package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pivotal-golang/archiver/compressor"
)

type Source struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
	Ref  string `json:"ref"`
}

type Build struct {
	Guid     string `json:"guid"`
	Image    string `json:"image"`
	Script   string `json:"script"`
	Callback string `json:"callback"`
	Source   Source `json:"source"`
}

func main() {
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

	fmt.Printf("Created archive of current directory: %s", dest.Name())

	http.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, dest.Name())
	})

	http.HandleFunc("/die", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		os.Exit(0)
	})

	// RAAAAACE CONDITION
	go http.ListenAndServe(":8080", nil)

	build := Build{
		Guid:     "abc",
		Image:    "ubuntu",
		Script:   "ls -l",
		Callback: "http://localhost:8080/die",
		Source: Source{
			Type: "raw",
			URI:  "http://localhost:8080/file",
			Ref:  "",
		},
	}

	client := &http.Client{}
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.Encode(build)

	req, err := http.NewRequest("POST", "http://localhost:4637/builds", buffer)
	if err != nil {
		fmt.Println("Couldn't create request... %s", err)
		os.Exit(1)
	}
	req.Header.Add("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		fmt.Println("Request failed... %s", err)
		os.Exit(1)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Body failed... %s", err)
		os.Exit(1)
	}
	fmt.Println("%s", string(body))

	select {}
}
