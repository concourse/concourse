package handler

import (
	"io"
	"strconv"

	"github.com/concourse/atc/utf8stream"
	"github.com/pivotal-golang/lager"

	"code.google.com/p/go.net/websocket"
)

func (handler *Handler) LogInput(conn *websocket.Conn) {
	job := conn.Request().FormValue(":job")
	idStr := conn.Request().FormValue(":build")

	log := handler.logger.Session("logs-in", lager.Data{
		"job": job,
		"id":  idStr,
	})

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error("invalid-build-id", err)
		conn.Close()
		return
	}

	log.Debug("streaming")

	logFanout := handler.tracker.Register(job, id, conn)
	defer handler.tracker.Unregister(job, id, conn)

	defer conn.Close()
	defer logFanout.Close()

	_, err = io.Copy(utf8stream.NewWriter(logFanout), conn)
	if err != nil {
		log.Error("message-read-error", err)
		return
	}
}
