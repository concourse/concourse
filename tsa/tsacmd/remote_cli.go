package tsacmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/tsa"
	"golang.org/x/crypto/ssh"
)

type request interface {
	Handle(context.Context, ConnState, ssh.Channel) error
}

type forwardWorkerRequest struct {
	server *server

	gardenAddr       string
	baggageclaimAddr string
}

func (req forwardWorkerRequest) Handle(ctx context.Context, state ConnState, channel ssh.Channel) error {
	logger := lagerctx.FromContext(ctx)

	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return err
	}

	if err := checkTeam(state, worker); err != nil {
		return err
	}

	forwards := map[string]ForwardedTCPIP{}
	for i := 0; i < 2; i++ {
		select {
		case forwarded := <-state.ForwardedTCPIPs:
			logger.Info("forwarded-tcpip", lager.Data{
				"bind-addr":  forwarded.BindAddr,
				"bound-port": forwarded.BoundPort,
			})

			forwards[forwarded.BindAddr] = forwarded

		case <-time.After(10 * time.Second):
			logger.Info("never-forwarded-tcpip")
		}
	}

	gardenForward, found := forwards[req.gardenAddr]
	if !found {
		return fmt.Errorf("garden address (%s) not forwarded", req.gardenAddr)
	}

	baggageclaimForward, found := forwards[req.baggageclaimAddr]
	if !found {
		return fmt.Errorf("baggageclaim address (%s) not forwarded", req.baggageclaimAddr)
	}

	worker.GardenAddr = fmt.Sprintf("%s:%d", req.server.forwardHost, gardenForward.BoundPort)
	worker.BaggageclaimURL = fmt.Sprintf("http://%s:%d", req.server.forwardHost, baggageclaimForward.BoundPort)

	heartbeater := tsa.NewHeartbeater(
		clock.NewClock(),
		req.server.heartbeatInterval,
		req.server.cprInterval,
		gclient.BasicGardenClientWithRequestTimeout(
			lagerctx.WithSession(ctx, "garden-connection"),
			req.server.gardenRequestTimeout,
			gardenURL(worker.GardenAddr),
		),
		bclient.NewWithHTTPClient(worker.BaggageclaimURL, &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives:     true,
				ResponseHeaderTimeout: 1 * time.Minute,
			},
		}),
		req.server.atcEndpointPicker,
		req.server.httpClient,
		worker,
		tsa.NewEventWriter(channel),
	)

	err = heartbeater.Heartbeat(ctx)
	if err != nil {
		logger.Error("failed-to-heartbeat", err)
		return err
	}

	for _, forward := range forwards {
		// prevent new connections from being accepted
		close(forward.Drain)
	}

	// only drain if heartbeating was interrupted; otherwise the worker landed or
	// retired, so it's time to go away
	if ctx.Err() != nil {
		logger.Info("draining-forwarded-connections")

		for _, forward := range forwards {
			// wait for connections to drain
			forward.Wait()

			logger.Info("forward-process-exited", lager.Data{
				"bind-addr":  forward.BindAddr,
				"bound-port": forward.BoundPort,
			})
		}
	}

	return nil
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

type landWorkerRequest struct {
	server *server
}

func checkTeam(state ConnState, worker atc.Worker) error {
	if state.Team == "" {
		// global keys can be used for all teams
		return nil
	}

	if worker.Team == "" && state.Team != "" {
		return fmt.Errorf("key is authorized for team %s, but worker is global", state.Team)
	}

	if worker.Team != state.Team {
		return fmt.Errorf("key is authorized for team %s, but worker belongs to team %s", state.Team, worker.Team)
	}

	return nil
}

func (req landWorkerRequest) Handle(ctx context.Context, state ConnState, channel ssh.Channel) error {
	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return err
	}

	if err := checkTeam(state, worker); err != nil {
		return err
	}

	return (&tsa.Lander{
		ATCEndpoint: req.server.atcEndpointPicker.Pick(),
		HTTPClient:  req.server.httpClient,
	}).Land(ctx, worker)
}

type retireWorkerRequest struct {
	server *server
}

func (req retireWorkerRequest) Handle(ctx context.Context, state ConnState, channel ssh.Channel) error {
	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return err
	}

	if err := checkTeam(state, worker); err != nil {
		return err
	}

	return (&tsa.Retirer{
		ATCEndpoint: req.server.atcEndpointPicker.Pick(),
		HTTPClient:  req.server.httpClient,
	}).Retire(ctx, worker)
}

type deleteWorkerRequest struct {
	server *server
}

func (req deleteWorkerRequest) Handle(ctx context.Context, state ConnState, channel ssh.Channel) error {
	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return err
	}

	if err := checkTeam(state, worker); err != nil {
		return err
	}

	return (&tsa.Deleter{
		ATCEndpoint: req.server.atcEndpointPicker.Pick(),
		HTTPClient:  req.server.httpClient,
	}).Delete(ctx, worker)
}

type sweepContainersRequest struct {
	server *server
}

func (req sweepContainersRequest) Handle(ctx context.Context, state ConnState, channel ssh.Channel) error {
	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return err
	}

	if err := checkTeam(state, worker); err != nil {
		return err
	}

	sweeper := &tsa.Sweeper{
		ATCEndpoint: req.server.atcEndpointPicker.Pick(),
		HTTPClient:  req.server.httpClient,
	}

	handles, err := sweeper.Sweep(ctx, worker, tsa.SweepContainers)
	if err != nil {
		return err
	}

	_, err = channel.Write(handles)
	if err != nil {
		return err
	}

	return nil
}

type reportContainersRequest struct {
	server           *server
	containerHandles []string
}

func (req reportContainersRequest) Handle(ctx context.Context, state ConnState, channel ssh.Channel) error {
	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return err
	}

	if err := checkTeam(state, worker); err != nil {
		return err
	}

	return (&tsa.WorkerStatus{
		ATCEndpoint:      req.server.atcEndpointPicker.Pick(),
		HTTPClient:       req.server.httpClient,
		ContainerHandles: req.containerHandles,
	}).WorkerStatus(ctx, worker, tsa.ReportContainers)
}

type sweepVolumesRequest struct {
	server *server
}

func (req sweepVolumesRequest) Handle(ctx context.Context, state ConnState, channel ssh.Channel) error {
	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return err
	}

	if err := checkTeam(state, worker); err != nil {
		return err
	}

	sweeper := &tsa.Sweeper{
		ATCEndpoint: req.server.atcEndpointPicker.Pick(),
		HTTPClient:  req.server.httpClient,
	}

	handles, err := sweeper.Sweep(ctx, worker, tsa.SweepVolumes)
	if err != nil {
		return err
	}

	_, err = channel.Write(handles)
	if err != nil {
		return err
	}

	return nil
}

type reportVolumesRequest struct {
	server        *server
	volumeHandles []string
}

func (req reportVolumesRequest) Handle(ctx context.Context, state ConnState, channel ssh.Channel) error {
	var worker atc.Worker
	err := json.NewDecoder(channel).Decode(&worker)
	if err != nil {
		return err
	}

	if err := checkTeam(state, worker); err != nil {
		return err
	}

	return (&tsa.WorkerStatus{
		ATCEndpoint:   req.server.atcEndpointPicker.Pick(),
		HTTPClient:    req.server.httpClient,
		VolumeHandles: req.volumeHandles,
	}).WorkerStatus(ctx, worker, tsa.ReportVolumes)
}

func gardenURL(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}
