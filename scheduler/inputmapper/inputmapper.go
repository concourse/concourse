package inputmapper

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/scheduler/inputmapper/inputconfig"
)

//go:generate counterfeiter . InputMapper

type InputMapper interface {
	SaveNextInputMapping(
		logger lager.Logger,
		versions *algorithm.VersionsDB,
		job atc.JobConfig,
	) (algorithm.InputMapping, error)
}

func NewInputMapper(pipeline dbng.Pipeline, transformer inputconfig.Transformer) InputMapper {
	return &inputMapper{pipeline: pipeline, transformer: transformer}
}

type inputMapper struct {
	pipeline    dbng.Pipeline
	transformer inputconfig.Transformer
}

func (i *inputMapper) SaveNextInputMapping(
	logger lager.Logger,
	versions *algorithm.VersionsDB,
	job atc.JobConfig,
) (algorithm.InputMapping, error) {
	logger = logger.Session("save-next-input-mapping")

	inputConfigs := job.Inputs()

	algorithmInputConfigs, err := i.transformer.TransformInputConfigs(versions, job.Name, inputConfigs)
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

	err = i.pipeline.SaveIndependentInputMapping(independentMapping, job.Name)
	if err != nil {
		logger.Error("failed-to-save-independent-input-mapping", err)
		return nil, err
	}

	if len(independentMapping) < len(inputConfigs) {
		// this is necessary to prevent builds from running with missing pinned versions
		err := i.pipeline.DeleteNextInputMapping(job.Name)
		if err != nil {
			logger.Error("failed-to-delete-next-input-mapping-after-missing-pending", err)
		}

		return nil, err
	}

	resolvedMapping, ok := algorithmInputConfigs.Resolve(versions)
	if !ok {
		err := i.pipeline.DeleteNextInputMapping(job.Name)
		if err != nil {
			logger.Error("failed-to-delete-next-input-mapping-after-failed-resolve", err)
		}

		return nil, err
	}

	err = i.pipeline.SaveNextInputMapping(resolvedMapping, job.Name)
	if err != nil {
		logger.Error("failed-to-save-next-input-mapping", err)
		return nil, err
	}

	return resolvedMapping, nil
}
