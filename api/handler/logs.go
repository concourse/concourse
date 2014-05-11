package handler

import (
	"io"
	"log"
	"strconv"

	"code.google.com/p/go.net/websocket"

	"github.com/winston-ci/winston/ansistream"
	"github.com/winston-ci/winston/logbuffer"
)

func (handler *Handler) LogInput(conn *websocket.Conn) {
	job := conn.Request().FormValue(":job")
	idStr := conn.Request().FormValue(":build")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Println("error parsing build id:", err)
		conn.Close()
		return
	}

	handler.logsMutex.Lock()
	logBuffer, found := handler.logs[job+"-"+idStr]
	if !found {
		logBuffer = logbuffer.NewLogBuffer()
		handler.logs[job+"-"+idStr] = logBuffer
	}
	handler.logsMutex.Unlock()

	defer conn.Close()
	defer logBuffer.Close()

	_, err = io.Copy(logBuffer, conn)
	if err != nil {
		log.Println("error reading message:", err)
		return
	}

	err = handler.db.SaveBuildLog(job, id, logBuffer.Content())
	if err != nil {
		log.Println("failed saving build log:", err)
		return
	}
}

func (handler *Handler) LogOutput(conn *websocket.Conn) {
	job := conn.Request().FormValue(":job")
	idStr := conn.Request().FormValue(":build")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Println("error parsing build id:", err)
		conn.Close()
		return
	}

	logs, err := handler.db.BuildLog(job, id)
	if err == nil {
		ansiWriter := ansistream.NewWriter(conn)
		ansiWriter.Write(logs)
		ansiWriter.Close()
		return
	}

	handler.logsMutex.Lock()
	logBuffer, found := handler.logs[job+"-"+idStr]
	if !found {
		logBuffer = logbuffer.NewLogBuffer()
		handler.logs[job+"-"+idStr] = logBuffer
	}
	handler.logsMutex.Unlock()

	logBuffer.Attach(conn)
}
