package handler

import (
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/winston-ci/winston/logbuffer"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// allow all connections
		return true
	},
}

func (handler *Handler) LogInput(w http.ResponseWriter, r *http.Request) {
	job := r.FormValue(":job")
	idStr := r.FormValue(":build")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Println("error parsing build id:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	handler.logsMutex.Lock()
	logBuffer, found := handler.logs[job+"-"+idStr]
	if !found {
		logBuffer = logbuffer.NewLogBuffer()
		handler.logs[job+"-"+idStr] = logBuffer
	}
	handler.logsMutex.Unlock()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Println("error reading message:", err)
			break
		}

		logBuffer.Write(msg)
	}

	conn.Close()
	logBuffer.Close()

	err = handler.db.SaveBuildLog(job, id, logBuffer.Content())
	if err != nil {
		log.Println("failed saving build log:", err)
		return
	}
}

func (handler *Handler) LogOutput(w http.ResponseWriter, r *http.Request) {
	job := r.FormValue(":job")
	idStr := r.FormValue(":build")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Println("error parsing build id:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("failed to upgrade connection:", err)
		return
	}

	logs, err := handler.db.BuildLog(job, id)
	if err == nil {
		conn.WriteMessage(websocket.BinaryMessage, logs)
		conn.WriteControl(websocket.CloseMessage, nil, time.Time{})
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

	logBuffer.Attach(conn)
}
