package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/concourse/atc"
)

func main() {
	path := os.Args[1]
	file, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("could not read file: ", err.Error())
		os.Exit(1)
	}

	fmt.Println("checking " + path + "...")

	var config atc.TaskConfig

	if err := yaml.Unmarshal(file, &config); err != nil {
		fmt.Println("could not unmarshal file: ", err.Error())
		os.Exit(1)
	}

	if err := config.Validate(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
