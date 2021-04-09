package skycmd_test

import (
	"reflect"
	"testing"

	"github.com/concourse/concourse/skymarshal/skycmd"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConnectorsSuite struct {
	suite.Suite
	*require.Assertions
}

func TestConnectors(t *testing.T) {
	suite.Run(t, &ConnectorsSuite{
		Assertions: require.New(t),
	})
}

func (s *ConnectorsSuite) TestConfiguredConnectors() {
	var expectedConnectors []string
	v := reflect.ValueOf(skycmd.ConnectorsConfig{})
	for i := 0; i < v.NumField(); i++ {
		connector := v.Field(i).Interface().(skycmd.Config)
		expectedConnectors = append(expectedConnectors, connector.ID())
	}

	connectors := skycmd.ConnectorsConfig{
		BitbucketCloud: skycmd.BitbucketCloudFlags{
			Enabled: true,
		},
		CF: skycmd.CFFlags{
			Enabled: true,
		},
		Github: skycmd.GithubFlags{
			Enabled: true,
		},
		Gitlab: skycmd.GitlabFlags{
			Enabled: true,
		},
		LDAP: skycmd.LDAPFlags{
			Enabled: true,
		},
		Microsoft: skycmd.MicrosoftFlags{
			Enabled: true,
		},
		OAuth: skycmd.OAuthFlags{
			Enabled: true,
		},
		OIDC: skycmd.OIDCFlags{
			Enabled: true,
		},
		SAML: skycmd.SAMLFlags{
			Enabled: true,
		},
	}.ConfiguredConnectors()

	var actualConnectors []string
	for _, connector := range connectors {
		actualConnectors = append(actualConnectors, connector.ID())
	}

	s.Assert().ElementsMatch(expectedConnectors, actualConnectors, "list of auth connectors within skycmd.ConnectorsConfig does not match connectors that are configured in ConfiguredConnectors()")
}

func (s *ConnectorsSuite) TestConfiguredTeamConnectors() {
	var expectedTeamConnectors []string
	v := reflect.ValueOf(skycmd.TeamConnectorsConfig{})
	for i := 0; i < v.NumField(); i++ {
		teamConnector := v.Field(i).Interface().(skycmd.Config)
		expectedTeamConnectors = append(expectedTeamConnectors, teamConnector.ID())
	}

	var actualTeamConnectors []string
	teamConnectorsConfig := skycmd.TeamConnectorsConfig{}
	for _, teamConnector := range teamConnectorsConfig.AllConnectors() {
		actualTeamConnectors = append(actualTeamConnectors, teamConnector.ID())
	}

	s.Assert().ElementsMatch(expectedTeamConnectors, actualTeamConnectors, "list of auth team connectors within skycmd.TeamConnectorsConfig does not match team connectors that are configured in ConfiguredConnectors()")
}
