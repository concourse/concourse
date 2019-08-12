package configserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/vars"
	"github.com/ghodss/yaml"
	"github.com/hashicorp/go-multierror"
	"github.com/tedsuo/rata"
)

func (s *Server) SaveConfig(w http.ResponseWriter, r *http.Request) {
	session := s.logger.Session("set-config")

	query := r.URL.Query()

	checkCredentials := false
	if _, exists := query[atc.SaveConfigCheckCreds]; exists {
		checkCredentials = true
	}

	var version db.ConfigVersion
	if configVersionStr := r.Header.Get(atc.ConfigVersionHeader); len(configVersionStr) != 0 {
		_, err := fmt.Sscanf(configVersionStr, "%d", &version)
		if err != nil {
			session.Error("malformed-config-version", err)
			s.handleBadRequest(w, fmt.Sprintf("config version is malformed: %s", err))
			return
		}
	}

	var config atc.Config
	switch r.Header.Get("Content-type") {
	case "application/json", "application/x-yaml":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.handleBadRequest(w, fmt.Sprintf("read failed: %s", err))
			return
		}

		ignoredUnknownToplevels := map[string]interface{}{}

		// do a naive unmarshal first so we can ignore unknown top-level keys
		err = yaml.UnmarshalStrict(body, &ignoredUnknownToplevels)
		if err != nil {
			s.handleBadRequest(w, "malformed config")
			return
		}

		for k := range ignoredUnknownToplevels {
			switch k {
			case "groups", "jobs", "resources", "resource_types":
			default:
				delete(ignoredUnknownToplevels, k)
			}
		}

		configWithoutUnknownToplevels, err := yaml.Marshal(ignoredUnknownToplevels)
		if err != nil {
			s.handleBadRequest(w, fmt.Sprintf("yaml re-marshal failed: %s", err))
			return
		}

		err = yaml.UnmarshalStrict(configWithoutUnknownToplevels, &config, yaml.DisallowUnknownFields)
		if err != nil {
			session.Error("malformed-request-payload", err, lager.Data{
				"content-type": r.Header.Get("Content-Type"),
			})

			s.handleBadRequest(w, fmt.Sprintf("malformed config: %s", err))
			return
		}
	default:
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	warnings, errorMessages := config.Validate()
	if len(errorMessages) > 0 {
		session.Info("ignoring-invalid-config")
		s.handleBadRequest(w, errorMessages...)
		return
	}

	pipelineName := rata.Param(r, "pipeline_name")
	teamName := rata.Param(r, "team_name")

	if checkCredentials {
		variables := creds.NewVariables(s.secretManager, teamName, pipelineName)

		errs := validateCredParams(variables, config, session)
		if errs != nil {
			s.handleBadRequest(w, fmt.Sprintf("credential validation failed\n\n%s", errs))
			return
		}
	}

	session.Info("saving")

	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		session.Error("failed-to-find-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		session.Debug("team-not-found")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, created, err := team.SavePipeline(pipelineName, config, version, true)
	if err != nil {
		session.Error("failed-to-save-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to save config: %s", err)
		return
	}

	session.Info("saved")

	w.Header().Set("Content-Type", "application/json")

	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	s.writeSaveConfigResponse(w, atc.SaveConfigResponse{Warnings: warnings})
}

// Simply validate that the credentials exist; don't do anything with the actual secrets
func validateCredParams(credMgrVars vars.Variables, config atc.Config, session lager.Logger) error {
	var errs error

	for _, resourceType := range config.ResourceTypes {
		_, err := creds.NewSource(credMgrVars, resourceType.Source).Evaluate()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	for _, resource := range config.Resources {
		_, err := creds.NewSource(credMgrVars, resource.Source).Evaluate()
		if err != nil {
			errs = multierror.Append(errs, err)
		}

		_, err = creds.NewString(credMgrVars, resource.WebhookToken).Evaluate()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	for _, job := range config.Jobs {
		for _, plan := range job.Plan {
			_, err := creds.NewParams(credMgrVars, plan.Params).Evaluate()
			if err != nil {
				errs = multierror.Append(errs, err)
			}

			if plan.TaskConfigPath != "" {
				// external task - we can't really validate much right now, because task yaml will be
				// retrieved in runtime during job execution. but we can validate vars and params which will be
				// passed to this task
				err = creds.NewTaskParamsValidator(credMgrVars, plan.Params).Validate()
				if err != nil {
					errs = multierror.Append(errs, err)
				}

				err = creds.NewTaskVarsValidator(credMgrVars, plan.TaskVars).Validate()
				if err != nil {
					errs = multierror.Append(errs, err)
				}

			} else if plan.TaskConfig != nil {
				// embedded task - we can fully validate it, interpolating with cred mgr variables
				var taskConfigSource exec.TaskConfigSource
				embeddedTaskVars := []vars.Variables{credMgrVars}
				taskConfigSource = exec.StaticConfigSource{Config: plan.TaskConfig}
				taskConfigSource = exec.InterpolateTemplateConfigSource{ConfigSource: taskConfigSource, Vars: embeddedTaskVars}
				taskConfigSource = exec.ValidatingConfigSource{ConfigSource: taskConfigSource}
				_, err = taskConfigSource.FetchConfig(session, nil)
				if err != nil {
					errs = multierror.Append(errs, err)
				}
			}

		}
	}

	if errs != nil {
		session.Info("config-has-invalid-creds", lager.Data{"errors": errs.Error()})
	}

	return errs
}

func (s *Server) handleBadRequest(w http.ResponseWriter, errorMessages ...string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	s.writeSaveConfigResponse(w, atc.SaveConfigResponse{
		Errors: errorMessages,
	})
}

func (s *Server) writeSaveConfigResponse(w http.ResponseWriter, saveConfigResponse atc.SaveConfigResponse) {
	responseJSON, err := json.Marshal(saveConfigResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to generate error response: %s", err)
		return
	}

	_, err = w.Write(responseJSON)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
