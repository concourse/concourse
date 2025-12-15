//go:build !linux
// +build !linux

package workercmd

import (
	"os"
	"path/filepath"
	"runtime"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/flag/v2"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

type RuntimeConfiguration struct {
}

type GuardianRuntime struct {
	RequestTimeout time.Duration `long:"request-timeout" default:"5m" description:"How long to wait for requests to the Garden server to complete. 0 means no timeout."`
}

type ContainerdRuntime struct {
}

type Certs struct{}

func (cmd WorkerCommand) LessenRequirements(prefix string, command *flags.Command) {
	// created in the work-dir
	command.FindOptionByLongName(prefix + "baggageclaim-volumes").Required = false
}

func (cmd *WorkerCommand) gardenServerRunner(logger lager.Logger) (atc.Worker, ifrit.Runner, error) {
	worker := cmd.Worker.Worker()
	worker.Platform = runtime.GOOS
	var err error
	worker.Name, err = cmd.workerName()
	if err != nil {
		return atc.Worker{}, nil, err
	}

	runner, err := cmd.houdiniRunner(logger)
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
