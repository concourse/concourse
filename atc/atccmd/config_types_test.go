package atccmd_test

import (
	"reflect"
	"testing"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/emitter"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConfigTypesSuite struct {
	suite.Suite
	*require.Assertions
}

func TestConfigTypes(t *testing.T) {
	suite.Run(t, &ConfigTypesSuite{
		Assertions: require.New(t),
	})
}

func (s *ConfigTypesSuite) TestConfiguredCredentialManagers() {
	var expectedCredsManagers []string
	credsManagerConfig := atccmd.CredentialManagersConfig{}
	v := reflect.ValueOf(credsManagerConfig)
	for i := 0; i < v.NumField(); i++ {
		manager := v.Field(i).Interface().(creds.Manager)
		expectedCredsManagers = append(expectedCredsManagers, manager.Name())
	}

	var actualCredsManagers []string
	for name, _ := range credsManagerConfig.All() {
		actualCredsManagers = append(actualCredsManagers, name)
	}

	s.Assert().ElementsMatch(expectedCredsManagers, actualCredsManagers, "list of credential managers within atccmd.CredentialManagersConfig does not match managers that are configured in All()")
}

func (s *ConfigTypesSuite) TestConfiguredMetricsEmitter() {
	var expectedEmitters []string
	v := reflect.ValueOf(atccmd.MetricsEmitterConfig{})
	for i := 0; i < v.NumField(); i++ {
		emitter := v.Field(i).Interface().(metric.EmitterFactory)
		expectedEmitters = append(expectedEmitters, emitter.Description())
	}

	checkMetricEmitter := func(emitters []string, description string, emitterConfig atccmd.MetricsEmitterConfig) []string {
		emitter, err := emitterConfig.ConfiguredEmitter()
		s.NoError(err)
		s.Assert().Equal(description, emitter.Description())

		return append(emitters, emitter.Description())
	}

	var actualMetricsEmitter []string
	actualMetricsEmitter = checkMetricEmitter(actualMetricsEmitter, "Datadog", atccmd.MetricsEmitterConfig{
		Datadog: &emitter.DogstatsDBConfig{
			Host: "host",
			Port: "port",
		},
	})
	actualMetricsEmitter = checkMetricEmitter(actualMetricsEmitter, "InfluxDB", atccmd.MetricsEmitterConfig{
		InfluxDB: &emitter.InfluxDBConfig{
			URL: "some-url",
		},
	})
	actualMetricsEmitter = checkMetricEmitter(actualMetricsEmitter, "Lager", atccmd.MetricsEmitterConfig{
		Lager: &emitter.LagerConfig{
			Enabled: true,
		},
	})
	actualMetricsEmitter = checkMetricEmitter(actualMetricsEmitter, "NewRelic", atccmd.MetricsEmitterConfig{
		NewRelic: &emitter.NewRelicConfig{
			AccountID: "account",
			APIKey:    "key",
		},
	})
	actualMetricsEmitter = checkMetricEmitter(actualMetricsEmitter, "Prometheus", atccmd.MetricsEmitterConfig{
		Prometheus: &emitter.PrometheusConfig{
			BindPort: "port",
			BindIP:   "ip",
		},
	})

	s.Assert().ElementsMatch(expectedEmitters, actualMetricsEmitter, "list of metrics emitters within atccmd.MetricsEmitterConfig does not match emitters that are checked in ConfiguredEmitter()")
}
