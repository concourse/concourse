package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/flag"
	"github.com/concourse/reaper/api"
)

// ReaperCmd is the command structure for the reaper process
type ReaperCmd struct {
	// garden URL
	GardenAddr string `long:"garden-addr"`
	Port       string `long:"port" default:"8888"`
	Logger     flag.Lager
}

// Run will start up the reaper process on a worker
func (r *ReaperCmd) Execute() {
	rLogger, _ := r.Logger.Logger("reaper-server")

	gardenURL, err := url.Parse(r.GardenAddr)
	if err != nil {
		rLogger.Error("failed-to-parse-URL", err)
		os.Exit(1)
	}
	rLogger.Info("started-reaper-process",
		lager.Data{"garden-host": gardenURL.Hostname(),
			"port": r.Port})

	gConn := gconn.New("tcp", gardenURL.Host)
	err = gConn.Ping()
	if err != nil {
		rLogger.Error("failed-to-ping-garden", err)
		os.Exit(1)
	}

	handler, err := api.NewHandler(rLogger, gConn)

	if err != nil {
		rLogger.Error("failed-to-build-handler", err)
		os.Exit(1)
	}

	s := &http.Server{
		Addr:         fmt.Sprintf(":%s", r.Port),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	s.ListenAndServe()
}
