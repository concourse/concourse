package jobserver

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/stream"
	"github.com/concourse/concourse/atc/db/watch"
)

//go:generate counterfeiter . ListAllJobsWatcher

type ListAllJobsWatcher interface {
	WatchListAllJobs(ctx context.Context) (<-chan []watch.JobSummaryEvent, error)
}

func (s *Server) ListAllJobs(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-all-jobs")

	acc := accessor.GetAccessor(r)

	watchMode := stream.IsRequested(r)
	var watchEventsChan <-chan []watch.JobSummaryEvent
	if watchMode {
		var err error
		watchEventsChan, err = s.listAllJobsWatcher.WatchListAllJobs(r.Context())
		if err == watch.ErrDisabled {
			http.Error(w, "ListAllJobs watch endpoint is not enabled", http.StatusNotAcceptable)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var jobs []atc.JobSummary
	var err error
	if acc.IsAdmin() {
		jobs, err = s.jobFactory.AllActiveJobs()
	} else {
		jobs, err = s.jobFactory.VisibleJobs(acc.TeamNames())
	}
	if err != nil {
		logger.Error("failed-to-get-all-visible-jobs", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if jobs == nil {
		jobs = []atc.JobSummary{}
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
			var visibleEvents []watch.JobSummaryEvent
			for _, event := range events {
				if hasAccessTo(event, acc) {
					visibleEvents = append(visibleEvents, event)
				}
			}
			if len(visibleEvents) == 0 {
				continue
			}
			if err := writer.WriteEvent(eventID, "patch", visibleEvents); err != nil {
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

func hasAccessTo(event watch.JobSummaryEvent, access accessor.Access) bool {
	if event.Job == nil {
		// this means we always send DELETE events, even if the user didn't have access to the deleted pipeline.
		// given that there's no sensitive information (just the id, which is serial anyway), I suspect this is okay
		return true
	}
	if event.Job.PipelinePublic {
		return true
	}
	return access.IsAuthorized(event.Job.TeamName)
}
