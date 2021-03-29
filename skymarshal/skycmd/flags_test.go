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
		BitbucketCloud: &skycmd.BitbucketCloudFlags{
			ClientID: "bitbucket-id",
		},
		CF: &skycmd.CFFlags{
			ClientID: "cf-id",
		},
		Github: &skycmd.GithubFlags{
			ClientID: "github-id",
		},
		Gitlab: &skycmd.GitlabFlags{
			ClientID: "gitlab-id",
		},
		LDAP: &skycmd.LDAPFlags{
			Host: "ldap-host",
		},
		Microsoft: &skycmd.MicrosoftFlags{
			ClientID: "microsoft-id",
		},
		OAuth: &skycmd.OAuthFlags{
			ClientID: "oauth-id",
		},
		OIDC: &skycmd.OIDCFlags{
			ClientID: "oidc-id",
		},
		SAML: &skycmd.SAMLFlags{
			SsoURL: "sso-url",
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

	teamConnectors := skycmd.TeamConnectorsConfig{
		BitbucketCloud: &skycmd.BitbucketCloudTeamFlags{
			Users: []string{"bitbucket-user"},
		},
		CF: &skycmd.CFTeamFlags{
			Users: []string{"cf-user"},
		},
		Github: &skycmd.GithubTeamFlags{
			Users: []string{"github-user"},
		},
		Gitlab: &skycmd.GitlabTeamFlags{
			Users: []string{"gitlab-user"},
		},
		LDAP: &skycmd.LDAPTeamFlags{
			Users: []string{"ldap-user"},
		},
		Microsoft: &skycmd.MicrosoftTeamFlags{
			Users: []string{"microsoft-user"},
		},
		OAuth: &skycmd.OAuthTeamFlags{
			Users: []string{"oauth-user"},
		},
		OIDC: &skycmd.OIDCTeamFlags{
			Users: []string{"oidc-user"},
		},
		SAML: &skycmd.SAMLTeamFlags{
			Users: []string{"saml-user"},
		},
	}.ConfiguredConnectors()

	var actualTeamConnectors []string
	for _, teamConnector := range teamConnectors {
		actualTeamConnectors = append(actualTeamConnectors, teamConnector.ID())
	}

	s.Assert().ElementsMatch(expectedTeamConnectors, actualTeamConnectors, "list of auth team connectors within skycmd.TeamConnectorsConfig does not match team connectors that are configured in ConfiguredConnectors()")
}
