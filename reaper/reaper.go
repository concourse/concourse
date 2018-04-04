package reaper

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
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
	gardenURL, err := url.Parse(r.GardenAddr)
	if err != nil {
		r.Logger.Error("failed-to-parse-URL", err)
		return err
	}
	r.Logger.Info("started-reaper-process",
		lager.Data{"garden-host": gardenURL.Host, "port": r.Port})

	gConn := gconn.New("tcp", gardenURL.Host)
	err = gConn.Ping()
	if err != nil {
		r.Logger.Error("failed-to-ping-garden", err)
		return err
	}

	close(ready)
	handler, err := api.NewHandler(r.Logger, gConn)

	if err != nil {
		r.Logger.Error("failed-to-build-handler", err)
		return err
	}

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
		return err
	}
}
