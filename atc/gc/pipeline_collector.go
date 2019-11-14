package gc

import (
	"code.cloudfoundry.org/lager"
	"time"

	"github.com/concourse/concourse/atc/db"
)


// pipelineCollector takes a parameter `ttl`, and checks all pipelines. If a
// non-paused has no any build within `ttl`, then it will be paused; if a paused
// pipeline has no any build within 2*`ttl`, then it will be destroyed. It `ttl`
// is 0, then pipelineCollector will not touch any pipeline.
type pipelineCollector struct {
	pipelineFactory db.PipelineFactory
	ttl             time.Duration
}

func NewPipelineCollector(pipelineFactory db.PipelineFactory, ttl time.Duration) *pipelineCollector {
	return &pipelineCollector{
		pipelineFactory: pipelineFactory,
		ttl:             ttl,
	}
}

func (p *pipelineCollector) Collect(logger lager.Logger) error {
	if p.ttl == 0 {
		logger.Debug("ttl-is-zero")
		return nil
	}

	pipelines, err := p.pipelineFactory.AllPipelines()
	if err != nil {
		logger.Error("failed-to-get-pipelines", err)
		return err
	}
	logger.Debug("after-get-all-pipelines", lager.Data{"total-pipeline-count": len(pipelines)})

	for _, pipeline := range pipelines {
		log := logger.Session("tick").WithData(lager.Data{"pipeline-id": pipeline.ID()})
		if pipeline.Paused() {
			page := db.Page{
				Limit: 1,
				Since: int(time.Now().Unix()-2*int64(p.ttl.Seconds())),
			}
			builds, _, err := pipeline.BuildsWithTime(page)
			if err != nil {
				logger.Error("failed-to-query-build-of-pipeline", err)
			}
			logger.Debug("paused-query-builds", lager.Data{"buillds": len(builds)})
			if len(builds) == 0 {
				log.Info("destroy-pipeline")
				err := pipeline.Destroy()
				if err != nil {
					log.Error("failed-to-destroy-pipeline", err)
				}
			}
		} else {
			page := db.Page{
				Limit: 1,
				Since: int(time.Now().Unix()-int64(p.ttl.Seconds())),
			}
			builds, _, err := pipeline.BuildsWithTime(page)
			if err != nil {
				log.Error("failed-to-query-build-of-pipeline", err)
			}
			logger.Debug("running-query-builds", lager.Data{"buillds": len(builds)})
			if len(builds) == 0 {
				log.Info("pause-pipeline")
				err := pipeline.Pause()
				if err != nil {
					log.Error("failed-to-pause-pipeline", err)
				}
			}
		}
	}

	return nil
}
