package emitter

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
	"github.com/pkg/errors"

	influxclient "github.com/influxdata/influxdb1-client/v2"
)

type InfluxDBEmitter struct {
	Client        influxclient.Client
	Database      string
	BatchSize     int
	BatchDuration time.Duration
}

type InfluxDBConfig struct {
	URL string `long:"influxdb-url" description:"InfluxDB server address to emit points to."`

	Database string `long:"influxdb-database" description:"InfluxDB database to write points to."`

	Username string `long:"influxdb-username" description:"InfluxDB server username."`
	Password string `long:"influxdb-password" description:"InfluxDB server password."`

	InsecureSkipVerify bool `long:"influxdb-insecure-skip-verify" description:"Skip SSL verification when emitting to InfluxDB."`

	BatchSize     uint32        `long:"influxdb-batch-size" default:"5000" description:"Number of points to batch together when emitting to InfluxDB."`
	BatchDuration time.Duration `long:"influxdb-batch-duration" default:"300s" description:"The duration to wait before emitting a batch of points to InfluxDB, disregarding influxdb-batch-size."`
}

var (
	batch         []metric.Event
	lastBatchTime time.Time
)

func init() {
	batch = make([]metric.Event, 0)
	lastBatchTime = time.Now()
	metric.Metrics.RegisterEmitter(&InfluxDBConfig{})
}

func (config *InfluxDBConfig) Description() string { return "InfluxDB" }
func (config *InfluxDBConfig) IsConfigured() bool  { return config.URL != "" }

func (config *InfluxDBConfig) NewEmitter() (metric.Emitter, error) {
	client, err := influxclient.NewHTTPClient(influxclient.HTTPConfig{
		Addr:               config.URL,
		Username:           config.Username,
		Password:           config.Password,
		InsecureSkipVerify: config.InsecureSkipVerify,
		Timeout:            time.Minute,
	})
	if err != nil {
		return &InfluxDBEmitter{}, err
	}

	return &InfluxDBEmitter{
		Client:        client,
		Database:      config.Database,
		BatchSize:     int(config.BatchSize),
		BatchDuration: config.BatchDuration,
	}, nil
}

func emitBatch(emitter *InfluxDBEmitter, logger lager.Logger, events []metric.Event) {

	logger.Debug("influxdb-emit-batch", lager.Data{
		"size": len(events),
	})
	bp, err := influxclient.NewBatchPoints(influxclient.BatchPointsConfig{
		Database: emitter.Database,
	})
	if err != nil {
		logger.Error("failed-to-construct-batch-points", err)
		return
	}

	for _, event := range events {
		tags := map[string]string{
			"host": event.Host,
		}

		for k, v := range event.Attributes {
			tags[k] = v
		}

		point, err := influxclient.NewPoint(
			event.Name,
			tags,
			map[string]interface{}{
				"value": event.Value,
			},
			event.Time,
		)
		if err != nil {
			logger.Error("failed-to-construct-point", err)
			continue
		}

		bp.AddPoint(point)
	}

	err = emitter.Client.Write(bp)
	if err != nil {
		logger.Error("failed-to-send-points",
			errors.Wrap(metric.ErrFailedToEmit, err.Error()))
		return
	}
}

func (emitter *InfluxDBEmitter) Emit(logger lager.Logger, event metric.Event) {
	batch = append(batch, event)
	duration := time.Since(lastBatchTime)
	if len(batch) >= emitter.BatchSize || duration >= emitter.BatchDuration {
		logger.Debug("influxdb-pre-emit-batch", lager.Data{
			"influxdb-batch-size":     emitter.BatchSize,
			"current-batch-size":      len(batch),
			"influxdb-batch-duration": emitter.BatchDuration,
			"current-duration":        duration,
		})
		emitter.SubmitBatch(logger)
	}
}

func (emitter *InfluxDBEmitter) SubmitBatch(logger lager.Logger) {
	batchToSubmit := make([]metric.Event, len(batch))
	copy(batchToSubmit, batch)
	batch = make([]metric.Event, 0)
	lastBatchTime = time.Now()
	go emitBatch(emitter, logger, batchToSubmit)
}
