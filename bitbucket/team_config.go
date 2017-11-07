package bitbucket

import (
	"fmt"
	"strings"
)

type TeamConfig struct {
	TeamName string `json:"team_name,omitempty"`
	Role     string `json:"role,omitempty"`
}

func (flag *TeamConfig) UnmarshalFlag(value string) error {
	s := strings.SplitN(value, ":", 2)

	flag.TeamName = s[0]
	flag.Role = "member"

	if len(s) == 2 {
		if s[1] != "member" && s[1] != "contributor" && s[1] != "admin" {
			return fmt.Errorf("unknown role in Bitbucket team specification: '%s'", s[1])
		}

		flag.Role = s[1]
	}

	return nil
}
