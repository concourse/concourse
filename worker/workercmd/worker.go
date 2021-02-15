package workercmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc/worker/gclient"
	concourseCmd "github.com/concourse/concourse/cmd"
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

	DebugBindIP   flag.IP `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16  `long:"debug-bind-port" default:"7776"      description:"Port on which to listen for the pprof debugger endpoints."`

	HealthcheckBindIP   flag.IP       `long:"healthcheck-bind-ip"    default:"0.0.0.0"  description:"IP address on which to listen for health checking requests."`
	HealthcheckBindPort uint16        `long:"healthcheck-bind-port"  default:"8888"     description:"Port on which to listen for health checking requests."`
	HealthCheckTimeout  time.Duration `long:"healthcheck-timeout"    default:"5s"       description:"HTTP timeout for the full duration of health checking."`

	SweepInterval               time.Duration `long:"sweep-interval" default:"30s" description:"Interval on which containers and volumes will be garbage collected from the worker."`
	VolumeSweeperMaxInFlight    uint16        `long:"volume-sweeper-max-in-flight" default:"3" description:"Maximum number of volumes which can be swept in parallel."`
	ContainerSweeperMaxInFlight uint16        `long:"container-sweeper-max-in-flight" default:"5" description:"Maximum number of containers which can be swept in parallel."`

	RebalanceInterval time.Duration `long:"rebalance-interval" default:"4h" description:"Duration after which the registration should be swapped to another random SSH gateway."`

	ConnectionDrainTimeout time.Duration `long:"connection-drain-timeout" default:"1h" description:"Duration after which a worker should give up draining forwarded connections on shutdown."`

	RuntimeConfiguration `group:"Runtime Configuration"`

	// This refers to flags relevant to the operation of the Guardian runtime.
	// For historical reasons it is namespaced under "garden" i.e. CONCOURSE_GARDEN instead of "guardian" i.e. CONCOURSE_GUARDIAN
	Guardian GuardianRuntime `group:"Guardian Configuration" namespace:"garden"`

	Containerd ContainerdRuntime `group:"Containerd Configuration" namespace:"containerd"`

	ExternalGardenURL flag.URL `long:"external-garden-url" description:"API endpoint of an externally managed Garden server to use instead of running the embedded Garden server."`

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
		cmd.ResourceTypes = flag.Dir(concourseCmd.DiscoverAsset("resource-types"))
	}

	logger, _ := cmd.Logger.Logger("worker")

	atcWorker, gardenServerRunner, err := cmd.gardenServerRunner(logger.Session("garden"))
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

	tsaClient := cmd.TSA.Client(atcWorker)

	beaconRunner := worker.NewBeaconRunner(
		logger.Session("beacon-runner"),
		tsaClient,
		cmd.RebalanceInterval,
		cmd.ConnectionDrainTimeout,
		cmd.gardenAddr(),
		cmd.baggageclaimAddr(),
	)

	gardenClient := gclient.BasicGardenClientWithRequestTimeout(
		logger.Session("garden-connection"),
		cmd.Guardian.RequestTimeout,
		cmd.gardenURL(),
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
			// we've seen destroy calls to baggageclaim hang and lock gc
			// gc is periodic so we don't need to retry here, we can rely
			// on the next sweeper tick.
			Timeout: 5 * time.Minute,
		},
	)

	containerSweeper := worker.NewContainerSweeper(
		logger.Session("container-sweeper"),
		cmd.SweepInterval,
		tsaClient,
		gardenClient,
		cmd.ContainerSweeperMaxInFlight,
	)

	volumeSweeper := worker.NewVolumeSweeper(
		logger.Session("volume-sweeper"),
		cmd.SweepInterval,
		tsaClient,
		baggageclaimClient,
		cmd.VolumeSweeperMaxInFlight,
	)

	var members grouper.Members

	if !cmd.gardenServerIsExternal() {
		members = append(members, grouper.Member{
			Name:   "garden",
			Runner: concourseCmd.NewLoggingRunner(logger.Session("garden-runner"), gardenServerRunner),
		})
	}

	members = append(members, grouper.Members{
		{
			Name:   "baggageclaim",
			Runner: concourseCmd.NewLoggingRunner(logger.Session("baggageclaim-runner"), baggageclaimRunner),
		},
		{
			Name: "debug",
			Runner: concourseCmd.NewLoggingRunner(
				logger.Session("debug-runner"),
				http_server.New(
					fmt.Sprintf("%s:%d", cmd.DebugBindIP.IP, cmd.DebugBindPort),
					http.DefaultServeMux,
				),
			),
		},
		{
			Name: "healthcheck",
			Runner: concourseCmd.NewLoggingRunner(
				logger.Session("healthcheck-runner"),
				http_server.New(
					fmt.Sprintf("%s:%d", cmd.HealthcheckBindIP.IP, cmd.HealthcheckBindPort),
					http.HandlerFunc(healthChecker.CheckHealth),
				),
			),
		},
		{
			Name: "beacon",
			Runner: concourseCmd.NewLoggingRunner(
				logger.Session("beacon-runner"),
				beaconRunner,
			),
		},
		{
			Name: "container-sweeper",
			Runner: concourseCmd.NewLoggingRunner(
				logger.Session("container-sweeper"),
				containerSweeper,
			),
		},
		{
			Name: "volume-sweeper",
			Runner: concourseCmd.NewLoggingRunner(
				logger.Session("volume-sweeper"),
				volumeSweeper,
			),
		},
	}...)

	return grouper.NewParallel(os.Interrupt, members), nil
}

func (cmd *WorkerCommand) gardenServerIsExternal() bool {
	return cmd.ExternalGardenURL.URL != nil
}

func (cmd *WorkerCommand) gardenAddr() string {
	if cmd.gardenServerIsExternal() {
		return cmd.ExternalGardenURL.URL.Host
	}

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
