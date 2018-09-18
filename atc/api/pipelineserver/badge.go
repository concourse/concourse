package pipelineserver

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/api/jobserver"
	"github.com/concourse/atc/db"
)

func badgeForPipeline(pipeline db.Pipeline, logger lager.Logger) (*jobserver.Badge, error) {
	var build db.Build

	jobStatusPrecedence := map[db.BuildStatus]int{
		db.BuildStatusFailed:    1,
		db.BuildStatusErrored:   2,
		db.BuildStatusAborted:   3,
		db.BuildStatusSucceeded: 4,
	}

	jobs, err := pipeline.Jobs()
	if err != nil {
		logger.Error("could-not-get-jobs", err)
		return nil, err
	}

	for _, job := range jobs {
		b, _, err := job.FinishedAndNextBuild()
		if err != nil {
			logger.Error("could-not-get-finished-and-next-build", err)
			return nil, err
		}

		if b == nil {
			continue
		}

		if build == nil || jobStatusPrecedence[b.Status()] < jobStatusPrecedence[build.Status()] {
			build = b
		}
	}

	return jobserver.BadgeForBuild(build), nil
}

func (s *Server) PipelineBadge(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("pipeline-badge")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "image/svg+xml")

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Expires", "0")

		w.WriteHeader(http.StatusOK)

		badge, err := badgeForPipeline(pipeline, logger)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, badge)
	})
}
