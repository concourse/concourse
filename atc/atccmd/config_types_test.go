package atccmd_test

import (
	"reflect"
	"testing"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/conjur"
	"github.com/concourse/concourse/atc/creds/credhub"
	"github.com/concourse/concourse/atc/creds/dummy"
	"github.com/concourse/concourse/atc/creds/kubernetes"
	"github.com/concourse/concourse/atc/creds/secretsmanager"
	"github.com/concourse/concourse/atc/creds/ssm"
	"github.com/concourse/concourse/atc/creds/vault"
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
	v := reflect.ValueOf(atccmd.CredentialManagersConfig{})
	for i := 0; i < v.NumField(); i++ {
		manager := v.Field(i).Interface().(creds.Manager)
		expectedCredsManagers = append(expectedCredsManagers, manager.Name())
	}

	checkCredentialManager := func(managers []string, managerName string, managersConfig atccmd.CredentialManagersConfig) []string {
		manager, err := managersConfig.ConfiguredCredentialManager()
		s.NoError(err)
		s.Assert().Equal(managerName, manager.Name())

		return append(managers, manager.Name())
	}

	var actualCredsManagers []string
	actualCredsManagers = checkCredentialManager(actualCredsManagers, "conjur", atccmd.CredentialManagersConfig{
		Conjur: &conjur.Manager{
			ConjurApplianceUrl: "some-url",
		},
	})
	actualCredsManagers = checkCredentialManager(actualCredsManagers, "credhub", atccmd.CredentialManagersConfig{
		CredHub: &credhub.CredHubManager{
			URL: "some-url",
		},
	})
	actualCredsManagers = checkCredentialManager(actualCredsManagers, "dummy", atccmd.CredentialManagersConfig{
		Dummy: &dummy.Manager{
			Vars: dummy.VarFlags{{Name: "some-var", Value: "some-value"}},
		},
	})
	actualCredsManagers = checkCredentialManager(actualCredsManagers, "kubernetes", atccmd.CredentialManagersConfig{
		Kubernetes: &kubernetes.KubernetesManager{
			InClusterConfig: true,
		},
	})
	actualCredsManagers = checkCredentialManager(actualCredsManagers, "secretsmanager", atccmd.CredentialManagersConfig{
		SecretsManager: &secretsmanager.Manager{
			AwsRegion: "some-region",
		},
	})
	actualCredsManagers = checkCredentialManager(actualCredsManagers, "ssm", atccmd.CredentialManagersConfig{
		SSM: &ssm.SsmManager{
			AwsRegion: "some-region",
		},
	})
	actualCredsManagers = checkCredentialManager(actualCredsManagers, "vault", atccmd.CredentialManagersConfig{
		Vault: &vault.VaultManager{
			URL: "some-url",
		},
	})

	s.Assert().ElementsMatch(expectedCredsManagers, actualCredsManagers, "list of credential managers within atccmd.CredentialManagersConfig does not match managers that are configured in ConfiguredCredentialManager()")
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
