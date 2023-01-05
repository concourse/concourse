package emitter

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
	"github.com/pkg/errors"

	influxclient "github.com/influxdata/influxdb-client-go/v2"
)

type InfluxDBEmitter struct {
	Client        influxclient.Client
	Org           string
	Bucket        string
	BatchSize     int
	BatchDuration time.Duration
}

type InfluxDBConfig struct {
	URL         string `long:"influxdb-url" description:"InfluxDB 2.0 server address to emit points to."`
	Org         string `long:"influxdb-org" description:"InfluxDB 2.0 bucket to write points to."`
	Bucket      string `long:"influxdb-bucket" description:"InfluxDB 2.0 bucket to write points to."`
	AccessToken string `long:"influxdb-access-token" description:"InfluxDB 2.0 access token."`

	BatchSize     uint32        `long:"influxdb-batch-size" default:"2000" description:"Number of points to batch together when emitting to InfluxDB."`
	BatchDuration time.Duration `long:"influxdb-batch-duration" default:"30s" description:"The duration to wait before emitting a batch of points to InfluxDB, disregarding influxdb-batch-size."`
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

func (config *InfluxDBConfig) NewEmitter(_ map[string]string) (metric.Emitter, error) {
	opts := influxclient.DefaultOptions()
	opts.SetApplicationName("concourse")
	opts.SetBatchSize(uint(config.BatchSize))
	opts.SetFlushInterval(uint(config.BatchDuration))
	opts.SetMaxRetryTime(uint(time.Minute))
	client := influxclient.NewClientWithOptions(config.URL, config.AccessToken, opts)

	return &InfluxDBEmitter{
		Client:        client,
		Org:           config.Org,
		Bucket:        config.Bucket,
		BatchSize:     int(config.BatchSize),
		BatchDuration: config.BatchDuration,
	}, nil
}

func emitBatch(emitter *InfluxDBEmitter, logger lager.Logger, events []metric.Event) {

	logger.Debug("influxdb-emit-batch", lager.Data{
		"size": len(events),
	})

	writeAPI := emitter.Client.WriteAPIBlocking(emitter.Org, emitter.Bucket)

	writeAPI.EnableBatching()

	for _, event := range events {
		tags := map[string]string{
			"host": event.Host,
		}

		for k, v := range event.Attributes {
			tags[k] = v
		}

		point := influxclient.NewPoint(
			event.Name,
			tags,
			map[string]interface{}{
				"value": event.Value,
			},
			event.Time,
		)

		err := writeAPI.WritePoint(context.Background(), point)
		if err != nil {
			logger.Error("failed-to-add-point-to-batch",
				errors.Wrap(metric.ErrFailedToEmit, err.Error()))

			continue
		}
	}

	err := writeAPI.Flush(context.Background())

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
