package scheduler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/turbine"
	"github.com/concourse/turbine/event"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

type tracker struct {
	logger lager.Logger

	db     TrackerDB
	locker Locker

	trackingBuilds map[int]bool
	lock           *sync.Mutex
}

type TrackerDB interface {
	GetLastBuildEventID(buildID int) (int, error)
	SaveBuildEvent(buildID int, event db.BuildEvent) error

	SaveBuildStartTime(buildID int, startTime time.Time) error
	SaveBuildEndTime(buildID int, startTime time.Time) error

	SaveBuildInput(buildID int, vr db.VersionedResource) error
	SaveBuildOutput(buildID int, vr db.VersionedResource) error

	SaveBuildStatus(buildID int, status db.Status) error
}

func NewTracker(logger lager.Logger, db TrackerDB, locker Locker) BuildTracker {
	return &tracker{
		logger: logger,
		db:     db,
		locker: locker,

		trackingBuilds: make(map[int]bool),
		lock:           new(sync.Mutex),
	}
}

func (tracker *tracker) TrackBuild(build db.Build) error {
	lock, err := tracker.locker.AcquireWriteLockImmediately([]db.NamedLock{db.BuildTrackingLock(build.Guid)})
	if err != nil {
		return nil
	}
	defer lock.Release()

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

	generator := rata.NewRequestGenerator(build.Endpoint, turbine.Routes)

	events, err := generator.CreateRequest(
		turbine.GetBuildEvents,
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

		err := tracker.db.SaveBuildStatus(build.ID, db.StatusErrored)
		if err != nil {
			tLog.Error("failed-to-save-build-as-errored", err)
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
				tLog.Info("event-stream-completed")
				return nil
			}

			return err
		}

		id, err := strconv.Atoi(se.ID)
		if err != nil {
			tLog.Error("non-numerical-event-id", err, lager.Data{
				"id": se.ID,
			})

			return err
		}

		err = tracker.db.SaveBuildEvent(build.ID, db.BuildEvent{
			ID:      id,
			Type:    se.Name,
			Payload: string(se.Data),
		})
		if err != nil {
			tLog.Error("failed-to-save-build-event", err, lager.Data{
				"event": se,
			})

			return err
		}

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
				turbine.DeleteBuild,
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

				if status.Status == turbine.StatusStarted {
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

				err = tracker.db.SaveBuildStatus(build.ID, db.Status(status.Status))
				if err != nil {
					tLog.Error("failed-to-save-build-status", err)
					return err
				}

				if status.Status == turbine.StatusSucceeded {
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

				if input.Input.Resource == "" {
					input.Input.Resource = input.Input.Name
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

func vrFromInput(input turbine.Input) db.VersionedResource {
	metadata := make([]db.MetadataField, len(input.Metadata))
	for i, md := range input.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return db.VersionedResource{
		Name:     input.Resource,
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
		Name:     output.Name,
		Type:     output.Type,
		Source:   db.Source(output.Source),
		Version:  db.Version(output.Version),
		Metadata: metadata,
	}
}
