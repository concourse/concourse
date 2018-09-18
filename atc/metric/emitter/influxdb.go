package emitter

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/metric"

	influxclient "github.com/influxdata/influxdb/client/v2"
)

type InfluxDBEmitter struct {
	client   influxclient.Client
	database string
}

type InfluxDBConfig struct {
	URL string `long:"influxdb-url" description:"InfluxDB server address to emit points to."`

	Database string `long:"influxdb-database" description:"InfluxDB database to write points to."`

	Username string `long:"influxdb-username" description:"InfluxDB server username."`
	Password string `long:"influxdb-password" description:"InfluxDB server password."`

	InsecureSkipVerify bool `long:"influxdb-insecure-skip-verify" description:"Skip SSL verification when emitting to InfluxDB."`
}

func init() {
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
	}, nil
}

func (emitter *InfluxDBEmitter) Emit(logger lager.Logger, event metric.Event) {
	bp, err := influxclient.NewBatchPoints(influxclient.BatchPointsConfig{
		Database: emitter.database,
	})
	if err != nil {
		logger.Error("failed-to-construct-batch-points", err)
		return
	}

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
		return
	}

	bp.AddPoint(point)

	err = emitter.client.Write(bp)
	if err != nil {
		logger.Error("failed-to-send-points", err)
		return
	}
}
