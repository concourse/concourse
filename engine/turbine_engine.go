package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc/db"
	"github.com/concourse/turbine"
	"github.com/concourse/turbine/event"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

//go:generate counterfeiter . EngineDB
type EngineDB interface {
	GetLastBuildEventID(buildID int) (int, error)
	SaveBuildEvent(buildID int, event db.BuildEvent) error

	SaveBuildStartTime(buildID int, startTime time.Time) error
	SaveBuildEndTime(buildID int, startTime time.Time) error

	SaveBuildInput(buildID int, input db.BuildInput) error
	SaveBuildOutput(buildID int, vr db.VersionedResource) error

	SaveBuildStatus(buildID int, status db.Status) error
}

type turbineEngine struct {
	turbineEndpoint *rata.RequestGenerator
	httpClient      *http.Client
	db              EngineDB
}

func NewTurbine(turbineEndpoint *rata.RequestGenerator, db EngineDB) Engine {
	return &turbineEngine{
		turbineEndpoint: turbineEndpoint,
		db:              db,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,

				// allow DNS to resolve differently every time
				DisableKeepAlives: true,
			},
		},
	}
}

func (engine *turbineEngine) CreateBuild(build turbine.Build) (Build, error) {
	// POST /builds, immediately start saving events
	return nil, nil
}

func (engine *turbineEngine) LookupBuild(build db.Build) (Build, error) {
	return &turbineBuild{
		guid: build.Guid,

		httpClient:      engine.httpClient,
		turbineEndpoint: rata.NewRequestGenerator(build.Endpoint, turbine.Routes),
	}, nil
}

func (engine *turbineEngine) ResumeBuild(build db.Build, logger lager.Logger) error {
	if build.Endpoint == "" || build.Guid == "" {
		logger.Info("saving-untrackable-build-as-errored")

		err := engine.db.SaveBuildStatus(build.ID, db.StatusErrored)
		if err != nil {
			logger.Error("failed-to-save-untrackable-build-as-errored", err)
			return err
		}

		return nil
	}

	generator := rata.NewRequestGenerator(build.Endpoint, turbine.Routes)

	events, err := generator.CreateRequest(
		turbine.GetBuildEvents,
		rata.Params{"guid": build.Guid},
		nil,
	)
	if err != nil {
		logger.Error("failed-to-create-events-request", err)
		return err
	}

	resp, err := http.DefaultClient.Do(events)
	if err != nil {
		logger.Error("failed-to-get-events", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		logger.Info("saving-orphaned-build-as-errored")

		err := engine.db.SaveBuildStatus(build.ID, db.StatusErrored)
		if err != nil {
			logger.Error("failed-to-save-orphaned-build-as-errored", err)
			return err
		}

		return nil
	}

	reader := sse.NewReader(resp.Body)

	outputs := map[string]db.VersionedResource{}

	var currentVersion string

	for {
		se, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				logger.Info("event-stream-completed")
				return nil
			}

			return err
		}

		id, err := strconv.Atoi(se.ID)
		if err != nil {
			logger.Error("non-numerical-event-id", err, lager.Data{
				"id": se.ID,
			})

			return err
		}

		err = engine.db.SaveBuildEvent(build.ID, db.BuildEvent{
			ID:      id,
			Type:    se.Name,
			Payload: string(se.Data),
		})
		if err != nil {
			logger.Error("failed-to-save-build-event", err, lager.Data{
				"event": se,
			})

			return err
		}

		if se.Name == "version" {
			var version event.Version
			err := json.Unmarshal(se.Data, &version)
			if err != nil {
				logger.Error("failed-to-unmarshal-version", err)
				return err
			}

			logger.Info("event-stream-version", lager.Data{
				"version": string(version),
			})

			currentVersion = string(version)
			continue
		}

		if se.Name == "end" {
			logger.Info("event-stream-ended")

			del, err := generator.CreateRequest(
				turbine.DeleteBuild,
				rata.Params{"guid": build.Guid},
				nil,
			)
			if err != nil {
				logger.Error("failed-to-create-delete-request", err)
				return err
			}

			resp, err := http.DefaultClient.Do(del)
			if err != nil {
				logger.Error("failed-to-delete-build", err)
				return err
			}

			resp.Body.Close()
			continue
		}

		switch currentVersion {
		case "1.0":
			fallthrough
		case "1.1":
			switch se.Name {
			case "status":
				logger.Info("processing-build-status", lager.Data{
					"event": string(se.Data),
				})

				var status event.Status
				err := json.Unmarshal(se.Data, &status)
				if err != nil {
					logger.Error("failed-to-unmarshal-status", err)
					return err
				}

				if status.Status == turbine.StatusStarted {
					err = engine.db.SaveBuildStartTime(build.ID, time.Unix(status.Time, 0))
					if err != nil {
						logger.Error("failed-to-save-build-start-time", err)
						return err
					}
				} else {
					err = engine.db.SaveBuildEndTime(build.ID, time.Unix(status.Time, 0))
					if err != nil {
						logger.Error("failed-to-save-build-end-time", err)
						return err
					}
				}

				err = engine.db.SaveBuildStatus(build.ID, db.Status(status.Status))
				if err != nil {
					logger.Error("failed-to-save-build-status", err)
					return err
				}

				if status.Status == turbine.StatusSucceeded {
					for _, output := range outputs {
						err := engine.db.SaveBuildOutput(build.ID, output)
						if err != nil {
							logger.Error("failed-to-save-build-output", err)
							return err
						}
					}
				}

			case "input":
				if build.JobName == "" {
					logger.Info("ignoring-build-input-for-one-off")
					break
				}

				logger.Info("processing-build-input", lager.Data{
					"event": string(se.Data),
				})

				var input event.Input
				err := json.Unmarshal(se.Data, &input)
				if err != nil {
					logger.Error("failed-to-unarshal-input", err)
					return err
				}

				if input.Input.Resource == "" {
					input.Input.Resource = input.Input.Name
				}

				vr := vrFromInput(input.Input)

				err = engine.db.SaveBuildInput(build.ID, db.BuildInput{
					Name:              input.Input.Name,
					VersionedResource: vr,
				})
				if err != nil {
					logger.Error("failed-to-save-build-input", err)
					return err
				}

				// record implicit output
				outputs[input.Input.Resource] = vr

			case "output":
				if build.JobName == "" {
					logger.Info("ignoring-build-output-for-one-off")
					break
				}

				var output event.Output
				err := json.Unmarshal(se.Data, &output)
				if err != nil {
					logger.Error("failed-to-unarshal-output", err)
					return err
				}

				outputs[output.Output.Name] = vrFromOutput(output.Output)
			}
		}
	}

	return nil
}

