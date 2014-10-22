package scheduler

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/db"
	tbuilds "github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
	"github.com/concourse/turbine/routes"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

type tracker struct {
	logger lager.Logger

	db TrackerDB

	trackingBuilds map[int]bool
	lock           *sync.Mutex
}

type TrackerDB interface {
	AppendBuildEvent(buildID int, event db.BuildEvent) error

	SaveBuildStartTime(buildID int, startTime time.Time) error
	SaveBuildEndTime(buildID int, startTime time.Time) error

	SaveBuildInput(buildID int, vr builds.VersionedResource) error
	SaveBuildOutput(buildID int, vr builds.VersionedResource) error

	SaveBuildStatus(buildID int, status builds.Status) error
}

func NewTracker(logger lager.Logger, db TrackerDB) BuildTracker {
	return &tracker{
		logger: logger,
		db:     db,

		trackingBuilds: make(map[int]bool),
		lock:           new(sync.Mutex),
	}
}

func (tracker *tracker) TrackBuild(build builds.Build) error {
	tLog := tracker.logger.Session("track-build", lager.Data{
		"buld": build.ID,
	})

	alreadyTracking := tracker.markTracking(build.ID)
	if alreadyTracking {
		tLog.Info("already-tracking")
		return nil
	}

	tLog.Info("tracking")

	defer func() {
		tLog.Info("done-tracking")
		tracker.unmarkTracking(build.ID)
	}()

	generator := rata.NewRequestGenerator(build.Endpoint, routes.Routes)

	events, err := generator.CreateRequest(
		routes.GetBuildEvents,
		rata.Params{"guid": build.Guid},
		nil,
	)
	if err != nil {
		tLog.Error("failed-to-create-events-request", err)
		return err
	}

	resp, err := http.DefaultClient.Do(events)
	if err != nil {
		tLog.Error("failed-to-get-events", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		tLog.Info("saving-orphaned-build-as-errored")

		err := tracker.db.SaveBuildStatus(build.ID, builds.StatusErrored)
		if err != nil {
			tLog.Error("failed-to-save-build-as-errored", err)
			return err
		}

		return nil
	}

	reader := sse.NewReader(resp.Body)

	outputs := map[string]builds.VersionedResource{}

	var currentVersion string

	for {
		se, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				tLog.Info("event-stream-completed")
				return nil
			}

			return err
		}

		err = tracker.db.AppendBuildEvent(build.ID, db.BuildEvent{
			Type:    se.Name,
			Payload: string(se.Data),
		})

		if se.Name == "version" {
			var version event.Version
			err := json.Unmarshal(se.Data, &version)
			if err != nil {
				tLog.Error("failed-to-unmarshal-version", err)
				return err
			}

			tLog.Info("event-stream-version", lager.Data{
				"version": string(version),
			})

			currentVersion = string(version)
			continue
		}

		if se.Name == "end" {
			tLog.Info("event-stream-ended")

			del, err := generator.CreateRequest(
				routes.DeleteBuild,
				rata.Params{"guid": build.Guid},
				nil,
			)
			if err != nil {
				tLog.Error("failed-to-create-delete-request", err)
				return err
			}

			resp, err := http.DefaultClient.Do(del)
			if err != nil {
				tLog.Error("failed-to-delete-build", err)
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
				tLog.Info("processing-build-status", lager.Data{
					"event": string(se.Data),
				})

				var status event.Status
				err := json.Unmarshal(se.Data, &status)
				if err != nil {
					tLog.Error("failed-to-unmarshal-status", err)
					return err
				}

				if status.Status == tbuilds.StatusStarted {
					err = tracker.db.SaveBuildStartTime(build.ID, time.Unix(status.Time, 0))
					if err != nil {
						tLog.Error("failed-to-save-build-start-time", err)
						return err
					}
				} else {
					err = tracker.db.SaveBuildEndTime(build.ID, time.Unix(status.Time, 0))
					if err != nil {
						tLog.Error("failed-to-save-build-end-time", err)
						return err
					}
				}

				err = tracker.db.SaveBuildStatus(build.ID, builds.Status(status.Status))
				if err != nil {
					tLog.Error("failed-to-save-build-status", err)
					return err
				}

				if status.Status == tbuilds.StatusSucceeded {
					for _, output := range outputs {
						err := tracker.db.SaveBuildOutput(build.ID, output)
						if err != nil {
							tLog.Error("failed-to-save-build-output", err)
							return err
						}
					}
				}

			case "input":
				if build.JobName == "" {
					tLog.Info("ignoring-build-input-for-one-off")
					break
				}

				tLog.Info("processing-build-input", lager.Data{
					"event": string(se.Data),
				})

				var input event.Input
				err := json.Unmarshal(se.Data, &input)
				if err != nil {
					tLog.Error("failed-to-unarshal-input", err)
					return err
				}

				err = tracker.db.SaveBuildInput(build.ID, vrFromInput(input.Input))
				if err != nil {
					tLog.Error("failed-to-save-build-input", err)
					return err
				}

				// record implicit output
				outputs[input.Input.Resource] = vrFromInput(input.Input)

			case "output":
				if build.JobName == "" {
					tLog.Info("ignoring-build-output-for-one-off")
					break
				}

				var output event.Output
				err := json.Unmarshal(se.Data, &output)
				if err != nil {
					tLog.Error("failed-to-unarshal-output", err)
					return err
				}

				outputs[output.Output.Name] = vrFromOutput(output.Output)
			}
		}
	}

	return nil
}

func (tracker *tracker) markTracking(buildID int) bool {
	tracker.lock.Lock()
	alreadyTracking, found := tracker.trackingBuilds[buildID]
	if !found {
		tracker.trackingBuilds[buildID] = true
	}
	tracker.lock.Unlock()

	return alreadyTracking
}

func (tracker *tracker) unmarkTracking(buildID int) {
	tracker.lock.Lock()
	delete(tracker.trackingBuilds, buildID)
	tracker.lock.Unlock()
}

func vrFromInput(input tbuilds.Input) builds.VersionedResource {
	metadata := make([]builds.MetadataField, len(input.Metadata))
	for i, md := range input.Metadata {
		metadata[i] = builds.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return builds.VersionedResource{
		Name:     input.Resource,
		Type:     input.Type,
		Source:   builds.Source(input.Source),
		Version:  builds.Version(input.Version),
		Metadata: metadata,
	}
}

// same as input, but type is different.
//
// :(
func vrFromOutput(output tbuilds.Output) builds.VersionedResource {
	metadata := make([]builds.MetadataField, len(output.Metadata))
	for i, md := range output.Metadata {
		metadata[i] = builds.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return builds.VersionedResource{
		Name:     output.Name,
		Type:     output.Type,
		Source:   builds.Source(output.Source),
		Version:  builds.Version(output.Version),
		Metadata: metadata,
	}
}
