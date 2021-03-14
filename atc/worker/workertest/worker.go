package workertest

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"

	. "github.com/onsi/gomega"
)

type PoolFactory func(worker.Factory) worker.Pool

type Worker interface {
	Name() string
	Setup(*Scenario)
	Build(worker.Pool, db.Worker) runtime.Worker
}

type Factory struct {
	Workers []Worker
}

func (f Factory) NewWorker(_ lager.Logger, pool worker.Pool, dbWorker db.Worker) runtime.Worker {
	worker, _, ok := f.FindWorker(dbWorker.Name())
	Expect(ok).To(BeTrue(), "worker '%s' was not setup in the scenario", dbWorker.Name())

	return worker.Build(pool, dbWorker)
}

func (f Factory) FindWorker(name string) (Worker, int, bool) {
	for i, w := range f.Workers {
		if w.Name() == name {
			return w, i, true
		}
	}
	return nil, 0, false
}
