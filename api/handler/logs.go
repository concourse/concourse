package handler

import (
	"io"
	"strconv"

	"code.google.com/p/go.net/websocket"

	"github.com/concourse/atc/ansistream"
	"github.com/concourse/atc/logfanout"
	"github.com/concourse/atc/utf8stream"
)

func (handler *Handler) LogInput(conn *websocket.Conn) {
	handler.drain.Add(conn)
	defer handler.drain.Remove(conn)

	log := handler.logger.Session("logs-in")

	job := conn.Request().FormValue(":job")
	idStr := conn.Request().FormValue(":build")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error("invalid-build-id", err)
		conn.Close()
		return
	}

	handler.logsMutex.Lock()
	logFanout, found := handler.logs[job+"-"+idStr]
	if !found {
		logFanout = logfanout.NewLogFanout(job, id, handler.db)
		handler.drain.Add(logFanout)
		handler.logs[job+"-"+idStr] = logFanout
	}
	handler.logsMutex.Unlock()

	defer conn.Close()
	defer logFanout.Close()

	_, err = io.Copy(logFanout, conn)
	if err != nil {
		log.Error("message-read-error", err)
		return
	}
}

func (handler *Handler) LogOutput(conn *websocket.Conn) {
	handler.drain.Add(conn)
	defer handler.drain.Remove(conn)

	job := conn.Request().FormValue(":job")
	idStr := conn.Request().FormValue(":build")

	log := handler.logger.Session("logs-out")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error("invalid-build-id", err)
		conn.Close()
		return
	}

	logWriter := utf8stream.NewWriter(ansistream.NewWriter(conn))

	handler.logsMutex.Lock()
	logFanout, found := handler.logs[job+"-"+idStr]
	if !found {
		logFanout = logfanout.NewLogFanout(job, id, handler.db)
		handler.drain.Add(logFanout)
		handler.logs[job+"-"+idStr] = logFanout
	}
	handler.logsMutex.Unlock()

	err = logFanout.Attach(logWriter)
	if err != nil {
		log.Error("attach-failed", err)
		conn.Close()
		return
	}
}
