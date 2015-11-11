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

type BuildNotFoundError struct{}

func (b BuildNotFoundError) Error() string {
	return "Build Not Found"
}

func (s *Server) BuildResources(w http.ResponseWriter, r *http.Request) {
	log := s.logger.Session("build-resources")
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))
	if err != nil {
		log.Error("cannot-parse-build-id", err, lager.Data{"buildID": r.FormValue(":build_id")})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	inputs, outputs, err := s.getMeAllTheThings(buildID)
	if err != nil {
		if _, ok := err.(BuildNotFoundError); ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		log.Error("cannot-find-build", err, lager.Data{"buildID": r.FormValue(":build_id")})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	atcInputs := []atc.VersionedResource{}
	for _, input := range inputs {
		atcInputs = append(atcInputs, present.VersionedResource(input))
	}

	atcOutputs := []atc.VersionedResource{}
	for _, output := range outputs {
		atcOutputs = append(atcOutputs, present.VersionedResource(output))
	}

	output := atc.BuildInputsOutputs{
		Inputs:  atcInputs,
		Outputs: atcOutputs,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(output)
}

func (s *Server) getMeAllTheThings(buildID int) (db.SavedVersionedResources, db.SavedVersionedResources, error) {
	_, found, err := s.db.GetBuild(buildID)
	if err != nil {
		return db.SavedVersionedResources{}, db.SavedVersionedResources{}, err
	}

	if !found {
		return db.SavedVersionedResources{}, db.SavedVersionedResources{}, BuildNotFoundError{}
	}

	inputs, err := s.db.GetBuildInputVersionedResouces(buildID)
	if err != nil {
		return db.SavedVersionedResources{}, db.SavedVersionedResources{}, err
	}

	outputs, err := s.db.GetBuildOutputVersionedResouces(buildID)
	if err != nil {
		return db.SavedVersionedResources{}, db.SavedVersionedResources{}, err
	}

	return inputs, outputs, nil
}
