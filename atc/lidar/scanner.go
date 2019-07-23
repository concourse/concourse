package lidar

import (
	"context"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
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

	return nil
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

		if parentType.Version() == nil {
			return errors.New("parent type has no version")
		}
	}

	timeout := s.defaultCheckTimeout
	if to := checkable.CheckTimeout(); to != "" {
		timeout, err = time.ParseDuration(to)
		if err != nil {
			s.logger.Error("failed-to-parse-check-timeout", err)
			return err
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

	variables := creds.NewVariables(
		s.secrets,
		checkable.TeamName(),
		checkable.PipelineName(),
	)

	source, err := creds.NewSource(variables, checkable.Source()).Evaluate()
	if err != nil {
		s.logger.Error("failed-to-evaluate-source", err)
		return err
	}

	filteredTypes := resourceTypes.Filter(checkable).Deserialize()
	versionedResourceTypes, err := creds.NewVersionedResourceTypes(variables, filteredTypes).Evaluate()
	if err != nil {
		s.logger.Error("failed-to-evaluate-resource-types", err)
		return err
	}

	// This could have changed based on new variable interpolation so update it
	resourceConfigScope, err := checkable.SetResourceConfig(source, versionedResourceTypes)
	if err != nil {
		s.logger.Error("failed-to-update-resource-config", err)
		return err
	}

	var fromVersion atc.Version
	rcv, found, err := resourceConfigScope.LatestVersion()
	if err != nil {
		s.logger.Error("failed-to-get-current-version", err)
		return err
	}

	if found {
		fromVersion = atc.Version(rcv.Version())
	}

	plan := atc.Plan{
		Check: &atc.CheckPlan{
			Name:        checkable.Name(),
			Type:        checkable.Type(),
			Source:      source,
			Tags:        checkable.Tags(),
			Timeout:     timeout.String(),
			FromVersion: fromVersion,

			VersionedResourceTypes: versionedResourceTypes,
		},
	}

	_, created, err := s.checkFactory.CreateCheck(
		resourceConfigScope.ID(),
		resourceConfigScope.ResourceConfig().ID(),
		resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
		checkable.TeamID(),
		false,
		plan,
	)
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
