package start

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/flag"
	"github.com/concourse/concourse/worker"
	"github.com/concourse/concourse/worker/beacon"
	"github.com/concourse/concourse/worker/sweeper"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
)

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

	atcWorker := cmd.WorkerConfig.Worker()
	atcWorker.Platform = cmd.Platform
	atcWorker.GardenAddr = cmd.GardenAddr
	atcWorker.BaggageclaimURL = cmd.BaggageclaimURL
	atcWorker.ResourceTypes = resourceTypes
	atcWorker.CertsPath = cmd.CertsPath

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
