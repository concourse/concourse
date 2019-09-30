package courier

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . Runner

type Runner interface {
	Run(signals <-chan os.Signal, ready chan<- struct{}) error
}

//go:generate counterfeiter . Migrator

type Migrator interface {
	AcquireMigrationLock(lager.Logger) (lock.Lock, bool, error)
	Migrate(lager.Logger) error
}

type DataCourier struct {
	nextRunner Runner
	migrator   Migrator

	logger lager.Logger
}

func NewDataCourier(logger lager.Logger, runner Runner, migrator Migrator) *DataCourier {
	return &DataCourier{
		nextRunner: runner,
		migrator:   migrator,
		logger:     logger,
	}
}

func (df *DataCourier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	for {
		lock, acquired, err := df.migrator.AcquireMigrationLock(df.logger)
		if err != nil {
			return err
		}

		if !acquired {
			time.Sleep(time.Second)
			continue
		}

		defer lock.Release()

		err = df.migrator.Migrate(df.logger)
		if err != nil {
			return err
		}

		return df.nextRunner.Run(signals, ready)
	}
}
