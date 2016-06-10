package buildserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

func (s *Server) BuildResources(w http.ResponseWriter, r *http.Request) {
	log := s.logger.Session("build-resources")
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))
	if err != nil {
		log.Error("cannot-parse-build-id", err, lager.Data{"buildID": r.FormValue(":build_id")})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	inputs, outputs, found, err := s.getMeAllTheThings(buildID, getTeamName(r))
	if err != nil {
		log.Error("cannot-find-build", err, lager.Data{"buildID": r.FormValue(":build_id")})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	atcInputs := make([]atc.PublicBuildInput, 0, len(inputs))
	for _, input := range inputs {
		atcInputs = append(atcInputs, present.PublicBuildInput(input))
	}

	atcOutputs := []atc.VersionedResource{}
	for _, output := range outputs {
		atcOutputs = append(atcOutputs, present.VersionedResource(output.VersionedResource))
	}

	output := atc.BuildInputsOutputs{
		Inputs:  atcInputs,
		Outputs: atcOutputs,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(output)
}

func (s *Server) getMeAllTheThings(buildID int, teamName string) ([]db.BuildInput, []db.BuildOutput, bool, error) {
	teamDB := s.teamDBFactory.GetTeamDB(teamName)
	build, found, err := teamDB.GetBuild(buildID)
	if err != nil {
		return []db.BuildInput{}, []db.BuildOutput{}, false, err
	}

	if !found {
		return []db.BuildInput{}, []db.BuildOutput{}, false, nil
	}

	inputs, outputs, err := build.GetResources()
	return inputs, outputs, found, err
}
