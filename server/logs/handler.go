package logs

import (
	"strconv"
	"time"

	"code.google.com/p/go.net/websocket"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/ansistream"
	"github.com/concourse/atc/deadlinewriter"

	"github.com/concourse/atc/limitedstream"
	"github.com/concourse/atc/logfanout"
	"github.com/concourse/atc/utf8stream"
)

func NewHandler(logger lager.Logger, tracker *logfanout.Tracker) websocket.Handler {
	return func(conn *websocket.Conn) {
		job := conn.Request().FormValue(":job")
		idStr := conn.Request().FormValue(":build")

		log := logger.Session("logs-out")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.Error("invalid-build-id", err)
			conn.Close()
			return
		}

		timeoutWriter := deadlinewriter.TimeoutWriter{
			DeadlineWriter: conn,
			Timeout:        5 * time.Second,
		}

		logWriter := limitedstream.Writer{
			Limit:       1024,
			WriteCloser: utf8stream.NewWriter(ansistream.NewWriter(timeoutWriter)),
		}

		logFanout := tracker.Register(job, id, conn)
		defer tracker.Unregister(job, id, conn)

		defer conn.Close()

		err = logFanout.Attach(logWriter)
		if err != nil {
			log.Error("attach-failed", err)
			conn.Close()
			return
		}
	}
}
