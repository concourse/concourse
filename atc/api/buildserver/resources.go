package buildserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) BuildResources(build db.Build) http.Handler {
	logger := s.logger.Session("build-resources")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inputs, outputs, err := build.Resources()
		if err != nil {
			logger.Error("cannot-find-build", err, lager.Data{"buildID": r.FormValue(":build_id")})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		atcInputs := make([]atc.PublicBuildInput, 0, len(inputs))
		for _, input := range inputs {
			atcInputs = append(atcInputs, present.PublicBuildInput(input, build.PipelineID()))
		}

		atcOutputs := make([]atc.PublicBuildOutput, 0, len(outputs))
		for _, output := range outputs {
			atcOutputs = append(atcOutputs, present.PublicBuildOutput(output))
		}

		output := atc.BuildInputsOutputs{
			Inputs:  atcInputs,
			Outputs: atcOutputs,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(output)
		if err != nil {
			logger.Error("failed-to-encode-build-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
