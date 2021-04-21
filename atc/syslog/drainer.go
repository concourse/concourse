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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Drainer
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
	case event.EventTypeInitialize:
		var initEvent event.Initialize
		err := json.Unmarshal(*ev.Data, &initEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(initEvent.Time, 0)
		tag = build.SyslogTag(initEvent.Origin.ID)
		message = fmt.Sprintf("initializing")
	case event.EventTypeInitializeGet:
		var initGetEvent event.InitializeGet
		err := json.Unmarshal(*ev.Data, &initGetEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(initGetEvent.Time, 0)
		tag = build.SyslogTag(initGetEvent.Origin.ID)
		message = fmt.Sprintf("get initializing")
	case event.EventTypeInitializePut:
		var initPutEvent event.InitializePut
		err := json.Unmarshal(*ev.Data, &initPutEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(initPutEvent.Time, 0)
		tag = build.SyslogTag(initPutEvent.Origin.ID)
		message = fmt.Sprintf("put initializing")
	case event.EventTypeInitializeTask:
		var initTaskEvent event.InitializeTask
		err := json.Unmarshal(*ev.Data, &initTaskEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(initTaskEvent.Time, 0)
		tag = build.SyslogTag(initTaskEvent.Origin.ID)
		message = fmt.Sprintf("task initializing")
	case event.EventTypeSelectedWorker:
		var selectedWorkerEvent event.SelectedWorker
		err := json.Unmarshal(*ev.Data, &selectedWorkerEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(selectedWorkerEvent.Time, 0)
		tag = build.SyslogTag(selectedWorkerEvent.Origin.ID)
		message = fmt.Sprintf("selected worker: %s", selectedWorkerEvent.WorkerName)
	case event.EventTypeStartTask:
		var startTaskEvent event.StartTask
		err := json.Unmarshal(*ev.Data, &startTaskEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(startTaskEvent.Time, 0)
		tag = build.SyslogTag(startTaskEvent.Origin.ID)

		buildConfig := startTaskEvent.TaskConfig
		argv := strings.Join(append([]string{buildConfig.Run.Path}, buildConfig.Run.Args...), " ")
		message = fmt.Sprintf("running %s", argv)
	case event.EventTypeLog:
		var logEvent event.Log
		err := json.Unmarshal(*ev.Data, &logEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(logEvent.Time, 0)
		tag = build.SyslogTag(logEvent.Origin.ID)
		message = logEvent.Payload
	case event.EventTypeFinishGet:
		var finishGetEvent event.FinishGet
		err := json.Unmarshal(*ev.Data, &finishGetEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(finishGetEvent.Time, 0)
		tag = build.SyslogTag(finishGetEvent.Origin.ID)

		version, _ := json.Marshal(finishGetEvent.FetchedVersion)
		metadata, _ := json.Marshal(finishGetEvent.FetchedMetadata)
		message = fmt.Sprintf("get {\"version\": %s, \"metadata\": %s", string(version), string(metadata))
	case event.EventTypeFinishPut:
		var finishPutEvent event.FinishPut
		err := json.Unmarshal(*ev.Data, &finishPutEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(finishPutEvent.Time, 0)
		tag = build.SyslogTag(finishPutEvent.Origin.ID)

		version, _ := json.Marshal(finishPutEvent.CreatedVersion)
		metadata, _ := json.Marshal(finishPutEvent.CreatedMetadata)
		message = fmt.Sprintf("put {\"version\": %s, \"metadata\": %s", string(version), string(metadata))
	case event.EventTypeError:
		var errorEvent event.Error
		err := json.Unmarshal(*ev.Data, &errorEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(errorEvent.Time, 0)
		tag = build.SyslogTag(errorEvent.Origin.ID)
		message = errorEvent.Message
	case event.EventTypeStatus:
		var statusEvent event.Status
		err := json.Unmarshal(*ev.Data, &statusEvent)
		if err != nil {
			logger.Error("failed-to-unmarshal", err)
			return err
		}
		ts = time.Unix(statusEvent.Time, 0)
		tag = build.SyslogTag(event.OriginID(""))
		message = statusEvent.Status.String()
	}

	if message != "" {
		err := syslog.Write(hostname, tag, ts, message, ev.EventID)
		if err != nil {
			logger.Error("failed-to-write-to-server", err)
			return err
		}
	}

	return nil
}
