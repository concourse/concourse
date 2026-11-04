//go:build windows

package workercmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/flag"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

type RuntimeConfiguration struct {
	Runtime string `long:"runtime" default:"houdini" choice:"houdini" choice:"containerd" description:"Runtime to use with the worker. Please note that Houdini is insecure and doesn't run 'tasks' in containers, and that the containerd runtime on Windows is experimental."`
}

type GuardianRuntime struct {
	RequestTimeout time.Duration `long:"request-timeout" default:"5m" description:"How long to wait for requests to the Garden server to complete. 0 means no timeout."`
}

type ContainerdRuntime struct {
	Config         flag.File     `long:"config"     description:"Path to a config file to use for the Containerd daemon."`
	Bin            string        `long:"bin"        description:"Path to a containerd executable (non-absolute names get resolved from %PATH%)."`
	InitBin        string        `long:"init-bin"   description:"Path to an init executable. By default will search within the concourse/bin directory the concourse binary is in."`
	LogLevel       string        `long:"log-level" default:"info" choice:"trace" choice:"debug" choice:"info" choice:"warn" choice:"error" choice:"fatal" choice:"panic" description:"Minimum level of logs to see."`
	RequestTimeout time.Duration `long:"request-timeout" default:"5m" description:"How long to wait for requests to Containerd to complete. 0 means no timeout."`
	MaxContainers  int           `long:"max-containers" default:"250" description:"Max container capacity. 0 means no limit."`

	Network struct {
		Name    string `long:"network-name" default:"nat" description:"Name of the HNS network to attach containers to. Created as a NAT network if it doesn't exist."`
		Pool    string `long:"network-pool" default:"10.80.0.0/24" description:"Subnet to use when the HNS network has to be created."`
		Gateway string `long:"network-gateway" default:"10.80.0.1" description:"Gateway address to use when the HNS network has to be created."`
	} `group:"Containerd Networking"`
}

type Certs struct{}

const houdiniRuntime = "houdini"
const containerdRuntime = "containerd"

func (cmd WorkerCommand) LessenRequirements(prefix string, command *flags.Command) {
	// created in the work-dir
	command.FindOptionByLongName(prefix + "baggageclaim-volumes").Required = false
}

func (cmd *WorkerCommand) gardenServerRunner(logger lager.Logger) (atc.Worker, ifrit.Runner, error) {
	worker := cmd.Worker.Worker()
	worker.Platform = "windows"

	var err error
	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	worker.ResourceTypes, err = cmd.loadResources(logger.Session("load-resources"))
	if err != nil {
		return atc.Worker{}, nil, err
	}

	var runner ifrit.Runner
	switch cmd.Runtime {
	case houdiniRuntime:
		runner, err = cmd.houdiniRunner(logger)
	case containerdRuntime:
		runner, err = cmd.containerdRunner(logger)
	default:
		err = fmt.Errorf("unsupported Runtime :%s", cmd.Runtime)
	}

	if err != nil {
		return atc.Worker{}, nil, err
	}

	return worker, runner, nil
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
	volumesDir := filepath.Join(cmd.WorkDir.Path(), "volumes")

	err := os.MkdirAll(volumesDir, 0755)
	if err != nil {
		return nil, err
	}

	cmd.Baggageclaim.VolumesDir = flag.Dir(volumesDir)

	cmd.Baggageclaim.OverlaysDir = filepath.Join(cmd.WorkDir.Path(), "overlays")

	return cmd.Baggageclaim.Runner(logger, nil)
}
