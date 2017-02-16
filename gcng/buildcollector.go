package gcng

import "code.cloudfoundry.org/lager"

type buildCollector struct {
	logger       lager.Logger
	buildFactory buildFactory
}

//go:generate counterfeiter . buildFactory
type buildFactory interface {
	MarkNonInterceptibleBuilds() error
}

func NewBuildCollector(
	logger lager.Logger,
	buildFactory buildFactory,
) *buildCollector {
	return &buildCollector{
		logger:       logger,
		buildFactory: buildFactory,
	}

}

func (b *buildCollector) Run() error {
	b.logger.Debug("starting")
	defer b.logger.Debug("finishing")
	b.logger.Debug("marking builds as non-interceptible")

	err := b.buildFactory.MarkNonInterceptibleBuilds()
	if err != nil {
		b.logger.Error("error marking builds as non-interceptible", err)
		return err
	}

	return nil
}
