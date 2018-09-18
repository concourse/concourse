package syslog

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	sl "github.com/papertrail/remote_syslog2/syslog"
)

const ServerPollingInterval = 5 * time.Second

//go:generate counterfeiter . Drainer

type Drainer interface {
	Run(context.Context) error
}

type drainer struct {
	hostname     string
	transport    string `yaml:"transport"`
	address      string `yaml:"address"`
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
		logger.Error("Syslog drainer getting drainable builds error.", err)
		return err
	}

	if len(builds) > 0 {
		var certpool *x509.CertPool
		if d.transport == "tls" {

			certpool, err = x509.SystemCertPool()
			if err != nil {
				return err
			}

			for _, cert := range d.caCerts {
				content, err := ioutil.ReadFile(cert)
				if err != nil {
					return err
				}

				ok := certpool.AppendCertsFromPEM(content)
				if !ok {
					return errors.New("syslog drainer certificate error")
				}
			}
		}

		syslog, err := sl.Dial(
			d.hostname,
			d.transport,
			d.address,
			certpool,
			30*time.Second,
			30*time.Second,
			99990,
		)
		if err != nil {
			logger.Error("Syslog drainer connecting to server error.", err)
			return err
		}

		for _, build := range builds {
			events, err := build.Events(0)
			if err != nil {
				logger.Error("Syslog drainer getting build events error.", err)
				return err
			}

			for {
				ev, err := events.Next()
				if err != nil {
					if err == db.ErrEndOfBuildEventStream {
						break
					}
					logger.Error("Syslog drainer getting next event error.", err)
					return err
				}

				if ev.Event == "log" {
					var log event.Log

					err := json.Unmarshal(*ev.Data, &log)
					if err != nil {
						logger.Error("Syslog drainer unmarshalling log error.", err)
						return err
					}

					payload := log.Payload
					tag := build.TeamName() + "/" + build.PipelineName() + "/" + build.JobName() + "/" + build.Name() + "/" + string(log.Origin.ID)

					syslog.Packets <- sl.Packet{
						Severity: sl.SevInfo,
						Facility: sl.LogUser,
						Hostname: d.hostname,
						Tag:      tag,
						Time:     time.Unix(log.Time, 0),
						Message:  payload,
					}

					select {
					case err := <-syslog.Errors:
						logger.Error("Syslog drainer sending to server error.", err)
						return err
					default:
						continue
					}
				}
			}

			err = build.SetDrained(true)
			if err != nil {
				logger.Error("Syslog drainer setting drained on build error.", err)
				return err
			}
		}
	}
	return nil
}
