package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/concourse"
	"github.com/concourse/concourse/worker"
	"github.com/concourse/flag"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

type WorkerCommand struct {
	Worker WorkerConfig

	TSA worker.TSAConfig `group:"TSA Configuration" namespace:"tsa"`

	Certs Certs

	WorkDir flag.Dir `long:"work-dir" required:"true" description:"Directory in which to place container data."`

	BindIP   flag.IP `long:"bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the Garden server."`
	BindPort uint16  `long:"bind-port" default:"7777"      description:"Port on which to listen for the Garden server."`

	HealthcheckIP      flag.IP       `long:"healthcheck-ip"      default:"127.0.0.1" description:"IP address on which to listen for health checking requests"`
	HealthcheckPort    uint16        `long:"healthcheck-port"    default:"8888"      description:"Port on which to listen for health checking requests"`
	HealthCheckTimeout time.Duration `long:"healthcheck-timeout" default:"5s"        description:"HTTP timeout for the full duration of health checking"`

	SweepInterval time.Duration `long:"sweep-interval" default:"30s" description:"Interval on which containers and volumes will be garbage collected from the worker."`

	RebalanceInterval time.Duration `long:"rebalance-interval" description:"Duration after which the registration should be swapped to another random SSH gateway."`

	DrainTimeout time.Duration `long:"drain-timeout" default:"1h" description:"Duration after which a worker should give up draining forwarded connections on shutdown."`

	Garden GardenBackend `group:"Garden Configuration" namespace:"garden"`

	Baggageclaim baggageclaimcmd.BaggageclaimCommand `group:"Baggageclaim Configuration" namespace:"baggageclaim"`

	ResourceTypes flag.Dir `long:"resource-types" description:"Path to directory containing resource types the worker should advertise."`

	Logger flag.Lager
}

func (cmd *WorkerCommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *WorkerCommand) Runner(args []string) (ifrit.Runner, error) {
	if cmd.ResourceTypes == "" {
		cmd.ResourceTypes = flag.Dir(discoverAsset("resource-types"))
	}

	logger, _ := cmd.Logger.Logger("worker")

	atcWorker, gardenRunner, err := cmd.gardenRunner(logger.Session("garden"))
	if err != nil {
		return nil, err
	}

	atcWorker.Version = concourse.WorkerVersion

	baggageclaimRunner, err := cmd.baggageclaimRunner(logger.Session("baggageclaim"))
	if err != nil {
		return nil, err
	}

	healthChecker := worker.NewHealthChecker(
		logger.Session("healthchecker"),
		cmd.baggageclaimURL(),
		cmd.gardenURL(),
		cmd.HealthCheckTimeout,
	)

	members := grouper.Members{
		{
			Name:   "garden",
			Runner: NewLoggingRunner(logger.Session("garden-runner"), gardenRunner),
		},
		{
			Name:   "baggageclaim",
			Runner: NewLoggingRunner(logger.Session("baggageclaim-runner"), baggageclaimRunner),
		},
		{
			Name: "healthcheck",
			Runner: NewLoggingRunner(
				logger.Session("healthcheck-runner"),
				http_server.New(
					fmt.Sprintf("%s:%d", cmd.HealthcheckIP.IP, cmd.HealthcheckPort),
					http.HandlerFunc(healthChecker.CheckHealth)),
			),
		},
	}

	if cmd.TSA.WorkerPrivateKey != nil {
		tsaClient := cmd.TSA.Client(atcWorker)

		beacon := &worker.Beacon{
			Logger: logger.Session("beacon"),

			Client: tsaClient,

			RebalanceInterval: cmd.RebalanceInterval,
			DrainTimeout:      cmd.DrainTimeout,

			LocalGardenNetwork: "tcp",
			LocalGardenAddr:    cmd.gardenAddr(),

			LocalBaggageclaimNetwork: "tcp",
			LocalBaggageclaimAddr:    cmd.baggageclaimAddr(),
		}

		members = append(members, grouper.Member{
			Name: "beacon",
			Runner: NewLoggingRunner(
				logger.Session("beacon-runner"),
				worker.NewBeaconRunner(
					logger.Session("beacon-runner"),
					beacon,
					tsaClient,
				),
			),
		})

		gardenClient := gclient.New(
			gconn.NewWithLogger(
				"tcp",
				cmd.gardenAddr(),
				logger.Session("garden-connection"),
			),
		)

		baggageclaimClient := bclient.NewWithHTTPClient(
			cmd.baggageclaimURL(),

			// ensure we don't use baggageclaim's default retryhttp client; all
			// traffic should be local, so any failures are unlikely to be transient.
			// we don't want a retry loop to block up sweeping and prevent the worker
			// from existing.
			&http.Client{
				Transport: &http.Transport{
					// don't let a slow (possibly stuck) baggageclaim server slow down
					// sweeping too much
					ResponseHeaderTimeout: 1 * time.Minute,
				},
			},
		)

		members = append(members, grouper.Member{
			Name: "sweeper",
			Runner: NewLoggingRunner(
				logger.Session("sweeper"),
				&worker.SweepRunner{
					Logger:             logger.Session("sweeper-runner"),
					Interval:           cmd.SweepInterval,
					TSAClient:          tsaClient,
					GardenClient:       gardenClient,
					BaggageclaimClient: baggageclaimClient,
				},
			),
		})

	}

	return grouper.NewParallel(os.Interrupt, members), nil
}

func (cmd *WorkerCommand) gardenAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)
}

func (cmd *WorkerCommand) gardenURL() string {
	return fmt.Sprintf("http://%s", cmd.gardenAddr())
}

func (cmd *WorkerCommand) baggageclaimAddr() string {
	return fmt.Sprintf("%s:%d", cmd.Baggageclaim.BindIP, cmd.Baggageclaim.BindPort)
}

func (cmd *WorkerCommand) baggageclaimURL() string {
	return fmt.Sprintf("http://%s", cmd.baggageclaimAddr())
}

func (cmd *WorkerCommand) workerName() (string, error) {
	if cmd.Worker.Name != "" {
		return cmd.Worker.Name, nil
	}

	return os.Hostname()
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
	volumesDir := filepath.Join(cmd.WorkDir.Path(), "volumes")

	err := os.MkdirAll(volumesDir, 0755)
	if err != nil {
		return nil, err
	}

	cmd.Baggageclaim.VolumesDir = flag.Dir(volumesDir)

	cmd.Baggageclaim.OverlaysDir = filepath.Join(cmd.WorkDir.Path(), "overlays")

	return cmd.Baggageclaim.Runner(nil)
}
