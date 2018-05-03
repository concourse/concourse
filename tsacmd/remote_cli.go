package tsacmd

import (
	"flag"
	"fmt"
	"strings"
)

type request interface{}

type registerWorkerRequest struct{}

type landWorkerRequest struct{}
type retireWorkerRequest struct{}
type deleteWorkerRequest struct{}
type sweepContainerRequest struct{}
type reportContainerRequest struct {
	containerHandles []string
}

func (r reportContainerRequest) handles() []string {
	return r.containerHandles
}

type forwardWorkerRequest struct {
	gardenAddr       string
	baggageclaimAddr string
	reaperAddr       string
}

func (r forwardWorkerRequest) expectedForwards() int {
	expected := 0

	// Garden should always be forwarded;
	// if not explicitly given, the only given forward is used
	expected++

	if r.baggageclaimAddr != "" {
		expected++
	}
	if r.reaperAddr != "" {
		expected++
	}

	return expected
}

func parseRequest(cli string) (request, error) {
	argv := strings.Split(cli, " ")

	command := argv[0]
	args := argv[1:]

	switch command {
	case "register-worker":
		return registerWorkerRequest{}, nil
	case "forward-worker":
		var fs = flag.NewFlagSet(command, flag.ContinueOnError)

		var garden = fs.String("garden", "", "garden address to forward")
		var baggageclaim = fs.String("baggageclaim", "", "baggageclaim address to forward")
		var reaper = fs.String("reaper", "", "reaper address to forward")

		err := fs.Parse(args)
		if err != nil {
			return nil, err
		}

		return forwardWorkerRequest{
			gardenAddr:       *garden,
			baggageclaimAddr: *baggageclaim,
			reaperAddr:       *reaper,
		}, nil
	case "land-worker":
		return landWorkerRequest{}, nil
	case "retire-worker":
		return retireWorkerRequest{}, nil
	case "delete-worker":
		return deleteWorkerRequest{}, nil
	case "sweep-containers":
		return sweepContainerRequest{}, nil
	case "report-containers":
		return reportContainerRequest{
			containerHandles: args,
		}, nil
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}
