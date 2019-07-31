package lidar

import (
	"context"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/pkg/errors"
)

func NewScanner(
	logger lager.Logger,
	checkFactory db.CheckFactory,
	secrets creds.Secrets,
	defaultCheckTimeout time.Duration,
	defaultCheckInterval time.Duration,
) *scanner {
	return &scanner{
		logger:               logger,
		checkFactory:         checkFactory,
		secrets:              secrets,
		defaultCheckTimeout:  defaultCheckTimeout,
		defaultCheckInterval: defaultCheckInterval,
	}
}

type scanner struct {
	logger lager.Logger

	checkFactory         db.CheckFactory
	secrets              creds.Secrets
	defaultCheckTimeout  time.Duration
	defaultCheckInterval time.Duration
}

func (s *scanner) Run(ctx context.Context) error {

	lock, acquired, err := s.checkFactory.AcquireScanningLock(s.logger)
	if err != nil {
		s.logger.Error("failed-to-get-scanning-lock", err)
		return err
	}

	if !acquired {
		s.logger.Debug("scanning-already-in-progress")
		return nil
	}

	defer lock.Release()

	resources, err := s.checkFactory.Resources()
	if err != nil {
		s.logger.Error("failed-to-get-resources", err)
		return err
	}

	resourceTypes, err := s.checkFactory.ResourceTypes()
	if err != nil {
		s.logger.Error("failed-to-get-resources", err)
		return err
	}

	waitGroup := new(sync.WaitGroup)

	for _, resource := range resources {
		waitGroup.Add(1)

		go func(resource db.Resource, resourceTypes db.ResourceTypes) {
			defer waitGroup.Done()

			err = s.check(resource, resourceTypes)
			s.setCheckError(s.logger, resource, err)

		}(resource, resourceTypes)
	}

	waitGroup.Wait()

	return s.checkFactory.NotifyChecker()
}

func (s *scanner) check(checkable db.Checkable, resourceTypes db.ResourceTypes) error {

	var err error

	parentType, found := resourceTypes.Parent(checkable)
	if found {
		err = s.check(parentType, resourceTypes)
		s.setCheckError(s.logger, parentType, err)

		if err != nil {
			s.logger.Error("failed-to-create-type-check", err)
			return errors.Wrapf(err, "parent type '%v' error", parentType.Name())
		}
	}

	interval := s.defaultCheckInterval
	if every := checkable.CheckEvery(); every != "" {
		interval, err = time.ParseDuration(every)
		if err != nil {
			s.logger.Error("failed-to-parse-check-every", err)
			return err
		}
	}

	if time.Now().Before(checkable.LastCheckEndTime().Add(interval)) {
		s.logger.Debug("interval-not-reached", lager.Data{"interval": interval})
		return nil
	}

	version := checkable.CurrentPinnedVersion()

	_, created, err := s.checkFactory.TryCreateCheck(checkable, resourceTypes, version, false)
	if err != nil {
		s.logger.Error("failed-to-create-check", err)
		return err
	}

	if !created {
		s.logger.Info("check-already-exists")
	}

	return nil
}

func (s *scanner) setCheckError(logger lager.Logger, checkable db.Checkable, err error) {
	setErr := checkable.SetCheckSetupError(err)
	if setErr != nil {
		logger.Error("failed-to-set-check-error", setErr)
	}
}
