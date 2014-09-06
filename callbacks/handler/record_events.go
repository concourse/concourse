package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/pivotal-golang/lager"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

func (handler *Handler) RecordEvents(w http.ResponseWriter, r *http.Request) {
	job := r.FormValue(":job")
	idStr := r.FormValue(":build")

	log := handler.logger.Session("logs-in", lager.Data{
		"job": job,
		"id":  idStr,
	})

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("upgrade-failed", err)
		return
	}

	defer conn.Close()

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error("invalid-build-id", err)
		conn.Close()
		return
	}

	log.Debug("streaming")

	logFanout := handler.tracker.Register(job, id, conn)
	defer handler.tracker.Unregister(job, id, conn)

	defer logFanout.Close()

	for {
		var msg *json.RawMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Error("message-read-error", err)
			return
		}

		err = logFanout.WriteMessage(msg)
		if err != nil {
			log.Error("message-write-error", err)
			return
		}
	}
}
