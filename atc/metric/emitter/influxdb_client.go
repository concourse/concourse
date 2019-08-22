package emitter

import (
	"time"

	influxclient "github.com/influxdata/influxdb1-client/v2"
)

// This is a copy of the github.com/influxdata/influxdb1-client/v2/client.Client interface whose sole purpose is
// to allow counterfeiter to generate a fake implementation.
// counterfeiter is not able to resolve github.com/influxdata/influxdb1-client/v2/client.Client, possibly due to
// the v2 in the package name.

//go:generate counterfeiter . InfluxDBClient
// Client is a client interface for writing & querying the database.
type InfluxDBClient interface {
	// Ping checks that status of cluster, and will always return 0 time and no
	// error for UDP clients.
	Ping(timeout time.Duration) (time.Duration, string, error)

	// Write takes a BatchPoints object and writes all Points to InfluxDB.
	Write(bp influxclient.BatchPoints) error

	// Query makes an InfluxDB Query on the database. This will fail if using
	// the UDP client.
	Query(q influxclient.Query) (*influxclient.Response, error)

	// QueryAsChunk makes an InfluxDB Query on the database. This will fail if using
	// the UDP client.
	QueryAsChunk(q influxclient.Query) (*influxclient.ChunkedResponse, error)

	// Close releases any resources a Client may be using.
	Close() error
}
