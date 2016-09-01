package atc

import (
	"fmt"
	"strings"
)

type GitHubTeamFlag struct {
	OrganizationName string
	TeamName         string
}

func (flag *GitHubTeamFlag) UnmarshalFlag(value string) error {
	s := strings.SplitN(value, "/", 2)
	if len(s) != 2 {
		return fmt.Errorf("malformed GitHub team specification: '%s'", value)
	}

	flag.OrganizationName = s[0]
	flag.TeamName = s[1]

	return nil
}
