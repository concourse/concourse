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
	"github.com/concourse/worker/reaper"
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
	NoProxy    string `long:"no-proxy"    env:"no_proxy"    env-delim:","   description:"Blacklist of addresses to skip the proxy when reaching."`

	DebugBindPort uint16 `long:"bind-debug-port" default:"9099"    description:"Port on which to listen for beacon pprof server."`

	Version string `long:"version" hidden:"true" description:"Version of the worker. This is normally baked in to the binary, so this flag is hidden."`
}

// type ReaperConfig struct {
// 	Port string
// }

type StartCommand struct {
	WorkerConfig Config
	//	ReaperConfig ReaperConfig //{Port:"7799"} //`group:"Reaper Configuration"`

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

	// reaperURL := strings.Split(cmd.GardenAddr, ":")
	// if len(reaperURL) != 2 {
	// 	return fmt.Errorf("failed to parse GardenAddr: %s", cmd.GardenAddr)
	// }
	var atcWorker = atc.Worker{
		GardenAddr:      cmd.GardenAddr,
		BaggageclaimURL: cmd.BaggageclaimURL,
		ResourceTypes:   resourceTypes,
		//		ReaperAddr:      "http://" + reaperURL[0] + ":" + cmd.ReaperConfig.Port,
		Platform:      cmd.Platform,
		Tags:          cmd.WorkerConfig.Tags,
		Team:          cmd.WorkerConfig.TeamName,
		Name:          cmd.WorkerConfig.Name,
		StartTime:     time.Now().Unix(),
		Version:       cmd.WorkerConfig.Version,
		CertsPath:     cmd.CertsPath,
		HTTPProxyURL:  cmd.WorkerConfig.HTTPProxy,
		HTTPSProxyURL: cmd.WorkerConfig.HTTPSProxy,
		NoProxy:       cmd.WorkerConfig.NoProxy,
	}

	groupLogger, _ := cmd.Logger.Logger("worker")

	var gardenAddr string
	if cmd.TSA.GardenForwardAddr != "" {
		gardenAddr = cmd.TSA.GardenForwardAddr
	} else {
		gardenAddr = cmd.GardenAddr
	}

	groupMembers := grouper.Members{
		{Name: "beacon", Runner: worker.BeaconRunner(
			groupLogger,
			atcWorker,
			cmd.TSA,
		)},
		{Name: "reaper", Runner: reaper.NewReaperRunner(
			groupLogger,
			gardenAddr,
			beacon.ReaperPort,
			//cmd.ReaperConfig.Port,
		)},
		{Name: "debug-server", Runner: http_server.New(
			cmd.debugBindAddr(),
			http.DefaultServeMux,
		)},
		{Name: "sweeper-containers", Runner: sweeper.NewSweeperRunner(
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
