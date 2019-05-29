package emitter

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
	"github.com/pkg/errors"

	influxclient "github.com/influxdata/influxdb1-client/v2"
)

type InfluxDBEmitter struct {
	client   influxclient.Client
	database string
	batchSize int
	batchDuration time.Duration
}

type InfluxDBConfig struct {
	URL string `long:"influxdb-url" description:"InfluxDB server address to emit points to."`

	Database string `long:"influxdb-database" description:"InfluxDB database to write points to."`

	Username string `long:"influxdb-username" description:"InfluxDB server username."`
	Password string `long:"influxdb-password" description:"InfluxDB server password."`

	InsecureSkipVerify bool `long:"influxdb-insecure-skip-verify" description:"Skip SSL verification when emitting to InfluxDB."`

	// https://github.com/influxdata/docs.influxdata.com/issues/454
	// https://docs.influxdata.com/influxdb/v0.13/write_protocols/write_syntax/#write-a-batch-of-points-with-curl
	// 5000 seems to be the batch size recommended by the InfluxDB team
	BatchSize uint32 `long:"influxdb-batch-size" default:"5000" description:"Number of points to batch together when emitting to InfluxDB."`
	BatchDuration time.Duration `long:"influxdb-batch-duration" default:"300s" description:"The duration to wait before emitting a batch of points to InfluxDB, disregarding influxdb-batch-size."`
}

var (
	batch []metric.Event
	lastBatchTime time.Time
)

func init() {
	batch = make([]metric.Event, 0)
	lastBatchTime = time.Now()
	metric.RegisterEmitter(&InfluxDBConfig{})
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
		client:   client,
		database: config.Database,
		batchSize: int(config.BatchSize),
		batchDuration: config.BatchDuration,
	}, nil
}

func emitBatch(emitter *InfluxDBEmitter, logger lager.Logger, events []metric.Event) {

	logger.Debug("influxdb-emit-batch", lager.Data{
		"size": len(events),
	})
	bp, err := influxclient.NewBatchPoints(influxclient.BatchPointsConfig{
		Database: emitter.database,
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
				"state": string(event.State),
			},
			event.Time,
		)
		if err != nil {
			logger.Error("failed-to-construct-point", err)
			continue
		}

		bp.AddPoint(point)
	}

	err = emitter.client.Write(bp)
	if err != nil {
		logger.Error("failed-to-send-points",
			errors.Wrap(metric.ErrFailedToEmit, err.Error()))
		return
	}
	logger.Info("influxdb-emitter-fork-influxdb-batch-emitted", lager.Data{
		"size": len(events),
	})
}


func (emitter *InfluxDBEmitter) Emit(logger lager.Logger, event metric.Event) {
	batch = append(batch, event)
	duration := time.Since(lastBatchTime)
	if len(batch) > emitter.batchSize || duration > emitter.batchDuration {
		logger.Debug("influxdb-pre-emit-batch", lager.Data{
			"influxdb-batch-size": emitter.batchSize,
			"current-batch-size": len(batch),
			"influxdb-batch-duration": emitter.batchDuration,
			"current-duration": duration,
		})
		go emitBatch(emitter, logger, batch)
		batch = make([]metric.Event, 0)
		lastBatchTime = time.Now()
	}
}
