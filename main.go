package main

import (
	"flag"

	"os"

	"github.com/tedsuo/rata"

	"github.com/concourse/glider/routes"
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

var gliderURL = flag.String(
	"gliderURL",
	"http://127.0.0.1:5637",
	"address denoting the glider service (can also set $GLIDER_URL)",
)

var glider string

func main() {
	flag.Parse()

	envGlider := os.Getenv("GLIDER_URL")
	if envGlider != "" {
		glider = envGlider
	} else {
		glider = *gliderURL
	}

	reqGenerator := rata.NewRequestGenerator(glider, routes.Routes)

	switch flag.Arg(0) {
	case "", "--":
		execute(reqGenerator)

	case "hijack":
		hijack(reqGenerator)
	}
}
