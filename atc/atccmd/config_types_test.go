package atccmd_test

import (
	"reflect"
	"testing"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/metric"
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
	for name := range credsManagerConfig.All() {
		actualCredsManagers = append(actualCredsManagers, name)
	}

	s.Assert().ElementsMatch(expectedCredsManagers, actualCredsManagers, "list of credential managers within atccmd.CredentialManagersConfig does not match managers that are configured in All()")
}

func (s *ConfigTypesSuite) TestConfiguredMetricsEmitter() {
	var expectedEmitters []string
	metricsEmitterConfig := atccmd.MetricsEmitterConfig{}
	v := reflect.ValueOf(metricsEmitterConfig)
	for i := 0; i < v.NumField(); i++ {
		emitter := v.Field(i).Interface().(metric.EmitterFactory)
		expectedEmitters = append(expectedEmitters, emitter.Description())
	}

	var actualMetricsEmitter []string
	for name := range metricsEmitterConfig.All() {
		actualMetricsEmitter = append(actualMetricsEmitter, name)
	}

	s.Assert().ElementsMatch(expectedEmitters, actualMetricsEmitter, "list of metrics emitters within atccmd.MetricsEmitterConfig does not match emitters that are checked in All()")
}
