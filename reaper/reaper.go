package reaper

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	gardenClient "code.cloudfoundry.org/garden/client"
	gardenConnection "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/worker/reaper/api"
)

type ReaperCmd struct {
	GardenAddr string
	Port       string
	Logger     lager.Logger
}

// Run will start up the reaper process on a worker
// The reaper process on the worker allows for bulk removal
// of containers on the worker by calling the local garden server on behalf of the client
func (reaperCmd *ReaperCmd) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	gardenURLParts := strings.Split(reaperCmd.GardenAddr, ":")
	if len(gardenURLParts) > 2 {
		gardenURLErr := errors.New("URL should be of format IP:Port" + reaperCmd.GardenAddr)
		reaperCmd.Logger.Error("failed-to-parse-URL", gardenURLErr)
		return gardenURLErr
	}
	reaperCmd.Logger.Info("started-reaper-process",
		lager.Data{"garden-addr": reaperCmd.GardenAddr, "server-port": reaperCmd.Port})

	gardenClnt := gardenClient.New(gardenConnection.New("tcp", reaperCmd.GardenAddr))
	err := gardenClnt.Ping()
	if err != nil {
		reaperCmd.Logger.Error("failed-to-ping-garden", err)
		return err
	}
	reaperCmd.Logger.Info("ping-garden-server")
	handler, err := api.NewHandler(reaperCmd.Logger, gardenClnt)

	if err != nil {
		reaperCmd.Logger.Error("failed-to-build-handler", err)
		return err
	}
	close(ready)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", reaperCmd.Port),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	exited := make(chan error, 1)

	go func() {
		exited <- server.ListenAndServe()
	}()

	select {
	case <-signals:
		// ignore server closing error
		server.Close()
		return nil
	case err = <-exited:
		reaperCmd.Logger.Error("failed-to-listen-and-server-reaper-server", err)
		return err
	}
}
