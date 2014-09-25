package radar

import (
	"os"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/resources"
	"github.com/tedsuo/rata"
)

type Runner struct {
	Radar *Radar

	Noop      bool
	Resources config.Resources

	TurbineEndpoint *rata.RequestGenerator
}

func (runner *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	if runner.Noop {
		close(ready)
		<-signals
		return nil
	}

	for _, resource := range runner.Resources {
		checker := resources.NewTurbineChecker(runner.TurbineEndpoint)
		runner.Radar.Scan(checker, resource)
	}

	close(ready)

	<-signals

	return nil
}
