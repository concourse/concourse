package workercmd

import (
	"fmt"
	"net"
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
	"github.com/concourse/concourse/flag"
	"github.com/concourse/concourse/worker"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

type WorkerCommand struct {
	Worker WorkerConfig

	TSA worker.TSAConfig `yaml:"tsa,omitempty"`

	Certs Certs

	WorkDir flag.Dir `yaml:"work_dir,omitempty" validate:"required"`

	BindIP   net.IP `yaml:"bind_ip,omitempty"`
	BindPort uint16 `yaml:"bind_port,omitempty"`

	Debug DebugConfig `yaml:"debug,omitempty"`

	Healthcheck HealthcheckConfig `yaml:"healthcheck,omitempty"`

	SweepInterval               time.Duration `yaml:"sweep_interval,omitempty"`
	VolumeSweeperMaxInFlight    uint16        `yaml:"volume_sweeper_max_in_flight,omitempty"`
	ContainerSweeperMaxInFlight uint16        `yaml:"container_sweeper_max_in_flight,omitempty"`

	RebalanceInterval time.Duration `yaml:"rebalance_interval,omitempty"`

	ConnectionDrainTimeout time.Duration `yaml:"connection_drain_timeout,omitempty"`

	RuntimeConfiguration

	// This refers to flags relevant to the operation of the Guardian runtime.
	// For historical reasons it is namespaced under "garden" i.e. CONCOURSE_GARDEN instead of "guardian" i.e. CONCOURSE_GUARDIAN
	Guardian GuardianRuntime `yaml:"garden,omitempty"`

	Containerd ContainerdRuntime `yaml:"containerd,omitempty"`

	ExternalGardenURL flag.URL `yaml:"external_garden_url,omitempty"`

	Baggageclaim baggageclaimcmd.BaggageclaimCommand `yaml:"baggageclaim,omitempty"`

	ResourceTypes flag.Dir `yaml:"resource_types,omitempty"`

	Logger flag.Lager
}

type DebugConfig struct {
	BindIP   net.IP `yaml:"bind_ip,omitempty"`
	BindPort uint16 `yaml:"bind_port,omitempty"`
}

type HealthcheckConfig struct {
	BindIP   net.IP        `yaml:"bind_ip,omitempty"`
	BindPort uint16        `yaml:"bind_port,omitempty"`
	Timeout  time.Duration `yaml:"timeout,omitempty"`
}

var CmdDefaults = WorkerCommand{
	BindIP:   net.IPv4(127, 0, 0, 1),
	BindPort: 7777,

	TSA: worker.TSAConfig{
		Hosts: []string{"127.0.0.1:2222"},
	},

	Debug: DebugConfig{
		BindIP:   net.IPv4(127, 0, 0, 1),
		BindPort: 7776,
	},

	Healthcheck: HealthcheckConfig{
		BindIP:   net.IPv4(0, 0, 0, 0),
		BindPort: 8888,
		Timeout:  5 * time.Second,
	},

	SweepInterval:               30 * time.Second,
	VolumeSweeperMaxInFlight:    3,
	ContainerSweeperMaxInFlight: 5,

	RebalanceInterval:      4 * time.Hour,
	ConnectionDrainTimeout: 1 * time.Hour,

	Guardian: GuardianRuntime{
		RequestTimeout: 5 * time.Minute,
	},

	Baggageclaim: baggageclaimcmd.BaggageclaimCommand{
		BindIP:   net.IPv4(127, 0, 0, 1),
		BindPort: 7788,

		Debug: baggageclaimcmd.DebugConfig{
			BindIP:   net.IPv4(127, 0, 0, 1),
			BindPort: 7787,
		},

		P2p: baggageclaimcmd.P2pConfig{
			InterfaceNamePattern: "eth0",
			InterfaceFamily:      4,
		},

		Driver: "detect",

		BtrfsBin: "btrfs",
		MkfsBin:  "mkfs.btrfs",
	},
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
		cmd.Healthcheck.Timeout,
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
					fmt.Sprintf("%s:%d", cmd.Debug.BindIP, cmd.Debug.BindPort),
					http.DefaultServeMux,
				),
			),
		},
		{
			Name: "healthcheck",
			Runner: concourseCmd.NewLoggingRunner(
				logger.Session("healthcheck-runner"),
				http_server.New(
					fmt.Sprintf("%s:%d", cmd.Healthcheck.BindIP, cmd.Healthcheck.BindPort),
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
