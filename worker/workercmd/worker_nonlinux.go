// +build !linux

package workercmd

import (
	"runtime"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
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
