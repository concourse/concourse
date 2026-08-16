package skycmd

import (
	"testing"

	flags "github.com/jessevdk/go-flags"
	"github.com/vito/twentythousandtonnesofcrudeoil"
)

func TestSAMLTeamGroupEnvironmentValueMayContainCommas(t *testing.T) {
	var cmd struct {
		SAML SAMLTeamFlags `group:"SAML" namespace:"main-team-saml"`
	}

	parser := flags.NewParser(&cmd, flags.None)
	parser.NamespaceDelimiter = "-"
	twentythousandtonnesofcrudeoil.TheEnvironmentIsPerfectlySafe(parser, "CONCOURSE_")
	t.Setenv("CONCOURSE_MAIN_TEAM_SAML_GROUP", "CN=my_concourse_admin,OU=SecurityGroups,DC=example,DC=com")

	_, err := parser.ParseArgs(nil)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"CN=my_concourse_admin,OU=SecurityGroups,DC=example,DC=com"}
	if len(cmd.SAML.Groups) != len(want) || cmd.SAML.Groups[0] != want[0] {
		t.Fatalf("expected %q, got %q", want, cmd.SAML.Groups)
	}
}
