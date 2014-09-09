package main

import (
	"flag"
	"os"

	"github.com/tedsuo/rata"

	"github.com/concourse/atc/api/routes"
)

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

var atcURL = flag.String(
	"atcURL",
	"http://127.0.0.1:8080",
	"address of the ATC to use",
)

var atc string

func main() {
	flag.Parse()

	envATC := os.Getenv("ATC_URL")
	if envATC != "" {
		atc = envATC
	} else {
		atc = *atcURL
	}

	reqGenerator := rata.NewRequestGenerator(atc, routes.Routes)

	if len(os.Args) == 1 {
		execute(reqGenerator)
		return
	}

	switch os.Args[1] {
	case "--":
		execute(reqGenerator)

	case "hijack":
		hijack(reqGenerator)

	default:
		println("unknown command: " + flag.Arg(0))
	}
}
