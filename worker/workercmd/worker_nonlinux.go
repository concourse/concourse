// +build !linux

package workercmd

import (
	"runtime"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/tedsuo/ifrit"
)

type RuntimeConfiguration struct {
}

type GuardianRuntime struct {
	RequestTimeout time.Duration `yaml:"request_timeout,omitempty"`
}

type ContainerdRuntime struct {
}

type Certs struct{}

var ValidRuntimes = []string{}

var RuntimeDefaults = RuntimeConfiguration{}

var GuardianDefaults = GuardianRuntime{
	RequestTimeout: 5 * time.Minute,
}

var ContainerdDefaults = ContainerdRuntime{}

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
