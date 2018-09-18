package inputmapper

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/scheduler/inputmapper/inputconfig"
)

//go:generate counterfeiter . InputMapper

type InputMapper interface {
	SaveNextInputMapping(
		logger lager.Logger,
		versions *algorithm.VersionsDB,
		job db.Job,
		resources db.Resources,
	) (algorithm.InputMapping, error)
}

func NewInputMapper(pipeline db.Pipeline, transformer inputconfig.Transformer) InputMapper {
	return &inputMapper{pipeline: pipeline, transformer: transformer}
}

type inputMapper struct {
	pipeline    db.Pipeline
	transformer inputconfig.Transformer
}

func (i *inputMapper) SaveNextInputMapping(
	logger lager.Logger,
	versions *algorithm.VersionsDB,
	job db.Job,
	resources db.Resources,
) (algorithm.InputMapping, error) {
	logger = logger.Session("save-next-input-mapping")

	inputConfigs := job.Config().Inputs()

	for i, inputConfig := range inputConfigs {
		resource, found := resources.Lookup(inputConfig.Resource)

		if !found {
			logger.Debug("failed-to-find-resource")
			continue
		}

		if len(resource.PinnedVersion()) != 0 {
			inputConfigs[i].Version = &atc.VersionConfig{Pinned: resource.PinnedVersion()}
		}
	}

	algorithmInputConfigs, err := i.transformer.TransformInputConfigs(versions, job.Name(), inputConfigs)
	if err != nil {
		logger.Error("failed-to-get-algorithm-input-configs", err)
		return nil, err
	}

	independentMapping := algorithm.InputMapping{}
	for _, inputConfig := range algorithmInputConfigs {
		singletonMapping, ok := algorithm.InputConfigs{inputConfig}.Resolve(versions)
		if ok {
			independentMapping[inputConfig.Name] = singletonMapping[inputConfig.Name]
		}
	}

	err = job.SaveIndependentInputMapping(independentMapping)
	if err != nil {
		logger.Error("failed-to-save-independent-input-mapping", err)
		return nil, err
	}

	if len(independentMapping) < len(inputConfigs) {
		// this is necessary to prevent builds from running with missing pinned versions
		err := job.DeleteNextInputMapping()
		if err != nil {
			logger.Error("failed-to-delete-next-input-mapping-after-missing-pending", err)
		}

		return nil, err
	}

	resolvedMapping, ok := algorithmInputConfigs.Resolve(versions)
	if !ok {
		err := job.DeleteNextInputMapping()
		if err != nil {
			logger.Error("failed-to-delete-next-input-mapping-after-failed-resolve", err)
		}

		return nil, err
	}

	err = job.SaveNextInputMapping(resolvedMapping)
	if err != nil {
		logger.Error("failed-to-save-next-input-mapping", err)
		return nil, err
	}

	return resolvedMapping, nil
}
