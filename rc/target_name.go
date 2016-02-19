package rc

import (
	"fmt"
	"regexp"
)

type TargetName string

var validTargetName = regexp.MustCompile(`[[:alnum:]\-_]+`)

func (name *TargetName) UnmarshalFlag(value string) error {
	if !validTargetName.MatchString(value) {
		return fmt.Errorf("dfskjhdkgjhfdkjgh")
	}

	*name = TargetName(value)

	return nil
}
