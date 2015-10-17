package main

import (
	"fmt"
	_ "net/http/pprof"
	"os"

	_ "github.com/codahale/metrics/runtime"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

// var resourceTypes = flag.String(
// 	"resourceTypes",
// 	`[
// 		{"type": "archive", "image": "docker:///concourse/archive-resource" },
// 		{"type": "docker-image", "image": "docker:///concourse/docker-image-resource" },
// 		{"type": "git", "image": "docker:///concourse/git-resource" },
// 		{"type": "github-release", "image": "docker:///concourse/github-release-resource" },
// 		{"type": "s3", "image": "docker:///concourse/s3-resource" },
// 		{"type": "semver", "image": "docker:///concourse/semver-resource" },
// 		{"type": "time", "image": "docker:///concourse/time-resource" },
// 		{"type": "tracker", "image": "docker:///concourse/tracker-resource" },
// 		{"type": "pool", "image": "docker:///concourse/pool-resource" }
// 	]`,
// 	"map of resource type to its rootfs",
// )

func main() {
	cmd := &ATCCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	running := ifrit.Invoke(cmd)

	err = <-running.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
