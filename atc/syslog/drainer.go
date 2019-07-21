package syslog

import (
	"context"
	"encoding/json"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
)

//go:generate counterfeiter . Drainer

type Drainer interface {
	Run(context.Context) error
}

type drainer struct {
	hostname     string
	transport    string
	address      string
	caCerts      []string
	buildFactory db.BuildFactory
}

func NewDrainer(transport string, address string, hostname string, caCerts []string, buildFactory db.BuildFactory) Drainer {
	return &drainer{
		hostname:     hostname,
		transport:    transport,
		address:      address,
		buildFactory: buildFactory,
		caCerts:      caCerts,
	}
}

func (d *drainer) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("syslog")

	builds, err := d.buildFactory.GetDrainableBuilds()
	if err != nil {
		logger.Error("failed-to-get-drainable-builds", err)
		return err
	}

	if len(builds) > 0 {
		syslog, err := Dial(d.transport, d.address, d.caCerts)
		if err != nil {
			logger.Error("failed-to-connect", err)
			return err
		}

		// ignore any errors coming from syslog.Close()
		defer db.Close(syslog)

		for _, build := range builds {
			err := d.drainBuild(logger, build, syslog)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *drainer) drainBuild(logger lager.Logger, build db.Build, syslog *Syslog) error {
	logger = logger.Session("drain-build", lager.Data{
		"team":     build.TeamName(),
		"pipeline": build.PipelineName(),
		"job":      build.JobName(),
		"build":    build.Name(),
	})

	events, err := build.Events(0)
	if err != nil {
		return err
	}

	// ignore any errors coming from events.Close()
	defer db.Close(events)

	for {
		ev, err := events.Next()
		if err != nil {
			if err == db.ErrEndOfBuildEventStream {
				break
			}
			logger.Error("failed-to-get-next-event", err)
			return err
		}

		if ev.Event == event.EventTypeLog {
			var log event.Log

			err := json.Unmarshal(*ev.Data, &log)
			if err != nil {
				logger.Error("failed-to-unmarshal", err)
				return err
			}

			payload := log.Payload
			tag := build.TeamName() + "/" + build.PipelineName() + "/" + build.JobName() + "/" + build.Name() + "/" + string(log.Origin.ID)

			err = syslog.Write(d.hostname, tag, time.Unix(log.Time, 0), payload)
			if err != nil {
				logger.Error("failed-to-write-to-server", err)
				return err
			}
		}
	}

	err = build.SetDrained(true)
	if err != nil {
		logger.Error("failed-to-update-status", err)
		return err
	}

	return nil
}
