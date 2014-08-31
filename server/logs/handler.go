package logs

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/logfanout"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

func NewHandler(logger lager.Logger, tracker *logfanout.Tracker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		job := r.FormValue(":job")
		idStr := r.FormValue(":build")

		log := logger.Session("logs-out")

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("upgrade-failed", err)
			return
		}

		defer conn.Close()

		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.Error("invalid-build-id", err)
			return
		}

		logFanout := tracker.Register(job, id, conn)
		defer tracker.Unregister(job, id, conn)

		err = logFanout.Attach(conn)
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
