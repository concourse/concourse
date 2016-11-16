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

type forwardWorkerRequest struct {
	gardenAddr       string
	baggageclaimAddr string
}

func (r forwardWorkerRequest) expectedForwards() int {
	expected := 0

	// Garden should always be forwarded;
	// if not explicitly given, the only given forward is used
	expected++

	if r.baggageclaimAddr != "" {
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
		var baggageclaim = fs.String("baggageclaim", "", "garden address to forward")

		err := fs.Parse(args)
		if err != nil {
			return nil, err
		}

		return forwardWorkerRequest{
			gardenAddr:       *garden,
			baggageclaimAddr: *baggageclaim,
		}, nil
	case "land-worker":
		return landWorkerRequest{}, nil
	case "retire-worker":
		return retireWorkerRequest{}, nil
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}
