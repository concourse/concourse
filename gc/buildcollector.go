package gc

import "code.cloudfoundry.org/lager"

type buildCollector struct {
	logger       lager.Logger
	buildFactory buildFactory
}

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
	b.logger.Debug("start")
	defer b.logger.Debug("done")

	err := b.buildFactory.MarkNonInterceptibleBuilds()
	if err != nil {
		b.logger.Error("failed-to-mark-non-interceptible-builds", err)
		return err
	}

	return nil
}
