package logs

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/logfanout"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

func NewHandler(
	logger lager.Logger,
	validator auth.Validator,
	jobs config.Jobs,
	tracker *logfanout.Tracker,
	db db.DB,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildIDStr := r.FormValue(":build_id")

		log := logger.Session("logs-out", lager.Data{
			"build_id": buildIDStr,
		})

		buildID, err := strconv.Atoi(buildIDStr)
		if err != nil {
			log.Error("invalid-build-id", err)
			return
		}

		authenticated := validator.IsAuthenticated(r)

		if !authenticated {
			build, err := db.GetBuild(buildID)
			if err != nil {
				log.Error("invalid-build-id", err)
				w.WriteHeader(http.StatusNotFound)
				return
			}

			job, found := jobs.Lookup(build.JobName)
			if !found || !job.Public {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("upgrade-failed", err)
			return
		}

		defer conn.Close()

		logFanout := tracker.Register(buildID, conn)
		defer tracker.Unregister(buildID, conn)

		var sink logfanout.Sink
		if authenticated {
			sink = logfanout.NewRawSink(conn)
		} else {
			sink = logfanout.NewCensoredSink(conn)
		}

		err = logFanout.Attach(sink)
		if err != nil {
			log.Error("attach-failed", err)
			conn.Close()
			return
		}

		for {
			time.Sleep(5 * time.Second)

			err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Time{})
			if err != nil {
				log.Error("ping-failed", err)
				break
			}
		}
	})
}
