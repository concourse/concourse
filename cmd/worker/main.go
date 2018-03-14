package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/flag"
	"github.com/concourse/worker"
	"github.com/concourse/worker/beacon"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

// overridden via linker flags
var Version = "0.0.0-dev"

type WorkerCommand struct {
	GardenAddr      string            `long:"garden-addr"`
	BaggageclaimURL string            `long:"baggageclaim-url"`
	Resource        []beacon.FileFlag `long:"resource"`
	Platform        string            `long:"platform"`
	Tags            []string          `long:"tag"`
	Team            string            `long:"team"`
	Name            string            `long:"name"`
	StartTime       int64             `long:"start_time"`
	Version         string            `long:"version"`
	CertsPath       *string           `long:"certs_path"`
	HTTPProxyURL    string            `long:"http_proxy_url"`
	HTTPSProxyURL   string            `long:"https_proxy_url"`
	NoProxy         string            `long:"no_proxy"`
	Logger          flag.Lager
	BeaconConfig    beacon.Config `group:"TSA Beacon Configuration" namespace:"tsa"`
}

func main() {
	var cmd WorkerCommand

	parser := flags.NewParser(&cmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var resourceTypes []atc.WorkerResourceType
	for _, filePath := range cmd.Resource {

		resourceJSON, err := ioutil.ReadFile(string(filePath))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		var workerResourceType atc.WorkerResourceType
		err = json.Unmarshal(resourceJSON, &workerResourceType)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		resourceTypes = append(resourceTypes, workerResourceType)
	}

	var atcWorker = atc.Worker{
		GardenAddr:      cmd.GardenAddr,
		BaggageclaimURL: cmd.BaggageclaimURL,
		ResourceTypes:   resourceTypes,
		Platform:        cmd.Platform,
		Tags:            cmd.Tags,
		Team:            cmd.Team,
		Name:            cmd.Name,
		StartTime:       cmd.StartTime,
		Version:         cmd.Version,
		CertsPath:       cmd.CertsPath,
		HTTPProxyURL:    cmd.HTTPProxyURL,
		HTTPSProxyURL:   cmd.HTTPSProxyURL,
		NoProxy:         cmd.NoProxy,
	}

	logger, _ := cmd.Logger.Logger("beacon")
	runner := worker.BeaconRunner(logger, atcWorker, cmd.BeaconConfig)

	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}
