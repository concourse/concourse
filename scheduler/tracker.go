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
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

type tracker struct {
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

func NewTracker(db TrackerDB) BuildTracker {
	return &tracker{
		db: db,

		trackingBuilds: make(map[int]bool),
		lock:           new(sync.Mutex),
	}
}

func (tracker *tracker) TrackBuild(build builds.Build) error {
	alreadyTracking := tracker.markTracking(build.ID)
	if alreadyTracking {
		return nil
	}

	defer tracker.unmarkTracking(build.ID)

	generator := rata.NewRequestGenerator(build.Endpoint, routes.Routes)

	events, err := generator.CreateRequest(
		routes.GetBuildEvents,
		rata.Params{"guid": build.Guid},
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(events)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		return tracker.db.SaveBuildStatus(build.ID, builds.StatusErrored)
	}

	reader := sse.NewReader(resp.Body)

	var currentVersion string

	for {
		se, err := reader.Next()
		if err != nil {
			if err == io.EOF {
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
				return err
			}

			currentVersion = string(version)
			continue
		}

		if se.Name == "end" {
			del, err := generator.CreateRequest(
				routes.DeleteBuild,
				rata.Params{"guid": build.Guid},
				nil,
			)
			if err != nil {
				return err
			}

			resp, err := http.DefaultClient.Do(del)
			if err != nil {
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
				var status event.Status
				err := json.Unmarshal(se.Data, &status)
				if err != nil {
					return err
				}

				if status.Status == tbuilds.StatusStarted {
					err = tracker.db.SaveBuildStartTime(build.ID, time.Unix(status.Time, 0))
					if err != nil {
						return err
					}
				} else {
					err = tracker.db.SaveBuildEndTime(build.ID, time.Unix(status.Time, 0))
					if err != nil {
						return err
					}
				}

				err = tracker.db.SaveBuildStatus(build.ID, builds.Status(status.Status))
				if err != nil {
					return err
				}

			case "input":
				if build.JobName == "" {
					// one-off build; don't bother saving inputs
					break
				}

				var input event.Input
				err := json.Unmarshal(se.Data, &input)
				if err != nil {
					return err
				}

				err = tracker.db.SaveBuildInput(build.ID, vrFromInput(input.Input))
				if err != nil {
					return err
				}

			case "output":
				if build.JobName == "" {
					// one-off build; don't bother saving outputs
					break
				}

				var output event.Output
				err := json.Unmarshal(se.Data, &output)
				if err != nil {
					return err
				}

				err = tracker.db.SaveBuildOutput(build.ID, vrFromOutput(output.Output))
				if err != nil {
					return err
				}
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
		Name:     input.Name,
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
