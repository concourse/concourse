package flaghelpers

import (
	"strings"

	"github.com/concourse/concourse/fly/rc"
	"github.com/jessevdk/go-flags"
)

type WorkerFlag string

func (flag *WorkerFlag) Complete(match string) []flags.Completion {
	fly := parseFlags()

	target, err := rc.LoadTarget(fly.Target, false)
	if err != nil {
		return []flags.Completion{}
	}

	workers, err := target.Client().ListWorkers()
	if err != nil {
		return []flags.Completion{}
	}

	comps := []flags.Completion{}
	for _, worker := range workers {
		if strings.HasPrefix(worker.Name, match) {
			comps = append(comps, flags.Completion{Item: worker.Name})
		}
	}

	return comps
}

func (flag WorkerFlag) Name() string {
	return string(flag)
}
