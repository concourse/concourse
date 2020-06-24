package jobserver

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/api/stream"
	"github.com/concourse/concourse/atc/db/watch"
)

//go:generate counterfeiter . ListAllJobsWatcher

type ListAllJobsWatcher interface {
	WatchListAllJobs(ctx context.Context, access accessor.Access) <-chan []watch.ListAllJobsEvent
}

type listAllJobsWatchEvent struct {
	ID   int             `json:"id"`
	Type watch.EventType `json:"eventType"`
	Job  *atc.Job        `json:"job"`
}

func (s *Server) ListAllJobs(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-all-jobs")

	acc := accessor.GetAccessor(r)

	watchMode := stream.IsRequested(r)
	var watchEventsChan <-chan []watch.ListAllJobsEvent
	if watchMode {
		watchEventsChan = s.listAllJobsWatcher.WatchListAllJobs(r.Context(), acc)
	}

	var dashboard atc.Dashboard
	var err error

	if acc.IsAdmin() {
		dashboard, err = s.jobFactory.AllActiveJobs()
	} else {
		dashboard, err = s.jobFactory.VisibleJobs(acc.TeamNames())
	}

	if err != nil {
		logger.Error("failed-to-get-all-visible-jobs", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	jobs := make([]atc.Job, len(dashboard))
	for i, job := range dashboard {
		jobs[i] = present.DashboardJob(job.TeamName, job)
	}

	if watchMode {
		stream.WriteHeaders(w)
		writer := stream.EventWriter{WriteFlusher: w.(stream.WriteFlusher)}

		var eventID uint = 0
		if err := writer.WriteEvent(eventID, "initial", jobs); err != nil {
			logger.Error("failed-to-write-initial-event", err)
			return
		}
		eventID++
		for events := range watchEventsChan {
			if err := writer.WriteEvent(eventID, "patch", presentEvents(events)); err != nil {
				logger.Error("failed-to-write-patch-event", err)
				return
			}
			eventID++
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(jobs)
		if err != nil {
			logger.Error("failed-to-encode-jobs", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func presentEvents(events []watch.ListAllJobsEvent) []listAllJobsWatchEvent {
	presentEvents := make([]listAllJobsWatchEvent, len(events))
	for i, event := range events {
		presentEvent := listAllJobsWatchEvent{
			ID:   event.ID,
			Type: event.Type,
		}
		if event.Job != nil {
			j := present.DashboardJob(event.Job.TeamName, *event.Job)
			presentEvent.Job = &j
		}
		presentEvents[i] = presentEvent
	}
	return presentEvents
}
