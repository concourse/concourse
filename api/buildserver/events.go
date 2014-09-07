package buildserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pivotal-golang/lager"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

func (s *Server) BuildEvents(w http.ResponseWriter, r *http.Request) {
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	eLog := s.logger.Session("events")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		eLog.Error("upgrade-failed", err)
		return
	}

	defer conn.Close()

	logFanout := s.tracker.Register(buildID, conn)
	defer s.tracker.Unregister(buildID, conn)

	err = logFanout.Attach(conn)
	if err != nil {
		eLog.Error("attach-failed", err)
		conn.Close()
		return
	}

	for {
		time.Sleep(s.pingInterval)

		err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Time{})
		if err != nil {
			eLog.Debug("ping-failed", lager.Data{"error": err.Error()})
			break
		}
	}
}
