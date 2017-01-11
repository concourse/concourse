package cessna

import (
	"net/http"

	"code.cloudfoundry.org/garden"
	gclient "code.cloudfoundry.org/garden/client"
	"github.com/concourse/baggageclaim"
	bclient "github.com/concourse/baggageclaim/client"

	"code.cloudfoundry.org/garden/client/connection"
)

func NewWorker(gardenAddr string, baggageclaimAddr string) *Worker {
	return &Worker{
		gardenAddr:       gardenAddr,
		baggageclaimAddr: baggageclaimAddr,
	}
}

type Worker struct {
	gardenAddr       string
	baggageclaimAddr string
}

func (w *Worker) GardenClient() garden.Client {
	return gclient.New(connection.New("tcp", w.gardenAddr))
}

func (w *Worker) BaggageClaimClient() baggageclaim.Client {
	return bclient.New(w.baggageclaimAddr, http.DefaultTransport)
}
