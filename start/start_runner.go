package start

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/flag"
	"github.com/concourse/worker"
	"github.com/concourse/worker/beacon"
	"github.com/tedsuo/ifrit"
)

type Config struct {
	HTTPProxy  string        `long:"http-proxy-url"`
	HTTPSProxy string        `long:"https-proxy-url"`
	NoProxy    string        `long:"no-proxy"`
	Tags       []string      `long:"tag"`
	TeamName   string        `long:"team-name"`
	Name       string        `long:"name"`
	TSA        beacon.Config `group:"TSA Beacon Configuration"`
	StartTime  int64         `long:"start-time"`
	Version    string        `long:"version"`
}

type StartCommand struct {
	WorkerConfig    Config
	GardenAddr      string            `long:"garden-addr"`
	BaggageclaimURL string            `long:"baggageclaim-url"`
	Resource        []beacon.FileFlag `long:"resource"`
	Platform        string            `long:"platform"`
	CertsPath       *string           `long:"certs-path"`
	Logger          flag.Lager
}

func (cmd *StartCommand) Execute(args []string) error {
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
		Tags:            cmd.WorkerConfig.Tags,
		Team:            cmd.WorkerConfig.TeamName,
		Name:            cmd.WorkerConfig.Name,
		StartTime:       cmd.WorkerConfig.StartTime,
		Version:         cmd.WorkerConfig.Version,
		CertsPath:       cmd.CertsPath,
		HTTPProxyURL:    cmd.WorkerConfig.HTTPProxy,
		HTTPSProxyURL:   cmd.WorkerConfig.HTTPSProxy,
		NoProxy:         cmd.WorkerConfig.NoProxy,
	}

	logger, _ := cmd.Logger.Logger("beacon")
	runner := worker.BeaconRunner(logger, atcWorker, cmd.WorkerConfig.TSA)

	err := <-ifrit.Invoke(runner).Wait()
	if err != nil {
		logger.Error("beacon-start-command-failed", err)
		return errors.New("beacon-start-command-failed" + err.Error())
	}
	return nil
}
