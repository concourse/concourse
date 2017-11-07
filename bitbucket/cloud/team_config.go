package cloud

import (
	"fmt"
	"strings"
)

type TeamConfig struct {
	Name string `json:"team_name,omitempty"`
	Role Role   `json:"role,omitempty"`
}

func (flag *TeamConfig) UnmarshalFlag(value string) error {
	s := strings.SplitN(value, ":", 2)

	flag.Name = s[0]
	flag.Role = RoleMember

	if len(s) == 2 {
		if s[1] != "member" && s[1] != "contributor" && s[1] != "admin" {
			return fmt.Errorf("unknown role in Bitbucket team specification: '%s'", s[1])
		}

		flag.Role = Role(s[1])
	}

	return nil
}
