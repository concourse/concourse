package handler

import (
	"log"
	"net/http"

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
	id := r.FormValue(":build")

	handler.logsMutex.Lock()
	logBuffer, found := handler.logs[job+"-"+id]
	if !found {
		logBuffer = logbuffer.NewLogBuffer()
		handler.logs[job+"-"+id] = logBuffer
	}
	handler.logsMutex.Unlock()

	//if !found {
	//w.WriteHeader(http.StatusNotFound)
	//return
	//}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("error reading message:", err)
			break
		}

		logBuffer.Write(msg)
	}

	conn.Close()
}

func (handler *Handler) LogOutput(w http.ResponseWriter, r *http.Request) {
	job := r.FormValue(":job")
	id := r.FormValue(":build")

	handler.logsMutex.Lock()
	logBuffer, found := handler.logs[job+"-"+id]
	if !found {
		logBuffer = logbuffer.NewLogBuffer()
		handler.logs[job+"-"+id] = logBuffer
	}
	handler.logsMutex.Unlock()

	//if !found {
	//w.WriteHeader(http.StatusNotFound)
	//return
	//}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	logBuffer.Attach(conn)
}
