package reaper

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/worker/reaper/api"
)

type ReaperCmd struct {
	GardenAddr string
	Port       string
	Logger     lager.Logger
}

// Run will start up the reaper process on a worker
func (r *ReaperCmd) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	gardenURLParts := strings.Split(r.GardenAddr, ":")
	if len(gardenURLParts) > 2 {
		gardenURLErr := errors.New("URL should be of format IP:Port" + r.GardenAddr)
		r.Logger.Error("failed-to-parse-URL", gardenURLErr)
		return gardenURLErr
	}
	r.Logger.Info("started-reaper-process",
		lager.Data{"garden-addr": r.GardenAddr, "server-port": r.Port})

	gConn := gconn.New("tcp", r.GardenAddr)
	err := gConn.Ping()
	if err != nil {
		r.Logger.Error("failed-to-ping-garden", err)
		return err
	}
	r.Logger.Info("ping-garden-server")
	handler, err := api.NewHandler(r.Logger, gConn)

	if err != nil {
		r.Logger.Error("failed-to-build-handler", err)
		return err
	}
	close(ready)

	s := &http.Server{
		Addr:         fmt.Sprintf(":%s", r.Port),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	exited := make(chan error, 1)

	go func() {
		exited <- s.ListenAndServe()
	}()

	select {
	case <-signals:
		// ignore server closing error
		s.Close()
		return nil
	case err = <-exited:
		r.Logger.Error("failed-to-listen-and-server-reaper-server", err)
		return err
	}
}
