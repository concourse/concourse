package start

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/flag"
	"github.com/concourse/worker"
	"github.com/concourse/worker/beacon"
	"github.com/concourse/worker/sweeper"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
)

type Config struct {
	Name     string   `long:"name"  description:"The name to set for the worker during registration. If not specified, the hostname will be used."`
	Tags     []string `long:"tag"   description:"A tag to set during registration. Can be specified multiple times."`
	TeamName string   `long:"team"  description:"The name of the team that this worker will be assigned to."`

	HTTPProxy  string `long:"http-proxy"  env:"http_proxy"                  description:"HTTP proxy endpoint to use for containers."`
	HTTPSProxy string `long:"https-proxy" env:"https_proxy"                 description:"HTTPS proxy endpoint to use for containers."`
	NoProxy    string `long:"no-proxy"    env:"no_proxy"                    description:"Blacklist of addresses to skip the proxy when reaching."`

	Ephemeral bool `long:"ephemeral" description:"If set, the worker will be immediately removed upon stalling."`

	DebugBindPort uint16 `long:"bind-debug-port" default:"9099"    description:"Port on which to listen for beacon pprof server."`

	Version string `long:"version" hidden:"true" description:"Version of the worker. This is normally baked in to the binary, so this flag is hidden."`
}

type StartCommand struct {
	WorkerConfig Config

	TSA beacon.Config `group:"TSA Beacon Configuration"`

	GardenAddr      string            `long:"garden-addr"`
	BaggageclaimURL string            `long:"baggageclaim-url"`
	Resource        []beacon.FileFlag `long:"resource"`
	Platform        string            `long:"platform"`
	CertsPath       *string           `long:"certs-path"`
	Logger          flag.Lager
}

func (cmd *StartCommand) debugBindAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", cmd.WorkerConfig.DebugBindPort)
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
		StartTime:       time.Now().Unix(),
		Version:         cmd.WorkerConfig.Version,
		CertsPath:       cmd.CertsPath,
		HTTPProxyURL:    cmd.WorkerConfig.HTTPProxy,
		HTTPSProxyURL:   cmd.WorkerConfig.HTTPSProxy,
		NoProxy:         cmd.WorkerConfig.NoProxy,
		Ephemeral:       cmd.WorkerConfig.Ephemeral,
	}

	groupLogger, _ := cmd.Logger.Logger("worker")
	groupMembers := grouper.Members{
		{Name: "beacon", Runner: worker.BeaconRunner(
			groupLogger,
			atcWorker,
			cmd.TSA,
		)},
		{Name: "debug-server", Runner: http_server.New(
			cmd.debugBindAddr(),
			http.DefaultServeMux,
		)},
		{Name: "sweeper", Runner: sweeper.NewSweeperRunner(
			groupLogger,
			atcWorker,
			cmd.TSA,
		)},
	}

	parallelRunner := grouper.NewParallel(os.Interrupt, groupMembers)

	err := <-ifrit.Invoke(parallelRunner).Wait()
	if err != nil {
		groupLogger.Error("beacon-and-reaper-start-command-failed", err)
		return errors.New("beacon-and-reaper-start-command-failed" + err.Error())
	}

	return nil
}
