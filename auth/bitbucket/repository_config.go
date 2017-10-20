package bitbucket

import (
	"fmt"
	"strings"
)

type RepositoryConfig struct {
	OwnerName      string `json:"owner_name,omitempty"`
	RepositoryName string `json:"repository_name,omitempty"`
}

func (flag *RepositoryConfig) UnmarshalFlag(value string) error {
	s := strings.SplitN(value, "/", 2)
	if len(s) != 2 {
		return fmt.Errorf("malformed Bitbucket repository specification: '%s'", value)
	}

	flag.OwnerName = s[0]
	flag.RepositoryName = s[1]

	return nil
}
