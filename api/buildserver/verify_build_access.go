package buildserver

import (
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

func (s *Server) verifyBuildAcccess(build db.Build, r *http.Request) (bool, error) {
	if !auth.IsAuthenticated(r) {
		if build.IsOneOff() {
			return false, nil
		}

		pipeline, err := build.GetPipeline()
		if err != nil {
			s.logger.Error("failed-to-get-pipeline", err)
			return false, err
		}

		if !pipeline.Public {
			return false, nil
		}

		config, _, err := build.GetConfig()
		if err != nil {
			s.logger.Error("failed-to-get-config", err)
			return false, err
		}

		public, err := config.JobIsPublic(build.JobName())
		if err != nil {
			s.logger.Error("failed-to-see-job-is-public", err)
			return false, err
		}

		if !public {
			return false, nil
		}
	}

	return true, nil
}
