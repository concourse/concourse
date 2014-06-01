package handler

import (
	"io"
	"log"
	"strconv"

	"code.google.com/p/go.net/websocket"

	"github.com/winston-ci/logbuffer"
	"github.com/winston-ci/winston/ansistream"
	"github.com/winston-ci/winston/utf8stream"
)

func (handler *Handler) LogInput(conn *websocket.Conn) {
	handler.drain.Add(conn)
	defer handler.drain.Remove(conn)

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
		handler.drain.Add(logBuffer)
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
	handler.drain.Add(conn)
	defer handler.drain.Remove(conn)

	job := conn.Request().FormValue(":job")
	idStr := conn.Request().FormValue(":build")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Println("error parsing build id:", err)
		conn.Close()
		return
	}

	logWriter := utf8stream.NewWriter(ansistream.NewWriter(conn))

	logs, err := handler.db.BuildLog(job, id)
	if err == nil {
		logWriter.Write(logs)
		logWriter.Close()
		return
	}

	handler.logsMutex.Lock()
	logBuffer, found := handler.logs[job+"-"+idStr]
	if !found {
		logBuffer = logbuffer.NewLogBuffer()
		handler.drain.Add(logBuffer)
		handler.logs[job+"-"+idStr] = logBuffer
	}
	handler.logsMutex.Unlock()

	logBuffer.Attach(logWriter)
}