type turbineBuild struct {
	guid string

	turbineEndpoint *rata.RequestGenerator
	httpClient      *http.Client
}

func (build *turbineBuild) Guid() string {
	return build.guid
}

func (build *turbineBuild) Abort() error {
	abort, err := build.turbineEndpoint.CreateRequest(
		turbine.AbortBuild,
		rata.Params{"guid": build.guid},
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := build.httpClient.Do(abort)
	if err != nil {
		return err
	}

	resp.Body.Close()

	if resp.StatusCode > 300 {
		return fmt.Errorf("bad response: %s", resp.Status)
	}

	return nil
}

func (build *turbineBuild) Hijack(garden.ProcessSpec, garden.ProcessIO) error {
	// POST /hijack
	return nil
}

func (build *turbineBuild) Subscribe(from uint) (<-chan event.Event, chan<- struct{}, error) {
	// GET /events
	return nil, nil, nil
}

func vrFromInput(input turbine.Input) db.VersionedResource {
	metadata := make([]db.MetadataField, len(input.Metadata))
	for i, md := range input.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return db.VersionedResource{
		Resource: input.Resource,
		Type:     input.Type,
		Source:   db.Source(input.Source),
		Version:  db.Version(input.Version),
		Metadata: metadata,
	}
}

// same as input, but type is different.
//
// :(
func vrFromOutput(output turbine.Output) db.VersionedResource {
	metadata := make([]db.MetadataField, len(output.Metadata))
	for i, md := range output.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return db.VersionedResource{
		Resource: output.Name,
		Type:     output.Type,
		Source:   db.Source(output.Source),
		Version:  db.Version(output.Version),
		Metadata: metadata,
	}
}
