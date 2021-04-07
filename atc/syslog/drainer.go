package syslog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	logger = logger.Session("drain-build", build.LagerData())

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

		err = d.sendEvent(logger, build, syslog, ev)
		if err != nil {
			logger.Error("failed-to-send-event", err)
			return err
		}
	}

	err = build.SetDrained(true)
	if err != nil {
		logger.Error("failed-to-update-status", err)
		return err
	}

	return nil
}

func (d *drainer) sendEvent(logger lager.Logger, build db.Build, syslog *Syslog, ev event.Envelope) error {
	var (
		hostname string = d.hostname
		ts       time.Time
		tag      string
		message  string
	)

	switch ev.Event {
	case event.EventTypeStartTask:
		var startTask event.StartTask
		err := json.Unmarshal(*ev.Data, &startTask)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(startTask.Time, 0)
		tag = build.SyslogTag(startTask.Origin.ID)

		buildConfig := startTask.TaskConfig
		argv := strings.Join(append([]string{buildConfig.Run.Path}, buildConfig.Run.Args...), " ")
		message = fmt.Sprintf("running %s", argv)
	case event.EventTypeLog:
		var log event.Log
		err := json.Unmarshal(*ev.Data, &log)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(log.Time, 0)
		tag = build.SyslogTag(log.Origin.ID)
		message = log.Payload
	case event.EventTypeFinishGet:
		var finishGet event.FinishGet
		err := json.Unmarshal(*ev.Data, &finishGet)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(finishGet.Time, 0)
		tag = build.SyslogTag(finishGet.Origin.ID)

		version, _ := json.Marshal(finishGet.FetchedVersion)
		metadata, _ := json.Marshal(finishGet.FetchedMetadata)
		message = fmt.Sprintf("{\"version\": %s, \"metadata\": %s", string(version), string(metadata))
	case event.EventTypeFinishPut:
		var finishPut event.FinishPut
		err := json.Unmarshal(*ev.Data, &finishPut)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(finishPut.Time, 0)
		tag = build.SyslogTag(finishPut.Origin.ID)

		version, _ := json.Marshal(finishPut.CreatedVersion)
		metadata, _ := json.Marshal(finishPut.CreatedMetadata)
		message = fmt.Sprintf("{\"version\": %s, \"metadata\": %s", string(version), string(metadata))
	}

	if message != "" && tag != "" {
		err := syslog.Write(hostname, tag, ts, message, ev.EventID)
		if err != nil {
			logger.Error("failed-to-write-to-server", err)
			return err
		}
	}

	return nil
}
